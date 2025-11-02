// Package types provides job type implementations for the queue-based job system.
//
// Architecture Overview:
//
// The job system follows a clean separation of concerns:
//   - JobManager (internal/services/jobs/manager.go): CRUD operations for jobs
//   - JobLogger (logger.go): Structured logging with correlation context
//   - Job Types (crawler.go, summarizer.go, etc.): Execution logic
//
// Dependency Injection Pattern:
//
// All job types follow the same pattern:
//   1. Define a *Deps struct with all dependencies (interfaces only)
//   2. Embed BaseJob for common functionality
//   3. Accept deps via constructor (NewXxxJob)
//   4. Implement Job interface: Execute(), Validate(), GetType()
//
// Example:
//   type CrawlerJobDeps struct {
//       CrawlerService  interface{}
//       JobStorage      interfaces.JobStorage
//       // ... other dependencies
//   }
//
//   type CrawlerJob struct {
//       *BaseJob
//       deps *CrawlerJobDeps
//   }
//
// Job Lifecycle:
//
//   1. Worker receives message from queue
//   2. Worker creates BaseJob with correlation context (jobID, parentID)
//   3. Worker creates job type (e.g., NewCrawlerJob) with BaseJob + deps
//   4. Worker calls Validate() to check message structure
//   5. Worker calls Execute() to run job logic
//   6. Job logs via JobLogger (logs flow to LogService via Arbor context channel)
//   7. Job updates status via JobStorage.UpdateJobStatus()
//   8. Worker deletes message from queue on completion
//
// Parent-Child Job Hierarchy:
//
// The system uses a FLAT hierarchy model (not nested tree):
//   - Parent jobs spawn child jobs via EnqueueChildJob()
//   - Child jobs inherit parent's jobID as CorrelationID for log aggregation
//   - All children reference the root parent ID (not immediate parent)
//   - Progress tracked at job level via TotalURLs/CompletedURLs/PendingURLs
//   - See manager.go lines 395-416 for detailed rationale
//
// Error Handling:
//
// All job types should follow this pattern:
//   1. Validate message before execution
//   2. On validation error: log, update job status to 'failed', return error
//   3. On execution error: log, update job status to 'failed', return error
//   4. Use formatJobError() (crawler.go) for user-friendly error messages
//   5. Format: "Category: Brief description" (e.g., "HTTP 404: Not Found")
//
// Logging Strategy:
//
// Use JobLogger helper methods for consistent structured logging:
//   - LogJobStart(name, sourceType, config) - Job initialization
//   - LogJobProgress(completed, total, message) - Progress updates
//   - LogJobComplete(duration, resultCount) - Successful completion
//   - LogJobError(err, context) - Errors with context
//   - LogJobCancelled(reason) - Cancellation
//
// All logs automatically include jobID via CorrelationID and flow to:
//   - LogService → Database (job_logs table)
//   - LogService → WebSocket (real-time UI updates)
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

// BaseJob provides common functionality for all job types.
//
// Responsibilities:
//   - Correlation context management via JobLogger
//   - Child job enqueueing via QueueManager
//   - Child job record creation via JobManager
//   - Structured logging via JobLogger helper methods
//
// Usage:
//   base := NewBaseJob(messageID, jobDefID, jobID, parentID, logger, jobMgr, queueMgr, logStorage)
//   crawler := NewCrawlerJob(base, deps)
//
// All job types should embed BaseJob to inherit common functionality.
// BaseJob handles correlation context automatically - child jobs inherit parent's jobID.
type BaseJob struct {
	messageID       string
	jobDefinitionID string
	logger          *JobLogger
	jobManager      interfaces.JobManager
	queueManager    interfaces.QueueManager
	jobLogStorage   interfaces.JobLogStorage
}

// NewBaseJob creates a new base job with JobLogger correlation context
func NewBaseJob(messageID, jobDefinitionID, jobID, parentID string, baseLogger arbor.ILogger, jobManager interfaces.JobManager, queueManager interfaces.QueueManager, jobLogStorage interfaces.JobLogStorage) *BaseJob {
	logger := NewJobLogger(baseLogger, jobID, parentID)

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
		b.logger.Error().Err(err).Str("message_id", msg.ID).Str("parent_id", msg.ParentID).Msg("Failed to enqueue child job")
		return fmt.Errorf("failed to enqueue child job: %w", err)
	}

	b.logger.Debug().
		Str("message_id", msg.ID).
		Str("parent_id", msg.ParentID).
		Str("type", msg.Type).
		Msg("Child job enqueued")

	return nil
}

// GetLogger returns the JobLogger for custom logging
func (b *BaseJob) GetLogger() *JobLogger {
	return b.logger
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

	// Persist child job to database via JobStorage
	// JobStorage.SaveJob handles both create and update (upsert semantics)
	if err := b.jobManager.UpdateJob(ctx, childJob); err != nil {
		b.logger.Error().Err(err).Str("child_id", childID).Str("child_url", url).Msg("Failed to persist child job to database")
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
