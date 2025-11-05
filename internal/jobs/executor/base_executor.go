// -----------------------------------------------------------------------
// Base Job Executor - Common functionality for all job executors
// -----------------------------------------------------------------------

package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// BaseExecutor provides common functionality for all job executors
type BaseExecutor struct {
	jobManager *jobs.Manager
	queueMgr   *queue.Manager
	logger     arbor.ILogger
	logService interfaces.LogService
	wsHandler  interfaces.WebSocketHandler
}

// NewBaseExecutor creates a new base executor
func NewBaseExecutor(
	jobManager *jobs.Manager,
	queueMgr *queue.Manager,
	logger arbor.ILogger,
	logService interfaces.LogService,
	wsHandler interfaces.WebSocketHandler,
) *BaseExecutor {
	return &BaseExecutor{
		jobManager: jobManager,
		queueMgr:   queueMgr,
		logger:     logger,
		logService: logService,
		wsHandler:  wsHandler,
	}
}

// CreateJobLogger creates a job-specific logger with correlation ID
// This ensures all logs are associated with the job ID
func (b *BaseExecutor) CreateJobLogger(job *models.JobModel) arbor.ILogger {
	// Create logger with correlation ID set to job ID
	// This allows LogService to extract jobID from logs and store them properly
	// The WithCorrelationId method returns a new logger with the correlation ID set
	jobLogger := b.logger.WithCorrelationId(job.ID)

	return jobLogger
}

// CreateJobRecord creates a job record in the database
func (b *BaseExecutor) CreateJobRecord(ctx context.Context, job *models.JobModel) error {
	// Convert job model to database job record
	dbJob := &jobs.Job{
		ID:              job.ID,
		ParentID:        job.ParentID,
		Type:            job.Type,
		Name:            job.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       job.CreatedAt,
		ProgressCurrent: 0,
		ProgressTotal:   0,
	}

	// Serialize payload (the entire job model)
	payloadBytes, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job model: %w", err)
	}
	dbJob.Payload = string(payloadBytes)

	// Create job record
	if err := b.jobManager.CreateJob(ctx, dbJob); err != nil {
		return fmt.Errorf("failed to create job record: %w", err)
	}

	return nil
}

// UpdateJobStatus updates the job status in the database
func (b *BaseExecutor) UpdateJobStatus(ctx context.Context, jobID, status string) error {
	return b.jobManager.UpdateJobStatus(ctx, jobID, status)
}

// UpdateJobProgress updates the job progress in the database
func (b *BaseExecutor) UpdateJobProgress(ctx context.Context, jobID string, current, total int) error {
	return b.jobManager.UpdateJobProgress(ctx, jobID, current, total)
}

// SetJobError sets the job error in the database
func (b *BaseExecutor) SetJobError(ctx context.Context, jobID, errorMsg string) error {
	return b.jobManager.SetJobError(ctx, jobID, errorMsg)
}

// SpawnChildJob creates and enqueues a child job
func (b *BaseExecutor) SpawnChildJob(ctx context.Context, parentJob *models.JobModel, childType, childName string, config map[string]interface{}) error {
	// Create child job model
	childMetadata := make(map[string]interface{})

	// Copy parent metadata if exists
	if parentJob.Metadata != nil {
		for k, v := range parentJob.Metadata {
			childMetadata[k] = v
		}
	}

	// Create child job
	childJob := models.NewChildJobModel(
		parentJob.ID,
		childType,
		childName,
		config,
		childMetadata,
		parentJob.Depth+1,
	)

	// Validate child job
	if err := childJob.Validate(); err != nil {
		return fmt.Errorf("invalid child job model: %w", err)
	}

	// Create job record in database
	if err := b.CreateJobRecord(ctx, childJob); err != nil {
		return fmt.Errorf("failed to create child job record: %w", err)
	}

	// Serialize job model to JSON for queue
	jobBytes, err := childJob.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize child job: %w", err)
	}

	// Enqueue child job
	queueMsg := queue.Message{
		JobID:   childJob.ID,
		Type:    childJob.Type,
		Payload: json.RawMessage(jobBytes),
	}

	if err := b.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return fmt.Errorf("failed to enqueue child job: %w", err)
	}

	b.logger.Info().
		Str("parent_job_id", parentJob.ID).
		Str("child_job_id", childJob.ID).
		Str("child_type", childType).
		Str("child_name", childName).
		Int("child_depth", childJob.Depth).
		Msg("Child job spawned and enqueued")

	return nil
}

// LogJobStart logs the start of a job
func (b *BaseExecutor) LogJobStart(jobLogger arbor.ILogger, job *models.JobModel) {
	jobLogger.Info().
		Msg("Job execution started")
}

// LogJobComplete logs the completion of a job
func (b *BaseExecutor) LogJobComplete(jobLogger arbor.ILogger, job *models.JobModel) {
	jobLogger.Info().
		Msg("Job execution completed successfully")
}

// LogJobError logs a job error
func (b *BaseExecutor) LogJobError(jobLogger arbor.ILogger, job *models.JobModel, err error) {
	jobLogger.Error().
		Err(err).
		Msg("Job execution failed")
}

// LogJobProgress logs job progress
func (b *BaseExecutor) LogJobProgress(jobLogger arbor.ILogger, job *models.JobModel, current, total int, message string) {
	jobLogger.Info().
		Int("current", current).
		Int("total", total).
		Msg(message)
}

// GetJobManager returns the job manager
func (b *BaseExecutor) GetJobManager() *jobs.Manager {
	return b.jobManager
}

// GetQueueManager returns the queue manager
func (b *BaseExecutor) GetQueueManager() *queue.Manager {
	return b.queueMgr
}

// GetLogger returns the base logger
func (b *BaseExecutor) GetLogger() arbor.ILogger {
	return b.logger
}

// GetLogService returns the log service
func (b *BaseExecutor) GetLogService() interfaces.LogService {
	return b.logService
}

// GetWebSocketHandler returns the WebSocket handler
func (b *BaseExecutor) GetWebSocketHandler() interfaces.WebSocketHandler {
	return b.wsHandler
}
