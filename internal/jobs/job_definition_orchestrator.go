package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// JobManager interface is defined locally to avoid import cycle with manager package.
// Canonical interface: internal/jobs/manager/interfaces.go
// Manager implementations automatically satisfy this interface via duck typing.
type JobManager interface {
	CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (jobID string, err error)
	GetManagerType() string
}

// ParentJobOrchestrator interface is defined locally to avoid import cycle with orchestrator package.
// Canonical interface: internal/jobs/orchestrator/interfaces.go
// Orchestrator implementations automatically satisfy this interface via duck typing.
type ParentJobOrchestrator interface {
	StartMonitoring(ctx context.Context, job *models.JobModel)
	SubscribeToChildStatusChanges()
}

// JobDefinitionOrchestrator orchestrates job definition execution by routing steps to appropriate JobManagers and managing parent-child hierarchy
type JobDefinitionOrchestrator struct {
	stepExecutors         map[string]JobManager         // Job managers keyed by action type
	jobManager            *Manager
	parentJobOrchestrator ParentJobOrchestrator
	logger                arbor.ILogger
}

// NewJobDefinitionOrchestrator creates a new job definition orchestrator for routing job definition steps to managers
func NewJobDefinitionOrchestrator(jobManager *Manager, parentJobOrchestrator ParentJobOrchestrator, logger arbor.ILogger) *JobDefinitionOrchestrator {
	return &JobDefinitionOrchestrator{
		stepExecutors:         make(map[string]JobManager), // Initialize job manager map
		jobManager:            jobManager,
		parentJobOrchestrator: parentJobOrchestrator,
		logger:                logger,
	}
}

// RegisterStepExecutor registers a job manager for an action type
func (o *JobDefinitionOrchestrator) RegisterStepExecutor(mgr JobManager) {
	o.stepExecutors[mgr.GetManagerType()] = mgr
	o.logger.Info().
		Str("action_type", mgr.GetManagerType()).
		Msg("Job manager registered")
}

