// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 6:08:59 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/interfaces/jobtypes"
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
	GetAllTags() ([]string, error)

	// NOTE: Phase 5 - Chunk operations removed (no longer using chunking for embeddings)

	// Force sync operations
	SetForceSyncPending(id string, pending bool) error
	GetDocumentsForceSync() ([]*models.Document, error)
	// NOTE: Phase 5 - Force embed operations removed: SetForceEmbedPending, GetDocumentsForceEmbed, GetUnvectorizedDocuments

	// NOTE: Phase 5 - Embedding operations removed: ClearAllEmbeddings

	// Bulk operations
	ClearAll() error

	// Index maintenance
	RebuildFTS5Index() error
}

// JobChildStats holds aggregate statistics for a parent job's children
// This is a type alias to jobtypes.JobChildStats for backward compatibility
type JobChildStats = jobtypes.JobChildStats

// StepStats holds aggregate statistics for step jobs under a manager
// This is a type alias to jobtypes.StepStats for backward compatibility
type StepStats = jobtypes.StepStats

// QueueStorage - interface for queue execution and state persistence
// Handles QueueJob (immutable queued work) and QueueJobState (runtime execution state)
// This is separate from JobDefinitionStorage which handles job definitions
type QueueStorage interface {
	SaveJob(ctx context.Context, job interface{}) error
	GetJob(ctx context.Context, jobID string) (interface{}, error)
	UpdateJob(ctx context.Context, job interface{}) error
	ListJobs(ctx context.Context, opts *JobListOptions) ([]*models.QueueJobState, error)
	GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*JobChildStats, error)
	// GetChildJobs retrieves all child jobs for a given parent job ID
	// Returns jobs ordered by created_at DESC (newest first)
	// Returns empty slice if parent has no children or parent doesn't exist
	GetChildJobs(ctx context.Context, parentID string) ([]*models.QueueJob, error)
	// GetStepStats returns aggregate statistics for step jobs under a manager
	// Used by ManagerMonitor to track overall progress of multi-step job definitions
	// Returns stats for all step jobs (type="step") with the given manager as parent
	GetStepStats(ctx context.Context, managerID string) (*StepStats, error)
	// ListStepJobs returns all step jobs under a manager
	// Returns jobs with type="step" and parent_id=managerID, ordered by created_at ASC (execution order)
	ListStepJobs(ctx context.Context, managerID string) ([]*models.QueueJob, error)
	GetJobsByStatus(ctx context.Context, status string) ([]*models.QueueJob, error)
	UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error
	UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error
	UpdateProgressCountersAtomic(ctx context.Context, jobID string, completedDelta, pendingDelta, totalDelta, failedDelta int) error
	// IncrementDocumentCountAtomic atomically increments the document_count in job metadata
	// Returns the new count after incrementing. Thread-safe for concurrent worker access.
	IncrementDocumentCountAtomic(ctx context.Context, jobID string) (int, error)
	UpdateJobHeartbeat(ctx context.Context, jobID string) error
	GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]*models.QueueJob, error)
	DeleteJob(ctx context.Context, jobID string) error
	CountJobs(ctx context.Context) (int, error)
	CountJobsByStatus(ctx context.Context, status string) (int, error)
	CountJobsWithFilters(ctx context.Context, opts *JobListOptions) (int, error)

	// Deprecated: Use LogService.AppendLog() instead. This method writes to the crawl_jobs.logs
	// JSON column (limited to 100 entries). The new LogService writes to the dedicated job_logs
	// table with unlimited history and better performance.
	AppendJobLog(ctx context.Context, jobID string, logEntry models.LogEntry) error

	// Deprecated: Use LogService.GetLogs() instead. This method reads from the crawl_jobs.logs
	// JSON column (limited to 100 entries). The new LogService reads from the dedicated job_logs
	// table with full history and indexed queries.
	GetJobLogs(ctx context.Context, jobID string) ([]models.LogEntry, error)

	// VERIFICATION COMMENT 1: Concurrency-safe URL deduplication
	// MarkURLSeen atomically records a URL as seen for a job and returns whether it was newly added.
	// Returns (true, nil) if URL was newly added, (false, nil) if URL was already seen.
	// This prevents race conditions when multiple workers try to enqueue the same URL.
	MarkURLSeen(ctx context.Context, jobID string, url string) (bool, error)

	// MarkRunningJobsAsPending marks all running jobs as pending (for graceful shutdown)
	// Returns the count of jobs marked as pending
	MarkRunningJobsAsPending(ctx context.Context, reason string) (int, error)
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
	DeleteAllJobDefinitions(ctx context.Context) error // Clear all job definitions (for config reload)
	CountJobDefinitions(ctx context.Context) (int, error)
}

