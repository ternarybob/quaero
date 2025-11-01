package jobs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Manager manages job CRUD operations
type Manager struct {
	queueManager interfaces.QueueManager
	jobStorage   interfaces.JobStorage
	logService   interfaces.LogService
	logger       arbor.ILogger
}

// VERIFICATION COMMENT 2: JobTree removed - flat hierarchy model does not require tree structure
// All crawler_url messages point to root job ID via ParentID field
// Progress tracked at job level (TotalURLs, CompletedURLs, PendingURLs)
// UI displays job-level progress, not hierarchical tree

// NewManager creates a new job manager
func NewManager(jobStorage interfaces.JobStorage, queueMgr interfaces.QueueManager, logService interfaces.LogService, logger arbor.ILogger) *Manager {
	return &Manager{
		queueManager: queueMgr,
		jobStorage:   jobStorage,
		logService:   logService,
		logger:       logger,
	}
}

func (m *Manager) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	stats, err := m.jobStorage.GetJobChildStats(ctx, parentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get child stats: %w", err)
	}

	m.logger.Debug().Int("parent_count", len(parentIDs)).Int("stats_count", len(stats)).Msg("Retrieved child statistics")
	return stats, nil
}

// CreateJob creates a new job and enqueues parent message
func (m *Manager) CreateJob(ctx context.Context, sourceType, sourceID string, config map[string]interface{}) (string, error) {
	// Generate job ID
	jobID := uuid.New().String()

	// Create job with basic config
	job := &models.CrawlJob{
		ID:         jobID,
		SourceType: sourceType,
		EntityType: sourceID,
		Status:     models.JobStatusPending,
	}

	// Save job to storage
	if err := m.jobStorage.SaveJob(ctx, job); err != nil {
		return "", fmt.Errorf("failed to save job: %w", err)
	}

	// NOTE: Parent message enqueuing removed - seed URLs are enqueued directly
	// by CrawlerService.StartCrawl() which creates individual crawler_url messages.
	// Job tracking is handled via JobStorage, not via queue messages.

	m.logger.Info().
		Str("job_id", jobID).
		Str("source_type", sourceType).
		Str("entity_type", sourceID).
		Msg("Job created (seed URLs will be enqueued by StartCrawl)")

	return jobID, nil
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	jobInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return jobInterface, nil
}

