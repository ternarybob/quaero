package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// StepManager creates parent jobs, enqueues child jobs to the queue, and manages job orchestration
// for a specific action type (job definition step). Different implementations handle different action
// types (crawl, agent, database_maintenance, transform, reindex, places_search).
// This is distinct from interfaces.JobManager which handles job CRUD operations.
type StepManager interface {
	// CreateParentJob creates a parent job record, enqueues child jobs to the queue, and returns
	// the parent job ID for tracking. The jobID is used to track the parent-child hierarchy.
	// jobDef contains source configuration (source_type, base_url, auth_id).
	CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (jobID string, err error)

	// GetManagerType returns the action type this manager handles (e.g., 'crawl', 'agent',
	// 'database_maintenance', 'transform', 'reindex', 'places_search')
	GetManagerType() string

	// ReturnsChildJobs returns true if this manager creates asynchronous child jobs that need monitoring.
	// Returns false if the manager performs work synchronously or doesn't create child jobs.
	ReturnsChildJobs() bool
}

// JobMonitor monitors parent job progress and aggregates child job statistics.
// It runs in background goroutines (not via queue) and publishes real-time progress events.
// Monitors subscribe to child job status changes for real-time tracking.
type JobMonitor interface {
	// StartMonitoring begins monitoring a parent job in a background goroutine.
	// Takes the full job model (not just ID) to access config fields like source_type and entity_type.
	// Returns immediately after starting the monitoring goroutine.
	StartMonitoring(ctx context.Context, job *models.JobModel)

	// SubscribeToJobEvents sets up event subscriptions for real-time child job tracking.
	// This is called during orchestrator initialization.
	SubscribeToJobEvents()
}

// JobWorker defines the interface that all job workers must implement.
// The queue engine uses this interface to execute jobs in a type-agnostic manner.
// Workers process individual jobs from the queue and perform the actual work.
type JobWorker interface {
	// Execute processes a single job from the queue. Returns error if execution fails.
	// Worker is responsible for updating job status and logging progress.
	Execute(ctx context.Context, job *models.JobModel) error

	// GetWorkerType returns the job type this worker handles.
	// Examples: "database_maintenance", "crawler_url", "agent"
	GetWorkerType() string

	// Validate validates that the job model is compatible with this worker.
	// Returns error if the job model is invalid for this worker.
	Validate(job *models.JobModel) error
}

// JobSpawner defines the interface for workers that can spawn child jobs.
// This is optional - not all job workers need to implement this.
type JobSpawner interface {
	// SpawnChildJob creates and enqueues a child job
	// The child job will be linked to the parent via ParentID
	SpawnChildJob(ctx context.Context, parentJob *models.JobModel, childType, childName string, config map[string]interface{}) error
}
