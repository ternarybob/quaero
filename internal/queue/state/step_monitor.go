package state

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// StepMonitor monitors a step job's children (worker jobs) and marks the step
// as complete when all children finish. Each step with children gets its own
// StepMonitor running in a goroutine.
//
// Hierarchy: Manager -> Steps -> Jobs
// StepMonitor handles: Step -> Jobs (monitors jobs under a step)
type StepMonitor struct {
	jobMgr       interfaces.JobStatusManager
	eventService interfaces.EventService
	logger       arbor.ILogger
}

// NewStepMonitor creates a new step monitor
func NewStepMonitor(
	jobMgr interfaces.JobStatusManager,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *StepMonitor {
	return &StepMonitor{
		jobMgr:       jobMgr,
		eventService: eventService,
		logger:       logger,
	}
}

// StartMonitoring starts monitoring a step job's children in a background goroutine.
// When all children complete, the step is marked as completed.
func (m *StepMonitor) StartMonitoring(ctx context.Context, stepJob *models.QueueJob) {
	// Validate step job
	if stepJob.Type != string(models.JobTypeStep) {
		m.logger.Error().
			Str("job_id", stepJob.ID).
			Str("job_type", stepJob.Type).
			Msg("StepMonitor requires a step job, got different type")
		return
	}

	// Start monitoring in a separate goroutine with panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				m.logger.Fatal().
					Str("panic", fmt.Sprintf("%v", r)).
					Str("stack", string(buf[:n])).
					Str("step_id", stepJob.ID).
					Msg("FATAL: Step monitor goroutine panicked")
			}
		}()

		if err := m.monitorStepChildren(ctx, stepJob); err != nil {
			m.logger.Error().
				Err(err).
				Str("step_id", stepJob.ID).
				Msg("Step monitoring failed")
		}
	}()

	m.logger.Debug().
		Str("step_id", stepJob.ID).
		Str("step_name", stepJob.Name).
		Msg("Step monitoring started in background goroutine")
}

