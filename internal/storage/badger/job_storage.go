package badger

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

// JobStorage implements the JobStorage interface for Badger
type JobStorage struct {
	db     *BadgerDB
	logger arbor.ILogger
}

// NewJobStorage creates a new JobStorage instance
func NewJobStorage(db *BadgerDB, logger arbor.ILogger) interfaces.JobStorage {
	return &JobStorage{
		db:     db,
		logger: logger,
	}
}

func (s *JobStorage) SaveJob(ctx context.Context, job interface{}) error {
	j, ok := job.(*models.QueueJobState)
	if !ok {
		return fmt.Errorf("invalid job type")
	}
	if j.ID == "" {
		return fmt.Errorf("job ID is required")
	}

	// Store ONLY QueueJob (immutable queued job definition)
	// Runtime state (Status, Progress) is tracked via job logs/events
	queueJob := j.ToQueueJob()
	if err := s.db.Store().Upsert(queueJob.ID, queueJob); err != nil {
		return fmt.Errorf("failed to save job: %w", err)
	}
	return nil
}

func (s *JobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	// Load QueueJob from storage (immutable queued job)
	var queueJob models.QueueJob
	if err := s.db.Store().Get(jobID, &queueJob); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, fmt.Errorf("job not found: %s", jobID)
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Convert to QueueJobState for in-memory use
	// Runtime state will be populated from job logs/events by the caller
	job := models.NewQueueJobState(&queueJob)
	return job, nil
}

func (s *JobStorage) UpdateJob(ctx context.Context, job interface{}) error {
	return s.SaveJob(ctx, job)
}

func (s *JobStorage) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.QueueJobState, error) {
	query := badgerhold.Where("ID").Ne("")

	if opts != nil {
		// TODO: Status filtering removed - QueueJob doesn't have Status field
		// Status is now tracked in job logs/events, not in stored QueueJob
		// Need to implement status filtering via job logs or store status separately
		// For now, all jobs are returned regardless of status filter

		// Note: Type field is not available in JobListOptions in current interface definition
		// If needed, interface should be updated. For now, ignoring Type filter if not present.

		if opts.ParentID != "" {
			// Special handling for "root" value - query for jobs with nil ParentID
			if opts.ParentID == "root" {
				// Query for root jobs (ParentID is nil)
				query = query.And("ParentID").IsNil()
			} else {
				// Query for child jobs with specific parent ID
				query = query.And("ParentID").Eq(&opts.ParentID)
			}
		}
		if opts.Limit > 0 {
			query = query.Limit(opts.Limit)
		}
		if opts.Offset > 0 {
			query = query.Skip(opts.Offset)
		}
		// Sorting - TEMPORARILY DISABLED to debug BadgerHold embedded struct issue
		// TODO: Re-enable sorting once BadgerHold field path is fixed
		// if opts.OrderBy != "" {
		// 	if opts.OrderDir == "DESC" {
		// 		query = query.SortBy(opts.OrderBy).Reverse()
		// 	} else {
		// 		query = query.SortBy(opts.OrderBy)
		// 	}
		// } else {
		// 	// Default sort
		// 	query = query.SortBy("CreatedAt").Reverse()
		// }
	}

	// Query QueueJob from storage (immutable queued jobs)
	var queueJobs []models.QueueJob
	if err := s.db.Store().Find(&queueJobs, query); err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	// Convert QueueJob to QueueJobState structs for in-memory use
	// Runtime state will be populated from job logs/events by the caller
	result := make([]*models.QueueJobState, len(queueJobs))
	for i := range queueJobs {
		result[i] = models.NewQueueJobState(&queueJobs[i])
	}
	return result, nil
}

func (s *JobStorage) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	stats := make(map[string]*interfaces.JobChildStats)
	for _, parentID := range parentIDs {
		var children []models.QueueJobState
		// This is inefficient, but BadgerHold doesn't support aggregation easily
		if err := s.db.Store().Find(&children, badgerhold.Where("ParentID").Eq(parentID)); err != nil {
			return nil, err
		}

		childStats := &interfaces.JobChildStats{
			ChildCount: len(children),
		}

		for _, child := range children {
			switch child.Status {
			case models.JobStatusCompleted:
				childStats.CompletedChildren++
			case models.JobStatusFailed:
				childStats.FailedChildren++
			case models.JobStatusRunning:
				childStats.RunningChildren++
			case models.JobStatusPending:
				childStats.PendingChildren++
			case models.JobStatusCancelled:
				childStats.CancelledChildren++
			}
		}

		stats[parentID] = childStats
	}
	return stats, nil
}

func (s *JobStorage) GetChildJobs(ctx context.Context, parentID string) ([]*models.QueueJob, error) {
	var queueJobs []models.QueueJob
	if err := s.db.Store().Find(&queueJobs, badgerhold.Where("ParentID").Eq(parentID).SortBy("CreatedAt").Reverse()); err != nil {
		return nil, fmt.Errorf("failed to get child jobs: %w", err)
	}

	result := make([]*models.QueueJob, len(queueJobs))
	for i := range queueJobs {
		result[i] = &queueJobs[i]
	}
	return result, nil
}

func (s *JobStorage) GetJobsByStatus(ctx context.Context, status string) ([]*models.QueueJob, error) {
	// TODO: Status is not stored in QueueJob (immutable), it's tracked in job logs
	// This method needs to be refactored to query job logs instead
	// For now, return empty slice
	s.logger.Warn().Str("status", status).Msg("GetJobsByStatus called but status not stored in QueueJob - needs refactoring")
	return []*models.QueueJob{}, nil
}

