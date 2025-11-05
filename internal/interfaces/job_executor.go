// -----------------------------------------------------------------------
// Job Executor Interface - Common interface for all job executors
// -----------------------------------------------------------------------

package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// JobExecutor defines the interface that all job executors must implement.
// The queue engine uses this interface to execute jobs in a type-agnostic manner.
type JobExecutor interface {
	// Execute executes a job based on the job model
	// Returns error if execution fails
	Execute(ctx context.Context, job *models.JobModel) error

	// GetJobType returns the job type this executor handles
	// Examples: "database_maintenance", "crawler", "summarizer"
	GetJobType() string

	// Validate validates that the job model is compatible with this executor
	// Returns error if the job model is invalid for this executor
	Validate(job *models.JobModel) error
}

// JobSpawner defines the interface for jobs that can spawn child jobs.
// This is optional - not all job executors need to implement this.
type JobSpawner interface {
	// SpawnChildJob creates and enqueues a child job
	// The child job will be linked to the parent via ParentID
	SpawnChildJob(ctx context.Context, parentJob *models.JobModel, childType, childName string, config map[string]interface{}) error
}

