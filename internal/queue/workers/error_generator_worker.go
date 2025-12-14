// -----------------------------------------------------------------------
// Error Generator Worker - Worker for testing error tolerance and logging
// - DefinitionWorker: Creates error generator jobs for testing
// - JobWorker: Generates logs with random warnings/errors and creates recursive children
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ErrorGeneratorWorker generates logs with random warnings/errors for testing error tolerance.
// Implements both DefinitionWorker (for job definition steps) and JobWorker (for queue execution).
type ErrorGeneratorWorker struct {
	jobMgr       *queue.Manager
	queueMgr     interfaces.QueueManager
	logger       arbor.ILogger
	eventService interfaces.EventService
}

// Compile-time assertions: ErrorGeneratorWorker implements both interfaces
var _ interfaces.DefinitionWorker = (*ErrorGeneratorWorker)(nil)
var _ interfaces.JobWorker = (*ErrorGeneratorWorker)(nil)

// NewErrorGeneratorWorker creates a new error generator worker
func NewErrorGeneratorWorker(
	jobMgr *queue.Manager,
	queueMgr interfaces.QueueManager,
	logger arbor.ILogger,
	eventService interfaces.EventService,
) *ErrorGeneratorWorker {
	return &ErrorGeneratorWorker{
		jobMgr:       jobMgr,
		queueMgr:     queueMgr,
		logger:       logger,
		eventService: eventService,
	}
}

// GetWorkerType returns "error_generator" - the job type this worker handles
func (w *ErrorGeneratorWorker) GetWorkerType() string {
	return "error_generator"
}

// Validate validates that the queue job is compatible with this worker
func (w *ErrorGeneratorWorker) Validate(job *models.QueueJob) error {
	if job.Type != "error_generator" {
		return fmt.Errorf("invalid job type: expected error_generator, got %s", job.Type)
	}
	return nil
}

// Execute executes an error generator job with the following behavior:
// 1. Generate log items with configurable delays
// 2. Randomly generate INFO, WARN, and ERROR level logs
// 3. Optionally spawn child jobs (some of which may fail)
// 4. Fail based on configured failure_rate probability
func (w *ErrorGeneratorWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	parentID := job.GetParentID()
	if parentID == "" {
		parentID = job.ID
	}
	jobLogger := w.logger.WithCorrelationId(parentID)

	// Extract configuration with defaults
	logCount := getConfigIntWithDefault(job.Config, "log_count", 10)
	logDelay := getConfigIntWithDefault(job.Config, "log_delay_ms", 100)
	failureRate := getConfigFloatWithDefault(job.Config, "failure_rate", 0.1) // 10% chance of failure
	childCount := getConfigIntWithDefault(job.Config, "child_count", 0)
	recursionDepth := getConfigIntWithDefault(job.Config, "recursion_depth", 0)
	currentDepth := job.Depth

	jobLogger.Debug().
		Str("job_id", job.ID).
		Int("log_count", logCount).
		Int("log_delay_ms", logDelay).
		Float64("failure_rate", failureRate).
		Int("child_count", childCount).
		Int("recursion_depth", recursionDepth).
		Int("current_depth", currentDepth).
		Msg("Starting error generator job execution")

	// Update job status to running
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Error generator starting: %d logs, %dms delay, %.0f%% failure rate", logCount, logDelay, failureRate*100))

	// Generate logs with configured distribution
	infoCount := 0
	warnCount := 0
	errorCount := 0
	delay := time.Duration(logDelay) * time.Millisecond

	for i := 0; i < logCount; i++ {
		// Check for cancellation
		select {
		case <-ctx.Done():
			w.jobMgr.AddJobLog(ctx, job.ID, "info", "Job cancelled")
			return ctx.Err()
		default:
		}

		// Random log level distribution: 80% INFO, 15% WARN, 5% ERROR
		randVal := rand.Float64()
		var level, message string
		if randVal < 0.80 {
			level = "info"
			infoCount++
			message = fmt.Sprintf("Processing item %d/%d", i+1, logCount)
		} else if randVal < 0.95 {
			level = "warn"
			warnCount++
			message = fmt.Sprintf("Warning at item %d: resource usage high", i+1)
		} else {
			level = "error"
			errorCount++
			message = fmt.Sprintf("Error at item %d: operation failed", i+1)
		}

		w.jobMgr.AddJobLog(ctx, job.ID, level, message)

		// Add delay between logs
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	// Create child jobs if configured and within recursion depth
	childJobsCreated := 0
	if childCount > 0 && currentDepth < recursionDepth {
		for i := 0; i < childCount; i++ {
			childJobID, err := w.spawnChildJob(ctx, job, i, recursionDepth)
			if err != nil {
				jobLogger.Warn().Err(err).Int("child_index", i).Msg("Failed to spawn child job")
				w.jobMgr.AddJobLog(ctx, job.ID, "warn", fmt.Sprintf("Failed to spawn child job %d: %v", i+1, err))
				continue
			}
			childJobsCreated++
			jobLogger.Debug().Str("child_job_id", childJobID).Int("child_index", i).Msg("Child job spawned")
		}

		if childJobsCreated > 0 {
			w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Spawned %d child jobs", childJobsCreated))
		}
	}

	// Determine if this job should fail based on failure_rate
	shouldFail := rand.Float64() < failureRate

	// Log final statistics
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Log summary: INF=%d, WRN=%d, ERR=%d", infoCount, warnCount, errorCount))

	if shouldFail {
		errorMsg := "Simulated failure triggered by failure_rate configuration"
		w.jobMgr.AddJobLog(ctx, job.ID, "error", errorMsg)
		w.jobMgr.SetJobError(ctx, job.ID, errorMsg)
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf(errorMsg)
	}

	// Mark job as completed
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
		return fmt.Errorf("failed to update job status: %w", err)
	}

	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Error generator completed successfully: %d children spawned", childJobsCreated))

	return nil
}

