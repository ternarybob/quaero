// -----------------------------------------------------------------------
// Last Modified: Monday, 21st October 2025 5:45:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package jobs

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/sources"
)

// JobExecutor orchestrates the execution of job definitions by iterating through steps,
// retrieving action handlers from the registry, and implementing error handling strategies
type JobExecutor struct {
	registry      *JobTypeRegistry
	sourceService *sources.Service
	eventService  interfaces.EventService
	logger        arbor.ILogger
}

// NewJobExecutor creates a new job executor instance
func NewJobExecutor(registry *JobTypeRegistry, sourceService *sources.Service, eventService interfaces.EventService, logger arbor.ILogger) (*JobExecutor, error) {
	// Validate inputs
	if registry == nil {
		return nil, fmt.Errorf("registry cannot be nil")
	}
	if sourceService == nil {
		return nil, fmt.Errorf("sourceService cannot be nil")
	}
	if eventService == nil {
		return nil, fmt.Errorf("eventService cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	executor := &JobExecutor{
		registry:      registry,
		sourceService: sourceService,
		eventService:  eventService,
		logger:        logger,
	}

	logger.Info().Msg("Job executor initialized")

	return executor, nil
}

// Execute executes a job definition by iterating through its steps
func (e *JobExecutor) Execute(ctx context.Context, definition *models.JobDefinition) error {
	// Validate job definition
	if err := definition.Validate(); err != nil {
		return fmt.Errorf("invalid job definition: %w", err)
	}

	// Log execution start
	e.logger.Info().
		Str("job_id", definition.ID).
		Str("job_name", definition.Name).
		Str("job_type", string(definition.Type)).
		Int("step_count", len(definition.Steps)).
		Msg("Starting job execution")

	startTime := time.Now()

	// Fetch sources
	fetchedSources, err := e.fetchSources(ctx, definition.Sources)
	if err != nil {
		return fmt.Errorf("failed to fetch sources: %w", err)
	}

	// Publish job start event
	e.publishProgressEvent(ctx, definition, 0, "", "", "running", "")

	// Initialize error slice for aggregation
	errors := make([]error, 0)

	// Iterate through steps
	for stepIndex, step := range definition.Steps {
		stepStartTime := time.Now()

		// Log step start
		e.logger.Info().
			Str("job_id", definition.ID).
			Int("step_index", stepIndex).
			Str("step_name", step.Name).
			Str("action", step.Action).
			Msg("Starting step execution")

		// Publish step start event
		e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "running", "")

		// Retrieve action handler
		handler, err := e.registry.GetAction(definition.Type, step.Action)
		if err != nil {
			stepErr := fmt.Errorf("action handler not found: %w", err)
			if handledErr := e.handleStepError(ctx, definition, step, stepIndex, stepErr); handledErr != nil {
				errors = append(errors, handledErr)
				if step.OnError == models.ErrorStrategyFail {
					// Publish failure event
					e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "failed", handledErr.Error())
					return fmt.Errorf("job execution failed at step %d: %w", stepIndex, handledErr)
				}
			}
			continue
		}

		// Execute handler
		err = handler(ctx, step, fetchedSources)
		if err != nil {
			if handledErr := e.handleStepError(ctx, definition, step, stepIndex, err); handledErr != nil {
				errors = append(errors, handledErr)
				if step.OnError == models.ErrorStrategyFail {
					// Publish failure event
					e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "failed", handledErr.Error())
					return fmt.Errorf("job execution failed at step %d: %w", stepIndex, handledErr)
				}
			}
		}

		// Log step completion
		stepDuration := time.Since(stepStartTime)
		e.logger.Info().
			Str("job_id", definition.ID).
			Int("step_index", stepIndex).
			Str("step_name", step.Name).
			Dur("duration", stepDuration).
			Msg("Step execution completed")
	}

	// Check if errors exist
	if len(errors) > 0 {
		totalDuration := time.Since(startTime)
		e.logger.Warn().
			Str("job_id", definition.ID).
			Int("error_count", len(errors)).
			Dur("duration", totalDuration).
			Msg("Job execution completed with errors")

		// Publish completion event with errors
		e.publishProgressEvent(ctx, definition, len(definition.Steps)-1, "", "", "completed", fmt.Sprintf("%d error(s) occurred", len(errors)))

		return fmt.Errorf("job execution completed with %d error(s): %v", len(errors), errors)
	}

	// Log successful completion
	totalDuration := time.Since(startTime)
	e.logger.Info().
		Str("job_id", definition.ID).
		Dur("duration", totalDuration).
		Msg("Job execution completed successfully")

	// Publish completion event
	e.publishProgressEvent(ctx, definition, len(definition.Steps)-1, "", "", "completed", "")

	return nil
}

// handleStepError handles errors based on the step's error strategy
func (e *JobExecutor) handleStepError(ctx context.Context, definition *models.JobDefinition, step models.JobStep, stepIndex int, err error) error {
	// Log error with structured fields
	logEvent := e.logger.Error().
		Str("job_id", definition.ID).
		Str("step_name", step.Name).
		Str("action", step.Action).
		Int("step_index", stepIndex).
		Err(err)

	// Handle based on error strategy
	switch step.OnError {
	case models.ErrorStrategyContinue:
		// Log warning, add to error aggregation, and continue
		e.logger.Warn().
			Str("job_id", definition.ID).
			Str("step_name", step.Name).
			Str("action", step.Action).
			Int("step_index", stepIndex).
			Err(err).
			Msg("Step failed but continuing execution (ErrorStrategyContinue)")
		return err // Return error for aggregation, but execution continues

	case models.ErrorStrategyFail:
		// Log error and stop execution
		logEvent.Msg("Step failed, stopping execution (ErrorStrategyFail)")
		return err

	case models.ErrorStrategyRetry:
		// Attempt retry
		retryErr := e.retryStep(ctx, definition, step, nil)
		if retryErr != nil {
			logEvent.Msg("Step failed after retries (ErrorStrategyRetry)")
			return retryErr
		}
		return nil

	default:
		// Default to fail strategy
		logEvent.Msg("Step failed with unknown error strategy, stopping execution")
		return err
	}
}

