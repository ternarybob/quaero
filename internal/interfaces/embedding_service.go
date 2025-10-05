package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// EmbeddingService generates vector embeddings
type EmbeddingService interface {
	// Generate embedding for raw text
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

	// Generate and set embedding for a document
	EmbedDocument(ctx context.Context, doc *models.Document) error

	// Generate and set embeddings for multiple documents
	EmbedDocuments(ctx context.Context, docs []*models.Document) error

	// Generate query embedding (may have different prompt than document embedding)
	GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error)

	// Get model information
	ModelName() string
	Dimension() int

	// Check if service is available
	IsAvailable(ctx context.Context) bool
}
