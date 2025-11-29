package interfaces

import (
	"context"

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
type DefinitionWorker interface {
	// GetType returns the WorkerType this worker handles.
	// Used by the manager for routing steps to the correct worker.
	GetType() models.WorkerType

	// CreateJobs creates queue jobs for the step and returns the parent job ID.
	// The worker is responsible for creating job records and enqueueing work items.
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - step: The job step definition containing configuration
	//   - jobDef: The parent job definition containing source info
	//   - parentJobID: The ID of the parent orchestration job for hierarchy tracking
	// Returns the job ID for tracking (may be same as parentJobID for inline steps)
	CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, parentJobID string) (string, error)

	// ReturnsChildJobs indicates if this worker creates async child jobs.
	// If true, the orchestrator will start job monitoring for completion tracking.
	// If false, the step is considered complete when CreateJobs returns.
	ReturnsChildJobs() bool

	// ValidateConfig validates step configuration before execution.
	// Called by the manager before CreateJobs to fail fast on misconfigurations.
	// Should check for required config fields and valid values.
	ValidateConfig(step models.JobStep) error
}
