// -----------------------------------------------------------------------
// JobTemplateWorker - Executes job templates with variable substitution
// Loads templates from {exe}/job-templates/, applies variable replacements,
// and executes the resulting job definition.
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// JobTemplateOrchestrator defines the interface for executing job definitions.
// This is implemented by queue.Orchestrator.
type JobTemplateOrchestrator interface {
	ExecuteJobDefinition(ctx context.Context, jobDef *models.JobDefinition, jobMonitor interfaces.JobMonitor, stepMonitor interfaces.StepMonitor) (string, error)
}

// JobTemplateWorker executes job templates with variable substitution.
// Templates are loaded from {exe}/job-templates/ directory.
// Variables use {variable:key} syntax where variable is from step config.
type JobTemplateWorker struct {
	jobDefStorage interfaces.JobDefinitionStorage
	jobService    *jobs.Service
	orchestrator  JobTemplateOrchestrator
	jobMgr        *queue.Manager
	eventService  interfaces.EventService
	logger        arbor.ILogger
	templatesDir  string
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*JobTemplateWorker)(nil)

// NewJobTemplateWorker creates a new job template worker
func NewJobTemplateWorker(
	jobDefStorage interfaces.JobDefinitionStorage,
	jobService *jobs.Service,
	orchestrator JobTemplateOrchestrator,
	jobMgr *queue.Manager,
	eventService interfaces.EventService,
	logger arbor.ILogger,
	templatesDir string,
) *JobTemplateWorker {
	return &JobTemplateWorker{
		jobDefStorage: jobDefStorage,
		jobService:    jobService,
		orchestrator:  orchestrator,
		jobMgr:        jobMgr,
		eventService:  eventService,
		logger:        logger,
		templatesDir:  templatesDir,
	}
}

// GetType returns WorkerTypeJobTemplate
func (w *JobTemplateWorker) GetType() models.WorkerType {
	return models.WorkerTypeJobTemplate
}

// Init performs initialization for the job template step.
// Validates config and loads the template file.
// Supports global variables at job definition level that steps can inherit.
func (w *JobTemplateWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for job_template")
	}

	// Extract template name (required)
	template, ok := stepConfig["template"].(string)
	if !ok || template == "" {
		return nil, fmt.Errorf("'template' is required in step config")
	}

	// Build template file path
	templateFile := filepath.Join(w.templatesDir, template+".toml")
	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("template file not found: %s", templateFile)
	}

	// Extract variables - supports global (job-level) and step-level variables
	// Priority: step-level variables override job-level variables
	// If step has `variables = false`, skip variables entirely
	variablesRaw, hasStepVars := stepConfig["variables"]

	// Check if step explicitly opts out of variables
	if v, isBool := variablesRaw.(bool); isBool && !v {
		return nil, fmt.Errorf("step has variables disabled (variables = false)")
	}

	// If no step-level variables, check job definition config for global variables
	if !hasStepVars && jobDef.Config != nil {
		if globalVars, hasGlobalVars := jobDef.Config["variables"]; hasGlobalVars {
			variablesRaw = globalVars
			w.logger.Info().
				Str("step_name", step.Name).
				Msg("Using global variables from job definition config")
		}
	}

	if variablesRaw == nil {
		return nil, fmt.Errorf("'variables' array is required (at step or job level)")
	}

	variables, err := w.parseVariables(variablesRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid 'variables' format: %w", err)
	}

	if len(variables) == 0 {
		return nil, fmt.Errorf("'variables' array cannot be empty")
	}

	// Optional: execute in parallel or sequential
	parallel := false
	if p, ok := stepConfig["parallel"].(bool); ok {
		parallel = p
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("template", template).
		Int("variable_sets", len(variables)).
		Bool("parallel", parallel).
		Msg("Job template worker initialized")

	// Create work items for each variable set
	workItems := make([]interfaces.WorkItem, len(variables))
	for i, varSet := range variables {
		// Use stable identifier extraction (ticker > name > id > first string)
		identifier := getVariableIdentifier(varSet)

		workItems[i] = interfaces.WorkItem{
			ID:     fmt.Sprintf("template-%d-%s", i, identifier),
			Name:   fmt.Sprintf("Execute template for %s", identifier),
			Type:   "job_template_instance",
			Config: varSet,
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(variables),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"template":      template,
			"template_file": templateFile,
			"variables":     variables,
			"parallel":      parallel,
			"step_config":   stepConfig,
		},
	}, nil
}

