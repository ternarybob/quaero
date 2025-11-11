package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
)

// jobOrchestrator monitors parent job progress and aggregates child job statistics.
// It runs in background goroutines (not via queue) and publishes real-time progress events.
// NOTE: Parent jobs are NOT processed via the queue - they run in separate goroutines
// to avoid blocking queue workers with long-running monitoring loops.
type jobOrchestrator struct {
	jobMgr       *jobs.Manager
	eventService interfaces.EventService
	logger       arbor.ILogger
}

// NewJobOrchestrator creates a new parent job orchestrator for monitoring parent job lifecycle and aggregating child job progress
func NewJobOrchestrator(
	jobMgr *jobs.Manager,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) interfaces.JobOrchestrator {
	orchestrator := &jobOrchestrator{
		jobMgr:       jobMgr,
		eventService: eventService,
		logger:       logger,
	}

	// Subscribe to child job status changes for real-time progress tracking
	orchestrator.SubscribeToChildStatusChanges()

	return orchestrator
}

// StartMonitoring starts monitoring a parent job in a separate goroutine.
// This is the primary entry point for parent job orchestration - NOT via queue.
// Returns immediately after starting the goroutine.
func (o *jobOrchestrator) StartMonitoring(ctx context.Context, job *models.JobModel) {
	// Validate job before starting
	if err := o.validate(job); err != nil {
		o.logger.Error().
			Err(err).
			Str("job_id", job.ID).
			Msg("Invalid parent job model - cannot start monitoring")

		// Update job status to failed
		o.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Invalid job model: %v", err))
		o.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		// Set finished_at timestamp for failed parent jobs
		if err := o.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
			o.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to set finished_at timestamp")
		}
		return
	}

	// Start monitoring in a separate goroutine
	go func() {
		if err := o.monitorChildJobs(ctx, job); err != nil {
			o.logger.Error().
				Err(err).
				Str("job_id", job.ID).
				Msg("Parent job monitoring failed")
		}
	}()

	o.logger.Info().
		Str("job_id", job.ID).
		Msg("Parent job monitoring started in background goroutine")
}

// validate validates that the job model is compatible with this orchestrator
func (o *jobOrchestrator) validate(job *models.JobModel) error {
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
func (o *jobOrchestrator) monitorChildJobs(ctx context.Context, job *models.JobModel) error {
	// Create job-specific logger for consistent logging
	jobLogger := o.logger.WithCorrelationId(job.ID)

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
	if err := o.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Add job log for execution start
	o.jobMgr.AddJobLog(ctx, job.ID, "info", logMsg)

	// Publish initial progress update
	o.publishParentJobProgress(ctx, job, "running", "Monitoring child job progress")

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
			o.jobMgr.UpdateJobStatus(ctx, job.ID, "cancelled")
			// Set finished_at timestamp for cancelled parent jobs
			if err := o.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
				jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
			}
			return ctx.Err()

		case <-timeout:
			jobLogger.Warn().Msg("Parent job timed out waiting for child jobs")
			o.jobMgr.SetJobError(ctx, job.ID, "Timed out waiting for child jobs to complete")
			o.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
			// Set finished_at timestamp for failed parent jobs
			if err := o.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
				jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
			}
			return fmt.Errorf("parent job timed out")

		case <-ticker.C:
			// Check child job progress
			completed, err := o.checkChildJobProgress(ctx, job.ID, jobLogger)
			if err != nil {
				jobLogger.Error().Err(err).Msg("Failed to check child job progress")
				continue
			}

			if completed {
				// All child jobs are complete
				jobLogger.Info().Msg("All child jobs completed, finishing parent job")

				// Update job status to completed
				if err := o.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
					return fmt.Errorf("failed to update job status: %w", err)
				}

				// Set finished_at timestamp for completed parent jobs
				if err := o.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
				}

				// Add final job log
				o.jobMgr.AddJobLog(ctx, job.ID, "info", "Parent job completed successfully")

				// Publish completion event
				o.publishParentJobProgress(ctx, job, "completed", "All child jobs completed")

				jobLogger.Info().Str("job_id", job.ID).Msg("Parent job execution completed successfully")
				return nil
			}
		}
	}
}

