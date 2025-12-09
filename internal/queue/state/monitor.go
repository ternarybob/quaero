package state

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// jobMonitor monitors parent job progress and aggregates child job statistics.
// It runs in background goroutines (not via queue) and publishes real-time progress events.
// NOTE: Parent jobs are NOT processed via the queue - they run in separate goroutines
// to avoid blocking queue workers with long-running monitoring loops.
type jobMonitor struct {
	jobMgr       *queue.Manager
	eventService interfaces.EventService
	logger       arbor.ILogger
}

// NewJobMonitor creates a new job monitor for monitoring parent job lifecycle and aggregating child job progress
func NewJobMonitor(
	jobMgr *queue.Manager,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) interfaces.JobMonitor {
	monitor := &jobMonitor{
		jobMgr:       jobMgr,
		eventService: eventService,
		logger:       logger,
	}

	// Subscribe to job events for real-time progress tracking
	monitor.SubscribeToJobEvents()

	return monitor
}

// StartMonitoring starts monitoring a parent job in a separate goroutine.
// This is the primary entry point for parent job monitoring - NOT via queue.
// Returns immediately after starting the goroutine.
func (m *jobMonitor) StartMonitoring(ctx context.Context, job *models.QueueJob) {
	// Validate job before starting
	if err := m.validate(job); err != nil {
		m.logger.Error().
			Err(err).
			Str("job_id", job.ID).
			Msg("Invalid parent queue job - cannot start monitoring")

		// Update job status to failed
		m.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Invalid job: %v", err))
		m.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		// Set finished_at timestamp for failed parent jobs
		if err := m.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
			m.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to set finished_at timestamp")
		}
		return
	}

	// Start monitoring in a separate goroutine with panic recovery
	go func() {
		// CRITICAL: Panic recovery to capture fatal crashes in monitoring goroutine
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				// Log to structured logger first
				m.logger.Error().
					Str("panic", fmt.Sprintf("%v", r)).
					Str("stack", stackTrace).
					Str("job_id", job.ID).
					Msg("FATAL: Job monitor goroutine panicked - writing crash file")

				// Write crash file for reliable persistence
				common.WriteCrashFile(r, stackTrace)

				// Note: Don't os.Exit here - this is a monitor goroutine, not main processor
				// The parent job will be left in running state and can be manually recovered
			}
		}()

		if err := m.monitorChildJobs(ctx, job); err != nil {
			m.logger.Error().
				Err(err).
				Str("job_id", job.ID).
				Msg("Parent job monitoring failed")
		}
	}()

	m.logger.Debug().
		Str("job_id", job.ID).
		Msg("Parent job monitoring started in background goroutine")
}

// validate validates that the queue job is compatible with this monitor
func (m *jobMonitor) validate(job *models.QueueJob) error {
	// Accept both parent (deprecated) and manager (new architecture) types
	if job.Type != string(models.JobTypeParent) && job.Type != string(models.JobTypeManager) {
		return fmt.Errorf("invalid job type: expected %s or %s, got %s", models.JobTypeParent, models.JobTypeManager, job.Type)
	}

	// source_type is required for parent jobs, optional for manager jobs
	// Manager jobs use step-based config instead
	if job.Type == string(models.JobTypeParent) {
		if _, ok := job.Config["source_type"]; !ok {
			return fmt.Errorf("missing required config field: source_type")
		}
	}

	// entity_type is optional (not required for generic web crawlers)

	return nil
}

