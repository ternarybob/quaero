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

	// Track step job IDs for monitoring
	stepJobIDs := make([]string, len(jobDef.Steps))

	// Execute steps sequentially
	for i, step := range jobDef.Steps {
		// Create step job (child of manager, parent of spawned jobs)
		stepID := uuid.New().String()
		stepJobIDs[i] = stepID

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
		managerStepMetadata := map[string]interface{}{
			"current_step":        i + 1,
			"current_step_name":   step.Name,
			"current_step_type":   step.Type.String(),
			"current_step_status": "running",
			"current_step_id":     stepID,
			"total_steps":         len(jobDef.Steps),
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
		o.jobManager.AddJobLog(ctx, stepID, "info", "Initializing worker...")
		initResult, err := o.stepManager.Init(ctx, resolvedStep, *jobDef)
		if err != nil {
			o.jobManager.AddJobLog(ctx, managerID, "error", fmt.Sprintf("Step %s init failed: %v", step.Name, err))
			o.jobManager.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Init failed: %v", err))
			o.jobManager.SetJobError(ctx, managerID, err.Error())
			o.jobManager.UpdateJobStatus(ctx, stepID, "failed")

			if step.OnError == models.ErrorStrategyFail {
				return managerID, fmt.Errorf("step %s init failed: %w", step.Name, err)
			}
			lastValidationError = fmt.Sprintf("Step %s init failed: %v", step.Name, err)
			continue
		}

		// Log init result for visibility
		initLogMsg := fmt.Sprintf("Worker initialized: %d work items, strategy=%s",
			initResult.TotalCount, initResult.Strategy)
		o.jobManager.AddJobLog(ctx, stepID, "info", initLogMsg)

		// Phase 2: Create jobs based on init result
		// Execute step via StepManager, passing the init result
		childJobID, err := o.stepManager.Execute(ctx, resolvedStep, *jobDef, stepID, initResult)
		if err != nil {
			o.jobManager.AddJobLog(ctx, managerID, "error", fmt.Sprintf("Step %s failed: %v", step.Name, err))
			o.jobManager.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed: %v", err))
			o.jobManager.SetJobError(ctx, managerID, err.Error())
			o.jobManager.UpdateJobStatus(ctx, stepID, "failed")

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
		worker := o.stepManager.GetWorker(models.WorkerType(step.Type))
		returnsChildJobs := false
		if worker != nil {
			returnsChildJobs = worker.ReturnsChildJobs()
		}

		if returnsChildJobs {
			hasChildJobs = true
			o.jobManager.AddJobLog(ctx, managerID, "info", fmt.Sprintf("Step %s spawned child jobs", step.Name))
			o.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Spawned child jobs (job: %s)", childJobID))
		} else {
			o.jobManager.AddJobLog(ctx, managerID, "info", fmt.Sprintf("Step %s completed", step.Name))
			o.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Completed (job: %s)", childJobID))
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

		// Store step statistics for UI
		stepStats[i] = map[string]interface{}{
			"step_index":     i,
			"step_id":        stepID,
			"step_name":      step.Name,
			"step_type":      step.Type.String(),
			"child_count":    stepChildCount,
			"document_count": stepDocCount,
		}

		// Determine step status
		stepStatus := "completed"
		o.logger.Debug().
			Str("step_id", stepID).
			Bool("returns_child_jobs", returnsChildJobs).
			Int("step_child_count", stepChildCount).
			Bool("step_monitor_nil", stepMonitor == nil).
			Msg("[orchestrator] Determining step status for step monitor")

		o.jobManager.AddJobLog(ctx, managerID, "info", fmt.Sprintf("Step status check: returns_child_jobs=%v, step_child_count=%d, step_monitor_nil=%v",
			returnsChildJobs, stepChildCount, stepMonitor == nil))

		if returnsChildJobs && stepChildCount > 0 {
			stepStatus = "spawned"
		}

		// Update step job status
		if stepStatus == "completed" {
			o.jobManager.UpdateJobStatus(ctx, stepID, "completed")
			o.jobManager.SetJobFinished(ctx, stepID)
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
			o.jobManager.AddJobLog(ctx, stepID, "info", "Step monitor started for spawned children")
		}

		// Update manager metadata with step progress
		managerCompletedMetadata := map[string]interface{}{
			"current_step":        i + 1,
			"current_step_name":   step.Name,
			"current_step_type":   step.Type.String(),
			"current_step_status": stepStatus,
			"current_step_id":     stepID,
			"completed_steps":     i + 1,
			"step_stats":          stepStats[:i+1],
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

	// Handle completion
	if hasChildJobs && jobMonitor != nil {
		o.jobManager.AddJobLog(ctx, managerID, "info", "Steps have child jobs - starting manager job monitoring")

		stepIDsMetadata := map[string]interface{}{
			"step_job_ids": stepJobIDs,
		}
		if err := o.jobManager.UpdateJobMetadata(ctx, managerID, stepIDsMetadata); err != nil {
			// Log but continue
		}

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
