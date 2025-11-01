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

// CreateChildJobRecord creates and persists a child job record to the database
// This centralizes child job creation logic for consistency across all job types
func (b *BaseJob) CreateChildJobRecord(ctx context.Context, parentID, childID, url, sourceType, entityType string, config models.CrawlConfig) error {
	// Create child CrawlJob record with proper hierarchy
	childJob := &models.CrawlJob{
		ID:         childID,
		ParentID:   parentID, // Inherit root job ID (flat hierarchy)
		JobType:    models.JobTypeCrawlerURL,
		Name:       fmt.Sprintf("URL: %s", url),
		SourceType: sourceType,
		EntityType: entityType,
		Config:     config,
		Status:     models.JobStatusPending,
		Progress: models.CrawlProgress{
			TotalURLs:     1,
			PendingURLs:   1,
			CompletedURLs: 0,
			FailedURLs:    0,
			StartTime:     time.Now(),
		},
		CreatedAt: time.Now(),
	}

	// Persist child job to database via JobManager
	if err := b.jobManager.UpdateJob(ctx, childJob); err != nil {
		b.logger.Warn().
			Err(err).
			Str("child_id", childID).
			Str("child_url", url).
			Msg("Failed to persist child job to database")
		return fmt.Errorf("failed to persist child job: %w", err)
	}

	b.logger.Debug().
		Str("child_id", childID).
		Str("child_url", url).
		Str("parent_id", parentID).
		Str("source_type", sourceType).
		Str("entity_type", entityType).
		Msg("Child job persisted to database")

	return nil
}