// ListJobs lists jobs with optional filters
func (m *Manager) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.CrawlJob, error) {
	jobs, err := m.jobStorage.ListJobs(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	return jobs, nil
}

// CountJobs counts jobs matching the provided filters
func (m *Manager) CountJobs(ctx context.Context, opts *interfaces.JobListOptions) (int, error) {
	// If filters are present, use filtered count
	if opts != nil && (opts.Status != "" || opts.SourceType != "" || opts.EntityType != "" || opts.ParentID != "") {
		count, err := m.jobStorage.CountJobsWithFilters(ctx, opts)
		if err != nil {
			return 0, fmt.Errorf("failed to count filtered jobs: %w", err)
		}
		return count, nil
	}

	// No filters: use global count
	count, err := m.jobStorage.CountJobs(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}
	return count, nil
}

// UpdateJob updates job metadata
func (m *Manager) UpdateJob(ctx context.Context, job interface{}) error {
	// Type assert to concrete type for storage
	crawlJob, ok := job.(*models.CrawlJob)
	if !ok {
		return fmt.Errorf("invalid job type: expected *models.CrawlJob")
	}

	if err := m.jobStorage.SaveJob(ctx, crawlJob); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	m.logger.Debug().
		Str("job_id", crawlJob.ID).
		Msg("Job updated")

	return nil
}

// DeleteJob deletes a job and all its child jobs recursively.
// If the job has children, they are deleted first in a cascade operation.
// Each deletion is logged individually for audit purposes.
// If any child deletion fails, the error is logged but deletion continues.
// The parent job is deleted even if some children fail to delete.
// Returns an aggregated error if any deletions failed.
func (m *Manager) DeleteJob(ctx context.Context, jobID string) error {
	return m.deleteJobRecursive(ctx, jobID, 0)
}

// deleteJobRecursive handles recursive deletion with depth tracking
func (m *Manager) deleteJobRecursive(ctx context.Context, jobID string, depth int) error {
	// Prevent infinite recursion
	const maxDepth = 10
	if depth > maxDepth {
		return fmt.Errorf("maximum recursion depth (%d) exceeded for job %s", maxDepth, jobID)
	}

	// Get job to check status
	jobInterface, err := m.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	// Type assert to concrete type
	job, ok := jobInterface.(*models.CrawlJob)
	if !ok {
		return fmt.Errorf("invalid job type: expected *models.CrawlJob")
	}

	// Check for child jobs and cascade delete
	children, err := m.jobStorage.GetChildJobs(ctx, jobID)
	if err != nil {
		m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to get child jobs, continuing with deletion")
	}

	if len(children) > 0 {
		m.logger.Info().
			Str("parent_id", jobID).
			Int("child_count", len(children)).
			Int("depth", depth).
			Msg("Cascading delete to child jobs")

		var errs []error
		successCount := 0
		errorCount := 0

		for _, child := range children {
			m.logger.Debug().
				Str("parent_id", jobID).
				Str("child_id", child.ID).
				Msg("Deleting child job")

			// Recursively delete child (which will delete its children if any)
			if err := m.deleteJobRecursive(ctx, child.ID, depth+1); err != nil {
				m.logger.Warn().
					Err(err).
					Str("parent_id", jobID).
					Str("child_id", child.ID).
					Msg("Failed to delete child job, continuing")
				errs = append(errs, fmt.Errorf("child %s: %w", child.ID, err))
				errorCount++
			} else {
				successCount++
			}
		}

		m.logger.Info().
			Str("job_id", jobID).
			Int("children_deleted", successCount).
			Int("children_failed", errorCount).
			Msg("Cascade deletion completed")

		// If any child deletions failed, log aggregated errors but continue with parent deletion
		if len(errs) > 0 {
			m.logger.Warn().
				Str("job_id", jobID).
				Int("error_count", len(errs)).
				Msg("Some child deletions failed, but continuing with parent deletion")
		}
	}

	// Cancel if running
	if job.Status == models.JobStatusRunning {
		job.Status = models.JobStatusCancelled
		if err := m.jobStorage.SaveJob(ctx, job); err != nil {
			m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job status to cancelled")
		}
	}

	// Delete job from storage
	// Note: FK CASCADE automatically deletes associated job_logs and job_seen_urls
	if err := m.jobStorage.DeleteJob(ctx, jobID); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	m.logger.Info().
		Str("job_id", jobID).
		Msg("Job deleted successfully (logs cascade deleted by FK)")

	return nil
}

// CopyJob duplicates a job with a new ID
func (m *Manager) CopyJob(ctx context.Context, jobID string) (string, error) {
	// Get original job
	jobInterface, err := m.GetJob(ctx, jobID)
	if err != nil {
		return "", err
	}

	// Type assert to concrete type
	originalJob, ok := jobInterface.(*models.CrawlJob)
	if !ok {
		return "", fmt.Errorf("invalid job type: expected *models.CrawlJob")
	}

	// Generate new name
	newName := originalJob.Name
	if newName == "" {
		newName = fmt.Sprintf("Copy of job %s", jobID)
	} else {
		newName = fmt.Sprintf("Copy of %s", newName)
	}

	// Create new job with copied config
	newJob := &models.CrawlJob{
		ID:                   uuid.New().String(),
		Name:                 newName,
		Description:          originalJob.Description,
		SourceType:           originalJob.SourceType,
		EntityType:           originalJob.EntityType,
		Config:               originalJob.Config,
		SourceConfigSnapshot: originalJob.SourceConfigSnapshot,
		AuthSnapshot:         originalJob.AuthSnapshot,
		RefreshSource:        originalJob.RefreshSource,
		SeedURLs:             originalJob.SeedURLs,
		Status:               models.JobStatusPending,
	}

	// Save new job
	if err := m.jobStorage.SaveJob(ctx, newJob); err != nil {
		return "", fmt.Errorf("failed to copy job: %w", err)
	}

	m.logger.Info().
		Str("original_job_id", jobID).
		Str("new_job_id", newJob.ID).
		Msg("Job copied")

	return newJob.ID, nil
}

// StopAllChildJobs cancels all running and pending child jobs of the specified parent job.
// This is used by error tolerance threshold management to stop child jobs
// when the parent job's failure threshold is exceeded.
// Returns the count of jobs that were successfully cancelled.
func (m *Manager) StopAllChildJobs(ctx context.Context, parentID string) (int, error) {
	// Query all running child jobs
	runningChildren, err := m.jobStorage.ListJobs(ctx, &interfaces.JobListOptions{
		ParentID: parentID,
		Status:   string(models.JobStatusRunning),
		Limit:    0, // No limit - get all running children
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list running child jobs: %w", err)
	}

	// Query all pending child jobs
	pendingChildren, err := m.jobStorage.ListJobs(ctx, &interfaces.JobListOptions{
		ParentID: parentID,
		Status:   string(models.JobStatusPending),
		Limit:    0, // No limit - get all pending children
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list pending child jobs: %w", err)
	}

	totalChildren := len(runningChildren) + len(pendingChildren)
	if totalChildren == 0 {
		m.logger.Debug().
			Str("parent_id", parentID).
			Msg("No running or pending child jobs to cancel")
		return 0, nil
	}

	m.logger.Info().
		Str("parent_id", parentID).
		Int("running_count", len(runningChildren)).
		Int("pending_count", len(pendingChildren)).
		Int("total_count", totalChildren).
		Msg("Stopping all running and pending child jobs due to error tolerance threshold")

	cancelledRunning := 0
	cancelledPending := 0

	// Cancel running children
	for _, child := range runningChildren {
		child.Status = models.JobStatusCancelled
		child.Error = "Cancelled by parent job error tolerance threshold"

		if err := m.jobStorage.SaveJob(ctx, child); err != nil {
			m.logger.Warn().
				Err(err).
				Str("parent_id", parentID).
				Str("child_id", child.ID).
				Msg("Failed to cancel running child job, continuing with others")
			continue
		}

		m.logger.Debug().
			Str("parent_id", parentID).
			Str("child_id", child.ID).
			Str("original_status", "running").
			Msg("Child job cancelled")

		cancelledRunning++
	}

	// Cancel pending children
	for _, child := range pendingChildren {
		child.Status = models.JobStatusCancelled
		child.Error = "Cancelled by parent job error tolerance threshold"

		if err := m.jobStorage.SaveJob(ctx, child); err != nil {
			m.logger.Warn().
				Err(err).
				Str("parent_id", parentID).
				Str("child_id", child.ID).
				Msg("Failed to cancel pending child job, continuing with others")
			continue
		}

		m.logger.Debug().
			Str("parent_id", parentID).
			Str("child_id", child.ID).
			Str("original_status", "pending").
			Msg("Child job cancelled")

		cancelledPending++
	}

	totalCancelled := cancelledRunning + cancelledPending

	m.logger.Info().
		Str("parent_id", parentID).
		Int("cancelled_running", cancelledRunning).
		Int("cancelled_pending", cancelledPending).
		Int("total_cancelled", totalCancelled).
		Int("total_children", totalChildren).
		Msg("Completed stopping child jobs")

	return totalCancelled, nil
}

// VERIFICATION COMMENT 2: GetJobWithChildren removed - flat hierarchy model adopted
// Design Decision: FLAT HIERARCHY (chosen over nested tree)
//
// Rationale:
// - All child crawler_url messages inherit root job's ParentID (msg.ParentID)
// - Progress tracked at single job level via TotalURLs/CompletedURLs/PendingURLs
// - URL deduplication via job_seen_urls table (job_id + url composite key)
// - Simplified completion detection (single PendingURLs counter)
// - No recursive traversal or complex aggregation needed
//
// UI Implications:
// - Display job-level progress bars (% complete based on job.Progress)
// - Show aggregate stats (total/completed/failed URLs)
// - No tree visualization needed (list view of jobs only)
// - Queue stats show pending messages count for operational monitoring
//
// Alternative (rejected): Nested tree would require:
// - Recursive message traversal to build hierarchy
// - Complex progress aggregation across tree levels
// - Parent-child relationships stored in messages (msg.ParentID = immediate parent)
// - Tree-building queries and rendering logic
// - Added complexity with minimal benefit for crawler use case