// checkChildJobProgress checks the progress of child jobs and returns true if all are complete
func (o *jobOrchestrator) checkChildJobProgress(ctx context.Context, parentJobID string, logger arbor.ILogger) (bool, error) {
	// Get child job statistics
	childStats, err := o.jobMgr.GetChildJobStats(ctx, parentJobID)
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
	o.jobMgr.AddJobLog(ctx, parentJobID, "info", progressText)

	// Publish progress update
	o.publishChildJobStats(ctx, parentJobID, childStats, progressText)

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
func (o *jobOrchestrator) publishParentJobProgress(ctx context.Context, job *models.JobModel, status, activity string) {
	if o.eventService == nil {
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
		if err := o.eventService.Publish(ctx, event); err != nil {
			o.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to publish parent job progress event")
		}
	}()
}

// publishChildJobStats publishes child job statistics for real-time monitoring
func (o *jobOrchestrator) publishChildJobStats(ctx context.Context, parentJobID string, stats *jobs.ChildJobStats, progressText string) {
	if o.eventService == nil {
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
		if err := o.eventService.Publish(ctx, event); err != nil {
			o.logger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to publish child job stats event")
		}
	}()
}

// SubscribeToChildStatusChanges subscribes to child job status change events
// This enables real-time progress tracking without polling
func (o *jobOrchestrator) SubscribeToChildStatusChanges() {
	if o.eventService == nil {
		return
	}

	// Subscribe to all job status changes
	if err := o.eventService.Subscribe(interfaces.EventJobStatusChange, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			o.logger.Warn().Msg("Invalid job_status_change payload type")
			return nil
		}

		// Extract event data
		jobID := getStringFromPayload(payload, "job_id")
		parentID := getStringFromPayload(payload, "parent_id")
		status := getStringFromPayload(payload, "status")
		jobType := getStringFromPayload(payload, "job_type")

		// Only process child job status changes
		if parentID == "" {
			return nil // Not a child job, ignore
		}

		// Log the status change
		o.logger.Debug().
			Str("job_id", jobID).
			Str("parent_id", parentID).
			Str("status", status).
			Str("job_type", jobType).
			Msg("Child job status changed")

		// Get fresh child job stats for the parent
		stats, err := o.jobMgr.GetChildJobStats(ctx, parentID)
		if err != nil {
			o.logger.Error().Err(err).
				Str("parent_id", parentID).
				Msg("Failed to get child job stats after status change")
			return nil // Don't fail the event handler
		}

		// Generate progress text in required format
		progressText := o.formatProgressText(stats)

		// Add job log for parent job
		o.jobMgr.AddJobLog(ctx, parentID, "info",
			fmt.Sprintf("Child job %s → %s. %s",
				jobID[:8], // Short job ID for readability
				status,
				progressText))

		// Publish parent job progress update
		o.publishParentJobProgressUpdate(ctx, parentID, stats, progressText)

		return nil
	}); err != nil {
		o.logger.Error().Err(err).Msg("Failed to subscribe to EventJobStatusChange")
		return
	}

	// Subscribe to document_saved events for real-time document count tracking
	if err := o.eventService.Subscribe(interfaces.EventDocumentSaved, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			o.logger.Warn().Msg("Invalid document_saved payload type")
			return nil
		}

		// Extract parent job ID from payload
		parentJobID := getStringFromPayload(payload, "parent_job_id")
		if parentJobID == "" {
			return nil // No parent job, ignore
		}

		// Extract additional fields for logging
		documentID := getStringFromPayload(payload, "document_id")
		jobID := getStringFromPayload(payload, "job_id")

		// Increment document count in parent job metadata (async operation)
		go func() {
			if err := o.jobMgr.IncrementDocumentCount(context.Background(), parentJobID); err != nil {
				o.logger.Error().Err(err).
					Str("parent_job_id", parentJobID).
					Str("document_id", documentID).
					Str("job_id", jobID).
					Msg("Failed to increment document count for parent job")
				return
			}

			o.logger.Debug().
				Str("parent_job_id", parentJobID).
				Str("document_id", documentID).
				Str("job_id", jobID).
				Msg("Incremented document count for parent job")
		}()

		return nil
	}); err != nil {
		o.logger.Error().Err(err).Msg("Failed to subscribe to EventDocumentSaved")
		return
	}

	o.logger.Info().Msg("JobOrchestrator subscribed to child job status changes and document_saved events")
}

