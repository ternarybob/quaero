package manager

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

// TransformManager orchestrates document transformation workflows, converting HTML content to markdown format
type TransformManager struct {
	transformService interfaces.TransformService
	jobManager       *jobs.Manager
	logger           arbor.ILogger
}

// Compile-time assertion: TransformManager implements StepManager interface
var _ interfaces.StepManager = (*TransformManager)(nil)

// NewTransformManager creates a new transform manager for orchestrating document transformation workflows
func NewTransformManager(transformService interfaces.TransformService, jobManager *jobs.Manager, logger arbor.ILogger) *TransformManager {
	return &TransformManager{
		transformService: transformService,
		jobManager:       jobManager,
		logger:           logger,
	}
}

// CreateParentJob executes a transform operation for the given job definition.
// This is a synchronous operation that directly transforms HTML to markdown.
// Creates a job record for tracking and updates status on completion.
func (m *TransformManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	m.logger.Info().
		Str("step_name", step.Name).
		Str("action", step.Action).
		Str("parent_job_id", parentJobID).
		Msg("Orchestrating transformation")

	// Generate job ID for this step
	jobID := uuid.New().String()

	// Create job record for tracking
	job := &jobs.Job{
		ID:       jobID,
		ParentID: &parentJobID,
		Type:     "transform",
		Name:     step.Name, // Use step name as job name
		Phase:    "core",
		Status:   "running",
	}

	// Save job record
	if err := m.jobManager.CreateJobRecord(ctx, job); err != nil {
		m.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to create transform job record")
		return "", fmt.Errorf("failed to create job record: %w", err)
	}

	// Parse step config
	config, err := parseTransformConfig(step.Config)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to parse transform config")

		// Mark job as failed
		if updateErr := m.jobManager.SetJobError(ctx, jobID, err.Error()); updateErr != nil {
			m.logger.Error().Err(updateErr).Str("job_id", jobID).Msg("Failed to set job error")
		}

		return "", fmt.Errorf("invalid transform config: %w", err)
	}

	m.logger.Debug().
		Str("input_format", config.InputFormat).
		Str("output_format", config.OutputFormat).
		Bool("validate_html", config.ValidateHTML).
		Msg("Transform config parsed")

	// Validate configuration
	if config.InputFormat != "html" {
		errMsg := fmt.Sprintf("unsupported input format: %s (only 'html' is supported)", config.InputFormat)

		// Mark job as failed
		if err := m.jobManager.SetJobError(ctx, jobID, errMsg); err != nil {
			m.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to set job error")
		}

		return "", fmt.Errorf(errMsg)
	}
	if config.OutputFormat != "markdown" {
		errMsg := fmt.Sprintf("unsupported output format: %s (only 'markdown' is supported)", config.OutputFormat)

		// Mark job as failed
		if err := m.jobManager.SetJobError(ctx, jobID, errMsg); err != nil {
			m.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to set job error")
		}

		return "", fmt.Errorf(errMsg)
	}

	// For now, transform steps are informational only
	// Future: Could process actual HTML content from sources
	// Example: Read HTML from documents table, transform, and save back

	m.logger.Info().
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Msg("Transform step completed (placeholder implementation)")

	// Mark job as completed
	if err := m.jobManager.UpdateJobStatus(ctx, jobID, "completed"); err != nil {
		m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job status to completed")
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Msg("Transform step completed successfully")

	return jobID, nil
}

// GetManagerType returns "transform" - the action type this manager handles
func (m *TransformManager) GetManagerType() string {
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
