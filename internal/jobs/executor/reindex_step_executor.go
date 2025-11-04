package executor

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
)

// ReindexStepExecutor handles "reindex" action steps
// It rebuilds the FTS5 full-text search index for optimal search performance
type ReindexStepExecutor struct {
	documentStorage interfaces.DocumentStorage
	jobManager      *jobs.Manager
	logger          arbor.ILogger
}

// NewReindexStepExecutor creates a new reindex step executor
func NewReindexStepExecutor(documentStorage interfaces.DocumentStorage, jobManager *jobs.Manager, logger arbor.ILogger) *ReindexStepExecutor {
	return &ReindexStepExecutor{
		documentStorage: documentStorage,
		jobManager:      jobManager,
		logger:          logger,
	}
}

// ExecuteStep executes a reindex step
func (e *ReindexStepExecutor) ExecuteStep(ctx context.Context, step models.JobStep, sources []string, parentJobID string) (string, error) {
	e.logger.Info().
		Str("step_name", step.Name).
		Str("action", step.Action).
		Str("parent_job_id", parentJobID).
		Msg("Starting reindex step")

	// Generate job ID for this step
	jobID := uuid.New().String()

	// Create job record for tracking
	job := &jobs.Job{
		ID:       jobID,
		ParentID: &parentJobID,
		Type:     "reindex",
		Phase:    "core",
		Status:   "running",
	}

	// Save job record
	if err := e.jobManager.CreateJob(ctx, job); err != nil {
		e.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to create reindex job record")
		return "", fmt.Errorf("failed to create job record: %w", err)
	}

	// Check for dry_run configuration
	dryRun := false
	if step.Config != nil {
		if val, ok := step.Config["dry_run"]; ok {
			if boolVal, ok := val.(bool); ok {
				dryRun = boolVal
			}
		}
	}

	if dryRun {
		e.logger.Info().
			Str("job_id", jobID).
			Msg("Dry run mode - skipping actual index rebuild")
		
		// Mark job as completed
		if err := e.jobManager.UpdateJobStatus(ctx, jobID, "completed"); err != nil {
			e.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job status to completed")
		}
	} else {
		// Perform actual index rebuild
		e.logger.Info().
			Str("job_id", jobID).
			Msg("Rebuilding FTS5 full-text search index")

		if err := e.documentStorage.RebuildFTS5Index(); err != nil {
			e.logger.Error().
				Err(err).
				Str("job_id", jobID).
				Msg("Failed to rebuild FTS5 index")

			// Mark job as failed
			if updateErr := e.jobManager.SetJobError(ctx, jobID, err.Error()); updateErr != nil {
				e.logger.Error().Err(updateErr).Str("job_id", jobID).Msg("Failed to set job error")
			}

			return "", fmt.Errorf("failed to rebuild FTS5 index: %w", err)
		}

		e.logger.Info().
			Str("job_id", jobID).
			Msg("FTS5 index rebuilt successfully")

		// Mark job as completed
		if err := e.jobManager.UpdateJobStatus(ctx, jobID, "completed"); err != nil {
			e.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job status to completed")
		}
	}

	e.logger.Info().
		Str("step_name", step.Name).
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Bool("dry_run", dryRun).
		Msg("Reindex step completed successfully")

	return jobID, nil
}

// GetStepType returns the step type this executor handles
func (e *ReindexStepExecutor) GetStepType() string {
	return "reindex"
}
