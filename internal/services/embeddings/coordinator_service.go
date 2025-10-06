package embeddings

import (
	"context"
	"fmt"
	"sync"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/workers"
)

// CoordinatorService coordinates embedding generation
type CoordinatorService struct {
	embeddingService interfaces.EmbeddingService
	documentStorage  interfaces.DocumentStorage
	eventService     interfaces.EventService
	logger           arbor.ILogger
	isProcessing     bool
	mu               sync.Mutex
}

// NewCoordinatorService creates a new embedding coordinator
func NewCoordinatorService(
	embeddingService interfaces.EmbeddingService,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *CoordinatorService {
	return &CoordinatorService{
		embeddingService: embeddingService,
		documentStorage:  documentStorage,
		eventService:     eventService,
		logger:           logger,
	}
}

// Start subscribes to embedding events
func (s *CoordinatorService) Start() error {
	handler := func(ctx context.Context, event interfaces.Event) error {
		return s.handleEmbeddingEvent(ctx, event)
	}

	return s.eventService.Subscribe(interfaces.EventEmbeddingTriggered, handler)
}

// handleEmbeddingEvent processes embedding triggered events
func (s *CoordinatorService) handleEmbeddingEvent(ctx context.Context, event interfaces.Event) error {
	// Panic recovery to prevent service crash
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Msg("PANIC RECOVERED in embedding event handler")
		}
	}()

	// Check if already processing (prevent concurrent runs)
	s.mu.Lock()
	if s.isProcessing {
		s.mu.Unlock()
		s.logger.Warn().Msg("@@@ EMBEDDING ALREADY IN PROGRESS - Skipping concurrent run")
		return nil
	}
	s.isProcessing = true
	s.mu.Unlock()

	// Ensure we reset processing flag when done
	defer func() {
		s.mu.Lock()
		s.isProcessing = false
		s.mu.Unlock()
	}()

	s.logger.Debug().Msg("@@@ EMBEDDING COORDINATOR START @@@")
	s.logger.Info().Msg("@@@ Step 1: Embedding event triggered")

	// Validate dependencies
	s.logger.Debug().Msg("@@@ Step 2: Validating embedding service")
	if s.embeddingService == nil {
		s.logger.Error().Msg("@@@ FAILED: embeddingService is nil")
		return fmt.Errorf("embedding service is nil - cannot process embedding event")
	}
	s.logger.Debug().Msg("@@@ Step 3: Embedding service OK")

	s.logger.Debug().Msg("@@@ Step 4: Validating document storage")
	if s.documentStorage == nil {
		s.logger.Error().Msg("@@@ FAILED: documentStorage is nil")
		return fmt.Errorf("document storage is nil - cannot process embedding event")
	}
	s.logger.Debug().Msg("@@@ Step 5: Document storage OK")

	s.logger.Debug().Msg("@@@ Step 6: Creating worker pool for embeddings")
	pool := workers.NewPool(1, s.logger) // Single worker to eliminate SQLite concurrency issues
	s.logger.Debug().Msg("@@@ Step 7: Starting worker pool")
	pool.Start()
	defer func() {
		s.logger.Debug().Msg("@@@ Shutting down worker pool")
		pool.Shutdown()
		s.logger.Debug().Msg("@@@ Worker pool shutdown complete")
	}()
	s.logger.Debug().Msg("@@@ Step 8: Worker pool started")

	s.logger.Debug().Msg("@@@ Step 9: Fetching force embed documents from storage (limit 100)")
	forceEmbedDocs, err := s.documentStorage.GetDocumentsForceEmbed(100)
	if err != nil {
		s.logger.Error().Err(err).Msg("@@@ FAILED: Error querying force embed documents")
		return fmt.Errorf("failed to get force embed documents: %w", err)
	}
	s.logger.Debug().
		Int("count", len(forceEmbedDocs)).
		Msg("@@@ Step 10: Found force embed documents")

	if len(forceEmbedDocs) > 0 {
		s.logger.Info().
			Int("count", len(forceEmbedDocs)).
			Msg("@@@ Step 11: Processing force embed documents")

		for i, doc := range forceEmbedDocs {
			doc := doc
			s.logger.Debug().
				Int("job_index", i).
				Str("doc_id", doc.ID).
				Msg("@@@ Step 11.a: Submitting force embed job")

			job := func(ctx context.Context) error {
				return s.embedDocument(ctx, doc, true)
			}
			if err := pool.Submit(job); err != nil {
				s.logger.Error().
					Err(err).
					Str("doc_id", doc.ID).
					Int("job_index", i).
					Msg("@@@ FAILED: Error submitting force embed job to pool")
			}
		}
		s.logger.Debug().Msg("@@@ Step 12: All force embed jobs submitted")
	} else {
		s.logger.Debug().Msg("@@@ Step 11: No force embed documents found")
	}

	s.logger.Debug().Msg("@@@ Step 13: Fetching unvectorized documents from storage (limit 100)")
	unvectorizedDocs, err := s.documentStorage.GetUnvectorizedDocuments(100)
	if err != nil {
		s.logger.Error().Err(err).Msg("@@@ FAILED: Error querying unvectorized documents")
		return fmt.Errorf("failed to get unvectorized documents: %w", err)
	}
	s.logger.Debug().
		Int("count", len(unvectorizedDocs)).
		Msg("@@@ Step 14: Found unvectorized documents")

	if len(unvectorizedDocs) > 0 {
		s.logger.Info().
			Int("count", len(unvectorizedDocs)).
			Msg("@@@ Step 15: Processing unvectorized documents")

		for i, doc := range unvectorizedDocs {
			doc := doc
			s.logger.Debug().
				Int("job_index", i).
				Str("doc_id", doc.ID).
				Msg("@@@ Step 15.a: Submitting embed job")

			job := func(ctx context.Context) error {
				return s.embedDocument(ctx, doc, false)
			}
			if err := pool.Submit(job); err != nil {
				s.logger.Error().
					Err(err).
					Str("doc_id", doc.ID).
					Int("job_index", i).
					Msg("@@@ FAILED: Error submitting embed job to pool")
			}
		}
		s.logger.Debug().Msg("@@@ Step 16: All unvectorized jobs submitted")
	} else {
		s.logger.Debug().Msg("@@@ Step 15: No unvectorized documents found")
	}

	s.logger.Debug().Msg("@@@ Step 17: Waiting for all embedding jobs to complete")
	pool.Wait()
	s.logger.Debug().Msg("@@@ Step 18: All jobs completed")

	errors := pool.Errors()
	s.logger.Debug().
		Int("error_count", len(errors)).
		Msg("@@@ Step 19: Checked for errors")

	if len(errors) > 0 {
		s.logger.Warn().
			Int("error_count", len(errors)).
			Msg("@@@ WARNING: Some embedding jobs failed")
		for i, err := range errors {
			s.logger.Error().
				Int("error_index", i).
				Err(err).
				Msg("@@@ Job error detail")
		}
		return fmt.Errorf("embedding completed with %d errors", len(errors))
	}

	s.logger.Info().Msg("@@@ EMBEDDING COORDINATOR END - Success")
	return nil
}

// embedDocument generates embedding for a single document
func (s *CoordinatorService) embedDocument(ctx context.Context, doc *models.Document, isForceEmbed bool) error {
	// Validate inputs
	if doc == nil {
		return fmt.Errorf("document is nil - cannot embed")
	}
	if s.embeddingService == nil {
		return fmt.Errorf("embedding service is nil - cannot embed document")
	}
	if s.documentStorage == nil {
		return fmt.Errorf("document storage is nil - cannot save embedded document")
	}

	s.logger.Info().
		Str("doc_id", doc.ID).
		Str("force_embed", fmt.Sprintf("%v", isForceEmbed)).
		Msg("Generating embedding")

	if err := s.embeddingService.EmbedDocument(ctx, doc); err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	if isForceEmbed {
		doc.ForceEmbedPending = false
	}

	if err := s.documentStorage.UpdateDocument(doc); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	s.logger.Info().
		Str("doc_id", doc.ID).
		Msg("Document embedded successfully")

	return nil
}
