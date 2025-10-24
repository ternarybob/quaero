// -----------------------------------------------------------------------
// Last Modified: Friday, 24th October 2025 12:00:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

// Package jobs provides the JobExecutor for orchestrating user-defined job workflows.
//
// ARCHITECTURE NOTES:
// JobExecutor is NOT replaced by the queue system - it serves a different purpose:
//
// - JobExecutor: Orchestrates multi-step workflows defined by users (JobDefinitions)
//   - Executes steps sequentially with retry logic and error handling
//   - Polls crawl jobs asynchronously when wait_for_completion is enabled
//   - Publishes progress events for UI updates
//   - Supports error strategies: fail, continue, retry
//
// - Queue System: Handles individual task execution (CrawlerJob, SummarizerJob, CleanupJob)
//   - Processes URLs, generates summaries, cleans up old jobs
//   - Provides persistent queue with worker pool
//   - Enables job spawning and depth tracking
//
// Both systems coexist and complement each other:
// - JobDefinitions can trigger crawl jobs via the crawl action
// - JobExecutor polls those crawl jobs until completion
// - Crawl jobs are executed by the queue-based CrawlerJob type

package jobs

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/sources"
)

// ExecutionResult contains the result of Execute(), indicating whether async polling was launched
type ExecutionResult struct {
	AsyncPollingActive bool // True if async polling goroutine was launched
}

// StatusUpdateCallback is called when async polling completes to update parent job status
type StatusUpdateCallback func(ctx context.Context, status string, errorMsg string) error

// JobExecutor orchestrates the execution of job definitions by iterating through steps,
// retrieving action handlers from the registry, and implementing error handling strategies
type JobExecutor struct {
	registry       *JobTypeRegistry
	sourceService  *sources.Service
	eventService   interfaces.EventService
	crawlerService interfaces.CrawlerService
	logger         arbor.ILogger

	// Context for lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
}

