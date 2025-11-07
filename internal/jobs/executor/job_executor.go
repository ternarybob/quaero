package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/jobs/processor"
	"github.com/ternarybob/quaero/internal/models"
)

// JobExecutor orchestrates job definition execution
// It routes steps to appropriate StepExecutors and manages parent-child hierarchy
type JobExecutor struct {
	stepExecutors     map[string]StepExecutor
	jobManager        *jobs.Manager
	parentJobExecutor *processor.ParentJobExecutor
	logger            arbor.ILogger
}

// NewJobExecutor creates a new job executor
func NewJobExecutor(jobManager *jobs.Manager, parentJobExecutor *processor.ParentJobExecutor, logger arbor.ILogger) *JobExecutor {
	return &JobExecutor{
		stepExecutors:     make(map[string]StepExecutor),
		jobManager:        jobManager,
		parentJobExecutor: parentJobExecutor,
		logger:            logger,
	}
}

// RegisterStepExecutor registers a step executor for an action type
func (e *JobExecutor) RegisterStepExecutor(executor StepExecutor) {
	e.stepExecutors[executor.GetStepType()] = executor
	e.logger.Info().
		Str("action_type", executor.GetStepType()).
		Msg("Step executor registered")
}

// Execute executes a job definition sequentially
// Returns the parent job ID for tracking
func (e *JobExecutor) Execute(ctx context.Context, jobDef *models.JobDefinition) (string, error) {
	// Generate parent job ID
	parentJobID := uuid.New().String()

	// Create a logger with correlation ID set to parent job ID
	// This ensures all parent job logs are associated with the parent job ID
	parentLogger := e.logger.WithCorrelationId(parentJobID)

	parentLogger.Info().
		Str("job_def_id", jobDef.ID).
		Str("parent_job_id", parentJobID).
		Str("job_name", jobDef.Name).
		Int("step_count", len(jobDef.Steps)).
		Str("source_type", jobDef.SourceType).
		Str("base_url", jobDef.BaseURL).
		Msg("Starting job definition execution")

	// Create parent job record in database to track overall progress
	// Use old jobs.Job format for now (will be migrated to models.Job later)
	parentJob := &jobs.Job{
		ID:              parentJobID,
		ParentID:        nil,         // This is a root job
		Type:            "parent",    // Always use "parent" type for parent jobs created by JobExecutor
		Name:            jobDef.Name, // Use job definition name
		Phase:           "execution",
		Status:          "pending",
		ProgressCurrent: 0,
		ProgressTotal:   len(jobDef.Steps),
	}

	if err := e.jobManager.CreateJobRecord(ctx, parentJob); err != nil {
		parentLogger.Error().Err(err).
			Str("parent_job_id", parentJobID).
			Str("job_def_id", jobDef.ID).
			Msg("Failed to create parent job record")
		return "", fmt.Errorf("failed to create parent job: %w", err)
	}

	// Add initial job log for debugging
	initialLog := fmt.Sprintf("🚀 Starting job definition execution: %s (ID: %s, Type: '%s', Steps: %d)",
		jobDef.Name, jobDef.ID, string(jobDef.Type), len(jobDef.Steps))
	if err := e.jobManager.AddJobLog(ctx, parentJobID, "info", initialLog); err != nil {
		parentLogger.Warn().Err(err).Msg("Failed to add initial job log")
	}
	parentLogger.Info().Str("job_def_id", jobDef.ID).Str("type", string(jobDef.Type)).Msg("Job definition loaded")

	// Build job definition config for parent job
	jobDefConfig := make(map[string]interface{})

	// Include job definition configuration for display in UI
	if len(jobDef.Steps) > 0 {
		// Merge all step configs into the parent job config for display
		for i, step := range jobDef.Steps {
			stepKey := fmt.Sprintf("step_%d_%s", i+1, step.Action)
			jobDefConfig[stepKey] = step.Config
		}
	}

	// Add job definition metadata
	jobDefConfig["job_definition_id"] = jobDef.ID
	jobDefConfig["source_type"] = jobDef.SourceType
	jobDefConfig["base_url"] = jobDef.BaseURL
	jobDefConfig["schedule"] = jobDef.Schedule
	jobDefConfig["timeout"] = jobDef.Timeout
	jobDefConfig["enabled"] = jobDef.Enabled

	// Update the job config in the database
	if err := e.jobManager.UpdateJobConfig(ctx, parentJobID, jobDefConfig); err != nil {
		parentLogger.Warn().Err(err).
			Str("parent_job_id", parentJobID).
			Msg("Failed to update job config, continuing without config display")
	}

	parentLogger.Info().
		Str("parent_job_id", parentJobID).
		Str("job_def_id", jobDef.ID).
		Int("total_steps", len(jobDef.Steps)).
		Msg("Parent job record created successfully")

	// Mark parent job as running
	if err := e.jobManager.UpdateJobStatus(ctx, parentJobID, "running"); err != nil {
		parentLogger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to update parent job status to running")
	} else {
		parentLogger.Info().Str("parent_job_id", parentJobID).Msg("✓ Parent job status updated to 'running'")
	}

	// Execute pre-jobs if any
	if len(jobDef.PreJobs) > 0 {
		parentLogger.Info().
			Int("pre_job_count", len(jobDef.PreJobs)).
			Msg("Executing pre-jobs (not yet implemented)")
		// TODO: Load and execute pre-job definitions
	}

	// Execute steps sequentially
	for i, step := range jobDef.Steps {
		parentLogger.Info().
			Str("step_name", step.Name).
			Str("action", step.Action).
			Int("step_index", i).
			Int("total_steps", len(jobDef.Steps)).
			Msg("Executing step")

		// Get executor for this step
		executor, exists := e.stepExecutors[step.Action]
		if !exists {
			err := fmt.Errorf("no executor registered for action: %s", step.Action)
			parentLogger.Error().
				Err(err).
				Str("action", step.Action).
				Str("step_name", step.Name).
				Msg("Failed to find executor")

			// Set parent job error
			if setErr := e.jobManager.SetJobError(ctx, parentJobID, err.Error()); setErr != nil {
				parentLogger.Error().Err(setErr).Str("parent_job_id", parentJobID).Msg("Failed to set parent job error")
			}

			// Handle based on error strategy
			if step.OnError == models.ErrorStrategyFail {
				return parentJobID, err
			}
			// Log and continue for "continue" strategy
			parentLogger.Warn().
				Str("step_name", step.Name).
				Msg("Continuing despite missing executor")

			// Check error tolerance after error
			if jobDef.ErrorTolerance != nil {
				shouldStop, tolErr := e.checkErrorTolerance(ctx, parentJobID, jobDef.ErrorTolerance)
				if tolErr != nil {
					parentLogger.Error().Err(tolErr).Msg("Failed to check error tolerance")
				}
				if shouldStop {
					parentLogger.Error().
						Str("parent_job_id", parentJobID).
						Msg("Stopping execution due to error tolerance threshold")
					if err := e.jobManager.UpdateJobStatus(ctx, parentJobID, "failed"); err != nil {
						parentLogger.Warn().Err(err).Msg("Failed to update parent job status")
					}
					return parentJobID, fmt.Errorf("execution stopped: error tolerance threshold exceeded")
				}
			}
			continue
		}

		// Execute step
		childJobID, err := executor.ExecuteStep(ctx, step, jobDef, parentJobID)
		if err != nil {
			parentLogger.Error().
				Err(err).
				Str("step_name", step.Name).
				Str("action", step.Action).
				Msg("Step execution failed")

			// Set parent job error
			if setErr := e.jobManager.SetJobError(ctx, parentJobID, err.Error()); setErr != nil {
				parentLogger.Error().Err(setErr).Str("parent_job_id", parentJobID).Msg("Failed to set parent job error")
			}

			// Handle based on error strategy
			if step.OnError == models.ErrorStrategyFail {
				return parentJobID, fmt.Errorf("step %s failed: %w", step.Name, err)
			}
			// Log and continue for "continue" strategy
			parentLogger.Warn().
				Str("step_name", step.Name).
				Msg("Continuing despite step failure")

			// Check error tolerance after error
			if jobDef.ErrorTolerance != nil {
				shouldStop, tolErr := e.checkErrorTolerance(ctx, parentJobID, jobDef.ErrorTolerance)
				if tolErr != nil {
					parentLogger.Error().Err(tolErr).Msg("Failed to check error tolerance")
				}
				if shouldStop {
					parentLogger.Error().
						Str("parent_job_id", parentJobID).
						Msg("Stopping execution due to error tolerance threshold")
					if err := e.jobManager.UpdateJobStatus(ctx, parentJobID, "failed"); err != nil {
						parentLogger.Warn().Err(err).Msg("Failed to update parent job status")
					}
					return parentJobID, fmt.Errorf("execution stopped: error tolerance threshold exceeded")
				}
			}
			continue
		}

		parentLogger.Info().
			Str("step_name", step.Name).
			Str("child_job_id", childJobID).
			Str("parent_job_id", parentJobID).
			Msg("Step completed successfully")

		// Update progress after successful step
		if err := e.jobManager.UpdateJobProgress(ctx, parentJobID, i+1, len(jobDef.Steps)); err != nil {
			parentLogger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to update parent job progress")
		}

		parentLogger.Debug().
			Str("parent_job_id", parentJobID).
			Int("completed_steps", i+1).
			Int("total_steps", len(jobDef.Steps)).
			Msg("Parent job progress updated")
	}

	// Execute post-jobs if any
	if len(jobDef.PostJobs) > 0 {
		parentLogger.Info().
			Int("post_job_count", len(jobDef.PostJobs)).
			Msg("Executing post-jobs (not yet implemented)")
		// TODO: Load and execute post-job definitions
	}

	// For crawler jobs, don't mark as completed immediately - let ParentJobExecutor handle completion
	// For other job types, mark as completed immediately

	// Add job log for debugging - show exact string comparison
	typeComparison := fmt.Sprintf("Type check: '%s' == '%s' ? %v (len: %d vs %d)",
		string(jobDef.Type), string(models.JobDefinitionTypeCrawler),
		jobDef.Type == models.JobDefinitionTypeCrawler,
		len(string(jobDef.Type)), len(string(models.JobDefinitionTypeCrawler)))
	e.jobManager.AddJobLog(ctx, parentJobID, "info", typeComparison)

	parentLogger.Info().
		Str("job_def_type", string(jobDef.Type)).
		Str("expected_type", string(models.JobDefinitionTypeCrawler)).
		Bool("is_crawler", jobDef.Type == models.JobDefinitionTypeCrawler).
		Int("type_len", len(string(jobDef.Type))).
		Int("expected_len", len(string(models.JobDefinitionTypeCrawler))).
		Msg("Checking job definition type for completion handling")

	// Check if this is a crawler job by looking at the job definition type OR the steps
	isCrawlerJob := jobDef.Type == models.JobDefinitionTypeCrawler

	// Log all step actions for debugging
	stepActions := make([]string, len(jobDef.Steps))
	for i, step := range jobDef.Steps {
		stepActions[i] = step.Action
	}
	e.jobManager.AddJobLog(ctx, parentJobID, "info", fmt.Sprintf("Job has %d steps with actions: %v", len(jobDef.Steps), stepActions))

	// Also check if any step has action "crawl" as a fallback
	if !isCrawlerJob && len(jobDef.Steps) > 0 {
		for _, step := range jobDef.Steps {
			if step.Action == "crawl" {
				isCrawlerJob = true
				e.jobManager.AddJobLog(ctx, parentJobID, "info", "✓ Crawler job detected via step action (type mismatch - please check job definition)")
				break
			}
		}
	}

	if isCrawlerJob {
		// Add job log for UI visibility
		e.jobManager.AddJobLog(ctx, parentJobID, "info", "✓ Crawler job detected - leaving in running state for ParentJobExecutor to monitor child jobs")

		parentLogger.Info().
			Str("parent_job_id", parentJobID).
			Msg("✓ Crawler job detected - starting parent job monitoring in background")

		// Start parent job monitoring in a separate goroutine (NOT via queue)
		// This prevents blocking the queue worker with long-running monitoring loops
		parentJobModel := &models.JobModel{
			ID:        parentJobID,
			ParentID:  nil,
			Type:      "parent",
			Name:      jobDef.Name,
			Config:    jobDefConfig,
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
			Depth:     0,
		}

		// Start monitoring in background goroutine
		e.parentJobExecutor.StartMonitoring(ctx, parentJobModel)

		parentLogger.Info().Msg("✓ Parent job monitoring started in background goroutine")
		e.jobManager.AddJobLog(ctx, parentJobID, "info", "✓ Parent job monitoring started - tracking child job progress")

		// NOTE: Do NOT set finished_at for crawler jobs - ParentJobExecutor will handle this
		// when all children complete
	} else {
		// For non-crawler jobs, mark as completed immediately
		e.jobManager.AddJobLog(ctx, parentJobID, "info", fmt.Sprintf("Non-crawler job (type: %s) - marking as completed immediately", string(jobDef.Type)))

		if err := e.jobManager.UpdateJobStatus(ctx, parentJobID, "completed"); err != nil {
			parentLogger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to mark parent job as completed")
		} else {
			e.jobManager.AddJobLog(ctx, parentJobID, "info", "✓ Job marked as completed")
		}

		// Set finished_at timestamp for non-crawler jobs
		if err := e.jobManager.SetJobFinished(ctx, parentJobID); err != nil {
			parentLogger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to set finished_at timestamp")
		}
	}

	parentLogger.Info().
		Str("job_def_id", jobDef.ID).
		Str("parent_job_id", parentJobID).
		Int("completed_steps", len(jobDef.Steps)).
		Bool("is_crawler", jobDef.Type == models.JobDefinitionTypeCrawler).
		Msg("Job definition execution completed successfully")

	return parentJobID, nil
}

