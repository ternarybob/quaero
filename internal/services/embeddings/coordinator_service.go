package embeddings

import (
	"context"
	"fmt"

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
	s.logger.Info().Msg("Embedding event triggered")

	pool := workers.NewPool(10, s.logger)
	pool.Start()
	defer pool.Shutdown()

	forceEmbedDocs, err := s.documentStorage.GetDocumentsForceEmbed()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get force embed documents")
	} else {
		s.logger.Info().
			Int("count", len(forceEmbedDocs)).
			Msg("Processing force embed documents")

		for _, doc := range forceEmbedDocs {
			doc := doc
			job := func(ctx context.Context) error {
				return s.embedDocument(ctx, doc, true)
			}
			if err := pool.Submit(job); err != nil {
				s.logger.Error().Err(err).Str("doc_id", doc.ID).Msg("Failed to submit embed job")
			}
		}
	}

	unvectorizedDocs, err := s.documentStorage.GetUnvectorizedDocuments()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get unvectorized documents")
	} else {
		s.logger.Info().
			Int("count", len(unvectorizedDocs)).
			Msg("Processing unvectorized documents")

		for _, doc := range unvectorizedDocs {
			doc := doc
			job := func(ctx context.Context) error {
				return s.embedDocument(ctx, doc, false)
			}
			if err := pool.Submit(job); err != nil {
				s.logger.Error().Err(err).Str("doc_id", doc.ID).Msg("Failed to submit embed job")
			}
		}
	}

	pool.Wait()

	errors := pool.Errors()
	if len(errors) > 0 {
		s.logger.Warn().Int("error_count", len(errors)).Msg("Some embedding jobs failed")
		return fmt.Errorf("embedding completed with %d errors", len(errors))
	}

	s.logger.Info().Msg("Embedding event completed successfully")
	return nil
}

// embedDocument generates embedding for a single document
func (s *CoordinatorService) embedDocument(ctx context.Context, doc *models.Document, isForceEmbed bool) error {
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