// Execute executes a job definition sequentially
// Returns the parent job ID for tracking
func (o *JobDefinitionOrchestrator) Execute(ctx context.Context, jobDef *models.JobDefinition) (string, error) {
	// Generate parent job ID
	parentJobID := uuid.New().String()

	// Create a logger with correlation ID set to parent job ID
	// This ensures all parent job logs are associated with the parent job ID
	parentLogger := o.logger.WithCorrelationId(parentJobID)

	parentLogger.Info().
		Str("job_def_id", jobDef.ID).
		Str("parent_job_id", parentJobID).
		Str("job_name", jobDef.Name).
		Int("step_count", len(jobDef.Steps)).
		Str("source_type", jobDef.SourceType).
		Str("base_url", jobDef.BaseURL).
		Msg("Starting job definition execution")

	// Create parent job record in database to track overall progress
	// Use old Job format for now (will be migrated to models.Job later)
	parentJob := &Job{
		ID:              parentJobID,
		ParentID:        nil,         // This is a root job
		Type:            "parent",    // Always use "parent" type for parent jobs created by JobDefinitionOrchestrator
		Name:            jobDef.Name, // Use job definition name
		Phase:           "execution",
		Status:          "pending",
		ProgressCurrent: 0,
		ProgressTotal:   len(jobDef.Steps),
	}

	if err := o.jobManager.CreateJobRecord(ctx, parentJob); err != nil {
		parentLogger.Error().Err(err).
			Str("parent_job_id", parentJobID).
			Str("job_def_id", jobDef.ID).
			Msg("Failed to create parent job record")
		return "", fmt.Errorf("failed to create parent job: %w", err)
	}

	// Persist metadata immediately after job creation to avoid race with child jobs
	// This ensures auth_id and job_definition_id are available when crawler jobs start
	parentMetadata := make(map[string]interface{})

	// Include auth_id in metadata if present (required for cookie injection)
	if jobDef.AuthID != "" {
		parentMetadata["auth_id"] = jobDef.AuthID
	}
	// Include job_definition_id in metadata as fallback
	if jobDef.ID != "" {
		parentMetadata["job_definition_id"] = jobDef.ID
	}
	// Include phase in metadata
	parentMetadata["phase"] = "execution"

	// Persist metadata to database so child jobs can retrieve it
	if err := o.jobManager.UpdateJobMetadata(ctx, parentJobID, parentMetadata); err != nil {
		parentLogger.Warn().
			Err(err).
			Str("parent_job_id", parentJobID).
			Msg("Failed to update job metadata, auth may not work for child jobs")
	} else {
		parentLogger.Debug().
			Str("parent_job_id", parentJobID).
			Int("metadata_keys", len(parentMetadata)).
			Msg("Job metadata persisted to database")
	}

	// Add initial job log for debugging
	initialLog := fmt.Sprintf("🚀 Starting job definition execution: %s (ID: %s, Type: '%s', Steps: %d)",
		jobDef.Name, jobDef.ID, string(jobDef.Type), len(jobDef.Steps))
	if err := o.jobManager.AddJobLog(ctx, parentJobID, "info", initialLog); err != nil {
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

	// Include auth_id if present (required for cookie injection)
	if jobDef.AuthID != "" {
		jobDefConfig["auth_id"] = jobDef.AuthID
		parentLogger.Debug().
			Str("auth_id", jobDef.AuthID).
			Msg("Auth ID included in job config for cookie injection")
	}

	// Update the job config in the database
	if err := o.jobManager.UpdateJobConfig(ctx, parentJobID, jobDefConfig); err != nil {
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
	if err := o.jobManager.UpdateJobStatus(ctx, parentJobID, "running"); err != nil {
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

		// Get manager for this step
		mgr, exists := o.stepExecutors[step.Action]
		if !exists {
			err := fmt.Errorf("no manager registered for action: %s", step.Action)
			parentLogger.Error().
				Err(err).
				Str("action", step.Action).
				Str("step_name", step.Name).
				Msg("Failed to find manager")

			// Set parent job error
			if setErr := o.jobManager.SetJobError(ctx, parentJobID, err.Error()); setErr != nil {
				parentLogger.Error().Err(setErr).Str("parent_job_id", parentJobID).Msg("Failed to set parent job error")
			}

			// Handle based on error strategy
			if step.OnError == models.ErrorStrategyFail {
				return parentJobID, err
			}
			// Log and continue for "continue" strategy
			parentLogger.Warn().
				Str("step_name", step.Name).
				Msg("Continuing despite missing manager")

			// Check error tolerance after error
			if jobDef.ErrorTolerance != nil {
				shouldStop, tolErr := o.checkErrorTolerance(ctx, parentJobID, jobDef.ErrorTolerance)
				if tolErr != nil {
					parentLogger.Error().Err(tolErr).Msg("Failed to check error tolerance")
				}
				if shouldStop {
					parentLogger.Error().
						Str("parent_job_id", parentJobID).
						Msg("Stopping execution due to error tolerance threshold")
					if err := o.jobManager.UpdateJobStatus(ctx, parentJobID, "failed"); err != nil {
						parentLogger.Warn().Err(err).Msg("Failed to update parent job status")
					}
					return parentJobID, fmt.Errorf("execution stopped: error tolerance threshold exceeded")
				}
			}
			continue
		}

		// Execute step via manager (creates parent job and orchestrates children)
		childJobID, err := mgr.CreateParentJob(ctx, step, jobDef, parentJobID)
		if err != nil {
			parentLogger.Error().
				Err(err).
				Str("step_name", step.Name).
				Str("action", step.Action).
				Msg("Step execution failed")

			// Set parent job error
			if setErr := o.jobManager.SetJobError(ctx, parentJobID, err.Error()); setErr != nil {
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
				shouldStop, tolErr := o.checkErrorTolerance(ctx, parentJobID, jobDef.ErrorTolerance)
				if tolErr != nil {
					parentLogger.Error().Err(tolErr).Msg("Failed to check error tolerance")
				}
				if shouldStop {
					parentLogger.Error().
						Str("parent_job_id", parentJobID).
						Msg("Stopping execution due to error tolerance threshold")
					if err := o.jobManager.UpdateJobStatus(ctx, parentJobID, "failed"); err != nil {
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
		if err := o.jobManager.UpdateJobProgress(ctx, parentJobID, i+1, len(jobDef.Steps)); err != nil {
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

	// For crawler jobs, don't mark as completed immediately - let ParentJobOrchestrator handle completion
	// For other job types, mark as completed immediately

	// Add job log for debugging - show exact string comparison
	typeComparison := fmt.Sprintf("Type check: '%s' == '%s' ? %v (len: %d vs %d)",
		string(jobDef.Type), string(models.JobDefinitionTypeCrawler),
		jobDef.Type == models.JobDefinitionTypeCrawler,
		len(string(jobDef.Type)), len(string(models.JobDefinitionTypeCrawler)))
	o.jobManager.AddJobLog(ctx, parentJobID, "info", typeComparison)

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
	o.jobManager.AddJobLog(ctx, parentJobID, "info", fmt.Sprintf("Job has %d steps with actions: %v", len(jobDef.Steps), stepActions))

	// Also check if any step has action "crawl" as a fallback
	if !isCrawlerJob && len(jobDef.Steps) > 0 {
		for _, step := range jobDef.Steps {
			if step.Action == "crawl" {
				isCrawlerJob = true
				o.jobManager.AddJobLog(ctx, parentJobID, "info", "✓ Crawler job detected via step action (type mismatch - please check job definition)")
				break
			}
		}
	}

	if isCrawlerJob {
		// Add job log for UI visibility
		o.jobManager.AddJobLog(ctx, parentJobID, "info", "✓ Crawler job detected - leaving in running state for ParentJobOrchestrator to monitor child jobs")

		parentLogger.Info().
			Str("parent_job_id", parentJobID).
			Msg("✓ Crawler job detected - starting parent job monitoring in background")

		// Start parent job monitoring in a separate goroutine (NOT via queue)
		// This prevents blocking the queue worker with long-running monitoring loops
		// Note: parentMetadata was already persisted earlier (right after CreateJobRecord)
		// We reconstruct it here for the in-memory parentJobModel only
		parentMetadata := make(map[string]interface{})

		// Include auth_id in metadata if present (required for cookie injection)
		if jobDef.AuthID != "" {
			parentMetadata["auth_id"] = jobDef.AuthID
		}
		// Include job_definition_id in metadata as fallback
		if jobDef.ID != "" {
			parentMetadata["job_definition_id"] = jobDef.ID
		}
		// Include phase in metadata
		parentMetadata["phase"] = "execution"

		parentJobModel := &models.JobModel{
			ID:        parentJobID,
			ParentID:  nil,
			Type:      "parent",
			Name:      jobDef.Name,
			Config:    jobDefConfig,
			Metadata:  parentMetadata,
			CreatedAt: time.Now(),
			Depth:     0,
		}

		// Start monitoring in background goroutine
		o.parentJobOrchestrator.StartMonitoring(ctx, parentJobModel)

		parentLogger.Info().Msg("✓ Parent job monitoring started in background goroutine")
		o.jobManager.AddJobLog(ctx, parentJobID, "info", "✓ Parent job monitoring started - tracking child job progress")

		// NOTE: Do NOT set finished_at for crawler jobs - ParentJobOrchestrator will handle this
		// when all children complete
	} else {
		// For non-crawler jobs, mark as completed immediately
		o.jobManager.AddJobLog(ctx, parentJobID, "info", fmt.Sprintf("Non-crawler job (type: %s) - marking as completed immediately", string(jobDef.Type)))

		if err := o.jobManager.UpdateJobStatus(ctx, parentJobID, "completed"); err != nil {
			parentLogger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to mark parent job as completed")
		} else {
			o.jobManager.AddJobLog(ctx, parentJobID, "info", "✓ Job marked as completed")
		}

		// Set finished_at timestamp for non-crawler jobs
		if err := o.jobManager.SetJobFinished(ctx, parentJobID); err != nil {
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
func (o *JobDefinitionOrchestrator) checkErrorTolerance(ctx context.Context, parentJobID string, tolerance *models.ErrorTolerance) (bool, error) {
	// If no error tolerance is configured, never stop
	if tolerance == nil {
		return false, nil
	}

	// If MaxChildFailures is 0, unlimited failures are allowed
	if tolerance.MaxChildFailures == 0 {
		o.logger.Debug().
			Str("parent_job_id", parentJobID).
			Msg("Error tolerance: unlimited failures allowed (MaxChildFailures=0)")
		return false, nil
	}

	// Query the number of failed child jobs using Manager method
	failedCount, err := o.jobManager.GetFailedChildCount(ctx, parentJobID)
	if err != nil {
		return false, fmt.Errorf("failed to query failed job count: %w", err)
	}

	o.logger.Debug().
		Str("parent_job_id", parentJobID).
		Int("failed_count", failedCount).
		Int("max_failures", tolerance.MaxChildFailures).
		Str("action", tolerance.FailureAction).
		Msg("Error tolerance check")

	// Check if threshold exceeded
	if failedCount >= tolerance.MaxChildFailures {
		o.logger.Warn().
			Str("parent_job_id", parentJobID).
			Int("failed_count", failedCount).
			Int("max_failures", tolerance.MaxChildFailures).
			Str("action", tolerance.FailureAction).
			Msg("Error tolerance threshold exceeded")

		switch tolerance.FailureAction {
		case "stop_all":
			o.logger.Error().
				Str("parent_job_id", parentJobID).
				Msg("Stopping all execution due to error tolerance threshold")
			return true, nil
		case "mark_warning":
			o.logger.Warn().
				Str("parent_job_id", parentJobID).
				Msg("Error tolerance threshold exceeded, marking as warning but continuing")
			return false, nil
		case "continue":
			o.logger.Info().
				Str("parent_job_id", parentJobID).
				Msg("Error tolerance threshold exceeded, but continuing execution")
			return false, nil
		default:
			o.logger.Warn().
				Str("parent_job_id", parentJobID).
				Str("unknown_action", tolerance.FailureAction).
				Msg("Unknown failure action, defaulting to continue")
			return false, nil
		}
	}

	return false, nil
}