// monitorStepChildren monitors child jobs and updates step status.
// Blocks until all children complete or timeout.
func (m *StepMonitor) monitorStepChildren(ctx context.Context, stepJob *models.QueueJob) error {
	stepLogger := m.logger.WithCorrelationId(stepJob.ID)

	// Extract manager ID from step metadata
	managerID := stepJob.GetManagerID()
	if managerID == "" {
		managerID = stepJob.GetParentID() // Fallback
	}

	stepLogger.Debug().
		Str("step_id", stepJob.ID).
		Str("manager_id", managerID).
		Msg("Starting step child monitoring")

	// Publish initial progress (starting message will include worker count once known)
	m.publishStepProgress(ctx, stepJob.ID, managerID, stepJob.Name, "running", nil)

	// Monitor child jobs until completion
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	maxWaitTime := 30 * time.Minute
	noChildrenGracePeriod := 30 * time.Second
	timeout := time.After(maxWaitTime)
	stepStartTime := time.Now()
	hasSeenChildren := false
	hasPublishedStarting := false

	for {
		select {
		case <-ctx.Done():
			stepLogger.Debug().Msg("Step monitoring cancelled")
			m.jobMgr.UpdateJobStatus(ctx, stepJob.ID, "cancelled")
			m.jobMgr.SetJobFinished(ctx, stepJob.ID)
			return ctx.Err()

		case <-timeout:
			stepLogger.Warn().Msg("Step timed out waiting for child jobs")
			m.jobMgr.SetJobError(ctx, stepJob.ID, "Timed out waiting for child jobs")
			m.jobMgr.UpdateJobStatus(ctx, stepJob.ID, "failed")
			m.jobMgr.SetJobFinished(ctx, stepJob.ID)
			return fmt.Errorf("step timed out")

		case <-ticker.C:
			// Check if step job has been cancelled via API (database status check)
			// This allows cancellation even without context cancellation
			currentJobInterface, jobErr := m.jobMgr.GetJob(ctx, stepJob.ID)
			if jobErr == nil {
				if currentJob, ok := currentJobInterface.(*models.QueueJobState); ok {
					if currentJob.Status == models.JobStatusCancelled {
						stepLogger.Debug().Msg("Step job was cancelled via API, stopping monitor")
						// Job already marked as cancelled, set finished timestamp and exit
						m.jobMgr.SetJobFinished(ctx, stepJob.ID)
						m.publishStepProgress(ctx, stepJob.ID, managerID, stepJob.Name, "cancelled", nil)
						return nil
					}
				}
			}

			// Check child job progress for THIS step
			completed, childCount, stats, err := m.checkStepChildProgress(ctx, stepJob.ID, stepLogger)
			if err != nil {
				stepLogger.Error().Err(err).Msg("Failed to check step child progress")
				continue
			}

			if childCount > 0 {
				hasSeenChildren = true
				// Publish "Starting N workers..." when we first see children
				if !hasPublishedStarting {
					hasPublishedStarting = true
					m.publishStepLog(ctx, managerID, stepJob.Name, "info", fmt.Sprintf("Starting %d workers...", childCount))
				}
			}

			// If no children after grace period, mark step complete
			if !hasSeenChildren && time.Since(stepStartTime) > noChildrenGracePeriod {
				duration := time.Since(stepStartTime)
				stepLogger.Debug().
					Dur("elapsed", duration).
					Msg("No child jobs spawned after grace period, completing step")

				// Publish starting message if not yet published
				if !hasPublishedStarting {
					m.publishStepLog(ctx, managerID, stepJob.Name, "info", "Starting workers...")
				}
				m.publishStepLog(ctx, managerID, stepJob.Name, "info", fmt.Sprintf("Step completed (no jobs) in %s", formatDuration(duration)))
				m.jobMgr.UpdateJobStatus(ctx, stepJob.ID, "completed")
				m.jobMgr.SetJobFinished(ctx, stepJob.ID)
				m.publishStepProgress(ctx, stepJob.ID, managerID, stepJob.Name, "completed", stats)
				return nil
			}

			if completed {
				// All child jobs complete
				duration := time.Since(stepStartTime)
				stepLogger.Debug().Msg("All step children completed, determining final status")

				// Determine final status based on child outcomes
				finalStatus := "completed"
				logLevel := "info"
				durationStr := formatDuration(duration)
				stepLogMsg := fmt.Sprintf("Step finished successfully in %s (%d jobs)", durationStr, childCount)

				if stats != nil {
					if stats.FailedChildren > 0 && stats.CompletedChildren == 0 {
						// All failed, none completed
						finalStatus = "failed"
						logLevel = "error"
						stepLogMsg = fmt.Sprintf("Step failed in %s: all %d jobs failed", durationStr, stats.FailedChildren)
					} else if stats.FailedChildren > 0 {
						// Some failed, some completed - partial success
						finalStatus = "completed"
						logLevel = "warn"
						stepLogMsg = fmt.Sprintf("Step finished with errors in %s: %d succeeded, %d failed", durationStr, stats.CompletedChildren, stats.FailedChildren)
					} else if stats.CancelledChildren > 0 && stats.CompletedChildren == 0 {
						// All cancelled
						finalStatus = "cancelled"
						logLevel = "warn"
						stepLogMsg = fmt.Sprintf("Step cancelled in %s: %d jobs cancelled", durationStr, stats.CancelledChildren)
					}
				}

				// Publish step log to manager for UI step panel (AddJobLog also publishes, so only use one)
				m.publishStepLog(ctx, managerID, stepJob.Name, logLevel, stepLogMsg)
				m.jobMgr.UpdateJobStatus(ctx, stepJob.ID, finalStatus)
				m.jobMgr.SetJobFinished(ctx, stepJob.ID)
				m.publishStepProgress(ctx, stepJob.ID, managerID, stepJob.Name, finalStatus, stats)

				stepLogger.Debug().
					Str("step_id", stepJob.ID).
					Int("child_count", childCount).
					Dur("duration", duration).
					Msg("Step monitoring completed successfully")
				return nil
			}

			// Publish progress update
			m.publishStepProgress(ctx, stepJob.ID, managerID, stepJob.Name, "running", stats)
		}
	}
}