// spawnChildJob creates and enqueues a child error generator job
func (w *ErrorGeneratorWorker) spawnChildJob(ctx context.Context, parentJob *models.QueueJob, childIndex int, maxDepth int) (string, error) {
	// Create child job configuration - each child has slightly different settings
	childConfig := map[string]interface{}{
		"log_count":       getConfigIntWithDefault(parentJob.Config, "log_count", 10) / 2,      // Half the logs
		"log_delay_ms":    getConfigIntWithDefault(parentJob.Config, "log_delay_ms", 100),      // Same delay
		"failure_rate":    getConfigFloatWithDefault(parentJob.Config, "failure_rate", 0.1),    // Same failure rate
		"child_count":     getConfigIntWithDefault(parentJob.Config, "child_count", 0) / 2,     // Half the children
		"recursion_depth": maxDepth,                                                            // Same max depth
	}

	// Create child job metadata - copy parent metadata
	childMetadata := make(map[string]interface{})
	if parentJob.Metadata != nil {
		for k, v := range parentJob.Metadata {
			childMetadata[k] = v
		}
	}
	childMetadata["parent_child_index"] = childIndex

	// Create child queue job
	childJob := models.NewQueueJobChild(
		parentJob.GetParentID(), // All children reference the same root parent
		"error_generator",
		fmt.Sprintf("Error Generator Child %d", childIndex+1),
		childConfig,
		childMetadata,
		parentJob.Depth+1,
	)

	// Validate child job
	if err := childJob.Validate(); err != nil {
		return "", fmt.Errorf("invalid child queue job: %w", err)
	}

	// Serialize job to JSON for payload
	payloadBytes, err := childJob.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to serialize child job: %w", err)
	}

	// Create job record in database
	if err := w.jobMgr.CreateJobRecord(ctx, &queue.Job{
		ID:              childJob.ID,
		ParentID:        childJob.ParentID,
		Type:            childJob.Type,
		Name:            childJob.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       childJob.CreatedAt,
		ProgressCurrent: 0,
		ProgressTotal:   1,
		Payload:         string(payloadBytes),
	}); err != nil {
		return "", fmt.Errorf("failed to create child job record: %w", err)
	}

	// Enqueue child job
	queueMsg := queue.Message{
		JobID:   childJob.ID,
		Type:    childJob.Type,
		Payload: payloadBytes,
	}

	if err := w.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return "", fmt.Errorf("failed to enqueue child job: %w", err)
	}

	return childJob.ID, nil
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS
// ============================================================================

