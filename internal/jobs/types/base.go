package types

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// Job interface defines the contract for job type implementations
type Job interface {
	Execute(ctx context.Context, msg *queue.JobMessage) error
	Validate(msg *queue.JobMessage) error
	GetType() string
}

// BaseJob provides common functionality for all job types
type BaseJob struct {
	messageID       string
	jobDefinitionID string
	logger          arbor.ILogger
	jobManager      interfaces.JobManager
	queueManager    interfaces.QueueManager
	jobLogStorage   interfaces.JobLogStorage
}

// NewBaseJob creates a new base job
func NewBaseJob(messageID, jobDefinitionID string, logger arbor.ILogger, jobManager interfaces.JobManager, queueManager interfaces.QueueManager, jobLogStorage interfaces.JobLogStorage) *BaseJob {
	return &BaseJob{
		messageID:       messageID,
		jobDefinitionID: jobDefinitionID,
		logger:          logger,
		jobManager:      jobManager,
		queueManager:    queueManager,
		jobLogStorage:   jobLogStorage,
	}
}

// EnqueueChildJob enqueues a child job to the queue
func (b *BaseJob) EnqueueChildJob(ctx context.Context, msg *queue.JobMessage) error {
	if err := b.queueManager.Enqueue(ctx, msg); err != nil {
		return fmt.Errorf("failed to enqueue child job: %w", err)
	}

	b.logger.Debug().
		Str("message_id", msg.ID).
		Str("parent_id", msg.ParentID).
		Str("type", msg.Type).
		Msg("Child job enqueued")

	return nil
}

// LogJobEvent logs a job event
func (b *BaseJob) LogJobEvent(ctx context.Context, jobID string, level string, message string) error {
	logEntry := models.JobLogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Level:     level,
		Message:   message,
	}

	// Append log via job log storage
	if err := b.jobLogStorage.AppendLog(ctx, jobID, logEntry); err != nil {
		b.logger.Warn().
			Err(err).
			Str("job_id", jobID).
			Msg("Failed to append job log")
		return err
	}

	return nil
}