// monitorChildJobs monitors child job progress and updates parent job status.
// This runs in a separate goroutine and blocks until all children complete or timeout.
func (m *jobMonitor) monitorChildJobs(ctx context.Context, job *models.QueueJob) error {
	// Create job-specific logger for consistent logging
	jobLogger := m.logger.WithCorrelationId(job.ID)

	// Extract configuration (may be empty for manager jobs)
	sourceType, _ := job.GetConfigString("source_type")
	entityType, _ := job.GetConfigString("entity_type")

	// Build log message based on job type and available fields
	var logMsg string
	if job.Type == string(models.JobTypeManager) {
		logMsg = fmt.Sprintf("Starting manager job: %s", job.Name)
	} else if entityType != "" {
		logMsg = fmt.Sprintf("Starting parent job for %s %s", sourceType, entityType)
	} else if sourceType != "" {
		logMsg = fmt.Sprintf("Starting parent job for %s", sourceType)
	} else {
		logMsg = fmt.Sprintf("Starting job: %s", job.Name)
	}

	jobLogger.Debug().
		Str("job_id", job.ID).
		Str("job_type", job.Type).
		Str("source_type", sourceType).
		Str("entity_type", entityType).
		Msg("Starting job monitoring")

	// Update job status to running
	if err := m.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Add job log for execution start
	m.jobMgr.AddJobLog(ctx, job.ID, "info", logMsg)

	// Publish initial progress update
	m.publishParentJobProgress(ctx, job, "running", "Monitoring child job progress")

	// Monitor child jobs until completion
	// The parent job's role is to:
	// 1. Track overall progress by aggregating child job status
	// 2. Determine when the crawl is complete
	// 3. Update its own status based on child job outcomes

	// Start monitoring loop
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	maxWaitTime := 30 * time.Minute           // Maximum time to wait for child jobs
	noChildrenGracePeriod := 30 * time.Second // Grace period for jobs that spawn no children
	timeout := time.After(maxWaitTime)
	monitorStartTime := time.Now()
	hasSeenChildren := false // Track if we've ever seen child jobs

	for {
		select {
		case <-ctx.Done():
			jobLogger.Debug().Msg("Parent job execution cancelled")
			m.jobMgr.UpdateJobStatus(ctx, job.ID, "cancelled")
			// Set finished_at timestamp for cancelled parent jobs
			if err := m.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
				jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
			}
			return ctx.Err()

		case <-timeout:
			jobLogger.Warn().Msg("Parent job timed out waiting for child jobs")
			m.jobMgr.SetJobError(ctx, job.ID, "Timed out waiting for child jobs to complete")
			m.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
			// Set finished_at timestamp for failed parent jobs
			if err := m.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
				jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
			}
			return fmt.Errorf("parent job timed out")

		case <-ticker.C:
			// Check if job has been cancelled via API (database status check)
			// This allows cancellation even without context cancellation
			currentJobInterface, jobErr := m.jobMgr.GetJob(ctx, job.ID)
			if jobErr == nil {
				if currentJob, ok := currentJobInterface.(*models.QueueJobState); ok {
					if currentJob.Status == models.JobStatusCancelled {
						jobLogger.Debug().Msg("Parent job was cancelled via API, stopping monitor")
						// Job already marked as cancelled, set finished timestamp and exit
						if err := m.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
							jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
						}
						m.publishParentJobProgress(ctx, job, "cancelled", "Job cancelled by user")
						return nil
					}
				}
			}

			// Check child job progress
			completed, childCount, err := m.checkChildJobProgressWithCount(ctx, job.ID, jobLogger)
			if err != nil {
				jobLogger.Error().Err(err).Msg("Failed to check child job progress")
				continue
			}

			// Track if we've seen any children
			if childCount > 0 {
				hasSeenChildren = true
			}

			// If no children have been spawned after the grace period, complete the job
			// This handles cases where workers return ReturnsChildJobs()=true but don't actually create any jobs
			if !hasSeenChildren && time.Since(monitorStartTime) > noChildrenGracePeriod {
				jobLogger.Debug().
					Dur("elapsed", time.Since(monitorStartTime)).
					Msg("No child jobs spawned after grace period, completing parent job")

				m.jobMgr.AddJobLog(ctx, job.ID, "info", "Job completed (no child jobs were spawned)")

				if err := m.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
					return fmt.Errorf("failed to update job status: %w", err)
				}

				if err := m.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
				}

				m.publishParentJobProgress(ctx, job, "completed", "Job completed (no child jobs)")
				return nil
			}

			if completed {
				// All child jobs are complete
				jobLogger.Debug().Msg("All child jobs completed, finishing parent job")

				// Update job status to completed
				if err := m.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
					return fmt.Errorf("failed to update job status: %w", err)
				}

				// Set finished_at timestamp for completed parent jobs
				if err := m.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
				}

				// Add final job log
				m.jobMgr.AddJobLog(ctx, job.ID, "info", "Parent job completed successfully")

				// Update step status to "completed" now that all children are done
				// This is important for steps that spawn child jobs - they start as "spawned"
				// and should only be marked "completed" when all children finish
				stepCompletedMetadata := map[string]interface{}{
					"current_step_status": "completed",
				}
				if err := m.jobMgr.UpdateJobMetadata(ctx, job.ID, stepCompletedMetadata); err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to update step status to completed")
				}

				// Publish job_step_progress event so UI updates step status in real-time
				m.publishStepCompletedEvent(ctx, job.ID, jobLogger)

				// Publish completion event with full stats (including document_count)
				// Get fresh child stats for final progress update
				childStatsMap, err := m.jobMgr.GetJobChildStats(ctx, []string{job.ID})
				if err != nil {
					jobLogger.Warn().Err(err).Msg("Failed to get final child stats for completion event")
					// Fall back to basic progress event
					m.publishParentJobProgress(ctx, job, "completed", "All child jobs completed")
				} else if interfaceStats, ok := childStatsMap[job.ID]; ok && interfaceStats != nil {
					// Convert to local stats struct
					finalStats := &ChildJobStats{
						TotalChildren:     interfaceStats.ChildCount,
						CompletedChildren: interfaceStats.CompletedChildren,
						FailedChildren:    interfaceStats.FailedChildren,
						CancelledChildren: interfaceStats.CancelledChildren,
						RunningChildren:   interfaceStats.RunningChildren,
						PendingChildren:   interfaceStats.PendingChildren,
					}
					progressText := m.formatProgressText(finalStats)
					// Use publishParentJobProgressUpdate which includes document_count
					m.publishParentJobProgressUpdate(ctx, job.ID, finalStats, progressText)
				} else {
					// No stats available, fall back to basic progress event
					m.publishParentJobProgress(ctx, job, "completed", "All child jobs completed")
				}

				jobLogger.Debug().Str("job_id", job.ID).Msg("Parent job execution completed successfully")
				return nil
			}
		}
	}
}

