package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
)

// ParentJobExecutor executes parent jobs (job orchestration and progress tracking)
// This executor manages the lifecycle of parent jobs and aggregates child job progress
// NOTE: Parent jobs are NOT processed via the queue - they run in separate goroutines
// to avoid blocking queue workers with long-running monitoring loops.
type ParentJobExecutor struct {
	jobMgr       *jobs.Manager
	eventService interfaces.EventService
	logger       arbor.ILogger
}

// NewParentJobExecutor creates a new parent job executor
func NewParentJobExecutor(
	jobMgr *jobs.Manager,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *ParentJobExecutor {
	return &ParentJobExecutor{
		jobMgr:       jobMgr,
		eventService: eventService,
		logger:       logger,
	}
}

// StartMonitoring starts monitoring a parent job in a separate goroutine.
// This is the primary entry point for parent job execution - NOT via queue.
// Returns immediately after starting the goroutine.
func (e *ParentJobExecutor) StartMonitoring(ctx context.Context, job *models.JobModel) {
	// Validate job before starting
	if err := e.validate(job); err != nil {
		e.logger.Error().
			Err(err).
			Str("job_id", job.ID).
			Msg("Invalid parent job model - cannot start monitoring")

		// Update job status to failed
		e.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Invalid job model: %v", err))
		e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return
	}

	// Start monitoring in a separate goroutine
	go func() {
		if err := e.monitorChildJobs(ctx, job); err != nil {
			e.logger.Error().
				Err(err).
				Str("job_id", job.ID).
				Msg("Parent job monitoring failed")
		}
	}()

	e.logger.Info().
		Str("job_id", job.ID).
		Msg("Parent job monitoring started in background goroutine")
}

// validate validates that the job model is compatible with this executor
func (e *ParentJobExecutor) validate(job *models.JobModel) error {
	if job.Type != string(models.JobTypeParent) {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeParent, job.Type)
	}

	// Validate required config fields
	if _, ok := job.Config["source_type"]; !ok {
		return fmt.Errorf("missing required config field: source_type")
	}

	// entity_type is optional (not required for generic web crawlers)

	return nil
}

// monitorChildJobs monitors child job progress and updates parent job status.
// This runs in a separate goroutine and blocks until all children complete or timeout.
func (e *ParentJobExecutor) monitorChildJobs(ctx context.Context, job *models.JobModel) error {
	// Create job-specific logger for consistent logging
	jobLogger := e.logger.WithCorrelationId(job.ID)

	// Extract configuration
	sourceType, _ := job.GetConfigString("source_type")
	entityType, _ := job.GetConfigString("entity_type")

	// Build log message based on available fields
	var logMsg string
	if entityType != "" {
		logMsg = fmt.Sprintf("Starting parent job for %s %s", sourceType, entityType)
	} else {
		logMsg = fmt.Sprintf("Starting parent job for %s", sourceType)
	}

	jobLogger.Info().
		Str("job_id", job.ID).
		Str("source_type", sourceType).
		Str("entity_type", entityType).
		Msg("Starting parent job execution")

	// Update job status to running
	if err := e.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Add job log for execution start
	e.jobMgr.AddJobLog(ctx, job.ID, "info", logMsg)

	// Publish initial progress update
	e.publishParentJobProgress(ctx, job, "running", "Monitoring child job progress")

	// Monitor child jobs until completion
	// The parent job's role is to:
	// 1. Track overall progress by aggregating child job status
	// 2. Determine when the crawl is complete
	// 3. Update its own status based on child job outcomes

	// Start monitoring loop
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	maxWaitTime := 30 * time.Minute // Maximum time to wait for child jobs
	timeout := time.After(maxWaitTime)

	for {
		select {
		case <-ctx.Done():
			jobLogger.Info().Msg("Parent job execution cancelled")
			e.jobMgr.UpdateJobStatus(ctx, job.ID, "cancelled")
			return ctx.Err()

		case <-timeout:
			jobLogger.Warn().Msg("Parent job timed out waiting for child jobs")
			e.jobMgr.SetJobError(ctx, job.ID, "Timed out waiting for child jobs to complete")
			e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
			return fmt.Errorf("parent job timed out")

		case <-ticker.C:
			// Check child job progress
			completed, err := e.checkChildJobProgress(ctx, job.ID, jobLogger)
			if err != nil {
				jobLogger.Error().Err(err).Msg("Failed to check child job progress")
				continue
			}

			if completed {
				// All child jobs are complete
				jobLogger.Info().Msg("All child jobs completed, finishing parent job")

				// Update job status to completed
				if err := e.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
					return fmt.Errorf("failed to update job status: %w", err)
				}

				// Add final job log
				e.jobMgr.AddJobLog(ctx, job.ID, "info", "Parent job completed successfully")

				// Publish completion event
				e.publishParentJobProgress(ctx, job, "completed", "All child jobs completed")

				jobLogger.Info().Str("job_id", job.ID).Msg("Parent job execution completed successfully")
				return nil
			}
		}
	}
}

