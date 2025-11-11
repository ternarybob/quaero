package executor

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// JobManager creates parent jobs, enqueues child jobs to the queue, and manages job orchestration
// for a specific action type. Different implementations handle different action types (crawl, agent,
// database_maintenance, transform, reindex, places_search).
type JobManager interface {
	// CreateParentJob creates a parent job record, enqueues child jobs to the queue, and returns
	// the parent job ID for tracking. The jobID is used to track the parent-child hierarchy.
	// jobDef contains source configuration (source_type, base_url, auth_id).
	CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (jobID string, err error)

	// GetManagerType returns the action type this manager handles (e.g., 'crawl', 'agent',
	// 'database_maintenance', 'transform', 'reindex', 'places_search')
	GetManagerType() string
}
