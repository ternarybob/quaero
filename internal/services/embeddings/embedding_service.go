package embeddings

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/llm"
)

// Service implements EmbeddingService interface
type Service struct {
	llmService  interfaces.LLMService
	auditLogger llm.AuditLogger
	dimension   int
	logger      arbor.ILogger
}

// NewService creates a new embedding service
func NewService(llmService interfaces.LLMService, auditLogger llm.AuditLogger, dimension int, logger arbor.ILogger) interfaces.EmbeddingService {
	return &Service{
		llmService:  llmService,
		auditLogger: auditLogger,
		dimension:   dimension,
		logger:      logger,
	}
}

// GenerateEmbedding creates a vector embedding for text
func (s *Service) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Generate embedding via LLM service
	start := time.Now()
	embedding, err := s.llmService.Embed(ctx, text)
	duration := time.Since(start)

	// Log to audit trail
	mode := s.llmService.GetMode()
	if s.auditLogger != nil {
		auditErr := s.auditLogger.LogEmbed(mode, err == nil, duration, err, "")
		if auditErr != nil {
			s.logger.Warn().Err(auditErr).Msg("Failed to log embedding operation")
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if len(embedding) == 0 {
		return nil, fmt.Errorf("LLM service returned empty embedding")
	}

	s.logger.Debug().
		Str("mode", string(mode)).
		Int("embedding_dim", len(embedding)).
		Dur("duration", duration).
		Msg("Generated embedding")

	return embedding, nil
}

// EmbedDocument generates and sets embedding for a document
func (s *Service) EmbedDocument(ctx context.Context, doc *models.Document) error {
	// Combine title and content for embedding
	text := s.prepareDocumentText(doc)

	embedding, err := s.GenerateEmbedding(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	doc.Embedding = embedding
	doc.EmbeddingModel = string(s.llmService.GetMode())

	s.logger.Debug().
		Str("doc_id", doc.ID).
		Int("embedding_dim", len(embedding)).
		Int("text_length", len(text)).
		Msg("Generated embedding")

	return nil
}

// EmbedDocuments generates embeddings for multiple documents
func (s *Service) EmbedDocuments(ctx context.Context, docs []*models.Document) error {
	for i, doc := range docs {
		if err := s.EmbedDocument(ctx, doc); err != nil {
			s.logger.Error().
				Err(err).
				Str("doc_id", doc.ID).
				Int("index", i).
				Msg("Failed to embed document")
			return err
		}
	}

	return nil
}

// GenerateQueryEmbedding generates embedding for search query
func (s *Service) GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	// For queries, we might want to add a prefix or special handling
	// For now, just use the query as-is
	return s.GenerateEmbedding(ctx, query)
}

// ModelName returns the model name
func (s *Service) ModelName() string {
	return string(s.llmService.GetMode())
}

// Dimension returns the embedding dimension
func (s *Service) Dimension() int {
	return s.dimension
}

// IsAvailable checks if the embedding service is available
func (s *Service) IsAvailable(ctx context.Context) bool {
	if s.llmService == nil {
		return false
	}

	err := s.llmService.HealthCheck(ctx)
	if err != nil {
		s.logger.Debug().Err(err).Msg("LLM service not available")
		return false
	}

	return true
}

// prepareDocumentText combines title and content for embedding
func (s *Service) prepareDocumentText(doc *models.Document) string {
	// Simple concatenation with title weighted more heavily
	// You might want to add more sophisticated text preparation:
	// - Truncate if too long (token limits)
	// - Add metadata fields
	// - Special formatting for code/tables
	return fmt.Sprintf("%s\n\n%s", doc.Title, doc.Content)
}
