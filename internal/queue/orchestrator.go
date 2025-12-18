package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Orchestrator handles the execution of job definitions by coordinating steps and workers.
type Orchestrator struct {
	jobManager   *Manager
	stepManager  interfaces.StepManager
	eventService interfaces.EventService
	kvStorage    interfaces.KeyValueStorage
	logger       arbor.ILogger
}

// NewOrchestrator creates a new Orchestrator
func NewOrchestrator(jobManager *Manager, stepManager interfaces.StepManager, eventService interfaces.EventService, kvStorage interfaces.KeyValueStorage, logger arbor.ILogger) *Orchestrator {
	return &Orchestrator{
		jobManager:   jobManager,
		stepManager:  stepManager,
		eventService: eventService,
		kvStorage:    kvStorage,
		logger:       logger,
	}
}

// ExecuteJobDefinition executes a job definition by creating a manager job and step jobs.
// It orchestrates the execution of steps defined in the job definition.
// Returns the manager job ID.
func (o *Orchestrator) ExecuteJobDefinition(ctx context.Context, jobDef *models.JobDefinition, jobMonitor interfaces.JobMonitor, stepMonitor interfaces.StepMonitor) (string, error) {
	// Create manager job (root parent)
	managerID := uuid.New().String()

	// Prepare config and metadata
	jobDefConfig := make(map[string]interface{})
	// Copy relevant fields from job definition to config
	jobDefConfig["job_def_id"] = jobDef.ID
	jobDefConfig["job_def_name"] = jobDef.Name
	jobDefConfig["job_def_type"] = string(jobDef.Type)

	managerMetadata := map[string]interface{}{
		"job_def_id":   jobDef.ID,
		"job_def_name": jobDef.Name,
		"phase":        "orchestration",
	}

	// Manually construct Job object for CreateJobRecord
	// CreateJobRecord expects Payload string with config/metadata
	payloadData := map[string]interface{}{
		"config":   jobDefConfig,
		"metadata": managerMetadata,
	}
	payloadBytes, _ := json.Marshal(payloadData)

	job := &Job{
		ID:        managerID,
		Type:      string(models.JobTypeManager),
		Name:      jobDef.Name,
		CreatedAt: time.Now(),
		Payload:   string(payloadBytes),
		Phase:     "orchestration",
		Status:    "pending",
	}

	if err := o.jobManager.CreateJobRecord(ctx, job); err != nil {
		return "", fmt.Errorf("create manager job record: %w", err)
	}

	// Persist metadata immediately after job creation
	managerMetadata = make(map[string]interface{})
	if jobDef.AuthID != "" {
		managerMetadata["auth_id"] = jobDef.AuthID
	}
	if jobDef.ID != "" {
		managerMetadata["job_definition_id"] = jobDef.ID
	}
	managerMetadata["phase"] = "execution"

	if err := o.jobManager.UpdateJobMetadata(ctx, managerID, managerMetadata); err != nil {
		// Log warning but continue
	}

	// Add initial job log
	initialLog := fmt.Sprintf("Starting job definition execution: %s (ID: %s, Steps: %d)",
		jobDef.Name, jobDef.ID, len(jobDef.Steps))
	o.jobManager.AddJobLog(ctx, managerID, "info", initialLog)

	// Build job definition config for manager job
	jobDefConfig = make(map[string]interface{})
	for i, step := range jobDef.Steps {
		stepKey := fmt.Sprintf("step_%d_%s", i+1, step.Type.String())
		jobDefConfig[stepKey] = step.Config
	}
	jobDefConfig["job_definition_id"] = jobDef.ID
	jobDefConfig["source_type"] = jobDef.SourceType
	jobDefConfig["base_url"] = jobDef.BaseURL
	jobDefConfig["schedule"] = jobDef.Schedule
	jobDefConfig["timeout"] = jobDef.Timeout
	jobDefConfig["enabled"] = jobDef.Enabled
	if jobDef.AuthID != "" {
		jobDefConfig["auth_id"] = jobDef.AuthID
	}

	if err := o.jobManager.UpdateJobConfig(ctx, managerID, jobDefConfig); err != nil {
		// Log warning but continue
	}

	// Build step_definitions for UI display
	stepDefs := make([]map[string]interface{}, len(jobDef.Steps))
	for i, step := range jobDef.Steps {
		stepDefs[i] = map[string]interface{}{
			"name":        step.Name,
			"type":        step.Type.String(),
			"description": step.Description,
		}
	}
	initialMetadata := map[string]interface{}{
		"step_definitions": stepDefs,
		"total_steps":      len(jobDef.Steps),
		"current_step":     0,
	}
	if err := o.jobManager.UpdateJobMetadata(ctx, managerID, initialMetadata); err != nil {
		// Log warning but continue
	}

	// Mark manager job as running
	if err := o.jobManager.UpdateJobStatus(ctx, managerID, "running"); err != nil {
		// Log warning but continue
	}

	// Track if any steps have child jobs
	hasChildJobs := false

	// Track validation errors that were skipped due to on_error="continue"
	var lastValidationError string

	// Track per-step statistics for UI display
	stepStats := make([]map[string]interface{}, len(jobDef.Steps))

	// Track step job IDs for monitoring (map step name -> step job ID)
	stepJobIDs := make(map[string]string, len(jobDef.Steps))

	// Execute steps sequentially
	for i, step := range jobDef.Steps {
		// Create step job (child of manager, parent of spawned jobs)
		stepID := uuid.New().String()
		stepJobIDs[step.Name] = stepID

		stepConfig := make(map[string]interface{})
		for k, v := range step.Config {
			stepConfig[k] = v
		}
		stepConfig["step_index"] = i
		stepConfig["step_name"] = step.Name
		stepConfig["step_type"] = step.Type.String()

		stepJob := &Job{
			ID:              stepID,
			ParentID:        &managerID,
			Type:            string(models.JobTypeStep),
			Name:            step.Name,
			Phase:           "execution",
			Status:          "pending",
			CreatedAt:       time.Now(),
			ProgressCurrent: 0,
			ProgressTotal:   0,
		}

		if err := o.jobManager.CreateJobRecord(ctx, stepJob); err != nil {
			o.jobManager.AddJobLog(ctx, managerID, "error", fmt.Sprintf("Failed to create step job: %v", err))
			continue
		}

		// Store step metadata - include auth_id for child job cookie injection
		stepJobMetadata := map[string]interface{}{
			"manager_id":        managerID,
			"step_index":        i,
			"step_name":         step.Name,
			"step_type":         step.Type.String(),
			"description":       step.Description,
			"job_definition_id": jobDef.ID,
		}
		// Propagate auth_id to step job so crawler workers can inject cookies
		if jobDef.AuthID != "" {
			stepJobMetadata["auth_id"] = jobDef.AuthID
		}
		if err := o.jobManager.UpdateJobMetadata(ctx, stepID, stepJobMetadata); err != nil {
			// Log but continue
		}

		// Mark step as running
		if err := o.jobManager.UpdateJobStatus(ctx, stepID, "running"); err != nil {
			// Log but continue
		}

		// Get document count BEFORE step execution
		docCountBefore, _ := o.jobManager.GetDocumentCount(ctx, managerID)

		// Update manager metadata with current step info
		// Include step_job_ids so UI can find step job ID for fetching events during execution
		managerStepMetadata := map[string]interface{}{
			"current_step":        i + 1,
			"current_step_name":   step.Name,
			"current_step_type":   step.Type.String(),
			"current_step_status": "running",
			"current_step_id":     stepID,
			"total_steps":         len(jobDef.Steps),
			"step_job_ids":        stepJobIDs, // Include for UI to fetch step events
		}
		if err := o.jobManager.UpdateJobMetadata(ctx, managerID, managerStepMetadata); err != nil {
			// Log but continue
		}

		// Publish step starting event
		if o.eventService != nil {
			payload := map[string]interface{}{
				"job_id":       managerID,
				"step_id":      stepID,
				"job_name":     jobDef.Name,
				"step_index":   i,
				"step_name":    step.Name,
				"step_type":    step.Type.String(),
				"current_step": i + 1,
				"total_steps":  len(jobDef.Steps),
				"step_status":  "running",
				"timestamp":    time.Now().Format(time.RFC3339),
			}
			event := interfaces.Event{
				Type:    interfaces.EventJobProgress,
				Payload: payload,
			}
			go func() {
				if err := o.eventService.Publish(ctx, event); err != nil {
					// Log but don't fail
				}
			}()

			// Also publish job_update event for direct UI tree status sync (step starting)
			// This matches the pattern used for step completion (line ~682)
			jobUpdatePayload := map[string]interface{}{
				"context":   "job_step",
				"job_id":    managerID,
				"step_name": step.Name,
				"status":    "running",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			jobUpdateEvent := interfaces.Event{
				Type:    interfaces.EventJobUpdate,
				Payload: jobUpdatePayload,
			}
			go func() {
				if err := o.eventService.Publish(ctx, jobUpdateEvent); err != nil {
					// Log but don't fail
				}
			}()
		}

		// Resolve placeholders in step config
		resolvedStep := step
		if step.Config != nil && o.kvStorage != nil {
			resolvedStep.Config = o.resolvePlaceholders(ctx, step.Config)
		}

		// Log step starting to the step job (which has step_name in metadata for UI filtering)
		stepStartLog := fmt.Sprintf("Starting Step %d/%d: %s", i+1, len(jobDef.Steps), step.Name)
		o.jobManager.AddJobLog(ctx, stepID, "info", stepStartLog)

		// Phase 1: Initialize step worker to assess work
		o.jobManager.AddJobLogWithPhase(ctx, stepID, "info", "Initializing worker...", "", "init")
		initResult, err := o.stepManager.Init(ctx, resolvedStep, *jobDef)
		if err != nil {
			o.jobManager.AddJobLogWithPhase(ctx, managerID, "error", fmt.Sprintf("Step %s init failed: %v", step.Name, err), "", "init")
			o.jobManager.AddJobLogWithPhase(ctx, stepID, "error", fmt.Sprintf("Init failed: %v", err), "", "init")
			o.jobManager.SetJobError(ctx, managerID, err.Error())
			o.jobManager.UpdateJobStatus(ctx, stepID, "failed")

			// Publish step_progress event so UI gets refresh trigger with finished=true
			if o.eventService != nil {
				stepProgressPayload := map[string]interface{}{
					"step_id":    stepID,
					"manager_id": managerID,
					"step_name":  step.Name,
					"status":     "failed",
					"timestamp":  time.Now().Format(time.RFC3339),
				}
				stepProgressEvent := interfaces.Event{
					Type:    interfaces.EventStepProgress,
					Payload: stepProgressPayload,
				}
				go func() {
					if err := o.eventService.Publish(ctx, stepProgressEvent); err != nil {
						// Log but don't fail
					}
				}()
			}

			// Store step statistics with failed status for UI display
			stepStats[i] = map[string]interface{}{
				"step_index":     i,
				"step_id":        stepID,
				"step_name":      step.Name,
				"step_type":      step.Type.String(),
				"child_count":    0,
				"document_count": 0,
				"status":         "failed",
			}
			// Update manager metadata with failed step progress
			failedStepMetadata := map[string]interface{}{
				"current_step":        i + 1,
				"current_step_name":   step.Name,
				"current_step_type":   step.Type.String(),
				"current_step_status": "failed",
				"current_step_id":     stepID,
				"step_stats":          stepStats[:i+1],
				"step_job_ids":        stepJobIDs, // Include for UI to fetch step events
			}
			o.jobManager.UpdateJobMetadata(ctx, managerID, failedStepMetadata)

			if step.OnError == models.ErrorStrategyFail {
				return managerID, fmt.Errorf("step %s init failed: %w", step.Name, err)
			}
			lastValidationError = fmt.Sprintf("Step %s init failed: %v", step.Name, err)
			continue
		}

		// Log init result for visibility
		initLogMsg := fmt.Sprintf("Worker initialized: %d work items, strategy=%s",
			initResult.TotalCount, initResult.Strategy)
		o.jobManager.AddJobLogWithPhase(ctx, stepID, "info", initLogMsg, "", "init")

		// Phase 2: Create jobs based on init result
		// Execute step via StepManager, passing the init result
		o.logger.Debug().
			Str("step_name", step.Name).
			Str("step_id", stepID).
			Msg("[orchestrator] Calling StepManager.Execute")

		// Check if worker returns child jobs BEFORE calling Execute
		// This ensures "Spawning child jobs" log appears before child job completion logs
		preExecWorker := o.stepManager.GetWorker(models.WorkerType(step.Type))
		if preExecWorker != nil && preExecWorker.ReturnsChildJobs() {
			o.jobManager.AddJobLogWithPhase(ctx, managerID, "info", fmt.Sprintf("Step %s spawning child jobs...", step.Name), "", "run")
			o.jobManager.AddJobLogWithPhase(ctx, stepID, "info", "Spawning child jobs...", "", "run")
		}

		childJobID, err := o.stepManager.Execute(ctx, resolvedStep, *jobDef, stepID, initResult)

		// CRITICAL: This log MUST appear immediately after StepManager.Execute returns
		// If this log doesn't appear, the goroutine is blocked or crashed
		o.logger.Info().
			Str("step_name", step.Name).
			Str("step_id", stepID).
			Str("child_job_id", childJobID).
			Bool("has_error", err != nil).
			Msg("[orchestrator] StepManager.Execute returned - CHECKPOINT 1")

		o.logger.Debug().
			Str("step_name", step.Name).
			Str("step_id", stepID).
			Str("child_job_id", childJobID).
			Err(err).
			Msg("[orchestrator] StepManager.Execute returned")

		if err != nil {
			o.jobManager.AddJobLogWithPhase(ctx, managerID, "error", fmt.Sprintf("Step %s failed: %v", step.Name, err), "", "run")
			o.jobManager.AddJobLogWithPhase(ctx, stepID, "error", fmt.Sprintf("Failed: %v", err), "", "run")
			o.jobManager.SetJobError(ctx, managerID, err.Error())
			o.jobManager.UpdateJobStatus(ctx, stepID, "failed")

			// Publish step_progress event so UI gets refresh trigger with finished=true
			if o.eventService != nil {
				stepProgressPayload := map[string]interface{}{
					"step_id":    stepID,
					"manager_id": managerID,
					"step_name":  step.Name,
					"status":     "failed",
					"timestamp":  time.Now().Format(time.RFC3339),
				}
				stepProgressEvent := interfaces.Event{
					Type:    interfaces.EventStepProgress,
					Payload: stepProgressPayload,
				}
				go func() {
					if err := o.eventService.Publish(ctx, stepProgressEvent); err != nil {
						// Log but don't fail
					}
				}()
			}

			// Store step statistics with failed status for UI display
			stepStats[i] = map[string]interface{}{
				"step_index":     i,
				"step_id":        stepID,
				"step_name":      step.Name,
				"step_type":      step.Type.String(),
				"child_count":    0,
				"document_count": 0,
				"status":         "failed",
			}
			// Update manager metadata with failed step progress
			failedStepMetadata := map[string]interface{}{
				"current_step":        i + 1,
				"current_step_name":   step.Name,
				"current_step_type":   step.Type.String(),
				"current_step_status": "failed",
				"current_step_id":     stepID,
				"step_stats":          stepStats[:i+1],
				"step_job_ids":        stepJobIDs, // Include for UI to fetch step events
			}
			o.jobManager.UpdateJobMetadata(ctx, managerID, failedStepMetadata)

			if step.OnError == models.ErrorStrategyFail {
				return managerID, fmt.Errorf("step %s failed: %w", step.Name, err)
			}

			// Check error tolerance
			if jobDef.ErrorTolerance != nil {
				shouldStop, _ := o.checkErrorTolerance(ctx, managerID, jobDef.ErrorTolerance)
				if shouldStop {
					o.jobManager.UpdateJobStatus(ctx, managerID, "failed")
					return managerID, fmt.Errorf("execution stopped: error tolerance threshold exceeded")
				}
			}
			// If validation failed in StepManager.Execute, it returns error, so we handle it here.
			// But StepManager.Execute also validates config.
			// We need to check if it was a validation error or execution error?
			// StepManager.Execute returns error for both.
			// We'll treat it as failure.
			// We need to track lastValidationError if we want to replicate the logic exactly,
			// but StepManager.Execute doesn't distinguish easily.
			// However, StepManager.Execute calls ValidateConfig first.
			lastValidationError = fmt.Sprintf("Step %s failed: %v", step.Name, err)
			continue
		}

		// Check if worker returns child jobs
		// We need to get the worker to check ReturnsChildJobs
		// StepManager doesn't expose the worker directly in Execute.
		// We should probably add ReturnsChildJobs to StepManager or check it here.
		// Or we can check if childJobID is returned?
		// Worker.CreateJobs returns jobID.
		// If ReturnsChildJobs is true, we expect child jobs.
		// We can use StepManager.GetWorker to check.
		o.logger.Debug().
			Str("step_name", step.Name).
			Str("step_type", step.Type.String()).
			Str("child_job_id", childJobID).
			Msg("[orchestrator] StepManager.Execute returned, checking worker type")

		worker := o.stepManager.GetWorker(models.WorkerType(step.Type))
		returnsChildJobs := false
		if worker != nil {
			returnsChildJobs = worker.ReturnsChildJobs()
			o.logger.Debug().
				Str("step_name", step.Name).
				Bool("returns_child_jobs", returnsChildJobs).
				Msg("[orchestrator] Worker found, checking ReturnsChildJobs")
		} else {
			o.logger.Warn().
				Str("step_name", step.Name).
				Str("step_type", step.Type.String()).
				Msg("[orchestrator] Worker not found for step type")
		}

		// CHECKPOINT 2: Log after worker check
		o.logger.Info().
			Str("step_name", step.Name).
			Bool("returns_child_jobs", returnsChildJobs).
			Bool("worker_found", worker != nil).
			Msg("[orchestrator] Worker check complete - CHECKPOINT 2")

		// Track whether we waited for children synchronously (vs async StepMonitor)
		childrenWaitedSynchronously := false

		if returnsChildJobs {
			hasChildJobs = true

			// Check if children are already complete (worker waited internally, e.g., AgentWorker with pollJobCompletion)
			initialStats, err := o.jobManager.GetJobChildStats(ctx, []string{stepID})
			if err != nil {
				o.logger.Warn().Err(err).Str("step_id", stepID).Msg("Failed to get initial child stats")
			}
			initialChildStats := initialStats[stepID]

			// If children are already all complete, skip the wait loop entirely
			if initialChildStats != nil && initialChildStats.PendingChildren == 0 && initialChildStats.RunningChildren == 0 && initialChildStats.CompletedChildren > 0 {
				o.jobManager.AddJobLogWithPhase(ctx, stepID, "info",
					fmt.Sprintf("All child jobs completed (%d completed, %d failed) - worker waited internally",
						initialChildStats.CompletedChildren, initialChildStats.FailedChildren), "", "run")
				childrenWaitedSynchronously = true
			} else {
				// Log that we're waiting for child jobs
				o.jobManager.AddJobLogWithPhase(ctx, stepID, "info", "Waiting for child jobs to complete...", "", "run")

				waitTimeout := 30 * time.Minute // Default timeout for waiting
				if jobDef.Timeout != "" {
					if parsedTimeout, err := time.ParseDuration(jobDef.Timeout); err == nil {
						waitTimeout = parsedTimeout
					}
				}

				waitStart := time.Now()
				pollInterval := 500 * time.Millisecond
				lastLoggedStats := ""
				lastProgressPublish := time.Now()
				progressPublishInterval := 2 * time.Second // Match unified aggregator threshold

				for {
					// Check context cancellation
					select {
					case <-ctx.Done():
						o.jobManager.AddJobLogWithPhase(ctx, stepID, "error", "Context cancelled while waiting for child jobs", "", "run")
						return managerID, ctx.Err()
					default:
					}

					// Check timeout
					if time.Since(waitStart) > waitTimeout {
						o.jobManager.AddJobLogWithPhase(ctx, stepID, "error", fmt.Sprintf("Timeout waiting for child jobs after %v", waitTimeout), "", "run")
						return managerID, fmt.Errorf("timeout waiting for child jobs of step %s", step.Name)
					}

					// Get current child stats
					stats, err := o.jobManager.GetJobChildStats(ctx, []string{stepID})
					if err != nil {
						o.logger.Warn().Err(err).Str("step_id", stepID).Msg("Failed to get child stats while waiting")
						time.Sleep(pollInterval)
						continue
					}

					childStats := stats[stepID]
					if childStats == nil {
						time.Sleep(pollInterval)
						continue
					}

					// Log progress periodically (only when stats change)
					currentStats := fmt.Sprintf("%d pending, %d running, %d completed, %d failed",
						childStats.PendingChildren, childStats.RunningChildren,
						childStats.CompletedChildren, childStats.FailedChildren)
					if currentStats != lastLoggedStats {
						o.jobManager.AddJobLogWithPhase(ctx, stepID, "info",
							fmt.Sprintf("Child jobs: %s", currentStats), "", "run")
						lastLoggedStats = currentStats
					}

					// Publish step_progress event periodically so UI receives refresh triggers
					// This enables real-time step event display during synchronous wait
					if time.Since(lastProgressPublish) >= progressPublishInterval && o.eventService != nil {
						stepProgressPayload := map[string]interface{}{
							"step_id":        stepID,
							"manager_id":     managerID,
							"step_name":      step.Name,
							"status":         "running",
							"total_jobs":     childStats.ChildCount,
							"pending_jobs":   childStats.PendingChildren,
							"running_jobs":   childStats.RunningChildren,
							"completed_jobs": childStats.CompletedChildren,
							"failed_jobs":    childStats.FailedChildren,
							"timestamp":      time.Now().Format(time.RFC3339),
						}
						stepProgressEvent := interfaces.Event{
							Type:    interfaces.EventStepProgress,
							Payload: stepProgressPayload,
						}
						go func() {
							if err := o.eventService.Publish(ctx, stepProgressEvent); err != nil {
								// Log but don't fail
							}
						}()
						lastProgressPublish = time.Now()
					}

					// Check if all children are in terminal state
					if childStats.PendingChildren == 0 && childStats.RunningChildren == 0 {
						o.jobManager.AddJobLogWithPhase(ctx, stepID, "info",
							fmt.Sprintf("All child jobs completed (%d completed, %d failed) in %v",
								childStats.CompletedChildren, childStats.FailedChildren, time.Since(waitStart)), "", "run")
						childrenWaitedSynchronously = true
						break
					}

					time.Sleep(pollInterval)
				}
			}

			// CHECKPOINT 3: After wait loop or immediate completion check
			o.logger.Info().
				Str("step_name", step.Name).
				Str("step_id", stepID).
				Bool("children_waited_synchronously", childrenWaitedSynchronously).
				Msg("[orchestrator] Wait loop completed - CHECKPOINT 3")
		} else {
			o.jobManager.AddJobLogWithPhase(ctx, managerID, "info", fmt.Sprintf("Step %s completed", step.Name), "", "run")
			o.jobManager.AddJobLogWithPhase(ctx, stepID, "info", fmt.Sprintf("Completed (job: %s)", childJobID), "", "run")
		}

		// Update manager progress
		if err := o.jobManager.UpdateJobProgress(ctx, managerID, i+1, len(jobDef.Steps)); err != nil {
			// Log warning but continue
		}

		// Get child stats for this step
		var stepChildCount int
		if stats, err := o.jobManager.GetJobChildStats(ctx, []string{stepID}); err == nil {
			if s := stats[stepID]; s != nil {
				stepChildCount = s.ChildCount
			}
		}

		// Get document count AFTER step execution
		docCountAfter, _ := o.jobManager.GetDocumentCount(ctx, managerID)

		// Calculate documents created by this step
		stepDocCount := docCountAfter - docCountBefore

		// Determine step status
		// If we waited synchronously for children, the step is completed.
		// Only use "spawned" status if we're using async StepMonitor (not waiting inline).
		stepStatus := "completed"
		o.logger.Debug().
			Str("phase", "orchestrator").
			Str("step_id", stepID).
			Bool("returns_child_jobs", returnsChildJobs).
			Int("step_child_count", stepChildCount).
			Bool("step_monitor_nil", stepMonitor == nil).
			Bool("children_waited_synchronously", childrenWaitedSynchronously).
			Msg("Determining step status for step monitor")

		o.jobManager.AddJobLogWithPhase(ctx, managerID, "info", fmt.Sprintf("Step status check: returns_child_jobs=%v, step_child_count=%d, children_waited=%v",
			returnsChildJobs, stepChildCount, childrenWaitedSynchronously), "", "orchestrator")

		// Only set status to "spawned" if we're NOT waiting synchronously
		// When childrenWaitedSynchronously is true, children already completed so status should be "completed"
		if returnsChildJobs && stepChildCount > 0 && !childrenWaitedSynchronously {
			stepStatus = "spawned"
		}

		// Store step statistics for UI with determined status
		stepStats[i] = map[string]interface{}{
			"step_index":     i,
			"step_id":        stepID,
			"step_name":      step.Name,
			"step_type":      step.Type.String(),
			"child_count":    stepChildCount,
			"document_count": stepDocCount,
			"status":         stepStatus,
		}

		// Update step job status
		if stepStatus == "completed" {
			o.jobManager.UpdateJobStatus(ctx, stepID, "completed")
			o.jobManager.SetJobFinished(ctx, stepID)

			// CHECKPOINT 4: After step status update
			o.logger.Info().
				Str("step_name", step.Name).
				Str("step_id", stepID).
				Str("step_status", stepStatus).
				Msg("[orchestrator] Step status updated - CHECKPOINT 4")

			// Publish step_progress event so UI gets refresh trigger with finished=true
			// This is critical for steps that complete synchronously (no StepMonitor)
			if o.eventService != nil {
				stepProgressPayload := map[string]interface{}{
					"step_id":    stepID,
					"manager_id": managerID,
					"step_name":  step.Name,
					"status":     "completed",
					"timestamp":  time.Now().Format(time.RFC3339),
				}
				stepProgressEvent := interfaces.Event{
					Type:    interfaces.EventStepProgress,
					Payload: stepProgressPayload,
				}
				go func() {
					if err := o.eventService.Publish(ctx, stepProgressEvent); err != nil {
						// Log but don't fail
					}
				}()

				// Also publish job_update event for direct UI status sync (bypasses throttling)
				// This is critical for fast-completing steps that don't use StepMonitor
				jobUpdatePayload := map[string]interface{}{
					"context":      "job_step",
					"job_id":       managerID,
					"step_name":    step.Name,
					"status":       "completed",
					"refresh_logs": true,
					"timestamp":    time.Now().Format(time.RFC3339),
				}
				jobUpdateEvent := interfaces.Event{
					Type:    interfaces.EventJobUpdate,
					Payload: jobUpdatePayload,
				}
				go func() {
					if err := o.eventService.Publish(ctx, jobUpdateEvent); err != nil {
						// Log but don't fail
					}
				}()
			}
		} else if stepStatus == "spawned" && stepMonitor != nil {
			stepQueueJob := &models.QueueJob{
				ID:        stepID,
				ParentID:  &managerID,
				ManagerID: &managerID,
				Type:      string(models.JobTypeStep),
				Name:      step.Name,
				Config:    stepConfig,
				Metadata:  stepJobMetadata,
				CreatedAt: time.Now(),
				Depth:     1,
			}
			stepMonitor.StartMonitoring(ctx, stepQueueJob)
			o.jobManager.AddJobLogWithPhase(ctx, stepID, "info", "Step monitor started for spawned children", "", "orchestrator")
		}

		// Update manager metadata with step progress
		// Include step_job_ids so UI can find step job IDs immediately (not just at end)
		managerCompletedMetadata := map[string]interface{}{
			"current_step":        i + 1,
			"current_step_name":   step.Name,
			"current_step_type":   step.Type.String(),
			"current_step_status": stepStatus,
			"current_step_id":     stepID,
			"completed_steps":     i + 1,
			"step_stats":          stepStats[:i+1],
			"step_job_ids":        stepJobIDs, // Include for UI to fetch step events
		}
		if err := o.jobManager.UpdateJobMetadata(ctx, managerID, managerCompletedMetadata); err != nil {
			// Log but continue
		}

		// Publish step progress event
		if o.eventService != nil {
			payload := map[string]interface{}{
				"job_id":           managerID,
				"step_id":          stepID,
				"job_name":         jobDef.Name,
				"step_index":       i,
				"step_name":        step.Name,
				"step_type":        step.Type.String(),
				"current_step":     i + 1,
				"total_steps":      len(jobDef.Steps),
				"step_status":      stepStatus,
				"step_child_count": stepChildCount,
				"timestamp":        time.Now().Format(time.RFC3339),
			}
			event := interfaces.Event{
				Type:    interfaces.EventJobProgress,
				Payload: payload,
			}
			go func() {
				if err := o.eventService.Publish(ctx, event); err != nil {
					// Log but don't fail
				}
			}()
		}
	}

	// Always save step_job_ids metadata so UI can fetch step events on page refresh
	// This must be saved regardless of whether steps have child jobs
	stepIDsMetadata := map[string]interface{}{
		"step_job_ids": stepJobIDs,
	}
	if err := o.jobManager.UpdateJobMetadata(ctx, managerID, stepIDsMetadata); err != nil {
		// Log but continue
	}

	// Handle completion
	if hasChildJobs && jobMonitor != nil {
		o.jobManager.AddJobLogWithPhase(ctx, managerID, "info", "Steps have child jobs - starting manager job monitoring", "", "orchestrator")

		managerQueueJob := &models.QueueJob{
			ID:        managerID,
			ParentID:  nil,
			ManagerID: nil,
			Type:      string(models.JobTypeManager),
			Name:      jobDef.Name,
			Config:    jobDefConfig,
			Metadata:  managerMetadata,
			CreatedAt: time.Now(),
			Depth:     0,
		}

		jobMonitor.StartMonitoring(ctx, managerQueueJob)
	} else {
		if lastValidationError != "" {
			o.jobManager.AddJobLog(ctx, managerID, "error", "Job failed: "+lastValidationError)
			o.jobManager.SetJobError(ctx, managerID, lastValidationError)
			o.jobManager.UpdateJobStatus(ctx, managerID, "failed")
			o.jobManager.SetJobFinished(ctx, managerID)
		} else {
			o.jobManager.AddJobLog(ctx, managerID, "info", "Job completed (no child jobs)")
			o.jobManager.UpdateJobStatus(ctx, managerID, "completed")
			o.jobManager.SetJobFinished(ctx, managerID)
		}
	}

	return managerID, nil
}

// resolvePlaceholders recursively resolves {key-name} placeholders in step config values
func (o *Orchestrator) resolvePlaceholders(ctx context.Context, config map[string]interface{}) map[string]interface{} {
	if config == nil || o.kvStorage == nil {
		return config
	}

	resolved := make(map[string]interface{})
	for key, value := range config {
		resolved[key] = o.resolveValue(ctx, value)
	}
	return resolved
}

// resolveValue recursively resolves placeholders in a single value
func (o *Orchestrator) resolveValue(ctx context.Context, value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		if len(v) > 2 && v[0] == '{' && v[len(v)-1] == '}' {
			keyName := v[1 : len(v)-1]
			kvValue, err := o.kvStorage.Get(ctx, keyName)
			if err == nil && kvValue != "" {
				return kvValue
			}
		}
		return v
	case map[string]interface{}:
		return o.resolvePlaceholders(ctx, v)
	case []interface{}:
		resolved := make([]interface{}, len(v))
		for i, item := range v {
			resolved[i] = o.resolveValue(ctx, item)
		}
		return resolved
	default:
		return v
	}
}

// checkErrorTolerance checks if the error tolerance threshold has been exceeded
func (o *Orchestrator) checkErrorTolerance(ctx context.Context, parentJobID string, tolerance *models.ErrorTolerance) (bool, error) {
	if tolerance == nil || tolerance.MaxChildFailures == 0 {
		return false, nil
	}

	failedCount, err := o.jobManager.GetFailedChildCount(ctx, parentJobID)
	if err != nil {
		return false, fmt.Errorf("failed to query failed job count: %w", err)
	}

	if failedCount >= tolerance.MaxChildFailures {
		switch tolerance.FailureAction {
		case "stop_all":
			return true, nil
		default:
			return false, nil
		}
	}

	return false, nil
}
