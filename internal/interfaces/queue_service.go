package interfaces

import (
	"context"
	"time"

	"maragu.dev/goqite"

	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	arbormodels "github.com/ternarybob/arbor/models"
)

// QueueManager manages the persistent message queue
type QueueManager interface {
	Start() error
	Stop() error
	Restart() error
	Enqueue(ctx context.Context, msg *queue.JobMessage) error
	EnqueueWithDelay(ctx context.Context, msg *queue.JobMessage, delay time.Duration) error
	Receive(ctx context.Context) (*goqite.Message, error)
	Delete(ctx context.Context, msg goqite.Message) error
	Extend(ctx context.Context, msg goqite.Message, duration time.Duration) error
	GetQueueLength(ctx context.Context) (int, error)
	GetQueueStats(ctx context.Context) (map[string]interface{}, error)
}

// LogEntry represents a log entry for WebSocket broadcasting
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// WebSocketHandler interface for broadcasting log entries
type WebSocketHandler interface {
	BroadcastLog(entry LogEntry)
}

// LogService manages batch log persistence
type LogService interface {
	Start() error
	Stop() error
	GetChannel() chan []arbormodels.LogEvent
	AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry) error
	AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry) error
	GetLogs(ctx context.Context, jobID string, limit int) ([]models.JobLogEntry, error)
	GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.JobLogEntry, error)
	DeleteLogs(ctx context.Context, jobID string) error
	CountLogs(ctx context.Context, jobID string) (int, error)
}

// JobManager manages job CRUD operations and queue integration
type JobManager interface {
	// Job lifecycle
	CreateJob(ctx context.Context, sourceType, sourceID string, config map[string]interface{}) (string, error)
	GetJob(ctx context.Context, jobID string) (interface{}, error)
	ListJobs(ctx context.Context, opts *JobListOptions) ([]*models.CrawlJob, error)
	CountJobs(ctx context.Context, opts *JobListOptions) (int, error)
	UpdateJob(ctx context.Context, job interface{}) error

	// DeleteJob deletes a job and all its child jobs recursively.
	// If the job has children, they are deleted first in a cascade operation.
	// Each deletion is logged individually for audit purposes.
	// If any child deletion fails, the error is logged but deletion continues.
	// The parent job is deleted even if some children fail to delete.
	// Returns an aggregated error if any deletions failed.
	DeleteJob(ctx context.Context, jobID string) error

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
	RegisterHandler(jobType string, handler queue.JobHandler)
	Start() error
	Stop() error
}