// parseVariables converts the raw variables config to a slice of variable maps
func (w *JobTemplateWorker) parseVariables(raw interface{}) ([]map[string]interface{}, error) {
	var result []map[string]interface{}

	switch v := raw.(type) {
	case []interface{}:
		for i, item := range v {
			if varMap, ok := item.(map[string]interface{}); ok {
				result = append(result, varMap)
			} else {
				return nil, fmt.Errorf("variables[%d] is not a map", i)
			}
		}
	case []map[string]interface{}:
		result = v
	default:
		return nil, fmt.Errorf("variables must be an array of objects")
	}

	return result, nil
}

// getVariableIdentifier extracts a stable identifier from variable set.
// Priority: "ticker" > "name" > "id" > first string value.
// This avoids non-deterministic behavior from Go's random map iteration order.
func getVariableIdentifier(varSet map[string]interface{}) string {
	// Try well-known identifier keys in order of priority
	for _, key := range []string{"ticker", "name", "id"} {
		if val, ok := varSet[key]; ok {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	// Fallback: first string value found (for backward compatibility)
	for _, v := range varSet {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "unknown"
}

// templateJobResult holds the result of executing a single template instance
type templateJobResult struct {
	index      int
	identifier string
	jobID      string
	err        error
}

// CreateJobs loads the template, applies variable substitution, and executes each instance.
// Supports parallel execution when parallel=true is set in step config.
func (w *JobTemplateWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize job_template worker: %w", err)
		}
	}

	template, _ := initResult.Metadata["template"].(string)
	templateFile, _ := initResult.Metadata["template_file"].(string)
	variables, _ := initResult.Metadata["variables"].([]map[string]interface{})
	parallel, _ := initResult.Metadata["parallel"].(bool)

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("template", template).
		Int("instances", len(variables)).
		Str("step_id", stepID).
		Bool("parallel", parallel).
		Msg("Starting job template execution")

	if w.jobMgr != nil {
		mode := "sequential"
		if parallel {
			mode = "parallel"
		}
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Executing template '%s' with %d variable sets (%s)", template, len(variables), mode))
	}

	// Load template content
	templateContent, err := os.ReadFile(templateFile)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	// Prepare all job definitions first
	var preparedJobs []preparedJob

	for i, varSet := range variables {
		// Get identifier for logging (stable extraction: ticker > name > id)
		identifier := getVariableIdentifier(varSet)

		// Apply variable substitution to template
		processedContent := w.substituteTemplateVariables(string(templateContent), varSet)

		// Parse the processed TOML
		jobFile, err := jobs.ParseTOML([]byte(processedContent))
		if err != nil {
			w.logger.Error().Err(err).
				Str("identifier", identifier).
				Msg("Failed to parse processed template")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to parse template for %s: %v", identifier, err))
			}
			continue
		}

		// Convert to job definition
		templatedJobDef, err := jobFile.ToJobDefinition()
		if err != nil {
			w.logger.Error().Err(err).
				Str("identifier", identifier).
				Msg("Failed to convert template to job definition")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to convert template for %s: %v", identifier, err))
			}
			continue
		}

		// Generate unique ID if not set or if it conflicts
		if templatedJobDef.ID == "" {
			templatedJobDef.ID = fmt.Sprintf("template-%s-%s", template, uuid.New().String()[:8])
		}

		// Set metadata
		templatedJobDef.CreatedAt = time.Now()
		templatedJobDef.UpdatedAt = time.Now()
		templatedJobDef.JobType = models.JobOwnerTypeSystem
		templatedJobDef.Description = fmt.Sprintf("%s\n\n[Generated from template '%s' for %s]",
			templatedJobDef.Description, template, identifier)

		preparedJobs = append(preparedJobs, preparedJob{
			index:      i,
			identifier: identifier,
			jobDef:     templatedJobDef,
		})
	}

	if len(preparedJobs) == 0 {
		return "", fmt.Errorf("failed to prepare any template instances")
	}

	// Check orchestrator availability
	if w.orchestrator == nil {
		w.logger.Error().Msg("Orchestrator not available")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", "Orchestrator not configured")
		}
		return "", fmt.Errorf("orchestrator not configured")
	}

	var results []templateJobResult

	if parallel {
		// Parallel execution: spawn all jobs concurrently
		results = w.executeParallel(ctx, stepID, preparedJobs, len(variables))
	} else {
		// Sequential execution: run one at a time (original behavior)
		results = w.executeSequential(ctx, stepID, preparedJobs, len(variables))
	}

	// Count successes and report results
	successCount := 0
	for _, result := range results {
		if result.err == nil {
			successCount++
		}
	}

	if successCount == 0 {
		return "", fmt.Errorf("failed to execute any template instances")
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Completed: %d/%d template instances executed", successCount, len(variables)))
	}

	return stepID, nil
}

