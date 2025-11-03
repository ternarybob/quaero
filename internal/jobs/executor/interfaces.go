package executor

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// StepExecutor executes a single step of a job definition
// Different implementations handle different action types (crawl, transform, summarize, etc.)
type StepExecutor interface {
	// ExecuteStep executes a step and returns the job ID created
	// The jobID is used to track the parent-child hierarchy
	ExecuteStep(ctx context.Context, step models.JobStep, sources []string, parentJobID string) (jobID string, err error)

	// GetStepType returns the action type this executor handles (e.g., "crawl", "transform")
	GetStepType() string
}