// retryStep retries a step with exponential backoff
func (e *JobExecutor) retryStep(ctx context.Context, definition *models.JobDefinition, step models.JobStep, stepSources []*models.SourceConfig) error {
	// Extract retry configuration
	maxRetries, initialBackoff, maxBackoff, multiplier := extractRetryConfig(step.Config)

	// Fetch sources if not provided
	var sources []*models.SourceConfig
	var err error
	if stepSources == nil {
		sources, err = e.fetchSources(ctx, definition.Sources)
		if err != nil {
			return fmt.Errorf("failed to fetch sources for retry: %w", err)
		}
	} else {
		sources = stepSources
	}

	// Retrieve action handler
	handler, err := e.registry.GetAction(definition.Type, step.Action)
	if err != nil {
		return fmt.Errorf("action handler not found for retry: %w", err)
	}

	// Implement retry loop with exponential backoff
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Calculate backoff duration
		backoff := time.Duration(float64(initialBackoff) * math.Pow(multiplier, float64(attempt-1)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		// Log retry attempt
		e.logger.Info().
			Str("job_id", definition.ID).
			Str("step_name", step.Name).
			Int("attempt", attempt).
			Int("max_retries", maxRetries).
			Dur("backoff", backoff).
			Msg("Retrying step")

		// Sleep for backoff duration (skip on first attempt)
		if attempt > 1 {
			time.Sleep(backoff)
		}

		// Execute handler
		err = handler(ctx, step, sources)
		if err == nil {
			// Success
			e.logger.Info().
				Str("job_id", definition.ID).
				Str("step_name", step.Name).
				Int("attempt", attempt).
				Msg("Step succeeded on retry")
			return nil
		}

		lastErr = err
	}

	// All retries exhausted
	return fmt.Errorf("step failed after %d retries: %w", maxRetries, lastErr)
}

// fetchSources fetches source configurations by IDs
func (e *JobExecutor) fetchSources(ctx context.Context, sourceIDs []string) ([]*models.SourceConfig, error) {
	// If no sources specified, return empty slice (not an error)
	if len(sourceIDs) == 0 {
		return []*models.SourceConfig{}, nil
	}

	// Initialize sources slice
	sources := make([]*models.SourceConfig, 0, len(sourceIDs))

	// Iterate through source IDs
	for _, id := range sourceIDs {
		source, err := e.sourceService.GetSource(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch source %s: %w", id, err)
		}
		sources = append(sources, source)
	}

	// Log fetched sources
	e.logger.Debug().
		Int("count", len(sources)).
		Msg("Sources fetched successfully")

	return sources, nil
}

// publishProgressEvent publishes a job progress event
func (e *JobExecutor) publishProgressEvent(ctx context.Context, definition *models.JobDefinition, stepIndex int, stepName, stepAction, status, errorMsg string) {
	payload := map[string]interface{}{
		"job_id":      definition.ID,
		"job_name":    definition.Name,
		"job_type":    string(definition.Type),
		"step_index":  stepIndex,
		"step_name":   stepName,
		"step_action": stepAction,
		"total_steps": len(definition.Steps),
		"status":      status,
		"timestamp":   time.Now(),
	}

	if errorMsg != "" {
		payload["error"] = errorMsg
	}

	// Publish event (non-blocking)
	e.eventService.Publish(ctx, interfaces.Event{
		Type:    interfaces.EventJobProgress,
		Payload: payload,
	})
}

// extractRetryConfig extracts retry configuration from step config with defaults
func extractRetryConfig(config map[string]interface{}) (maxRetries int, initialBackoff time.Duration, maxBackoff time.Duration, multiplier float64) {
	// Default values
	maxRetries = 3
	initialBackoff = 2 * time.Second
	maxBackoff = 60 * time.Second
	multiplier = 2.0

	if config == nil {
		return
	}

	// Extract max_retries
	if val, ok := config["max_retries"]; ok {
		switch v := val.(type) {
		case int:
			maxRetries = v
		case float64:
			maxRetries = int(v)
		}
	}

	// Extract initial_backoff
	if val, ok := config["initial_backoff"]; ok {
		switch v := val.(type) {
		case int:
			initialBackoff = time.Duration(v) * time.Second
		case float64:
			initialBackoff = time.Duration(v) * time.Second
		case string:
			if d, err := time.ParseDuration(v); err == nil {
				initialBackoff = d
			}
		}
	}

	// Extract max_backoff
	if val, ok := config["max_backoff"]; ok {
		switch v := val.(type) {
		case int:
			maxBackoff = time.Duration(v) * time.Second
		case float64:
			maxBackoff = time.Duration(v) * time.Second
		case string:
			if d, err := time.ParseDuration(v); err == nil {
				maxBackoff = d
			}
		}
	}

	// Extract backoff_multiplier
	if val, ok := config["backoff_multiplier"]; ok {
		switch v := val.(type) {
		case float64:
			multiplier = v
		case int:
			multiplier = float64(v)
		}
	}

	return
}