// checkChildJobProgressWithCount checks the progress of child jobs and returns:
// - completed: true if all children are in terminal state
// - childCount: total number of child jobs found
// - error: any error encountered
func (m *jobMonitor) checkChildJobProgressWithCount(ctx context.Context, parentJobID string, logger arbor.ILogger) (bool, int, error) {
	// Get child job statistics
	childStatsMap, err := m.jobMgr.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return false, 0, fmt.Errorf("failed to get child job stats: %w", err)
	}

	// Extract stats for this parent job
	interfaceStats, ok := childStatsMap[parentJobID]
	if !ok || interfaceStats == nil {
		// No children yet, keep waiting
		return false, 0, nil
	}

	// Convert interfaces.JobChildStats to local ChildJobStats
	childStats := &ChildJobStats{
		TotalChildren:     interfaceStats.ChildCount,
		CompletedChildren: interfaceStats.CompletedChildren,
		FailedChildren:    interfaceStats.FailedChildren,
		CancelledChildren: interfaceStats.CancelledChildren,
		RunningChildren:   interfaceStats.RunningChildren,
		PendingChildren:   interfaceStats.PendingChildren,
	}

	// Log current progress
	logger.Trace().
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
	m.jobMgr.AddJobLog(ctx, parentJobID, "info", progressText)

	// Publish progress update
	m.publishChildJobStats(ctx, parentJobID, childStats, progressText)

	// Check if all child jobs are in terminal state (completed, failed, or cancelled)
	// This ensures we wait until all children (including grandchildren) are done
	// and no more children are being spawned
	if childStats.TotalChildren > 0 {
		return terminalChildren >= childStats.TotalChildren, childStats.TotalChildren, nil
	}

	// If no child jobs exist yet, keep waiting
	return false, 0, nil
}

