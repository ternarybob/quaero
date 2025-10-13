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
	VectorSearch(embedding []float32, limit int) ([]*models.Document, error)
	HybridSearch(query string, embedding []float32, limit int) ([]*models.Document, error)
	SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error)

	// List operations
	ListDocuments(opts *ListOptions) ([]*models.Document, error)
	GetDocumentsBySource(sourceType string) ([]*models.Document, error)

	// Stats operations
	CountDocuments() (int, error)
	CountDocumentsBySource(sourceType string) (int, error)
	CountVectorized() (int, error)
	GetStats() (*models.DocumentStats, error)

	// Chunk operations
	SaveChunk(chunk *models.DocumentChunk) error
	GetChunks(documentID string) ([]*models.DocumentChunk, error)
	DeleteChunks(documentID string) error

	// Force sync/embed operations
	SetForceSyncPending(id string, pending bool) error
	SetForceEmbedPending(id string, pending bool) error
	GetDocumentsForceSync() ([]*models.Document, error)
	GetDocumentsForceEmbed(limit int) ([]*models.Document, error)
	GetUnvectorizedDocuments(limit int) ([]*models.Document, error)

	// Embedding operations
	ClearAllEmbeddings() (int, error)

	// Bulk operations
	ClearAll() error
}

// StorageManager - composite interface for all storage operations
type StorageManager interface {
	JiraStorage() JiraStorage
	ConfluenceStorage() ConfluenceStorage
	AuthStorage() AuthStorage
	DocumentStorage() DocumentStorage
	DB() interface{}
	Close() error
}