// GetType returns WorkerTypeErrorGenerator for the DefinitionWorker interface
func (w *ErrorGeneratorWorker) GetType() models.WorkerType {
	return models.WorkerTypeErrorGenerator
}

// Init performs the initialization phase for an error generator step
func (w *ErrorGeneratorWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract configuration with defaults
	workerCount := getConfigIntWithDefault(stepConfig, "worker_count", 10)
	logCount := getConfigIntWithDefault(stepConfig, "log_count", 100)
	logDelay := getConfigIntWithDefault(stepConfig, "log_delay_ms", 50)
	failureRate := getConfigFloatWithDefault(stepConfig, "failure_rate", 0.1)
	childCount := getConfigIntWithDefault(stepConfig, "child_count", 2)
	recursionDepth := getConfigIntWithDefault(stepConfig, "recursion_depth", 3)

	w.logger.Info().
		Str("step_name", step.Name).
		Int("worker_count", workerCount).
		Int("log_count", logCount).
		Int("log_delay_ms", logDelay).
		Float64("failure_rate", failureRate).
		Int("child_count", childCount).
		Int("recursion_depth", recursionDepth).
		Msg("Initializing error generator worker")

	// Create work items for each worker to spawn
	workItems := make([]interfaces.WorkItem, workerCount)
	for i := 0; i < workerCount; i++ {
		workItems[i] = interfaces.WorkItem{
			ID:   fmt.Sprintf("worker-%d", i+1),
			Name: fmt.Sprintf("Error Generator Worker %d", i+1),
			Type: "error_generator",
			Config: map[string]interface{}{
				"worker_index":    i,
				"log_count":       logCount,
				"log_delay_ms":    logDelay,
				"failure_rate":    failureRate,
				"child_count":     childCount,
				"recursion_depth": recursionDepth,
			},
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           workerCount,
		Strategy:             interfaces.ProcessingStrategyParallel,
		SuggestedConcurrency: 5,
		Metadata: map[string]interface{}{
			"worker_count":    workerCount,
			"log_count":       logCount,
			"log_delay_ms":    logDelay,
			"failure_rate":    failureRate,
			"child_count":     childCount,
			"recursion_depth": recursionDepth,
			"step_config":     stepConfig,
		},
	}, nil
}

// CreateJobs creates error generator jobs for the step
func (w *ErrorGeneratorWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize error generator worker: %w", err)
		}
	}

	// Get manager_id from step job's parent_id
	managerID := ""
	if stepJobInterface, err := w.jobMgr.GetJob(ctx, stepID); err == nil && stepJobInterface != nil {
		if stepJob, ok := stepJobInterface.(*models.QueueJobState); ok && stepJob != nil && stepJob.ParentID != nil {
			managerID = *stepJob.ParentID
		}
	}

	// Check if there are any work items
	if len(initResult.WorkItems) == 0 {
		w.logger.Warn().
			Str("step_name", step.Name).
			Msg("No work items for error generator")
		w.jobMgr.AddJobLog(ctx, stepID, "info", "No error generator jobs to create")
		return stepID, nil
	}

	w.logger.Info().
		Str("step_name", step.Name).
		Int("worker_count", len(initResult.WorkItems)).
		Msg("Creating error generator jobs")

	// Create and enqueue jobs for each work item
	jobIDs := make([]string, 0, len(initResult.WorkItems))
	for _, workItem := range initResult.WorkItems {
		jobID, err := w.createErrorGeneratorJob(ctx, workItem, stepID, step.Name, managerID)
		if err != nil {
			w.logger.Warn().Err(err).Str("work_item_id", workItem.ID).Msg("Failed to create error generator job")
			continue
		}
		jobIDs = append(jobIDs, jobID)
	}

	if len(jobIDs) == 0 {
		return "", fmt.Errorf("failed to create any error generator jobs for step %s", step.Name)
	}

	w.logger.Info().
		Str("step_name", step.Name).
		Int("jobs_created", len(jobIDs)).
		Msg("Error generator jobs created and enqueued")

	w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Created %d error generator jobs", len(jobIDs)))

	return stepID, nil
}