// preparedJob holds a prepared job definition ready for execution
type preparedJob struct {
	index      int
	identifier string
	jobDef     *models.JobDefinition
}

// executeSequential runs template jobs one at a time (original behavior)
func (w *JobTemplateWorker) executeSequential(ctx context.Context, stepID string, preparedJobs []preparedJob, totalCount int) []templateJobResult {
	results := make([]templateJobResult, 0, len(preparedJobs))

	for _, pj := range preparedJobs {
		w.logger.Info().
			Int("index", pj.index).
			Str("identifier", pj.identifier).
			Msg("Processing template instance (sequential)")

		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("[%d/%d] Processing: %s", pj.index+1, totalCount, pj.identifier))
		}

		// Run the job definition
		executedJobID, err := w.orchestrator.ExecuteJobDefinition(ctx, pj.jobDef, nil, nil)
		if err != nil {
			w.logger.Error().Err(err).
				Str("identifier", pj.identifier).
				Str("job_def_id", pj.jobDef.ID).
				Msg("Failed to execute templated job")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to execute job for %s: %v", pj.identifier, err))
			}
			results = append(results, templateJobResult{
				index:      pj.index,
				identifier: pj.identifier,
				err:        err,
			})
			continue
		}

		w.logger.Info().
			Str("identifier", pj.identifier).
			Str("job_id", executedJobID).
			Msg("Successfully executed templated job")

		// Publish job spawn event to notify UI of new child job
		if w.eventService != nil {
			spawnEvent := interfaces.Event{
				Type: interfaces.EventJobSpawn,
				Payload: map[string]interface{}{
					"parent_job_id": stepID,
					"child_job_id":  executedJobID,
					"job_type":      "job_template",
					"name":          pj.identifier,
					"timestamp":     time.Now().Format(time.RFC3339),
				},
			}
			if pubErr := w.eventService.Publish(ctx, spawnEvent); pubErr != nil {
				w.logger.Warn().Err(pubErr).Str("child_job_id", executedJobID).Msg("Failed to publish job spawn event")
			}
		}

		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Completed job %s for %s", executedJobID[:8], pj.identifier))
		}

		results = append(results, templateJobResult{
			index:      pj.index,
			identifier: pj.identifier,
			jobID:      executedJobID,
		})
	}

	return results
}