// checkChildJobProgress checks the progress of child jobs and returns true if all are complete
func (e *ParentJobExecutor) checkChildJobProgress(ctx context.Context, parentJobID string, logger arbor.ILogger) (bool, error) {
	// Get child job statistics
	childStats, err := e.jobMgr.GetChildJobStats(ctx, parentJobID)
	if err != nil {
		return false, fmt.Errorf("failed to get child job stats: %w", err)
	}

	// Log current progress
	logger.Debug().
		Int("total_children", childStats.TotalChildren).
		Int("completed_children", childStats.CompletedChildren).
		Int("failed_children", childStats.FailedChildren).
		Int("cancelled_children", childStats.CancelledChildren).
		Int("running_children", childStats.RunningChildren).
		Int("pending_children", childStats.PendingChildren).
		Msg("Child job progress check")

	// Update parent job progress
	terminalChildren := childStats.CompletedChildren + childStats.FailedChildren + childStats.CancelledChildren
	progressText := fmt.Sprintf("%d of %d URLs processed (completed: %d, failed: %d, cancelled: %d)",
		terminalChildren,
		childStats.TotalChildren,
		childStats.CompletedChildren,
		childStats.FailedChildren,
		childStats.CancelledChildren)

	// Add job log with progress update
	e.jobMgr.AddJobLog(ctx, parentJobID, "info", progressText)

	// Publish progress update
	e.publishChildJobStats(ctx, parentJobID, childStats, progressText)

	// Check if all child jobs are in terminal state (completed, failed, or cancelled)
	// This ensures we wait until all children (including grandchildren) are done
	// and no more children are being spawned
	if childStats.TotalChildren > 0 {
		return terminalChildren >= childStats.TotalChildren, nil
	}

	// If no child jobs exist yet, keep waiting
	return false, nil
}

// publishParentJobProgress publishes a parent job progress update for real-time monitoring
func (e *ParentJobExecutor) publishParentJobProgress(ctx context.Context, job *models.JobModel, status, activity string) {
	if e.eventService == nil {
		return
	}

	// Get source information
	sourceType, _ := job.GetConfigString("source_type")
	entityType, _ := job.GetConfigString("entity_type")

	payload := map[string]interface{}{
		"job_id":           job.ID,
		"parent_id":        job.ID, // Parent job is its own parent for UI purposes
		"status":           status,
		"job_type":         job.Type,
		"current_activity": activity,
		"timestamp":        time.Now().Format(time.RFC3339),
		"source_type":      sourceType,
		"entity_type":      entityType,
	}

	event := interfaces.Event{
		Type:    "parent_job_progress",
		Payload: payload,
	}

	// Publish asynchronously to avoid blocking job execution
	go func() {
		if err := e.eventService.Publish(ctx, event); err != nil {
			e.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to publish parent job progress event")
		}
	}()
}

// publishChildJobStats publishes child job statistics for real-time monitoring
func (e *ParentJobExecutor) publishChildJobStats(ctx context.Context, parentJobID string, stats *jobs.ChildJobStats, progressText string) {
	if e.eventService == nil {
		return
	}

	payload := map[string]interface{}{
		"job_id":             parentJobID,
		"total_children":     stats.TotalChildren,
		"completed_children": stats.CompletedChildren,
		"failed_children":    stats.FailedChildren,
		"cancelled_children": stats.CancelledChildren,
		"running_children":   stats.RunningChildren,
		"pending_children":   stats.PendingChildren,
		"progress_text":      progressText,
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	event := interfaces.Event{
		Type:    "child_job_stats",
		Payload: payload,
	}

	// Publish asynchronously
	go func() {
		if err := e.eventService.Publish(ctx, event); err != nil {
			e.logger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to publish child job stats event")
		}
	}()
}