// formatProgressText generates the required progress format
// Example: "66 pending, 1 running, 41 completed, 0 failed"
func (o *jobOrchestrator) formatProgressText(stats *jobs.ChildJobStats) string {
	return fmt.Sprintf("%d pending, %d running, %d completed, %d failed",
		stats.PendingChildren,
		stats.RunningChildren,
		stats.CompletedChildren,
		stats.FailedChildren)
}

// publishParentJobProgressUpdate publishes progress update for WebSocket consumption
func (o *jobOrchestrator) publishParentJobProgressUpdate(
	ctx context.Context,
	parentJobID string,
	stats *jobs.ChildJobStats,
	progressText string) {

	if o.eventService == nil {
		return
	}

	// Calculate overall status based on child states
	overallStatus := o.calculateOverallStatus(stats)

	// Get document count from job metadata (default to 0 if error)
	documentCount, err := o.jobMgr.GetDocumentCount(ctx, parentJobID)
	if err != nil {
		// Log error but don't fail - just use default count of 0
		o.logger.Debug().Err(err).
			Str("parent_job_id", parentJobID).
			Msg("Failed to retrieve document count from metadata, using default 0")
		documentCount = 0
	}

	payload := map[string]interface{}{
		"job_id":             parentJobID,
		"status":             overallStatus,
		"total_children":     stats.TotalChildren,
		"pending_children":   stats.PendingChildren,
		"running_children":   stats.RunningChildren,
		"completed_children": stats.CompletedChildren,
		"failed_children":    stats.FailedChildren,
		"cancelled_children": stats.CancelledChildren,
		"progress_text":      progressText,  // "X pending, Y running, Z completed, W failed"
		"document_count":     documentCount, // Real-time document count from metadata
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	event := interfaces.Event{
		Type:    "parent_job_progress",
		Payload: payload,
	}

	// Publish asynchronously
	go func() {
		if err := o.eventService.Publish(ctx, event); err != nil {
			o.logger.Warn().Err(err).
				Str("parent_job_id", parentJobID).
				Msg("Failed to publish parent job progress event")
		}
	}()
}

// calculateOverallStatus determines parent job status from child statistics
func (o *jobOrchestrator) calculateOverallStatus(stats *jobs.ChildJobStats) string {
	// If no children yet, status is determined by parent job state (handled elsewhere)
	if stats.TotalChildren == 0 {
		return "running" // Waiting for children to spawn
	}

	// If any children are running, parent is "Running"
	if stats.RunningChildren > 0 {
		return "running"
	}

	// If any children are pending, parent is "Running" (still orchestrating)
	if stats.PendingChildren > 0 {
		return "running"
	}

	// All children in terminal state
	terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
	if terminalCount >= stats.TotalChildren {
		// All children complete - determine success/failure
		if stats.FailedChildren > 0 {
			return "failed" // At least one child failed
		}
		if stats.CancelledChildren == stats.TotalChildren {
			return "cancelled" // All children cancelled
		}
		return "completed" // All children succeeded
	}

	return "running" // Default state
}

// Helper function to safely extract string from payload
func getStringFromPayload(payload map[string]interface{}, key string) string {
	if val, ok := payload[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
