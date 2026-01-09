package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// SearchOptions configures search behavior
type SearchOptions struct {
	// Limit maximum number of results
	Limit int

	// Offset for pagination (number of results to skip)
	Offset int

	// SourceTypes filters by document source (e.g., "jira", "confluence")
	SourceTypes []string

	// MetadataFilters filters by metadata fields (e.g., {"project": "PROJ-123"})
	MetadataFilters map[string]string

	// Tags filters documents that have ALL specified tags (AND operation)
	Tags []string

	// JobID filters documents that belong to a specific job (document.Jobs contains this ID)
	JobID string

	// CreatedAfter filters documents created after this timestamp (RFC3339 format)
	CreatedAfter string

	// UpdatedAfter filters documents updated after this timestamp (RFC3339 format)
	UpdatedAfter string

	// OrderBy specifies the field to order results by (created_at, updated_at)
	// Defaults to "updated_at" if not specified
	OrderBy string

	// OrderDir specifies the order direction (asc, desc)
	// Defaults to "desc" if not specified
	OrderDir string
}

// SearchService provides document search functionality
// This interface abstracts the search implementation, allowing
// different backends (FTS5, vector search, etc.) to be swapped
// without affecting the agent or other consumers.
type SearchService interface {
	// Search performs a full-text search across documents
	Search(ctx context.Context, query string, opts SearchOptions) ([]*models.Document, error)

	// GetByID retrieves a single document by its ID
	GetByID(ctx context.Context, id string) (*models.Document, error)

	// SearchByReference finds documents containing a specific reference
	// (e.g., issue keys like "PROJ-123" or user mentions like "@alice")
	SearchByReference(ctx context.Context, reference string, opts SearchOptions) ([]*models.Document, error)
}
