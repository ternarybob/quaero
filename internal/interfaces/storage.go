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
	GetCredentialsByID(ctx context.Context, id string) (*models.AuthCredentials, error)
	GetCredentialsBySiteDomain(ctx context.Context, siteDomain string) (*models.AuthCredentials, error)
	DeleteCredentials(ctx context.Context, id string) error
	ListCredentials(ctx context.Context) ([]*models.AuthCredentials, error)

	// Deprecated: Use GetCredentialsBySiteDomain instead
	GetCredentials(ctx context.Context, service string) (*models.AuthCredentials, error)
	// Deprecated: Use ListCredentials instead
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
	UpdateJob(ctx context.Context, job interface{}) error
	ListJobs(ctx context.Context, opts *ListOptions) ([]*models.CrawlJob, error)
	GetJobsByStatus(ctx context.Context, status string) ([]*models.CrawlJob, error)
	UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error
	UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error
	UpdateJobHeartbeat(ctx context.Context, jobID string) error
	GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]*models.CrawlJob, error)
	DeleteJob(ctx context.Context, jobID string) error
	CountJobs(ctx context.Context) (int, error)
	CountJobsByStatus(ctx context.Context, status string) (int, error)
	CountJobsWithFilters(ctx context.Context, opts *ListOptions) (int, error)

	// Deprecated: Use LogService.AppendLog() instead. This method writes to the crawl_jobs.logs
	// JSON column (limited to 100 entries). The new LogService writes to the dedicated job_logs
	// table with unlimited history and better performance.
	AppendJobLog(ctx context.Context, jobID string, logEntry models.JobLogEntry) error

	// Deprecated: Use LogService.GetLogs() instead. This method reads from the crawl_jobs.logs
	// JSON column (limited to 100 entries). The new LogService reads from the dedicated job_logs
	// table with full history and indexed queries.
	GetJobLogs(ctx context.Context, jobID string) ([]models.JobLogEntry, error)

	// VERIFICATION COMMENT 1: Concurrency-safe URL deduplication
	// MarkURLSeen atomically records a URL as seen for a job and returns whether it was newly added.
	// Returns (true, nil) if URL was newly added, (false, nil) if URL was already seen.
	// This prevents race conditions when multiple workers try to enqueue the same URL.
	MarkURLSeen(ctx context.Context, jobID string, url string) (bool, error)
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

// JobDefinitionListOptions represents filtering and pagination options for listing job definitions
type JobDefinitionListOptions struct {
	Type     string // Filter by job type (crawler, summarizer, custom)
	Enabled  *bool  // Filter by enabled status (nil = no filter, true = enabled only, false = disabled only)
	OrderBy  string // Order by field (created_at, updated_at, name)
	OrderDir string // Order direction (ASC, DESC)
	Limit    int    // Maximum number of results to return
	Offset   int    // Number of results to skip for pagination
}

// JobDefinitionStorage - interface for job definition persistence
type JobDefinitionStorage interface {
	SaveJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error
	UpdateJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error
	GetJobDefinition(ctx context.Context, id string) (*models.JobDefinition, error)
	ListJobDefinitions(ctx context.Context, opts *JobDefinitionListOptions) ([]*models.JobDefinition, error)
	GetJobDefinitionsByType(ctx context.Context, jobType string) ([]*models.JobDefinition, error)
	GetEnabledJobDefinitions(ctx context.Context) ([]*models.JobDefinition, error)
	DeleteJobDefinition(ctx context.Context, id string) error
	CountJobDefinitions(ctx context.Context) (int, error)
}

// JobLogStorage - interface for job log persistence
// ORDERING: GetLogs() and GetLogsByLevel() return logs in newest-first order (DESC).
// This matches typical web UI expectations where recent activity appears first.
type JobLogStorage interface {
	AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry) error
	AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry) error
	GetLogs(ctx context.Context, jobID string, limit int) ([]models.JobLogEntry, error)
	GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.JobLogEntry, error)
	DeleteLogs(ctx context.Context, jobID string) error
	CountLogs(ctx context.Context, jobID string) (int, error)
}

// StorageManager - composite interface for all storage operations
type StorageManager interface {
	AuthStorage() AuthStorage
	DocumentStorage() DocumentStorage
	JobStorage() JobStorage
	JobLogStorage() JobLogStorage
	SourceStorage() SourceStorage
	JobDefinitionStorage() JobDefinitionStorage
	DB() interface{}
	Close() error
}
