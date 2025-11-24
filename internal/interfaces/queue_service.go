package interfaces

import (
	"context"
	"time"

	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// QueueManager manages the persistent message queue
// BREAKING CHANGE: Extend method now uses string messageID instead of goqite.ID
// to support Badger-backed queue implementation (removed goqite dependency)
type QueueManager interface {
	Enqueue(ctx context.Context, msg queue.Message) error
	Receive(ctx context.Context) (*queue.Message, func() error, error)
	Extend(ctx context.Context, messageID string, duration time.Duration) error
	Close() error
}

// LogEntry represents a log entry for WebSocket broadcasting
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// AggregatedJobMeta contains metadata for a job used in aggregated logs
type AggregatedJobMeta struct {
	JobName  string `json:"job_name"`  // User-friendly job name
	JobURL   string `json:"job_url"`   // URL being crawled (from Config or Progress)
	JobDepth int    `json:"job_depth"` // Crawl depth (from Config)
	JobType  string `json:"job_type"`  // Job type (from JobType)
	ParentID string `json:"parent_id"` // Parent job ID (empty for parent jobs)
}

// WebSocketHandler interface for broadcasting log entries
type WebSocketHandler interface {
	BroadcastLog(entry LogEntry)
}

// LogService manages log storage operations only
type LogService interface {
	AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry) error
	AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry) error
	GetLogs(ctx context.Context, jobID string, limit int) ([]models.JobLogEntry, error)
	GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.JobLogEntry, error)
	DeleteLogs(ctx context.Context, jobID string) error
	CountLogs(ctx context.Context, jobID string) (int, error)
	// GetAggregatedLogs fetches logs for parent job and optionally all child jobs
	// Merges logs from all jobs using k-way merge with cursor-based pagination
	// Returns logs slice and metadata map containing job context for enrichment
	// Metadata map structure: map[jobID]*AggregatedJobMeta with fields:
	//   JobName: User-friendly job name
	//   JobURL: URL being crawled (from Config or Progress)
	//   JobDepth: Crawl depth (from Config)
	//   JobType: Job type (from JobType)
	//   ParentID: Parent job ID (empty for parent jobs)
	// If includeChildren is false, only parent logs are returned
	// If level is non-empty, filters logs by level before merging
	// limit caps total logs returned across all jobs (default: 1000)
	// cursor is an opaque base64-encoded string encoding (full_timestamp|job_id|seq) for pagination
	//   where seq is a per-job sequence number for stable tie-breaking when timestamps are equal
	//   If cursor is empty, starts from the beginning (oldest for asc, newest for desc)
	// order determines sort order: "asc" (oldest-first) or "desc" (newest-first)
	// Returns next_cursor for chaining pagination requests (empty string when no more results)
	// The cursor is opaque and should be treated as an implementation detail - clients should
	// simply pass the returned cursor in subsequent requests to continue pagination
	GetAggregatedLogs(ctx context.Context, parentJobID string, includeChildren bool, level string, limit int, cursor string, order string) ([]models.JobLogEntry, map[string]*AggregatedJobMeta, string, error)
}

// JobManager manages job CRUD operations and queue integration
type JobManager interface {
	// Job lifecycle
	CreateJob(ctx context.Context, sourceType, sourceID string, config map[string]interface{}) (string, error)
	GetJob(ctx context.Context, jobID string) (interface{}, error)
	ListJobs(ctx context.Context, opts *JobListOptions) ([]*models.QueueJobState, error)
	CountJobs(ctx context.Context, opts *JobListOptions) (int, error)
	UpdateJob(ctx context.Context, job interface{}) error

	// DeleteJob deletes a job and all its child jobs recursively.
	// If the job has children, they are deleted first in a cascade operation.
	// Each deletion is logged individually for audit purposes.
	// If any child deletion fails, the error is logged but deletion continues.
	// The parent job is deleted even if some children fail to delete.
	// Returns the count of cascade-deleted jobs (children + grandchildren + ...) and an error if any deletions failed.
	DeleteJob(ctx context.Context, jobID string) (int, error)

	CopyJob(ctx context.Context, jobID string) (string, error)
	GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*JobChildStats, error)

	// StopAllChildJobs cancels all running child jobs of the specified parent job.
	// Used by error tolerance threshold management to stop child jobs when the parent's failure threshold is exceeded.
	// Returns the count of jobs that were successfully cancelled.
	StopAllChildJobs(ctx context.Context, parentID string) (int, error)

	// VERIFICATION COMMENT 2: GetJobWithChildren removed - flat hierarchy does not require tree traversal
	// All child messages point to root job ID for centralized progress tracking
}

// WorkerPool manages concurrent job processing
type WorkerPool interface {
	// Note: This interface is deprecated - use worker.Executor instead
	RegisterHandler(jobType string, handler interface{})
	Start() error
	Stop() error
}
