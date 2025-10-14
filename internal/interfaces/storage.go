// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:10:32 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// AuthStorage - interface for authentication data
type AuthStorage interface {
	StoreCredentials(ctx context.Context, credentials *models.AuthCredentials) error
	GetCredentials(ctx context.Context, service string) (*models.AuthCredentials, error)
	DeleteCredentials(ctx context.Context, service string) error
	ListServices(ctx context.Context) ([]string, error)
}

// DocumentStorage - interface for normalized document persistence
type DocumentStorage interface {
	// CRUD operations
	SaveDocument(doc *models.Document) error
	SaveDocuments(docs []*models.Document) error
	GetDocument(id string) (*models.Document, error)
	GetDocumentBySource(sourceType, sourceID string) (*models.Document, error)
	UpdateDocument(doc *models.Document) error
	DeleteDocument(id string) error

	// Search operations
	FullTextSearch(query string, limit int) ([]*models.Document, error)
	// NOTE: Phase 5 - VectorSearch and HybridSearch removed (using FTS5 only)
	SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error)

	// List operations
	ListDocuments(opts *ListOptions) ([]*models.Document, error)
	GetDocumentsBySource(sourceType string) ([]*models.Document, error)

	// Stats operations
	CountDocuments() (int, error)
	CountDocumentsBySource(sourceType string) (int, error)
	// NOTE: Phase 5 - CountVectorized removed (no longer using embeddings)
	GetStats() (*models.DocumentStats, error)

	// NOTE: Phase 5 - Chunk operations removed (no longer using chunking for embeddings)

	// Force sync operations
	SetForceSyncPending(id string, pending bool) error
	GetDocumentsForceSync() ([]*models.Document, error)
	// NOTE: Phase 5 - Force embed operations removed: SetForceEmbedPending, GetDocumentsForceEmbed, GetUnvectorizedDocuments

	// NOTE: Phase 5 - Embedding operations removed: ClearAllEmbeddings

	// Bulk operations
	ClearAll() error
}

// JobStorage - interface for crawler job persistence
type JobStorage interface {
	SaveJob(ctx context.Context, job interface{}) error
	GetJob(ctx context.Context, jobID string) (interface{}, error)
	ListJobs(ctx context.Context, opts *ListOptions) ([]interface{}, error)
	GetJobsByStatus(ctx context.Context, status string) ([]interface{}, error)
	UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error
	UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error
	DeleteJob(ctx context.Context, jobID string) error
	CountJobs(ctx context.Context) (int, error)
	CountJobsByStatus(ctx context.Context, status string) (int, error)
}

// SourceStorage - interface for source configuration persistence
type SourceStorage interface {
	SaveSource(ctx context.Context, source *models.SourceConfig) error
	GetSource(ctx context.Context, id string) (*models.SourceConfig, error)
	ListSources(ctx context.Context) ([]*models.SourceConfig, error)
	DeleteSource(ctx context.Context, id string) error
	GetSourcesByType(ctx context.Context, sourceType string) ([]*models.SourceConfig, error)
	GetEnabledSources(ctx context.Context) ([]*models.SourceConfig, error)
}

// StorageManager - composite interface for all storage operations
type StorageManager interface {
	AuthStorage() AuthStorage
	DocumentStorage() DocumentStorage
	JobStorage() JobStorage
	SourceStorage() SourceStorage
	DB() interface{}
	Close() error
}