// publishParentJobProgress publishes a parent job progress update for real-time monitoring
func (m *jobMonitor) publishParentJobProgress(ctx context.Context, job *models.QueueJob, status, activity string) {
	if m.eventService == nil {
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
		if err := m.eventService.Publish(ctx, event); err != nil {
			m.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to publish parent job progress event")
		}
	}()
}

// publishChildJobStats publishes child job statistics for real-time monitoring
func (m *jobMonitor) publishChildJobStats(ctx context.Context, parentJobID string, stats *ChildJobStats, progressText string) {
	if m.eventService == nil {
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
		if err := m.eventService.Publish(ctx, event); err != nil {
			m.logger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to publish child job stats event")
		}
	}()
}

// SubscribeToJobEvents subscribes to job lifecycle events
// This enables real-time progress tracking without polling
func (m *jobMonitor) SubscribeToJobEvents() {
	if m.eventService == nil {
		return
	}

	// Subscribe to all job status changes
	if err := m.eventService.Subscribe(interfaces.EventJobStatusChange, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			m.logger.Warn().Msg("Invalid job_status_change payload type")
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
		m.logger.Trace().
			Str("job_id", jobID).
			Str("parent_id", parentID).
			Str("status", status).
			Str("job_type", jobType).
			Msg("Child job status changed")

		// Get fresh child job stats for the parent
		childStatsMap, err := m.jobMgr.GetJobChildStats(ctx, []string{parentID})
		if err != nil {
			m.logger.Error().Err(err).
				Str("parent_id", parentID).
				Msg("Failed to get child job stats after status change")
			return nil // Don't fail the event handler
		}

		// Extract stats for this parent job
		interfaceStats, ok := childStatsMap[parentID]
		if !ok || interfaceStats == nil {
			// No stats available, skip this update
			return nil
		}

		// Convert interfaces.JobChildStats to local ChildJobStats
		stats := &ChildJobStats{
			TotalChildren:     interfaceStats.ChildCount,
			CompletedChildren: interfaceStats.CompletedChildren,
			FailedChildren:    interfaceStats.FailedChildren,
			CancelledChildren: interfaceStats.CancelledChildren,
			RunningChildren:   interfaceStats.RunningChildren,
			PendingChildren:   interfaceStats.PendingChildren,
		}

		// Generate progress text in required format
		progressText := m.formatProgressText(stats)

		// Add job log for parent job with empty originator (system/monitor log)
		m.jobMgr.AddJobLogWithOriginator(ctx, parentID, "info",
			fmt.Sprintf("Child job %s → %s. %s",
				jobID[:8], // Short job ID for readability
				status,
				progressText),
			"") // Empty originator for monitor-generated logs

		// Publish parent job progress update
		m.publishParentJobProgressUpdate(ctx, parentID, stats, progressText)

		// Check if this job belongs to a step (has step_id in metadata)
		// If so, publish step_progress event for real-time step UI updates
		m.publishStepProgressOnChildChange(ctx, jobID)

		return nil
	}); err != nil {
		m.logger.Error().Err(err).Msg("Failed to subscribe to EventJobStatusChange")
		return
	}

	// Subscribe to document_saved events for real-time document count tracking
	if err := m.eventService.Subscribe(interfaces.EventDocumentSaved, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			m.logger.Warn().Msg("Invalid document_saved payload type")
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

		// Increment document count synchronously to ensure count is updated before
		// PublishSync returns. This is critical for Places jobs where documents are
		// saved in a loop and the job completes immediately after. The document count
		// must be accurate when the job is marked complete.
		if err := m.jobMgr.IncrementDocumentCount(ctx, parentJobID); err != nil {
			m.logger.Error().Err(err).
				Str("parent_job_id", parentJobID).
				Str("document_id", documentID).
				Str("job_id", jobID).
				Msg("Failed to increment document count for parent job")
		} else {
			m.logger.Trace().
				Str("parent_job_id", parentJobID).
				Str("document_id", documentID).
				Str("job_id", jobID).
				Msg("Incremented document count for parent job")
		}

		// Also increment the CHILD job's document_count so it can display its own count
		// This allows the UI to show how many documents each child job created
		if jobID != "" && jobID != parentJobID {
			if err := m.jobMgr.IncrementDocumentCount(ctx, jobID); err != nil {
				m.logger.Debug().Err(err).
					Str("job_id", jobID).
					Str("document_id", documentID).
					Msg("Failed to increment document count for child job")
			} else {
				m.logger.Trace().
					Str("job_id", jobID).
					Str("document_id", documentID).
					Msg("Incremented document count for child job")
			}
		}

		// Publish progress update to reflect new document count
		// Get fresh child job stats
		childStatsMap, err := m.jobMgr.GetJobChildStats(ctx, []string{parentJobID})
		if err != nil {
			m.logger.Warn().Err(err).
				Str("parent_id", parentJobID).
				Msg("Failed to get child job stats after document save")
		} else if interfaceStats, ok := childStatsMap[parentJobID]; ok && interfaceStats != nil {
			// Convert to local stats struct
			stats := &ChildJobStats{
				TotalChildren:     interfaceStats.ChildCount,
				CompletedChildren: interfaceStats.CompletedChildren,
				FailedChildren:    interfaceStats.FailedChildren,
				CancelledChildren: interfaceStats.CancelledChildren,
				RunningChildren:   interfaceStats.RunningChildren,
				PendingChildren:   interfaceStats.PendingChildren,
			}
			progressText := m.formatProgressText(stats)

			// Publish parent job progress update which includes the new document_count
			m.publishParentJobProgressUpdate(ctx, parentJobID, stats, progressText)
		}

		return nil
	}); err != nil {
		m.logger.Error().Err(err).Msg("Failed to subscribe to EventDocumentSaved")
		return
	}

	// Subscribe to document_updated events for agent jobs (e.g. keyword extraction)
	// NOTE: We do NOT increment the parent job's document_count for updates.
	// Document updates modify existing documents, they don't create new ones.
	// The document_count should reflect UNIQUE documents, not total operations.
	// Example: If Step 1 creates 20 docs and Step 2 updates those same 20 docs,
	// the count should remain 20 (unique documents), not 40 (operations).
	if err := m.eventService.Subscribe(interfaces.EventDocumentUpdated, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			m.logger.Warn().Msg("Invalid document_updated payload type")
			return nil
		}

		// Extract fields for logging
		parentJobID := getStringFromPayload(payload, "parent_job_id")
		documentID := getStringFromPayload(payload, "document_id")
		jobID := getStringFromPayload(payload, "job_id")

		// Log the document update for debugging, but don't increment count
		// Document updates don't create new documents, so count stays the same
		m.logger.Debug().
			Str("parent_job_id", parentJobID).
			Str("document_id", documentID).
			Str("job_id", jobID).
			Msg("Document updated (count not incremented - updates don't create new documents)")

		return nil
	}); err != nil {
		m.logger.Error().Err(err).Msg("Failed to subscribe to EventDocumentUpdated")
		return
	}

	// Subscribe to job error events for real-time error tracking
	if err := m.eventService.Subscribe(interfaces.EventJobError, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			m.logger.Warn().Msg("Invalid job_error payload type")
			return nil
		}

		// Extract event data
		jobID := getStringFromPayload(payload, "job_id")
		parentJobID := getStringFromPayload(payload, "parent_job_id")
		errorMessage := getStringFromPayload(payload, "error_message")

		// If this is a parent job error, track it directly
		if parentJobID == jobID || parentJobID == "" {
			// Add error to parent job's status_report
			if err := m.jobMgr.AddJobError(context.Background(), jobID, errorMessage); err != nil {
				m.logger.Error().Err(err).
					Str("job_id", jobID).
					Str("error_message", errorMessage).
					Msg("Failed to add error to job status_report")
			}

			m.logger.Debug().
				Str("job_id", jobID).
				Str("error_message", errorMessage).
				Msg("Added error to job status_report")
		}

		return nil
	}); err != nil {
		m.logger.Error().Err(err).Msg("Failed to subscribe to EventJobError")
		return
	}

	// Subscribe to job warning events for real-time warning tracking
	if err := m.eventService.Subscribe(interfaces.EventJobWarning, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			m.logger.Warn().Msg("Invalid job_warning payload type")
			return nil
		}

		// Extract event data
		jobID := getStringFromPayload(payload, "job_id")
		parentJobID := getStringFromPayload(payload, "parent_job_id")
		warningMessage := getStringFromPayload(payload, "warning_message")

		// If this is a parent job warning, track it directly
		if parentJobID == jobID || parentJobID == "" {
			// Add warning to parent job's status_report
			if err := m.jobMgr.AddJobWarning(context.Background(), jobID, warningMessage); err != nil {
				m.logger.Error().Err(err).
					Str("job_id", jobID).
					Str("warning_message", warningMessage).
					Msg("Failed to add warning to job status_report")
			}

			m.logger.Debug().
				Str("job_id", jobID).
				Str("warning_message", warningMessage).
				Msg("Added warning to job status_report")
		}

		return nil
	}); err != nil {
		m.logger.Error().Err(err).Msg("Failed to subscribe to EventJobWarning")
		return
	}

	m.logger.Debug().Msg("JobMonitor subscribed to child job status changes, document events, error events, and warning events")
}