// NewJobExecutor creates a new job executor instance
func NewJobExecutor(registry *JobTypeRegistry, sourceService *sources.Service, eventService interfaces.EventService, crawlerService interfaces.CrawlerService, logger arbor.ILogger) (*JobExecutor, error) {
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
	if crawlerService == nil {
		return nil, fmt.Errorf("crawlerService cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Create cancellable context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	executor := &JobExecutor{
		registry:       registry,
		sourceService:  sourceService,
		eventService:   eventService,
		crawlerService: crawlerService,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
	}

	logger.Info().Msg("Job executor initialized")

	return executor, nil
}

// Shutdown gracefully stops the executor and cancels all background tasks
func (e *JobExecutor) Shutdown() {
	e.logger.Info().Msg("Shutting down job executor - cancelling background tasks")
	e.cancel()
}

// Execute executes a job definition by iterating through its steps.
// Returns ExecutionResult indicating whether async polling was launched.
// If async polling is active, the statusCallback will be invoked when polling completes.
func (e *JobExecutor) Execute(ctx context.Context, definition *models.JobDefinition, statusCallback StatusUpdateCallback) (*ExecutionResult, error) {
	// Validate job definition
	if err := definition.Validate(); err != nil {
		return nil, fmt.Errorf("invalid job definition: %w", err)
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
		return nil, fmt.Errorf("failed to fetch sources: %w", err)
	}

	// Publish job start event
	e.publishProgressEvent(ctx, definition, 0, "", "", "running", "")

	// Initialize error slice for aggregation
	errors := make([]error, 0)

	// Track if any async polling was launched for this execution
	asyncPollingLaunched := false

	// Iterate through steps
	for stepIndex, step := range definition.Steps {
		// Check context cancellation before starting step
		if ctx.Err() != nil {
			e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "failed", ctx.Err().Error())
			return &ExecutionResult{AsyncPollingActive: false}, ctx.Err()
		}

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
			if handledErr := e.handleStepError(ctx, definition, step, stepIndex, stepErr, fetchedSources); handledErr != nil {
				errors = append(errors, handledErr)
				if shouldStopOnError(step) {
					// Publish failure event
					e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "failed", handledErr.Error())
					return &ExecutionResult{AsyncPollingActive: false}, fmt.Errorf("job execution failed at step %d: %w", stepIndex, handledErr)
				}
			}
			continue
		}

		// Execute handler (pass pointer to allow step.Config modifications)
		err = handler(ctx, &step, fetchedSources)
		if err != nil {
			if handledErr := e.handleStepError(ctx, definition, step, stepIndex, err, fetchedSources); handledErr != nil {
				errors = append(errors, handledErr)
				if shouldStopOnError(step) {
					// Publish failure event
					e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "failed", handledErr.Error())
					return &ExecutionResult{AsyncPollingActive: false}, fmt.Errorf("job execution failed at step %d: %w", stepIndex, handledErr)
				}
			} else {
				// Step succeeded after retry - publish completion event
				e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "completed", "")
			}
		} else {
			// Check if this is a crawl action with wait_for_completion enabled
			if step.Action == "crawl" && extractBool(step.Config, "wait_for_completion", true) {
				// Extract job IDs from step config (action handler must populate this)
				jobIDs := extractCrawlJobIDs(step.Config)
				if len(jobIDs) > 0 {
					// Set flag indicating async polling was launched
					asyncPollingLaunched = true

					e.logger.Debug().
						Str("job_id", definition.ID).
						Int("step_index", stepIndex).
						Str("step_name", step.Name).
						Int("crawl_job_count", len(jobIDs)).
						Msg("Launching async polling for crawl jobs")

					// Extract timeout from step config (default: 30 minutes)
					timeoutSeconds := extractInt(step.Config, "polling_timeout_seconds", 1800)
					pollingTimeout := time.Duration(timeoutSeconds) * time.Second

					// Launch async polling in goroutine
					go func() {
						// Create a context with timeout derived from executor context
						// This allows polling to be cancelled via executor.Shutdown()
						pollingCtx, cancel := context.WithTimeout(e.ctx, pollingTimeout)
						defer cancel()

						// Poll until all jobs complete
						pollErr := e.pollCrawlJobs(pollingCtx, definition, stepIndex, step, jobIDs)
						if pollErr != nil {
							// Publish step-level failure event
							e.publishProgressEvent(pollingCtx, definition, stepIndex, step.Name, step.Action, "failed", pollErr.Error())
							// Publish final job-level failure event
							e.publishProgressEvent(pollingCtx, definition, len(definition.Steps)-1, "", "", "failed", pollErr.Error())
							e.logger.Error().
								Err(pollErr).
								Str("job_id", definition.ID).
								Int("step_index", stepIndex).
								Msg("Async crawl polling failed")

							// Invoke status callback with failure
							if statusCallback != nil {
								if callbackErr := statusCallback(pollingCtx, "failed", pollErr.Error()); callbackErr != nil {
									e.logger.Error().
										Err(callbackErr).
										Str("job_id", definition.ID).
										Msg("Failed to invoke status callback for polling failure")
								}
							}
						} else {
							// Publish step-level completion event
							e.publishProgressEvent(pollingCtx, definition, stepIndex, step.Name, step.Action, "completed", "")
							// Publish final job-level completion event (deferred from Execute)
							e.publishProgressEvent(pollingCtx, definition, len(definition.Steps)-1, "", "", "completed", "")
							e.logger.Info().
								Str("job_id", definition.ID).
								Int("step_index", stepIndex).
								Msg("Async crawl polling completed successfully")

							// Invoke status callback with success
							if statusCallback != nil {
								if callbackErr := statusCallback(pollingCtx, "completed", ""); callbackErr != nil {
									e.logger.Error().
										Err(callbackErr).
										Str("job_id", definition.ID).
										Msg("Failed to invoke status callback for polling success")
								}
							}
						}
					}()

					// Immediately return - polling continues in background
					e.logger.Debug().
						Str("job_id", definition.ID).
						Int("step_index", stepIndex).
						Msg("Crawl step completed (polling in background)")
					e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "running", "")
				} else {
					// No job IDs found, but step succeeded - publish completion event
					e.logger.Warn().
						Str("job_id", definition.ID).
						Int("step_index", stepIndex).
						Str("step_name", step.Name).
						Msg("Crawl action completed but no job IDs found for polling")
					e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "completed", "")
				}
			} else {
				// Step succeeded initially - publish completion event
				e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "completed", "")
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

		// Publish failure event with aggregated error message
		e.publishProgressEvent(ctx, definition, len(definition.Steps)-1, "", "", "failed", fmt.Sprintf("%d error(s) occurred", len(errors)))

		return &ExecutionResult{AsyncPollingActive: asyncPollingLaunched}, fmt.Errorf("job execution completed with %d error(s): %v", len(errors), errors)
	}

	// Log successful completion
	totalDuration := time.Since(startTime)
	e.logger.Info().
		Str("job_id", definition.ID).
		Dur("duration", totalDuration).
		Msg("Job execution completed successfully")

	// Publish completion event only if no async polling was launched
	// If async polling was launched, the polling goroutine will publish the final completion event
	if !asyncPollingLaunched {
		e.publishProgressEvent(ctx, definition, len(definition.Steps)-1, "", "", "completed", "")
	} else {
		e.logger.Debug().
			Str("job_id", definition.ID).
			Msg("Async polling in progress - completion event deferred to polling goroutine")
	}

	return &ExecutionResult{AsyncPollingActive: asyncPollingLaunched}, nil
}

