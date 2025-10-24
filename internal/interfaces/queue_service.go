package interfaces

import (
	"context"
	"time"

	"maragu.dev/goqite"

	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
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

// LogService manages batch log persistence
type LogService interface {
	Start() error
	Stop() error
	AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry)
	AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry)
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
	ListJobs(ctx context.Context, opts *ListOptions) ([]*models.CrawlJob, error)
	CountJobs(ctx context.Context, opts *ListOptions) (int, error)
	UpdateJob(ctx context.Context, job interface{}) error
	DeleteJob(ctx context.Context, jobID string) error
	CopyJob(ctx context.Context, jobID string) (string, error)
	// VERIFICATION COMMENT 2: GetJobWithChildren removed - flat hierarchy does not require tree traversal
	// All child messages point to root job ID for centralized progress tracking
}

// WorkerPool manages concurrent job processing
type WorkerPool interface {
	RegisterHandler(jobType string, handler queue.JobHandler)
	Start() error
	Stop() error
}
