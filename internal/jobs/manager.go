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
func (m *Manager) ListJobs(ctx context.Context, opts *interfaces.ListOptions) ([]*models.CrawlJob, error) {
	jobs, err := m.jobStorage.ListJobs(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	return jobs, nil
}

// CountJobs counts jobs matching the provided filters
func (m *Manager) CountJobs(ctx context.Context, opts *interfaces.ListOptions) (int, error) {
	// If filters are present, use filtered count
	if opts != nil && (opts.Status != "" || opts.SourceType != "" || opts.EntityType != "") {
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

// DeleteJob deletes a job and cancels if running
func (m *Manager) DeleteJob(ctx context.Context, jobID string) error {
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

	// Cancel if running
	if job.Status == models.JobStatusRunning {
		job.Status = models.JobStatusCancelled
		if err := m.jobStorage.SaveJob(ctx, job); err != nil {
			m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job status to cancelled")
		}
	}

	// Delete job from storage
	if err := m.jobStorage.DeleteJob(ctx, jobID); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	// Delete job logs (optional but recommended to keep data consistent)
	if err := m.logService.DeleteLogs(ctx, jobID); err != nil {
		m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to delete job logs (non-critical)")
		// Continue even if log deletion fails - it's not critical
	}

	m.logger.Info().
		Str("job_id", jobID).
		Msg("Job deleted successfully")

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