// checkErrorTolerance checks if the error tolerance threshold has been exceeded
// Returns true if execution should stop, false if it should continue
func (e *JobExecutor) checkErrorTolerance(ctx context.Context, parentJobID string, tolerance *models.ErrorTolerance) (bool, error) {
	// If no error tolerance is configured, never stop
	if tolerance == nil {
		return false, nil
	}

	// If MaxChildFailures is 0, unlimited failures are allowed
	if tolerance.MaxChildFailures == 0 {
		e.logger.Debug().
			Str("parent_job_id", parentJobID).
			Msg("Error tolerance: unlimited failures allowed (MaxChildFailures=0)")
		return false, nil
	}

	// Query the number of failed child jobs using Manager method
	failedCount, err := e.jobManager.GetFailedChildCount(ctx, parentJobID)
	if err != nil {
		return false, fmt.Errorf("failed to query failed job count: %w", err)
	}

	e.logger.Debug().
		Str("parent_job_id", parentJobID).
		Int("failed_count", failedCount).
		Int("max_failures", tolerance.MaxChildFailures).
		Str("action", tolerance.FailureAction).
		Msg("Error tolerance check")

	// Check if threshold exceeded
	if failedCount >= tolerance.MaxChildFailures {
		e.logger.Warn().
			Str("parent_job_id", parentJobID).
			Int("failed_count", failedCount).
			Int("max_failures", tolerance.MaxChildFailures).
			Str("action", tolerance.FailureAction).
			Msg("Error tolerance threshold exceeded")

		switch tolerance.FailureAction {
		case "stop_all":
			e.logger.Error().
				Str("parent_job_id", parentJobID).
				Msg("Stopping all execution due to error tolerance threshold")
			return true, nil
		case "mark_warning":
			e.logger.Warn().
				Str("parent_job_id", parentJobID).
				Msg("Error tolerance threshold exceeded, marking as warning but continuing")
			return false, nil
		case "continue":
			e.logger.Info().
				Str("parent_job_id", parentJobID).
				Msg("Error tolerance threshold exceeded, but continuing execution")
			return false, nil
		default:
			e.logger.Warn().
				Str("parent_job_id", parentJobID).
				Str("unknown_action", tolerance.FailureAction).
				Msg("Unknown failure action, defaulting to continue")
			return false, nil
		}
	}

	return false, nil
}