// formatProgressText generates the required progress format
// Example: "66 pending, 1 running, 41 completed, 0 failed"
func (m *jobMonitor) formatProgressText(stats *ChildJobStats) string {
	return fmt.Sprintf("%d pending, %d running, %d completed, %d failed",
		stats.PendingChildren,
		stats.RunningChildren,
		stats.CompletedChildren,
		stats.FailedChildren)
}

// publishParentJobProgressUpdate publishes progress update for WebSocket consumption
func (m *jobMonitor) publishParentJobProgressUpdate(
	ctx context.Context,
	parentJobID string,
	stats *ChildJobStats,
	progressText string) {

	if m.eventService == nil {
		return
	}

	// Calculate overall status based on child states
	overallStatus := m.calculateOverallStatus(stats)

	// Get document count from job metadata (default to 0 if error)
	documentCount, err := m.jobMgr.GetDocumentCount(ctx, parentJobID)
	if err != nil {
		// Log error but don't fail - just use default count of 0
		m.logger.Debug().Err(err).
			Str("parent_job_id", parentJobID).
			Msg("Failed to retrieve document count from metadata, using default 0")
		documentCount = 0
	}

	// Get errors and warnings from job metadata (for real-time UI display)
	errors, warnings := m.getJobErrorsAndWarnings(ctx, parentJobID)

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
		"errors":             errors,        // Error messages from status_report
		"warnings":           warnings,      // Warning messages from status_report
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	event := interfaces.Event{
		Type:    "parent_job_progress",
		Payload: payload,
	}

	// Publish asynchronously
	go func() {
		if err := m.eventService.Publish(ctx, event); err != nil {
			m.logger.Warn().Err(err).
				Str("parent_job_id", parentJobID).
				Msg("Failed to publish parent job progress event")
		}
	}()
}