// shouldStopOnError returns true if the step's error strategy should stop execution
// Treats both ErrorStrategyFail and empty/default strategy as stop conditions
func shouldStopOnError(step models.JobStep) bool {
	return step.OnError == models.ErrorStrategyFail || step.OnError == ""
}

// handleStepError handles errors based on the step's error strategy
func (e *JobExecutor) handleStepError(ctx context.Context, definition *models.JobDefinition, step models.JobStep, stepIndex int, err error, stepSources []*models.SourceConfig) error {
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
		// Publish step-level failure event
		e.publishProgressEvent(ctx, definition, stepIndex, step.Name, step.Action, "failed", err.Error())
		return err // Return error for aggregation, but execution continues

	case models.ErrorStrategyFail:
		// Log error and stop execution
		logEvent.Msg("Step failed, stopping execution (ErrorStrategyFail)")
		return err

	case models.ErrorStrategyRetry:
		// Attempt retry with provided sources
		retryErr := e.retryStep(ctx, definition, step, stepSources)
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

		// Log retry attempt (execution detail)
		e.logger.Debug().
			Str("job_id", definition.ID).
			Str("step_name", step.Name).
			Int("attempt", attempt).
			Int("max_retries", maxRetries).
			Dur("backoff", backoff).
			Msg("Retrying step")

		// Sleep for backoff duration (skip on first attempt)
		if attempt > 1 {
			select {
			case <-time.After(backoff):
				// Backoff completed
			case <-ctx.Done():
				// Context cancelled during backoff
				return ctx.Err()
			}
		}

		// Execute handler (pass pointer to allow step.Config modifications)
		err = handler(ctx, &step, sources)
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

// pollCrawlJobs polls multiple crawl jobs until all reach terminal state
func (e *JobExecutor) pollCrawlJobs(ctx context.Context, definition *models.JobDefinition, stepIndex int, step models.JobStep, jobIDs []string) error {
	// Create ticker with 5-second interval
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Track completion status for each job ID
	completionStatus := make(map[string]bool)
	for _, jobID := range jobIDs {
		completionStatus[jobID] = false
	}

	// Track previous status for logging changes
	previousStatus := make(map[string]string)

	// Track failed/cancelled job IDs and their errors persistently across ticks
	failed := make(map[string]string)

	// Track consecutive errors per job (for failure threshold)
	consecutiveErrors := make(map[string]int)
	maxConsecutiveErrors := 5 // Fail after 5 consecutive GetJobStatus errors

	e.logger.Debug().
		Str("job_id", definition.ID).
		Int("step_index", stepIndex).
		Str("step_name", step.Name).
		Int("crawl_job_count", len(jobIDs)).
		Msg("Starting async polling for crawl jobs")

	// Loop until all jobs reach terminal state
	for {
		select {
		case <-ctx.Done():
			// Context cancelled or timeout
			if ctx.Err() == context.DeadlineExceeded {
				e.logger.Error().
					Str("job_id", definition.ID).
					Int("step_index", stepIndex).
					Msg("Polling timeout exceeded")
				return fmt.Errorf("polling timeout exceeded")
			}
			if ctx.Err() == context.Canceled {
				e.logger.Info().
					Str("job_id", definition.ID).
					Int("step_index", stepIndex).
					Msg("Polling cancelled via executor shutdown")
				return fmt.Errorf("polling cancelled")
			}
			return ctx.Err()
		case <-ticker.C:
			// Poll each incomplete job
			allComplete := true

			for _, jobID := range jobIDs {
				// Skip already completed jobs
				if completionStatus[jobID] {
					continue
				}

				// Get job status from crawler service
				result, err := e.crawlerService.GetJobStatus(jobID)
				if err != nil {
					// Increment consecutive error count
					consecutiveErrors[jobID]++

					e.logger.Warn().
						Err(err).
						Str("crawl_job_id", jobID).
						Int("consecutive_errors", consecutiveErrors[jobID]).
						Int("max_consecutive_errors", maxConsecutiveErrors).
						Msg("Failed to get crawl job status")

					// Check if threshold exceeded
					if consecutiveErrors[jobID] >= maxConsecutiveErrors {
						// Mark job as failed due to repeated GetJobStatus errors
						if _, exists := failed[jobID]; !exists {
							failed[jobID] = fmt.Sprintf("exceeded failure threshold (%d consecutive errors)", maxConsecutiveErrors)
						}
						completionStatus[jobID] = true
						e.logger.Error().
							Str("crawl_job_id", jobID).
							Int("consecutive_errors", consecutiveErrors[jobID]).
							Msg("Job marked as failed due to repeated GetJobStatus errors")
					}
					continue
				}

				// Type assert result to *crawler.CrawlJob with safety check
				cj, ok := result.(*crawler.CrawlJob)
				if !ok {
					// Increment consecutive error count for type assertion failure
					consecutiveErrors[jobID]++

					e.logger.Warn().
						Str("crawl_job_id", jobID).
						Int("consecutive_errors", consecutiveErrors[jobID]).
						Msgf("GetJobStatus returned unexpected type: %T", result)

					// Check if threshold exceeded
					if consecutiveErrors[jobID] >= maxConsecutiveErrors {
						if _, exists := failed[jobID]; !exists {
							failed[jobID] = fmt.Sprintf("type assertion failed after %d attempts", maxConsecutiveErrors)
						}
						completionStatus[jobID] = true
						e.logger.Error().
							Str("crawl_job_id", jobID).
							Msg("Job marked as failed due to repeated type assertion failures")
					}
					continue
				}

				// Reset consecutive error count on success
				consecutiveErrors[jobID] = 0

				// Extract status
				statusStr := string(cj.Status)

				// Log status changes
				if prevStatus, exists := previousStatus[jobID]; !exists || prevStatus != statusStr {
					e.logger.Info().
						Str("crawl_job_id", jobID).
						Str("previous_status", prevStatus).
						Str("new_status", statusStr).
						Msg("Crawl job status changed")
					previousStatus[jobID] = statusStr
				}

				// Check for terminal states
				switch cj.Status {
				case crawler.JobStatusCompleted:
					completionStatus[jobID] = true
					e.logger.Info().
						Str("crawl_job_id", jobID).
						Msg("Crawl job completed successfully")
					// Emit progress event with crawl-specific details
					e.publishCrawlProgressEvent(ctx, definition, stepIndex, step, jobID, cj, "completed")
				case crawler.JobStatusFailed:
					completionStatus[jobID] = true
					if _, exists := failed[jobID]; !exists {
						failed[jobID] = cj.Error
					}
					e.logger.Error().
						Str("crawl_job_id", jobID).
						Str("error", cj.Error).
						Msg("Crawl job failed")
					// Emit progress event with error
					e.publishCrawlProgressEvent(ctx, definition, stepIndex, step, jobID, cj, "failed")
				case crawler.JobStatusCancelled:
					completionStatus[jobID] = true
					if _, exists := failed[jobID]; !exists {
						failed[jobID] = "job was cancelled"
					}
					e.logger.Warn().
						Str("crawl_job_id", jobID).
						Msg("Crawl job was cancelled")
					// Emit progress event
					e.publishCrawlProgressEvent(ctx, definition, stepIndex, step, jobID, cj, "cancelled")
				case crawler.JobStatusRunning, crawler.JobStatusPending:
					// Still in progress
					allComplete = false
					// Emit progress event with current status
					e.publishCrawlProgressEvent(ctx, definition, stepIndex, step, jobID, cj, statusStr)
				default:
					// Unknown status - keep polling
					allComplete = false
				}
			}

			// Check if all jobs are complete
			if allComplete {
				// Check persistent failure map
				if len(failed) > 0 {
					// Build error list from persistent failures
					var errors []error
					for jobID, errMsg := range failed {
						errors = append(errors, fmt.Errorf("crawl job %s: %s", jobID, errMsg))
					}

					// Check step's OnError strategy
					if step.OnError == models.ErrorStrategyFail || step.OnError == "" {
						return fmt.Errorf("crawl polling completed with %d error(s): %v", len(errors), errors)
					}
					// Continue strategy - log warnings but return success
					e.logger.Warn().
						Int("error_count", len(errors)).
						Msg("Crawl polling completed with errors (continuing)")
					return nil
				}

				e.logger.Info().
					Str("job_id", definition.ID).
					Int("step_index", stepIndex).
					Int("crawl_job_count", len(jobIDs)).
					Msg("All crawl jobs completed successfully")
				return nil
			}
		}
	}
}

// publishCrawlProgressEvent publishes a job progress event with crawl-specific details
func (e *JobExecutor) publishCrawlProgressEvent(ctx context.Context, definition *models.JobDefinition, stepIndex int, step models.JobStep, crawlJobID string, cj *crawler.CrawlJob, status string) {
	payload := map[string]interface{}{
		"job_id":       definition.ID,
		"job_name":     definition.Name,
		"job_type":     string(definition.Type),
		"step_index":   stepIndex,
		"step_name":    step.Name,
		"step_action":  step.Action,
		"total_steps":  len(definition.Steps),
		"status":       status,
		"timestamp":    time.Now(),
		"crawl_job_id": crawlJobID,
	}

	// Populate source_type from CrawlJob
	payload["source_type"] = cj.SourceType

	// Populate progress fields from CrawlJob.Progress
	payload["total_urls"] = cj.Progress.TotalURLs
	payload["completed_urls"] = cj.Progress.CompletedURLs
	payload["failed_urls"] = cj.Progress.FailedURLs
	payload["pending_urls"] = cj.Progress.PendingURLs
	payload["percentage"] = cj.Progress.Percentage
	payload["current_url"] = cj.Progress.CurrentURL

	// Add error if present
	if cj.Error != "" {
		payload["error"] = cj.Error
	}

	// Publish event (non-blocking)
	e.eventService.Publish(ctx, interfaces.Event{
		Type:    interfaces.EventJobProgress,
		Payload: payload,
	})
}

// extractCrawlJobIDs extracts job IDs from step config after crawl action completes
// Supports both single string and slice of strings
func extractCrawlJobIDs(stepConfig map[string]interface{}) []string {
	if stepConfig == nil {
		return []string{}
	}

	jobIDsRaw, ok := stepConfig["crawl_job_ids"]
	if !ok {
		return []string{}
	}

	// Try single string (wrap in slice)
	if str, ok := jobIDsRaw.(string); ok {
		return []string{str}
	}

	// Try direct []string
	if jobIDsSlice, ok := jobIDsRaw.([]string); ok {
		return jobIDsSlice
	}

	// Try []interface{} with string elements
	if jobIDsIface, ok := jobIDsRaw.([]interface{}); ok {
		result := make([]string, 0, len(jobIDsIface))
		for _, item := range jobIDsIface {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	return []string{}
}

// extractBool extracts a boolean from config map with type assertion
func extractBool(config map[string]interface{}, key string, defaultValue bool) bool {
	if config == nil {
		return defaultValue
	}

	value, ok := config[key]
	if !ok {
		return defaultValue
	}

	if boolVal, ok := value.(bool); ok {
		return boolVal
	}

	return defaultValue
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

// extractInt extracts an integer from config map with type assertion
func extractInt(config map[string]interface{}, key string, defaultValue int) int {
	if config == nil {
		return defaultValue
	}

	value, ok := config[key]
	if !ok {
		return defaultValue
	}

	// Try direct int
	if intVal, ok := value.(int); ok {
		return intVal
	}

	// Try float64 (JSON unmarshaling)
	if floatVal, ok := value.(float64); ok {
		return int(floatVal)
	}

	return defaultValue
}