// checkStepChildProgress checks progress of child jobs for a step
func (m *StepMonitor) checkStepChildProgress(
	ctx context.Context,
	stepID string,
	logger arbor.ILogger,
) (completed bool, childCount int, stats *ChildJobStats, err error) {
	// Get child job statistics for this step
	childStatsMap, err := m.jobMgr.GetJobChildStats(ctx, []string{stepID})
	if err != nil {
		return false, 0, nil, fmt.Errorf("failed to get step child stats: %w", err)
	}

	interfaceStats, ok := childStatsMap[stepID]
	if !ok || interfaceStats == nil {
		// No children yet
		return false, 0, nil, nil
	}

	stats = &ChildJobStats{
		TotalChildren:     interfaceStats.ChildCount,
		CompletedChildren: interfaceStats.CompletedChildren,
		FailedChildren:    interfaceStats.FailedChildren,
		CancelledChildren: interfaceStats.CancelledChildren,
		RunningChildren:   interfaceStats.RunningChildren,
		PendingChildren:   interfaceStats.PendingChildren,
	}

	logger.Trace().
		Int("total", stats.TotalChildren).
		Int("completed", stats.CompletedChildren).
		Int("failed", stats.FailedChildren).
		Int("running", stats.RunningChildren).
		Int("pending", stats.PendingChildren).
		Msg("Step child progress check")

	// Check if all children are in terminal state
	terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
	if stats.TotalChildren > 0 && terminalCount >= stats.TotalChildren {
		return true, stats.TotalChildren, stats, nil
	}

	return false, stats.TotalChildren, stats, nil
}

// publishStepProgress publishes step progress event for WebSocket clients
func (m *StepMonitor) publishStepProgress(
	ctx context.Context,
	stepID string,
	managerID string,
	stepName string,
	status string,
	stats *ChildJobStats,
) {
	if m.eventService == nil {
		return
	}

	payload := map[string]interface{}{
		"step_id":    stepID,
		"manager_id": managerID,
		"step_name":  stepName, // Critical for UI filtering by step
		"status":     status,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	if stats != nil {
		payload["total_jobs"] = stats.TotalChildren
		payload["pending_jobs"] = stats.PendingChildren
		payload["running_jobs"] = stats.RunningChildren
		payload["completed_jobs"] = stats.CompletedChildren
		payload["failed_jobs"] = stats.FailedChildren
		payload["cancelled_jobs"] = stats.CancelledChildren
		payload["progress_text"] = fmt.Sprintf("%d pending, %d running, %d completed, %d failed",
			stats.PendingChildren, stats.RunningChildren, stats.CompletedChildren, stats.FailedChildren)
	}

	event := interfaces.Event{
		Type:    interfaces.EventStepProgress,
		Payload: payload,
	}

	go func() {
		if err := m.eventService.Publish(ctx, event); err != nil {
			m.logger.Warn().Err(err).
				Str("step_id", stepID).
				Msg("Failed to publish step progress event")
		}
	}()
}

// publishStepLog stores and publishes a job_log event for a step to the manager's log stream.
// This ensures step events appear in the UI's step events panel and persist after page refresh.
func (m *StepMonitor) publishStepLog(ctx context.Context, managerID, stepName, level, message string) {
	// Store to database with explicit step_name and "step" originator for persistence
	// This uses AddJobLogWithContext to set the correct step context
	if m.jobMgr != nil {
		if err := m.jobMgr.AddJobLogWithContext(ctx, managerID, level, message, stepName, "step"); err != nil {
			m.logger.Debug().Err(err).
				Str("manager_id", managerID).
				Str("step_name", stepName).
				Msg("Failed to store step log")
		}
	}
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs > 0 {
			return fmt.Sprintf("%dm %ds", mins, secs)
		}
		return fmt.Sprintf("%dm", mins)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}