// calculateOverallStatus determines parent job status from child statistics
func (m *jobMonitor) calculateOverallStatus(stats *ChildJobStats) string {
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

// getJobErrorsAndWarnings extracts errors and warnings from job metadata status_report
// Returns empty arrays if no errors/warnings exist or if metadata cannot be read
func (m *jobMonitor) getJobErrorsAndWarnings(ctx context.Context, jobID string) ([]string, []string) {
	// Get job to access metadata
	jobInterface, err := m.jobMgr.GetJob(ctx, jobID)
	if err != nil {
		m.logger.Debug().Err(err).
			Str("job_id", jobID).
			Msg("Failed to get job for errors/warnings extraction")
		return []string{}, []string{}
	}

	// Type assert to *models.QueueJobState
	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		m.logger.Debug().
			Str("job_id", jobID).
			Msg("Failed to type assert job for errors/warnings extraction")
		return []string{}, []string{}
	}

	// Extract status_report from metadata
	statusReport, ok := jobState.Metadata["status_report"].(map[string]interface{})
	if !ok {
		// No status_report yet, return empty arrays
		return []string{}, []string{}
	}

	// Extract errors array
	var errors []string
	if errorsInterface, ok := statusReport["errors"].([]interface{}); ok {
		for _, errInterface := range errorsInterface {
			if errStr, ok := errInterface.(string); ok {
				errors = append(errors, errStr)
			}
		}
	}

	// Extract warnings array
	var warnings []string
	if warningsInterface, ok := statusReport["warnings"].([]interface{}); ok {
		for _, warnInterface := range warningsInterface {
			if warnStr, ok := warnInterface.(string); ok {
				warnings = append(warnings, warnStr)
			}
		}
	}

	return errors, warnings
}