// LogStorage - interface for log persistence
// ORDERING: GetLogs() and GetLogsByLevel() return logs in newest-first order (DESC).
// This matches typical web UI expectations where recent activity appears first.
type LogStorage interface {
	// AppendLog stores a log entry and returns the assigned line number
	AppendLog(ctx context.Context, jobID string, entry models.LogEntry) (lineNumber int, err error)
	AppendLogs(ctx context.Context, jobID string, entries []models.LogEntry) error
	GetLogs(ctx context.Context, jobID string, limit int) ([]models.LogEntry, error)
	GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.LogEntry, error)
	DeleteLogs(ctx context.Context, jobID string) error
	CountLogs(ctx context.Context, jobID string) (int, error)
	CountLogsByLevel(ctx context.Context, jobID string, level string) (int, error)

	// GetLogsWithOffset fetches logs starting from an offset (for pagination)
	// offset is the number of most recent logs to skip
	GetLogsWithOffset(ctx context.Context, jobID string, limit int, offset int) ([]models.LogEntry, error)
	GetLogsByLevelWithOffset(ctx context.Context, jobID string, level string, limit int, offset int) ([]models.LogEntry, error)

	// GetLogsByManagerID retrieves logs for all jobs under a manager (uses ManagerID index)
	GetLogsByManagerID(ctx context.Context, managerID string, limit int) ([]models.LogEntry, error)
	// GetLogsByStepID retrieves logs for all jobs under a step (uses StepID index)
	GetLogsByStepID(ctx context.Context, stepID string, limit int) ([]models.LogEntry, error)
}

// ConnectorStorage - interface for connector persistence
type ConnectorStorage interface {
	SaveConnector(ctx context.Context, connector *models.Connector) error
	GetConnector(ctx context.Context, id string) (*models.Connector, error)
	ListConnectors(ctx context.Context) ([]*models.Connector, error)
	UpdateConnector(ctx context.Context, connector *models.Connector) error
	DeleteConnector(ctx context.Context, id string) error
	DeleteAllConnectors(ctx context.Context) error // Clear all connectors (for config reload)
}

// StorageManager - composite interface for all storage operations
type StorageManager interface {
	AuthStorage() AuthStorage
	DocumentStorage() DocumentStorage
	QueueStorage() QueueStorage
	LogStorage() LogStorage
	JobDefinitionStorage() JobDefinitionStorage
	KeyValueStorage() KeyValueStorage
	ConnectorStorage() ConnectorStorage
	DB() interface{}
	Close() error

	// MigrateAPIKeysToKVStore migrates API keys from auth_credentials table to key_value_store
	// This is idempotent and safe to call multiple times
	MigrateAPIKeysToKVStore(ctx context.Context) error

	// ClearAllConfigData clears all TOML-loaded configuration data from storage
	// This includes: job definitions, connectors, and key/value pairs
	// Used by config reload functionality to ensure clean slate before reloading
	ClearAllConfigData(ctx context.Context) error

	// LoadVariablesFromFiles loads variables (key/value pairs) from TOML files in the specified directory
	// This is used to load configuration secrets and other variables at startup
	LoadVariablesFromFiles(ctx context.Context, dirPath string) error

	// LoadEnvFile loads variables from a .env file into the KV store
	// Format: KEY=value or KEY="value", # comments, empty lines ignored
	// This is loaded after TOML variables, so .env values take precedence
	LoadEnvFile(ctx context.Context, filePath string) error

	// LoadJobDefinitionsFromFiles loads job definitions from TOML files in the specified directory
	// This is used to load job definitions at startup
	LoadJobDefinitionsFromFiles(ctx context.Context, dirPath string) error

	// LoadConnectorsFromFiles loads connectors from TOML files in the specified directory
	// This is used to load connector configurations at startup
	LoadConnectorsFromFiles(ctx context.Context, dirPath string) error

	// LoadEmailFromFile loads email configuration from email.toml file in the specified directory
	// This is used to load email/SMTP settings at startup
	LoadEmailFromFile(ctx context.Context, dirPath string) error
}