// executeParallel spawns all template jobs concurrently and waits for completion.
// Jobs are created by ExecuteJobDefinition and appear in the UI as they start.
func (w *JobTemplateWorker) executeParallel(ctx context.Context, stepID string, preparedJobs []preparedJob, totalCount int) []templateJobResult {
	results := make([]templateJobResult, len(preparedJobs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	w.logger.Info().
		Int("job_count", len(preparedJobs)).
		Str("step_id", stepID).
		Msg("Spawning parallel template jobs")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Spawning %d parallel jobs...", len(preparedJobs)))
	}

	// Log all jobs that will be spawned
	for i, pj := range preparedJobs {
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("[%d/%d] Queuing: %s", i+1, totalCount, pj.identifier))
		}
	}

	// Execute all jobs in parallel goroutines
	for i, pj := range preparedJobs {
		wg.Add(1)
		go func(idx int, job preparedJob) {
			defer wg.Done()

			w.logger.Info().
				Int("index", job.index).
				Str("identifier", job.identifier).
				Msg("Starting parallel template job execution")

			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Starting: %s", job.identifier))
			}

			// Execute the job definition - this creates the manager job and executes it
			executedJobID, err := w.orchestrator.ExecuteJobDefinition(ctx, job.jobDef, nil, nil)

			// Publish job spawn event immediately after job creation (before acquiring lock)
			// This ensures the UI sees spawned jobs as soon as they're created
			if err == nil && w.eventService != nil {
				spawnEvent := interfaces.Event{
					Type: interfaces.EventJobSpawn,
					Payload: map[string]interface{}{
						"parent_job_id": stepID,
						"child_job_id":  executedJobID,
						"job_type":      "job_template",
						"name":          job.identifier,
						"timestamp":     time.Now().Format(time.RFC3339),
					},
				}
				if pubErr := w.eventService.Publish(ctx, spawnEvent); pubErr != nil {
					w.logger.Warn().Err(pubErr).Str("child_job_id", executedJobID).Msg("Failed to publish job spawn event")
				}
			}

			mu.Lock()
			if err != nil {
				w.logger.Error().Err(err).
					Str("identifier", job.identifier).
					Msg("Failed to execute parallel templated job")
				if w.jobMgr != nil {
					w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed: %s - %v", job.identifier, err))
				}
				results[idx] = templateJobResult{
					index:      job.index,
					identifier: job.identifier,
					err:        err,
				}
			} else {
				w.logger.Info().
					Str("identifier", job.identifier).
					Str("job_id", executedJobID).
					Msg("Parallel templated job completed")
				if w.jobMgr != nil {
					w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Completed: %s (job: %s)", job.identifier, executedJobID[:8]))
				}
				results[idx] = templateJobResult{
					index:      job.index,
					identifier: job.identifier,
					jobID:      executedJobID,
				}
			}
			mu.Unlock()
		}(i, pj)
	}

	// Wait for all parallel jobs to complete
	wg.Wait()

	w.logger.Info().
		Int("job_count", len(preparedJobs)).
		Str("step_id", stepID).
		Msg("All parallel template jobs completed")

	return results
}

// substituteTemplateVariables replaces {variable:key} patterns with actual values
// Also handles special transformations like {variable:key_lower} for lowercase
func (w *JobTemplateWorker) substituteTemplateVariables(content string, variables map[string]interface{}) string {
	// Pattern: {namespace:key} or {namespace:key_modifier}
	// Examples: {stock:ticker}, {stock:ticker_lower}, {stock:name}
	pattern := regexp.MustCompile(`\{([a-zA-Z0-9_]+):([a-zA-Z0-9_]+)\}`)

	result := pattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract namespace and key from match
		parts := pattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		// namespace := parts[1] // Not currently used - all variables in same namespace
		key := parts[2]

		// Check for modifiers (e.g., ticker_lower)
		modifier := ""
		if strings.HasSuffix(key, "_lower") {
			modifier = "lower"
			key = strings.TrimSuffix(key, "_lower")
		} else if strings.HasSuffix(key, "_upper") {
			modifier = "upper"
			key = strings.TrimSuffix(key, "_upper")
		}

		// Look up the value
		value, ok := variables[key]
		if !ok {
			w.logger.Warn().
				Str("key", key).
				Str("match", match).
				Msg("Template variable not found")
			return match // Keep original if not found
		}

		strValue := fmt.Sprintf("%v", value)

		// Apply modifier
		switch modifier {
		case "lower":
			strValue = strings.ToLower(strValue)
		case "upper":
			strValue = strings.ToUpper(strValue)
		}

		return strValue
	})

	return result
}

// ReturnsChildJobs returns false since ExecuteJobDefinition runs jobs synchronously.
// The child jobs complete within CreateJobs before it returns, so the orchestrator
// doesn't need to wait for them separately.
func (w *JobTemplateWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration.
// Note: Variables are NOT required at step level since they can be inherited
// from job definition's global [config] section. The Init method handles
// the full validation including global variable inheritance.
func (w *JobTemplateWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("job_template step requires config")
	}

	template, ok := step.Config["template"].(string)
	if !ok || template == "" {
		return fmt.Errorf("job_template step requires 'template' in config")
	}

	// Note: We don't require 'variables' here because they can be inherited
	// from job definition config (global variables). The Init method will
	// validate that variables exist either at step or job level.

	return nil
}