// publishStepCompletedEvent publishes a job_step_progress event with step_status="completed"
// This is called when all child jobs finish to update the UI step status in real-time
func (m *jobMonitor) publishStepCompletedEvent(ctx context.Context, jobID string, logger arbor.ILogger) {
	if m.eventService == nil {
		return
	}

	// Get job metadata to read step info
	jobInterface, err := m.jobMgr.GetJob(ctx, jobID)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get job for step completed event")
		return
	}

	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		logger.Warn().Msg("Failed to type assert job for step completed event")
		return
	}

	// Extract step info from metadata
	metadata := jobState.Metadata
	currentStep, _ := metadata["current_step"].(float64)
	totalSteps, _ := metadata["total_steps"].(float64)
	stepName, _ := metadata["current_step_name"].(string)
	stepType, _ := metadata["current_step_type"].(string)

	payload := map[string]interface{}{
		"job_id":       jobID,
		"step_index":   int(currentStep) - 1,
		"step_name":    stepName,
		"step_type":    stepType,
		"current_step": int(currentStep),
		"total_steps":  int(totalSteps),
		"step_status":  "completed",
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	event := interfaces.Event{
		Type:    "job_step_progress",
		Payload: payload,
	}

	// Publish asynchronously
	go func() {
		if err := m.eventService.Publish(ctx, event); err != nil {
			logger.Warn().Err(err).Msg("Failed to publish step completed event")
		}
	}()
}

