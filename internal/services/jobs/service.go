// -----------------------------------------------------------------------
// Job Service - High-level service for creating and managing jobs
// -----------------------------------------------------------------------

package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	jobqueue "github.com/ternarybob/quaero/internal/jobs/queue"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// Service provides high-level job management operations
type Service struct {
	jobManager *jobqueue.Manager
	queueMgr   interfaces.QueueManager
	logger     arbor.ILogger
}

// NewService creates a new job service
func NewService(jobManager *jobqueue.Manager, queueMgr interfaces.QueueManager, logger arbor.ILogger) *Service {
	return &Service{
		jobManager: jobManager,
		queueMgr:   queueMgr,
		logger:     logger,
	}
}

// CreateAndEnqueueJob creates a job record and enqueues it for processing
func (s *Service) CreateAndEnqueueJob(ctx context.Context, job *models.QueueJob) error {
	// Validate queue job
	if err := job.Validate(); err != nil {
		return fmt.Errorf("invalid queue job: %w", err)
	}

	s.logger.Info().
		Str("job_id", job.ID).
		Str("job_type", job.Type).
		Str("job_name", job.Name).
		Bool("is_root", job.IsRootJob()).
		Msg("Creating and enqueueing job")

	// Create job record in database
	dbJob := &jobqueue.Job{
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

	// Serialize queue job as payload
	payloadBytes, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize queue job: %w", err)
	}
	dbJob.Payload = string(payloadBytes)

	// Create job record
	if err := s.jobManager.CreateJobRecord(ctx, dbJob); err != nil {
		return fmt.Errorf("failed to create job record: %w", err)
	}

	// Enqueue job
	queueMsg := queue.Message{
		JobID:   job.ID,
		Type:    job.Type,
		Payload: json.RawMessage(payloadBytes),
	}

	if err := s.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	s.logger.Info().
		Str("job_id", job.ID).
		Str("job_type", job.Type).
		Msg("Job created and enqueued successfully")

	return nil
}

// CreateJobFromDefinition creates a queue job from a job definition
func (s *Service) CreateJobFromDefinition(jobDef *models.JobDefinition) (*models.QueueJob, error) {
	// Build config from job definition
	config := make(map[string]interface{})

	// Copy job definition config
	if jobDef.Config != nil {
		for k, v := range jobDef.Config {
			config[k] = v
		}
	}

	// Add source fields
	config["source_type"] = jobDef.SourceType
	config["base_url"] = jobDef.BaseURL
	config["auth_id"] = jobDef.AuthID

	// Add steps
	config["steps"] = jobDef.Steps

	// Build metadata
	metadata := map[string]interface{}{
		"job_definition_id": jobDef.ID,
		"description":       jobDef.Description,
	}

	// Create queue job
	job := models.NewQueueJob(
		string(jobDef.Type),
		jobDef.Name,
		config,
		metadata,
	)

	return job, nil
}

// ExecuteJobDefinition creates and enqueues a job from a job definition
func (s *Service) ExecuteJobDefinition(ctx context.Context, jobDef *models.JobDefinition) (string, error) {
	// Validate job definition
	if err := jobDef.Validate(); err != nil {
		return "", fmt.Errorf("invalid job definition: %w", err)
	}

	// Create job model from definition
	job, err := s.CreateJobFromDefinition(jobDef)
	if err != nil {
		return "", fmt.Errorf("failed to create job model: %w", err)
	}

	// Create and enqueue job
	if err := s.CreateAndEnqueueJob(ctx, job); err != nil {
		return "", fmt.Errorf("failed to create and enqueue job: %w", err)
	}

	return job.ID, nil
}

// GetJobStatus retrieves the current status of a job
func (s *Service) GetJobStatus(ctx context.Context, jobID string) (*jobqueue.Job, error) {
	// Use GetJobInternal to get the internal jobqueue.Job type directly
	return s.jobManager.GetJobInternal(ctx, jobID)
}

// GetJobLogs retrieves logs for a job
func (s *Service) GetJobLogs(ctx context.Context, jobID string, limit int) ([]jobqueue.JobLog, error) {
	return s.jobManager.GetJobLogs(ctx, jobID, limit)
}

// CancelJob cancels a running job
func (s *Service) CancelJob(ctx context.Context, jobID string) error {
	return s.jobManager.UpdateJobStatus(ctx, jobID, "cancelled")
}