// createErrorGeneratorJob creates and enqueues a single error generator job
func (w *ErrorGeneratorWorker) createErrorGeneratorJob(ctx context.Context, workItem interfaces.WorkItem, parentJobID, stepName, managerID string) (string, error) {
	// Create job config from work item config
	jobConfig := workItem.Config

	// Create job metadata
	metadata := map[string]interface{}{
		"step_name":  stepName,
		"step_id":    parentJobID,
		"manager_id": managerID,
	}

	// Create queue job
	queueJob := models.NewQueueJobChild(
		parentJobID,
		"error_generator",
		workItem.Name,
		jobConfig,
		metadata,
		0, // Initial depth for root error generator jobs
	)

	// Validate queue job
	if err := queueJob.Validate(); err != nil {
		return "", fmt.Errorf("invalid queue job: %w", err)
	}

	// Serialize job to JSON
	payloadBytes, err := queueJob.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to serialize queue job: %w", err)
	}

	// Create job record in database
	if err := w.jobMgr.CreateJobRecord(ctx, &queue.Job{
		ID:              queueJob.ID,
		ParentID:        queueJob.ParentID,
		Type:            queueJob.Type,
		Name:            queueJob.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       queueJob.CreatedAt,
		ProgressCurrent: 0,
		ProgressTotal:   1,
		Payload:         string(payloadBytes),
	}); err != nil {
		return "", fmt.Errorf("failed to create job record: %w", err)
	}

	// Enqueue job
	queueMsg := queue.Message{
		JobID:   queueJob.ID,
		Type:    queueJob.Type,
		Payload: payloadBytes,
	}

	if err := w.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	return queueJob.ID, nil
}

// ReturnsChildJobs returns true since error generator creates child jobs
func (w *ErrorGeneratorWorker) ReturnsChildJobs() bool {
	return true
}

// ValidateConfig validates step configuration for error generator type
func (w *ErrorGeneratorWorker) ValidateConfig(step models.JobStep) error {
	// All configuration is optional with sensible defaults
	if step.Config != nil {
		// Validate failure_rate is between 0 and 1
		if failureRate, ok := step.Config["failure_rate"].(float64); ok {
			if failureRate < 0 || failureRate > 1 {
				return fmt.Errorf("failure_rate must be between 0 and 1, got %f", failureRate)
			}
		}

		// Validate recursion_depth is non-negative
		if depth, ok := step.Config["recursion_depth"].(float64); ok {
			if depth < 0 {
				return fmt.Errorf("recursion_depth must be >= 0, got %f", depth)
			}
		} else if depth, ok := step.Config["recursion_depth"].(int); ok {
			if depth < 0 {
				return fmt.Errorf("recursion_depth must be >= 0, got %d", depth)
			}
		}
	}

	return nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// getConfigIntWithDefault retrieves an int value from config with a default fallback
func getConfigIntWithDefault(config map[string]interface{}, key string, defaultVal int) int {
	if config == nil {
		return defaultVal
	}
	if val, ok := config[key].(float64); ok {
		return int(val)
	}
	if val, ok := config[key].(int); ok {
		return val
	}
	if val, ok := config[key].(int64); ok {
		return int(val)
	}
	return defaultVal
}

// getConfigFloatWithDefault retrieves a float64 value from config with a default fallback
func getConfigFloatWithDefault(config map[string]interface{}, key string, defaultVal float64) float64 {
	if config == nil {
		return defaultVal
	}
	if val, ok := config[key].(float64); ok {
		return val
	}
	if val, ok := config[key].(int); ok {
		return float64(val)
	}
	if val, ok := config[key].(int64); ok {
		return float64(val)
	}
	return defaultVal
}