// publishStepProgressOnChildChange publishes a step_progress event when a child job changes status.
// This provides real-time step progress updates without waiting for the 5-second polling interval.
func (m *jobMonitor) publishStepProgressOnChildChange(ctx context.Context, jobID string) {
	if m.eventService == nil {
		return
	}

	// Get job to check for step_id in metadata or parent_id
	jobInterface, err := m.jobMgr.GetJob(ctx, jobID)
	if err != nil {
		// Don't log error - this is best-effort real-time update
		return
	}

	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		return
	}

	// Check if job has step_id in metadata (agent worker sets this)
	stepID, ok := jobState.Metadata["step_id"].(string)
	if !ok || stepID == "" {
		// Fallback: Check if parent_id points to a step job (crawler worker uses parent_id)
		if jobState.ParentID != nil && *jobState.ParentID != "" {
			parentInterface, err := m.jobMgr.GetJob(ctx, *jobState.ParentID)
			if err == nil {
				if parentJob, ok := parentInterface.(*models.QueueJobState); ok && parentJob.Type == "step" {
					stepID = *jobState.ParentID
				}
			}
		}
	}

	if stepID == "" {
		// Not a step child job, nothing to do
		return
	}

	// Get manager_id from metadata, or from step job's parent_id
	managerID, _ := jobState.Metadata["manager_id"].(string)
	if managerID == "" {
		// Try to get manager_id from step job's parent_id
		stepInterface, err := m.jobMgr.GetJob(ctx, stepID)
		if err == nil {
			if stepJob, ok := stepInterface.(*models.QueueJobState); ok && stepJob.ParentID != nil {
				managerID = *stepJob.ParentID
			}
		}
	}

	// Get step_name from job metadata, or from step job's name
	stepName, _ := jobState.Metadata["step_name"].(string)
	if stepName == "" {
		// Try to get step_name from step job
		stepInterface, err := m.jobMgr.GetJob(ctx, stepID)
		if err == nil {
			if stepJob, ok := stepInterface.(*models.QueueJobState); ok {
				stepName = stepJob.Name
			}
		}
	}

	// Get fresh step child stats
	childStatsMap, err := m.jobMgr.GetJobChildStats(ctx, []string{stepID})
	if err != nil {
		m.logger.Debug().Err(err).
			Str("step_id", stepID).
			Msg("Failed to get step child stats for real-time update")
		return
	}

	// Extract stats for this step
	interfaceStats, ok := childStatsMap[stepID]
	if !ok || interfaceStats == nil {
		return
	}

	// Convert to ChildJobStats
	stats := &ChildJobStats{
		TotalChildren:     interfaceStats.ChildCount,
		CompletedChildren: interfaceStats.CompletedChildren,
		FailedChildren:    interfaceStats.FailedChildren,
		CancelledChildren: interfaceStats.CancelledChildren,
		RunningChildren:   interfaceStats.RunningChildren,
		PendingChildren:   interfaceStats.PendingChildren,
	}

	// Calculate step status based on child states
	stepStatus := "running"
	terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
	if stats.TotalChildren > 0 && terminalCount >= stats.TotalChildren {
		// All children in terminal state - determine final status
		if stats.FailedChildren > 0 && stats.CompletedChildren == 0 {
			// All failed, none completed
			stepStatus = "failed"
		} else if stats.FailedChildren > 0 {
			// Some failed, some completed - partial failure
			stepStatus = "completed" // Mark as completed but with failures noted
		} else if stats.CancelledChildren > 0 && stats.CompletedChildren == 0 {
			// All cancelled
			stepStatus = "cancelled"
		} else {
			// All completed successfully
			stepStatus = "completed"
		}
	}

	// Build progress text
	progressText := fmt.Sprintf("%d pending, %d running, %d completed, %d failed",
		stats.PendingChildren, stats.RunningChildren, stats.CompletedChildren, stats.FailedChildren)

	// Publish step_progress event
	payload := map[string]interface{}{
		"step_id":        stepID,
		"manager_id":     managerID,
		"step_name":      stepName, // Step name for UI aggregation by step
		"status":         stepStatus,
		"total_jobs":     stats.TotalChildren,
		"pending_jobs":   stats.PendingChildren,
		"running_jobs":   stats.RunningChildren,
		"completed_jobs": stats.CompletedChildren,
		"failed_jobs":    stats.FailedChildren,
		"cancelled_jobs": stats.CancelledChildren,
		"progress_text":  progressText,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	event := interfaces.Event{
		Type:    interfaces.EventStepProgress,
		Payload: payload,
	}

	// Publish asynchronously to avoid blocking the status change handler
	go func() {
		if err := m.eventService.Publish(ctx, event); err != nil {
			m.logger.Debug().Err(err).
				Str("step_id", stepID).
				Msg("Failed to publish real-time step progress event")
		} else {
			m.logger.Trace().
				Str("step_id", stepID).
				Str("status", stepStatus).
				Str("progress", progressText).
				Msg("Published real-time step progress event")
		}
	}()
}
