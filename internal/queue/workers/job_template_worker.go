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
	logger arbor.ILogger,
	templatesDir string,
) *JobTemplateWorker {
	return &JobTemplateWorker{
		jobDefStorage: jobDefStorage,
		jobService:    jobService,
		orchestrator:  orchestrator,
		jobMgr:        jobMgr,
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

	// Extract variables array (required)
	// Format: variables = [{ ticker = "CBA", name = "Commonwealth Bank", industry = "banking" }]
	variablesRaw, ok := stepConfig["variables"]
	if !ok {
		return nil, fmt.Errorf("'variables' array is required in step config")
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
		// Use first key in varSet as identifier (e.g., ticker)
		var identifier string
		for _, v := range varSet {
			if s, ok := v.(string); ok {
				identifier = s
				break
			}
		}

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

// CreateJobs loads the template, applies variable substitution, and executes each instance.
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

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("template", template).
		Int("instances", len(variables)).
		Str("step_id", stepID).
		Msg("Starting job template execution")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Executing template '%s' with %d variable sets", template, len(variables)))
	}

	// Load template content
	templateContent, err := os.ReadFile(templateFile)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	// Execute each variable set
	successCount := 0
	for i, varSet := range variables {
		// Get identifier for logging
		var identifier string
		for _, v := range varSet {
			if s, ok := v.(string); ok {
				identifier = s
				break
			}
		}

		w.logger.Info().
			Int("index", i).
			Str("identifier", identifier).
			Msg("Processing template instance")

		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("[%d/%d] Processing: %s", i+1, len(variables), identifier))
		}

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
		// Store source info in description since the model doesn't have dedicated fields
		templatedJobDef.Description = fmt.Sprintf("%s\n\n[Generated from template '%s' for %s]",
			templatedJobDef.Description, template, identifier)

		// Execute the templated job using Orchestrator
		if w.orchestrator == nil {
			w.logger.Error().Msg("Orchestrator not available")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", "Orchestrator not configured")
			}
			continue
		}

		// Run the job definition (nil monitors since we track via parent step)
		executedJobID, err := w.orchestrator.ExecuteJobDefinition(ctx, templatedJobDef, nil, nil)
		if err != nil {
			w.logger.Error().Err(err).
				Str("identifier", identifier).
				Str("job_def_id", templatedJobDef.ID).
				Msg("Failed to execute templated job")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to execute job for %s: %v", identifier, err))
			}
			continue
		}

		w.logger.Info().
			Str("identifier", identifier).
			Str("job_id", executedJobID).
			Msg("Successfully executed templated job")

		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Started job %s for %s", executedJobID[:8], identifier))
		}

		successCount++
	}

	if successCount == 0 {
		return "", fmt.Errorf("failed to execute any template instances")
	}

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Completed: %d/%d template instances executed", successCount, len(variables)))
	}

	return stepID, nil
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

// ReturnsChildJobs returns true since we spawn child job executions
func (w *JobTemplateWorker) ReturnsChildJobs() bool {
	return true
}

// ValidateConfig validates step configuration
func (w *JobTemplateWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("job_template step requires config")
	}

	template, ok := step.Config["template"].(string)
	if !ok || template == "" {
		return fmt.Errorf("job_template step requires 'template' in config")
	}

	if _, ok := step.Config["variables"]; !ok {
		return fmt.Errorf("job_template step requires 'variables' in config")
	}

	return nil
}
