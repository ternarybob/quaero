package manager

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
)

// ReindexManager orchestrates FTS5 full-text search index rebuilding workflows for optimal search performance
type ReindexManager struct {
	documentStorage interfaces.DocumentStorage
	jobManager      *jobs.Manager
	logger          arbor.ILogger
}

// Compile-time assertion: ReindexManager implements StepManager interface
var _ interfaces.StepManager = (*ReindexManager)(nil)

// NewReindexManager creates a new reindex manager for orchestrating FTS5 index rebuilding
func NewReindexManager(documentStorage interfaces.DocumentStorage, jobManager *jobs.Manager, logger arbor.ILogger) *ReindexManager {
	return &ReindexManager{
		documentStorage: documentStorage,
		jobManager:      jobManager,
		logger:          logger,
	}
}

// CreateParentJob executes a reindex operation to rebuild the FTS5 full-text search index.
// This is a synchronous operation. Returns a placeholder job ID since reindex doesn't create async jobs.
func (m *ReindexManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	m.logger.Info().
		Str("step_name", step.Name).
		Str("action", step.Action).
		Str("parent_job_id", parentJobID).
		Msg("Orchestrating reindex")

	// Generate job ID for this step
	jobID := uuid.New().String()

	// Create job record for tracking
	job := &jobs.Job{
		ID:       jobID,
		ParentID: &parentJobID,
		Type:     "reindex",
		Name:     step.Name, // Use step name as job name
		Phase:    "core",
		Status:   "running",
	}

	// Save job record
	if err := m.jobManager.CreateJobRecord(ctx, job); err != nil {
		m.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to create reindex job record")
		return "", fmt.Errorf("failed to create job record: %w", err)
	}

	// Check for dry_run configuration
	dryRun := false
	if step.Config != nil {
		if val, ok := step.Config["dry_run"]; ok {
			if boolVal, ok := val.(bool); ok {
				dryRun = boolVal
			}
		}
	}

	if dryRun {
		m.logger.Info().
			Str("job_id", jobID).
			Msg("Dry run mode - skipping actual index rebuild")

		// Mark job as completed
		if err := m.jobManager.UpdateJobStatus(ctx, jobID, "completed"); err != nil {
			m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job status to completed")
		}
	} else {
		// Perform actual index rebuild
		m.logger.Info().
			Str("job_id", jobID).
			Msg("Rebuilding FTS5 full-text search index")

		if err := m.documentStorage.RebuildFTS5Index(); err != nil {
			m.logger.Error().
				Err(err).
				Str("job_id", jobID).
				Msg("Failed to rebuild FTS5 index")

			// Mark job as failed
			if updateErr := m.jobManager.SetJobError(ctx, jobID, err.Error()); updateErr != nil {
				m.logger.Error().Err(updateErr).Str("job_id", jobID).Msg("Failed to set job error")
			}

			return "", fmt.Errorf("failed to rebuild FTS5 index: %w", err)
		}

		m.logger.Info().
			Str("job_id", jobID).
			Msg("FTS5 index rebuilt successfully")

		// Mark job as completed
		if err := m.jobManager.UpdateJobStatus(ctx, jobID, "completed"); err != nil {
			m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job status to completed")
		}
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Bool("dry_run", dryRun).
		Msg("Reindex step completed successfully")

	return jobID, nil
}

// GetManagerType returns "reindex" - the action type this manager handles
func (m *ReindexManager) GetManagerType() string {
	return "reindex"
}

// ReturnsChildJobs returns false since reindex is synchronous
func (m *ReindexManager) ReturnsChildJobs() bool {
	return false
}
