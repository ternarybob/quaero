package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// DocumentService handles normalized document operations
type DocumentService interface {
	// Save a single document (will generate embedding)
	SaveDocument(ctx context.Context, doc *models.Document) error

	// Save multiple documents in batch
	SaveDocuments(ctx context.Context, docs []*models.Document) error

	// Update existing document (will regenerate embedding if content changed)
	UpdateDocument(ctx context.Context, doc *models.Document) error

	// Get document by ID
	GetDocument(ctx context.Context, id string) (*models.Document, error)

	// Get document by source reference
	GetBySource(ctx context.Context, sourceType, sourceID string) (*models.Document, error)

	// Delete document
	DeleteDocument(ctx context.Context, id string) error

	// Search
	Search(ctx context.Context, query *SearchQuery) ([]*models.Document, error)

	// Stats
	GetStats(ctx context.Context) (*models.DocumentStats, error)
	Count(ctx context.Context, sourceType string) (int, error)

	// List documents with pagination
	List(ctx context.Context, opts *ListOptions) ([]*models.Document, error)
}

// SearchQuery represents search parameters
type SearchQuery struct {
	// Text query for keyword search
	Text string

	// Query embedding for vector search
	Embedding []float32

	// Filters
	SourceType string
	SourceIDs  []string

	// Date range
	UpdatedAfter  *string
	UpdatedBefore *string

	// Pagination
	Limit  int
	Offset int

	// Search mode
	Mode SearchMode
}

// SearchMode defines how search should be performed
type SearchMode string

const (
	SearchModeKeyword SearchMode = "keyword" // Full-text search only
	SearchModeVector  SearchMode = "vector"  // Vector similarity only
	SearchModeHybrid  SearchMode = "hybrid"  // Both keyword and vector
)

// ListOptions for listing documents
type ListOptions struct {
	SourceType    string
	Tags          []string // Filter by tags (OR logic - match any tag)
	Limit         int
	Offset        int
	OrderBy       string  // created_at, updated_at, title
	OrderDir      string  // asc, desc
	CreatedAfter  *string // RFC3339 timestamp for filtering documents created after this time
	CreatedBefore *string // RFC3339 timestamp for filtering documents created before this time
}
