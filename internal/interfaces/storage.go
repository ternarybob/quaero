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

	// Issue operations
	StoreIssue(ctx context.Context, issue *models.JiraIssue) error
	StoreIssues(ctx context.Context, issues []*models.JiraIssue) error
	GetIssue(ctx context.Context, key string) (*models.JiraIssue, error)
	GetIssuesByProject(ctx context.Context, projectKey string) ([]*models.JiraIssue, error)
	DeleteIssue(ctx context.Context, key string) error
	DeleteIssuesByProject(ctx context.Context, projectKey string) error
	CountIssues(ctx context.Context) (int, error)
	CountIssuesByProject(ctx context.Context, projectKey string) (int, error)

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

	// Page operations
	StorePage(ctx context.Context, page *models.ConfluencePage) error
	StorePages(ctx context.Context, pages []*models.ConfluencePage) error
	GetPage(ctx context.Context, id string) (*models.ConfluencePage, error)
	GetPagesBySpace(ctx context.Context, spaceID string) ([]*models.ConfluencePage, error)
	DeletePage(ctx context.Context, id string) error
	DeletePagesBySpace(ctx context.Context, spaceID string) error
	CountPages(ctx context.Context) (int, error)
	CountPagesBySpace(ctx context.Context, spaceID string) (int, error)

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

// StorageManager - composite interface for all storage operations
type StorageManager interface {
	JiraStorage() JiraStorage
	ConfluenceStorage() ConfluenceStorage
	AuthStorage() AuthStorage
	Close() error
}
