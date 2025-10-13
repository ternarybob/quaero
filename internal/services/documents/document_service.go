package documents

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/metadata"
)

// Service implements DocumentService interface
type Service struct {
	storage           interfaces.DocumentStorage
	embeddingService  interfaces.EmbeddingService
	metadataExtractor *metadata.Extractor
	logger            arbor.ILogger
}

// NewService creates a new document service
func NewService(
	storage interfaces.DocumentStorage,
	embeddingService interfaces.EmbeddingService,
	logger arbor.ILogger,
) interfaces.DocumentService {
	return &Service{
		storage:           storage,
		embeddingService:  embeddingService,
		metadataExtractor: metadata.NewExtractor(logger),
		logger:            logger,
	}
}

// SaveDocument saves a document WITHOUT embedding
// NOTE: Embedding is handled by independent embedding coordinator
func (s *Service) SaveDocument(ctx context.Context, doc *models.Document) error {
	// Generate ID if not present
	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc_%s", uuid.New().String())
	}

	// Extract and merge metadata
	extracted, err := s.metadataExtractor.ExtractMetadata(doc)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to extract metadata, continuing without it")
	} else if len(extracted) > 0 {
		doc.Metadata = s.metadataExtractor.MergeMetadata(doc.Metadata, extracted)
		s.logger.Debug().
			Str("doc_id", doc.ID).
			Int("extracted_fields", len(extracted)).
			Msg("Metadata extracted and merged")
	}

	// Save to storage without embedding
	if err := s.storage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	s.logger.Info().
		Str("doc_id", doc.ID).
		Str("source", doc.SourceType).
		Str("source_id", doc.SourceID).
		Msg("Document saved (embedding will be processed independently)")

	return nil
}

// SaveDocuments saves multiple documents in batch WITHOUT embedding
// NOTE: Embedding is handled by independent embedding coordinator
func (s *Service) SaveDocuments(ctx context.Context, docs []*models.Document) error {
	if len(docs) == 0 {
		return nil
	}

	// Generate IDs and extract metadata for each document
	for _, doc := range docs {
		if doc.ID == "" {
			doc.ID = fmt.Sprintf("doc_%s", uuid.New().String())
		}

		// Extract and merge metadata
		extracted, err := s.metadataExtractor.ExtractMetadata(doc)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("doc_id", doc.ID).
				Msg("Failed to extract metadata, continuing without it")
		} else if len(extracted) > 0 {
			doc.Metadata = s.metadataExtractor.MergeMetadata(doc.Metadata, extracted)
		}
	}

	// Save all documents without embedding
	if err := s.storage.SaveDocuments(docs); err != nil {
		return fmt.Errorf("failed to save documents: %w", err)
	}

	s.logger.Info().
		Int("total", len(docs)).
		Msg("Documents saved (embedding will be processed independently)")

	return nil
}

// UpdateDocument updates an existing document WITHOUT re-embedding
// NOTE: Embedding is handled by independent embedding coordinator
func (s *Service) UpdateDocument(ctx context.Context, doc *models.Document) error {
	// Check if document exists
	existing, err := s.storage.GetDocument(doc.ID)
	if err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	// Check if content changed (for logging only)
	contentChanged := existing.Content != doc.Content || existing.Title != doc.Title

	// Update in storage without re-embedding
	if err := s.storage.UpdateDocument(doc); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	changedStatus := "no"
	if contentChanged {
		changedStatus = "yes"
	}
	s.logger.Info().
		Str("doc_id", doc.ID).
		Str("content_changed", changedStatus).
		Msg("Document updated (re-embedding will be handled independently)")

	return nil
}

// GetDocument retrieves a document by ID
func (s *Service) GetDocument(ctx context.Context, id string) (*models.Document, error) {
	return s.storage.GetDocument(id)
}

// GetBySource retrieves a document by source reference
func (s *Service) GetBySource(ctx context.Context, sourceType, sourceID string) (*models.Document, error) {
	return s.storage.GetDocumentBySource(sourceType, sourceID)
}

// DeleteDocument deletes a document and its chunks
func (s *Service) DeleteDocument(ctx context.Context, id string) error {
	// Delete chunks first
	if err := s.storage.DeleteChunks(id); err != nil {
		s.logger.Warn().
			Err(err).
			Str("doc_id", id).
			Msg("Failed to delete chunks")
	}

	// Delete document
	if err := s.storage.DeleteDocument(id); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	s.logger.Info().Str("doc_id", id).Msg("Document deleted")
	return nil
}

// Search performs a search based on the query mode
func (s *Service) Search(ctx context.Context, query *interfaces.SearchQuery) ([]*models.Document, error) {
	if query.Limit <= 0 {
		query.Limit = 10
	}

	switch query.Mode {
	case interfaces.SearchModeKeyword:
		if query.Text == "" {
			return nil, fmt.Errorf("text query required for keyword search")
		}
		return s.storage.FullTextSearch(query.Text, query.Limit)

	case interfaces.SearchModeVector:
		if query.Embedding == nil {
			// Generate embedding from text query
			if query.Text == "" {
				return nil, fmt.Errorf("text or embedding required for vector search")
			}
			embedding, err := s.embeddingService.GenerateQueryEmbedding(ctx, query.Text)
			if err != nil {
				return nil, fmt.Errorf("failed to generate query embedding: %w", err)
			}
			query.Embedding = embedding
		}
		return s.storage.VectorSearch(query.Embedding, query.Limit)

	case interfaces.SearchModeHybrid:
		if query.Text == "" {
			return nil, fmt.Errorf("text query required for hybrid search")
		}
		if query.Embedding == nil {
			// Generate embedding from text query
			embedding, err := s.embeddingService.GenerateQueryEmbedding(ctx, query.Text)
			if err != nil {
				s.logger.Warn().Err(err).Msg("Failed to generate query embedding, falling back to keyword search")
				return s.storage.FullTextSearch(query.Text, query.Limit)
			}
			query.Embedding = embedding
		}
		return s.storage.HybridSearch(query.Text, query.Embedding, query.Limit)

	default:
		return nil, fmt.Errorf("invalid search mode: %s", query.Mode)
	}
}

// GetStats retrieves document statistics
func (s *Service) GetStats(ctx context.Context) (*models.DocumentStats, error) {
	return s.storage.GetStats()
}

// Count returns document count, optionally filtered by source
func (s *Service) Count(ctx context.Context, sourceType string) (int, error) {
	if sourceType == "" {
		return s.storage.CountDocuments()
	}
	return s.storage.CountDocumentsBySource(sourceType)
}

// List returns documents with pagination
func (s *Service) List(ctx context.Context, opts *interfaces.ListOptions) ([]*models.Document, error) {
	if opts == nil {
		opts = &interfaces.ListOptions{
			Limit:    50,
			Offset:   0,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}
	}

	return s.storage.ListDocuments(opts)
}
