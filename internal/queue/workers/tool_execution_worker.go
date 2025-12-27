// -----------------------------------------------------------------------
// ToolExecutionWorker - Executes individual tool calls as queue jobs
// Created by orchestrator as queue citizens with independent status tracking
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ToolExecutionWorker implements JobWorker for tool_execution job type.
// It receives tool execution requests from the orchestrator and executes them
// via the StepManager, making each tool call a visible queue citizen.
type ToolExecutionWorker struct {
	stepManager     interfaces.StepManager
	documentStorage interfaces.DocumentStorage
	searchService   interfaces.SearchService
	jobMgr          *queue.Manager
	logger          arbor.ILogger
}

// Compile-time assertion: ToolExecutionWorker implements JobWorker interface
var _ interfaces.JobWorker = (*ToolExecutionWorker)(nil)

// NewToolExecutionWorker creates a new tool execution worker
func NewToolExecutionWorker(
	stepManager interfaces.StepManager,
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	jobMgr *queue.Manager,
	logger arbor.ILogger,
) *ToolExecutionWorker {
	return &ToolExecutionWorker{
		stepManager:     stepManager,
		documentStorage: documentStorage,
		searchService:   searchService,
		jobMgr:          jobMgr,
		logger:          logger,
	}
}

// GetWorkerType returns the job type this worker handles
func (w *ToolExecutionWorker) GetWorkerType() string {
	return string(models.JobTypeToolExecution)
}

// Validate validates that the queued job is compatible with this worker
func (w *ToolExecutionWorker) Validate(job *models.QueueJob) error {
	if job.Type != string(models.JobTypeToolExecution) {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeToolExecution, job.Type)
	}

	// Check required config fields
	if job.Config == nil {
		return fmt.Errorf("job config is required")
	}

	if _, ok := job.Config["tool_name"].(string); !ok {
		return fmt.Errorf("tool_name is required in job config")
	}

	if _, ok := job.Config["worker_type"].(string); !ok {
		return fmt.Errorf("worker_type is required in job config")
	}

	return nil
}

// Execute processes a tool execution job
func (w *ToolExecutionWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	// Extract job configuration
	toolName, _ := job.Config["tool_name"].(string)
	workerType, _ := job.Config["worker_type"].(string)
	planStepID, _ := job.Config["plan_step_id"].(string)
	jobDefID, _ := job.Config["job_def_id"].(string)

	w.logger.Info().
		Str("job_id", job.ID).
		Str("tool_name", toolName).
		Str("worker_type", workerType).
		Str("plan_step_id", planStepID).
		Msg("Starting tool execution")

	// Update job status to running
	if w.jobMgr != nil {
		if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, string(models.JobStatusRunning)); err != nil {
			w.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to update job status to running")
		}
		w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Executing tool: %s (worker: %s)", toolName, workerType))
	}

	// Extract params - handle both map[string]interface{} and nested structures
	var params map[string]interface{}
	if p, ok := job.Config["params"].(map[string]interface{}); ok {
		params = p
	} else {
		params = make(map[string]interface{})
	}

	// Extract tool config
	var toolConfig map[string]interface{}
	if tc, ok := job.Config["tool_config"].(map[string]interface{}); ok {
		toolConfig = tc
	} else {
		toolConfig = make(map[string]interface{})
	}

	// Build step config from tool config and params
	stepConfig := make(map[string]interface{})

	// Copy tool-specific config (excluding name, description, worker)
	for k, v := range toolConfig {
		if k != "name" && k != "description" && k != "worker" {
			stepConfig[k] = v
		}
	}

	// Merge params (these override tool config)
	for k, v := range params {
		stepConfig[k] = v
	}

	// Create synthetic JobStep for the tool
	syntheticStep := models.JobStep{
		Name:        fmt.Sprintf("tool_%s_%s", toolName, planStepID),
		Type:        models.WorkerType(workerType),
		Description: fmt.Sprintf("Tool execution: %s", toolName),
		Config:      stepConfig,
		OnError:     models.ErrorStrategyContinue,
	}

	// Create minimal JobDefinition for context
	jobDef := models.JobDefinition{
		ID:   jobDefID,
		Name: fmt.Sprintf("Tool: %s", toolName),
	}

	// Execute via StepManager
	if w.stepManager == nil {
		err := fmt.Errorf("StepManager not configured - cannot execute tool")
		w.logger.Error().Err(err).Msg("Tool execution failed")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, job.ID, "error", err.Error())
			w.jobMgr.SetJobError(ctx, job.ID, err.Error())
		}
		return err
	}

	w.logger.Debug().
		Str("job_id", job.ID).
		Str("tool", toolName).
		Str("worker_type", workerType).
		Interface("step_config", stepConfig).
		Msg("Executing tool via StepManager")

	_, err := w.stepManager.Execute(ctx, syntheticStep, jobDef, job.ID, nil)
	if err != nil {
		w.logger.Error().Err(err).
			Str("job_id", job.ID).
			Str("tool", toolName).
			Msg("Tool execution failed")

		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("Tool execution failed: %v", err))
			w.jobMgr.SetJobError(ctx, job.ID, err.Error())
		}
		return err
	}

	// Try to find output documents created by the tool
	var output string
	if w.searchService != nil {
		searchOpts := interfaces.SearchOptions{
			Limit: 5,
		}

		docs, err := w.searchService.Search(ctx, syntheticStep.Name, searchOpts)
		if err == nil && len(docs) > 0 {
			var docSummaries []string
			for _, doc := range docs {
				summary := truncateString(doc.ContentMarkdown, 500)
				docSummaries = append(docSummaries, fmt.Sprintf("- %s: %s", doc.Title, summary))
			}
			if len(docSummaries) > 0 {
				outputJSON, _ := json.Marshal(docSummaries)
				output = string(outputJSON)
			}
		}
	}

	if output == "" {
		output = fmt.Sprintf("Tool %s executed successfully (worker: %s)", toolName, workerType)
	}

	// Update job metadata with output
	if w.jobMgr != nil {
		metadata := map[string]interface{}{
			"output":      output,
			"tool_name":   toolName,
			"worker_type": workerType,
		}
		if err := w.jobMgr.UpdateJobMetadata(ctx, job.ID, metadata); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to update job metadata with output")
		}

		w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Tool completed: %s", truncateString(output, 200)))
		if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, string(models.JobStatusCompleted)); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to update job status to completed")
		}
	}

	w.logger.Info().
		Str("job_id", job.ID).
		Str("tool", toolName).
		Msg("Tool execution completed successfully")

	return nil
}
