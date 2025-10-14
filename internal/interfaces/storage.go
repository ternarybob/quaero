// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:10:32 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// JiraStorage - interface for Jira data persistence
type JiraStorage interface {
	// Project operations
	StoreProject(ctx context.Context, project *models.JiraProject) error
	GetProject(ctx context.Context, key string) (*models.JiraProject, error)
	GetAllProjects(ctx context.Context) ([]*models.JiraProject, error)
	DeleteProject(ctx context.Context, key string) error
	CountProjects(ctx context.Context) (int, error)
	GetMostRecentProject(ctx context.Context) (*models.JiraProject, int64, error)

	// Issue operations
	StoreIssue(ctx context.Context, issue *models.JiraIssue) error
	StoreIssues(ctx context.Context, issues []*models.JiraIssue) error
	GetIssue(ctx context.Context, key string) (*models.JiraIssue, error)
	GetIssuesByProject(ctx context.Context, projectKey string) ([]*models.JiraIssue, error)
	DeleteIssue(ctx context.Context, key string) error
	DeleteIssuesByProject(ctx context.Context, projectKey string) error
	CountIssues(ctx context.Context) (int, error)
	CountIssuesByProject(ctx context.Context, projectKey string) (int, error)
	GetMostRecentIssue(ctx context.Context) (*models.JiraIssue, int64, error)

	// Search operations
	SearchIssues(ctx context.Context, query string) ([]*models.JiraIssue, error) // FTS5

	// Bulk operations
	ClearAll(ctx context.Context) error
}

// ConfluenceStorage - interface for Confluence data persistence
type ConfluenceStorage interface {
	// Space operations
	StoreSpace(ctx context.Context, space *models.ConfluenceSpace) error
	GetSpace(ctx context.Context, key string) (*models.ConfluenceSpace, error)
	GetAllSpaces(ctx context.Context) ([]*models.ConfluenceSpace, error)
	DeleteSpace(ctx context.Context, key string) error
	CountSpaces(ctx context.Context) (int, error)
	GetMostRecentSpace(ctx context.Context) (*models.ConfluenceSpace, int64, error)

	// Page operations
	StorePage(ctx context.Context, page *models.ConfluencePage) error
	StorePages(ctx context.Context, pages []*models.ConfluencePage) error
	GetPage(ctx context.Context, id string) (*models.ConfluencePage, error)
	GetPagesBySpace(ctx context.Context, spaceID string) ([]*models.ConfluencePage, error)
	DeletePage(ctx context.Context, id string) error
	DeletePagesBySpace(ctx context.Context, spaceID string) error
	CountPages(ctx context.Context) (int, error)
	CountPagesBySpace(ctx context.Context, spaceID string) (int, error)
	GetMostRecentPage(ctx context.Context) (*models.ConfluencePage, int64, error)

	// Search operations
	SearchPages(ctx context.Context, query string) ([]*models.ConfluencePage, error) // FTS5

	// Bulk operations
	ClearAll(ctx context.Context) error
}

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
	JiraStorage() JiraStorage
	ConfluenceStorage() ConfluenceStorage
	AuthStorage() AuthStorage
	DocumentStorage() DocumentStorage
	JobStorage() JobStorage
	SourceStorage() SourceStorage
	DB() interface{}
	Close() error
}
