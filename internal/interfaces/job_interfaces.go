package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/interfaces/jobtypes"
	"github.com/ternarybob/quaero/internal/models"
)

// JobMonitor monitors parent job progress and aggregates child job statistics.
// It runs in background goroutines (not via queue) and publishes real-time progress events.
// Monitors subscribe to child job status changes for real-time tracking.
type JobMonitor interface {
	// StartMonitoring begins monitoring a parent job in a background goroutine.
	// Takes the full queued job (not just ID) to access config fields like source_type and entity_type.
	// Returns immediately after starting the monitoring goroutine.
	StartMonitoring(ctx context.Context, job *models.QueueJob)

	// SubscribeToJobEvents sets up event subscriptions for real-time child job tracking.
	// This is called during orchestrator initialization.
	SubscribeToJobEvents()
}

// StepMonitor monitors a step job's children (worker jobs) and marks the step
// as complete when all children finish. Each step with children gets its own
// StepMonitor running in a goroutine.
//
// Hierarchy: Manager -> Steps -> Jobs
// StepMonitor handles: Step -> Jobs (monitors jobs under a step)
type StepMonitor interface {
	// StartMonitoring starts monitoring a step job's children in a background goroutine.
	// When all children complete, the step is marked as completed.
	StartMonitoring(ctx context.Context, stepJob *models.QueueJob)
}

// JobStatusManager provides methods for managing job status and lifecycle.
// Used by monitors to update job state without creating circular dependencies.
type JobStatusManager interface {
	// UpdateJobStatus updates the status of a job
	UpdateJobStatus(ctx context.Context, jobID string, status string) error
	// SetJobFinished marks a job as finished
	SetJobFinished(ctx context.Context, jobID string) error
	// SetJobError sets an error on a job
	SetJobError(ctx context.Context, jobID string, errorMsg string) error
	// AddJobLog adds a log entry to a job (originator determined by job type)
	AddJobLog(ctx context.Context, jobID string, level string, message string) error
	// AddJobLogWithOriginator adds a log entry with explicit originator:
	//   - "step" for StepMonitor logs (e.g., "Starting workers")
	//   - "worker" for worker logs (e.g., "Document saved")
	//   - "" (empty) for system/monitor logs (e.g., "Child job X → completed")
	AddJobLogWithOriginator(ctx context.Context, jobID string, level string, message string, originator string) error
	// AddJobLogWithContext adds a log entry with explicit step name and originator.
	// Use this when the caller knows the step context (e.g., StepMonitor).
	AddJobLogWithContext(ctx context.Context, jobID string, level string, message string, stepName string, originator string) error
	// GetJobChildStats returns child job statistics for given job IDs
	GetJobChildStats(ctx context.Context, jobIDs []string) (map[string]*jobtypes.JobChildStats, error)
}

// JobWorker defines the interface that all job workers must implement.
// The queue engine uses this interface to execute jobs in a type-agnostic manner.
// Workers process individual jobs from the queue and perform the actual work.
//
// JobWorker is used for queue job execution, while DefinitionWorker is used for
// creating jobs from job definition steps.
type JobWorker interface {
	// Execute processes a single job from the queue. Returns error if execution fails.
	// Worker is responsible for updating job status and logging progress.
	Execute(ctx context.Context, job *models.QueueJob) error

	// GetWorkerType returns the job type this worker handles.
	// Examples: "database_maintenance", "crawler_url", "agent"
	GetWorkerType() string

	// Validate validates that the queued job is compatible with this worker.
	// Returns error if the queued job is invalid for this worker.
	Validate(job *models.QueueJob) error
}

// JobSpawner defines the interface for workers that can spawn child jobs.
// This is optional - not all job workers need to implement this.
type JobSpawner interface {
	// SpawnChildJob creates and enqueues a child job
	// The child job will be linked to the parent via ParentID
	SpawnChildJob(ctx context.Context, parentJob *models.QueueJob, childType, childName string, config map[string]interface{}) error
}

// DefinitionWorker is the interface for workers that handle job definition steps.
// Each DefinitionWorker handles a specific WorkerType and provides type-safe routing.
// When a job definition is executed, the manager routes each step to its corresponding
// DefinitionWorker based on WorkerType.
//
// This is distinct from JobWorker which executes queue jobs directly.
//
// Hierarchy: Manager -> Steps -> Jobs
// Workers create jobs under their step (parent_id = stepID, manager_id = managerID)
type DefinitionWorker interface {
	// GetType returns the WorkerType this worker handles.
	// Used by the manager for routing steps to the correct worker.
	GetType() models.WorkerType

	// CreateJobs creates queue jobs for the step and returns the job ID.
	// The worker is responsible for creating job records and enqueueing work items.
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - step: The job step definition containing configuration
	//   - jobDef: The parent job definition containing source info
	//   - stepID: The ID of the step job - jobs should set parent_id = stepID
	// Returns the job ID for tracking (may be same as stepID for inline steps)
	// Note: Jobs should retrieve manager_id from step metadata or job context
	CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string) (string, error)

	// ReturnsChildJobs indicates if this worker creates async child jobs.
	// If true, the orchestrator will start job monitoring for completion tracking.
	// If false, the step is considered complete when CreateJobs returns.
	ReturnsChildJobs() bool

	// ValidateConfig validates step configuration before execution.
	// Called by the manager before CreateJobs to fail fast on misconfigurations.
	// Should check for required config fields and valid values.
	ValidateConfig(step models.JobStep) error
}

// StepManager defines the interface for managing step workers and routing steps.
type StepManager interface {
	// RegisterWorker registers a worker for a specific step type
	RegisterWorker(worker DefinitionWorker)

	// HasWorker checks if a worker exists for a type
	HasWorker(workerType models.WorkerType) bool

	// GetWorker retrieves a worker by type
	GetWorker(workerType models.WorkerType) DefinitionWorker

	// Execute routes a step to the appropriate worker
	Execute(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, parentJobID string) (string, error)
}
