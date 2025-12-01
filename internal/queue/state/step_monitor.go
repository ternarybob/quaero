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

	// Publish initial progress
	m.publishStepProgress(ctx, stepJob.ID, managerID, "running", nil)

	// Monitor child jobs until completion
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	maxWaitTime := 30 * time.Minute
	noChildrenGracePeriod := 30 * time.Second
	timeout := time.After(maxWaitTime)
	monitorStartTime := time.Now()
	hasSeenChildren := false

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
			// Check child job progress for THIS step
			completed, childCount, stats, err := m.checkStepChildProgress(ctx, stepJob.ID, stepLogger)
			if err != nil {
				stepLogger.Error().Err(err).Msg("Failed to check step child progress")
				continue
			}

			if childCount > 0 {
				hasSeenChildren = true
			}

			// If no children after grace period, mark step complete
			if !hasSeenChildren && time.Since(monitorStartTime) > noChildrenGracePeriod {
				stepLogger.Debug().
					Dur("elapsed", time.Since(monitorStartTime)).
					Msg("No child jobs spawned after grace period, completing step")

				m.jobMgr.AddJobLog(ctx, stepJob.ID, "info", "Step completed (no child jobs spawned)")
				m.jobMgr.UpdateJobStatus(ctx, stepJob.ID, "completed")
				m.jobMgr.SetJobFinished(ctx, stepJob.ID)
				m.publishStepProgress(ctx, stepJob.ID, managerID, "completed", stats)
				return nil
			}

			if completed {
				// All child jobs complete
				stepLogger.Debug().Msg("All step children completed, marking step complete")

				// Determine final status based on child outcomes
				finalStatus := "completed"
				if stats != nil && stats.FailedChildren > 0 {
					finalStatus = "completed" // Step completes even if some jobs failed
					// Could change to "failed" if all children failed
				}

				m.jobMgr.AddJobLog(ctx, stepJob.ID, "info", fmt.Sprintf("Step completed (%d jobs processed)", childCount))
				m.jobMgr.UpdateJobStatus(ctx, stepJob.ID, finalStatus)
				m.jobMgr.SetJobFinished(ctx, stepJob.ID)
				m.publishStepProgress(ctx, stepJob.ID, managerID, finalStatus, stats)

				stepLogger.Debug().
					Str("step_id", stepJob.ID).
					Int("child_count", childCount).
					Msg("Step monitoring completed successfully")
				return nil
			}

			// Publish progress update
			m.publishStepProgress(ctx, stepJob.ID, managerID, "running", stats)
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
	status string,
	stats *ChildJobStats,
) {
	if m.eventService == nil {
		return
	}

	payload := map[string]interface{}{
		"step_id":    stepID,
		"manager_id": managerID,
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
