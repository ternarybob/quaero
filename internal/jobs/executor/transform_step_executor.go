package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
)

// TransformStepExecutor executes transform steps in job definitions
// Transforms HTML content to markdown using the transform service
type TransformStepExecutor struct {
	transformService interfaces.TransformService
	jobManager       *jobs.Manager
	logger           arbor.ILogger
}

// NewTransformStepExecutor creates a new transform step executor
func NewTransformStepExecutor(transformService interfaces.TransformService, jobManager *jobs.Manager, logger arbor.ILogger) *TransformStepExecutor {
	return &TransformStepExecutor{
		transformService: transformService,
		jobManager:       jobManager,
		logger:           logger,
	}
}

// ExecuteStep executes a transform step for the given sources
// This is a synchronous operation that directly transforms HTML to markdown
// Returns a placeholder job ID since transforms don't create async jobs
func (e *TransformStepExecutor) ExecuteStep(ctx context.Context, step models.JobStep, sources []string, parentJobID string) (string, error) {
	e.logger.Info().
		Str("step_name", step.Name).
		Str("parent_job_id", parentJobID).
		Int("source_count", len(sources)).
		Msg("Executing transform step")

	// Generate a job ID for tracking (even though this is synchronous)
	jobID := uuid.New().String()

	// Parse step config
	config, err := parseTransformConfig(step.Config)
	if err != nil {
		e.logger.Error().Err(err).Msg("Failed to parse transform config")
		return "", fmt.Errorf("invalid transform config: %w", err)
	}

	e.logger.Debug().
		Str("input_format", config.InputFormat).
		Str("output_format", config.OutputFormat).
		Bool("validate_html", config.ValidateHTML).
		Msg("Transform config parsed")

	// Validate configuration
	if config.InputFormat != "html" {
		return "", fmt.Errorf("unsupported input format: %s (only 'html' is supported)", config.InputFormat)
	}
	if config.OutputFormat != "markdown" {
		return "", fmt.Errorf("unsupported output format: %s (only 'markdown' is supported)", config.OutputFormat)
	}

	// For now, transform steps are informational only
	// Future: Could process actual HTML content from sources
	// Example: Read HTML from documents table, transform, and save back

	e.logger.Info().
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Msg("Transform step completed (placeholder implementation)")

	return jobID, nil
}

// GetStepType returns the step type this executor handles
func (e *TransformStepExecutor) GetStepType() string {
	return "transform"
}

// TransformConfig represents configuration for transform steps
type TransformConfig struct {
	InputFormat  string `json:"input_format"`  // e.g., "html"
	OutputFormat string `json:"output_format"` // e.g., "markdown"
	BaseURL      string `json:"base_url"`      // Base URL for resolving relative links
	ValidateHTML bool   `json:"validate_html"` // Whether to validate HTML before transform
}

// parseTransformConfig parses transform config from generic map
func parseTransformConfig(configMap map[string]interface{}) (*TransformConfig, error) {
	// Marshal to JSON and back to struct for type-safe conversion
	jsonBytes, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	var config TransformConfig
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Set defaults
	if config.InputFormat == "" {
		config.InputFormat = "html"
	}
	if config.OutputFormat == "" {
		config.OutputFormat = "markdown"
	}

	return &config, nil
}
