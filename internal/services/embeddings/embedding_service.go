package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Service implements EmbeddingService interface
type Service struct {
	ollamaURL string
	modelName string
	dimension int
	logger    arbor.ILogger
	client    *http.Client
}

// NewService creates a new embedding service
func NewService(ollamaURL, modelName string, dimension int, logger arbor.ILogger) interfaces.EmbeddingService {
	return &Service{
		ollamaURL: ollamaURL,
		modelName: modelName,
		dimension: dimension,
		logger:    logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateEmbedding creates a vector embedding for text
func (s *Service) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	reqBody := map[string]interface{}{
		"model":  s.modelName,
		"prompt": text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/api/embeddings", s.ollamaURL),
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding")
	}

	return result.Embedding, nil
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
	doc.EmbeddingModel = s.modelName

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
	return s.modelName
}

// Dimension returns the embedding dimension
func (s *Service) Dimension() int {
	return s.dimension
}

// IsAvailable checks if the embedding service is available
func (s *Service) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/api/tags", s.ollamaURL),
		nil,
	)
	if err != nil {
		return false
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Debug().Err(err).Msg("Ollama not available")
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
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