func (s *JobStorage) UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error {
	// TODO: Status is not stored in QueueJob (immutable), it's tracked in job logs
	// This method needs to be refactored to update job logs instead
	// For now, log warning and return nil
	s.logger.Warn().Str("job_id", jobID).Str("status", status).Msg("UpdateJobStatus called but status not stored in QueueJob - needs refactoring")
	return nil
}

func (s *JobStorage) UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error {
	// TODO: Progress is not stored in QueueJob (immutable), it's tracked in job logs
	// This method needs to be refactored to update job logs instead
	// For now, log warning and return nil
	s.logger.Warn().Str("job_id", jobID).Msg("UpdateJobProgress called but progress not stored in QueueJob - needs refactoring")
	return nil
}

func (s *JobStorage) UpdateProgressCountersAtomic(ctx context.Context, jobID string, completedDelta, pendingDelta, totalDelta, failedDelta int) error {
	// TODO: Progress is not stored in QueueJob (immutable), it's tracked in job logs
	// This method needs to be refactored to update job logs instead
	// For now, log warning and return nil
	s.logger.Warn().Str("job_id", jobID).Msg("UpdateProgressCountersAtomic called but progress not stored in QueueJob - needs refactoring")
	return nil
}

func (s *JobStorage) UpdateJobHeartbeat(ctx context.Context, jobID string) error {
	// TODO: Heartbeat is not stored in QueueJob (immutable), it's tracked in job logs
	// This method needs to be refactored to update job logs instead
	// For now, log warning and return nil
	s.logger.Warn().Str("job_id", jobID).Msg("UpdateJobHeartbeat called but heartbeat not stored in QueueJob - needs refactoring")
	return nil
}

func (s *JobStorage) GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]*models.QueueJob, error) {
	// TODO: Status and LastHeartbeat are not stored in QueueJob (immutable), they're tracked in job logs
	// This method needs to be refactored to query job logs instead
	// For now, return empty slice
	s.logger.Warn().Int("threshold_minutes", staleThresholdMinutes).Msg("GetStaleJobs called but status/heartbeat not stored in QueueJob - needs refactoring")
	return []*models.QueueJob{}, nil
}

func (s *JobStorage) DeleteJob(ctx context.Context, jobID string) error {
	if err := s.db.Store().Delete(jobID, &models.QueueJob{}); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil
		}
		return err
	}
	return nil
}

func (s *JobStorage) CountJobs(ctx context.Context) (int, error) {
	count, err := s.db.Store().Count(&models.QueueJob{}, nil)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *JobStorage) CountJobsByStatus(ctx context.Context, status string) (int, error) {
	// TODO: Status is not stored in QueueJob (immutable), it's tracked in job logs
	// This method needs to be refactored to query job logs instead
	// For now, return 0
	s.logger.Warn().Str("status", status).Msg("CountJobsByStatus called but status not stored in QueueJob - needs refactoring")
	return 0, nil
}

func (s *JobStorage) CountJobsWithFilters(ctx context.Context, opts *interfaces.JobListOptions) (int, error) {
	query := badgerhold.Where("ID").Ne("")

	if opts != nil {
		// TODO: Status filtering removed - QueueJob doesn't have Status field
		// Status is now tracked in job logs/events, not in stored QueueJob
		// Need to implement status filtering via job logs or store status separately
		// For now, status filter is ignored

		// Type filter removed as it's not in options
		if opts.ParentID != "" {
			// Special handling for "root" value - query for jobs with nil ParentID
			if opts.ParentID == "root" {
				// Query for root jobs (ParentID is nil)
				query = query.And("ParentID").IsNil()
			} else {
				// Query for child jobs with specific parent ID
				query = query.And("ParentID").Eq(&opts.ParentID)
			}
		}
	}

	count, err := s.db.Store().Count(&models.QueueJob{}, query)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *JobStorage) AppendJobLog(ctx context.Context, jobID string, logEntry models.JobLogEntry) error {
	// Deprecated, but implemented for interface compliance
	// In Badger, we might store logs separately or in the job struct.
	// Given the deprecation notice, we'll skip implementation or redirect to JobLogStorage if possible.
	// For now, no-op or simple log.
	return nil
}

func (s *JobStorage) GetJobLogs(ctx context.Context, jobID string) ([]models.JobLogEntry, error) {
	// Deprecated
	return []models.JobLogEntry{}, nil
}

func (s *JobStorage) MarkURLSeen(ctx context.Context, jobID string, url string) (bool, error) {
	// We need a separate collection/type for seen URLs to ensure uniqueness
	type SeenURL struct {
		ID    string // Composite key: jobID + url
		JobID string
		URL   string
	}

	key := fmt.Sprintf("%s|%s", jobID, url)
	seen := SeenURL{
		ID:    key,
		JobID: jobID,
		URL:   url,
	}

	// Try to insert. If it exists, it's seen.
	// BadgerHold Upsert overwrites, so we need to check existence first or use Insert which fails on conflict?
	// BadgerHold Insert fails if key exists? No, it overwrites unless we check.
	// Actually, Insert documentation says: "If the key already exists, it will be overwritten."
	// Wait, Insert in BadgerHold usually implies new?
	// Let's check existence.

	var existing SeenURL
	err := s.db.Store().Get(key, &existing)
	if err == nil {
		return false, nil // Already seen
	}
	if err != badgerhold.ErrNotFound {
		return false, err
	}

	if err := s.db.Store().Insert(key, &seen); err != nil {
		return false, err
	}

	return true, nil
}

func (s *JobStorage) MarkRunningJobsAsPending(ctx context.Context, reason string) (int, error) {
	// TODO: Status is not stored in QueueJob (immutable), it's tracked in job logs
	// This method needs to be refactored to update job logs instead
	// For now, log warning and return 0
	s.logger.Warn().Str("reason", reason).Msg("MarkRunningJobsAsPending called but status not stored in QueueJob - needs refactoring")
	return 0, nil
}
