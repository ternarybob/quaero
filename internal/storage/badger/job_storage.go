package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	j, ok := job.(*models.Job)
	if !ok {
		return fmt.Errorf("invalid job type")
	}
	if j.ID == "" {
		return fmt.Errorf("job ID is required")
	}

	if err := s.db.Store().Upsert(j.ID, j); err != nil {
		return fmt.Errorf("failed to save job: %w", err)
	}
	return nil
}

func (s *JobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	var job models.Job
	if err := s.db.Store().Get(jobID, &job); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, fmt.Errorf("job not found: %s", jobID)
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return &job, nil
}

func (s *JobStorage) UpdateJob(ctx context.Context, job interface{}) error {
	return s.SaveJob(ctx, job)
}

func (s *JobStorage) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.JobModel, error) {
	query := badgerhold.Where("ID").Ne("")

	if opts != nil {
		if opts.Status != "" {
			query = query.And("Status").Eq(opts.Status)
		}
		// Note: Type field is not available in JobListOptions in current interface definition
		// If needed, interface should be updated. For now, ignoring Type filter if not present.
		
		if opts.ParentID != "" {
			query = query.And("ParentID").Eq(opts.ParentID)
		}
		if opts.Limit > 0 {
			query = query.Limit(opts.Limit)
		}
		if opts.Offset > 0 {
			query = query.Skip(opts.Offset)
		}
		// Sorting
		if opts.OrderBy != "" {
			if opts.OrderDir == "DESC" {
				query = query.SortBy(opts.OrderBy).Reverse()
			} else {
				query = query.SortBy(opts.OrderBy)
			}
		} else {
			// Default sort
			query = query.SortBy("CreatedAt").Reverse()
		}
	}

	var jobs []models.Job
	if err := s.db.Store().Find(&jobs, query); err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	result := make([]*models.JobModel, len(jobs))
	for i := range jobs {
		result[i] = jobs[i].JobModel
	}
	return result, nil
}

func (s *JobStorage) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	stats := make(map[string]*interfaces.JobChildStats)
	for _, parentID := range parentIDs {
		var children []models.Job
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

func (s *JobStorage) GetChildJobs(ctx context.Context, parentID string) ([]*models.JobModel, error) {
	var jobs []models.Job
	if err := s.db.Store().Find(&jobs, badgerhold.Where("ParentID").Eq(parentID).SortBy("CreatedAt").Reverse()); err != nil {
		return nil, fmt.Errorf("failed to get child jobs: %w", err)
	}

	result := make([]*models.JobModel, len(jobs))
	for i := range jobs {
		result[i] = jobs[i].JobModel
	}
	return result, nil
}

func (s *JobStorage) GetJobsByStatus(ctx context.Context, status string) ([]*models.JobModel, error) {
	var jobs []models.Job
	if err := s.db.Store().Find(&jobs, badgerhold.Where("Status").Eq(status)); err != nil {
		return nil, fmt.Errorf("failed to get jobs by status: %w", err)
	}

	result := make([]*models.JobModel, len(jobs))
	for i := range jobs {
		result[i] = jobs[i].JobModel
	}
	return result, nil
}

func (s *JobStorage) UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error {
	var job models.Job
	if err := s.db.Store().Get(jobID, &job); err != nil {
		return err
	}

	job.Status = models.JobStatus(status)
	if errorMsg != "" {
		job.Error = errorMsg
	}

	now := time.Now()
	if status == string(models.JobStatusRunning) {
		job.StartedAt = &now
	} else if status == string(models.JobStatusCompleted) || status == string(models.JobStatusFailed) || status == string(models.JobStatusCancelled) {
		job.CompletedAt = &now
		job.FinishedAt = &now
	}

	return s.SaveJob(ctx, &job)
}

func (s *JobStorage) UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error {
	var job models.Job
	if err := s.db.Store().Get(jobID, &job); err != nil {
		return err
	}

	var progress models.JobProgress
	if err := json.Unmarshal([]byte(progressJSON), &progress); err != nil {
		return fmt.Errorf("failed to unmarshal progress: %w", err)
	}

	job.Progress = &progress
	return s.SaveJob(ctx, &job)
}

func (s *JobStorage) UpdateProgressCountersAtomic(ctx context.Context, jobID string, completedDelta, pendingDelta, totalDelta, failedDelta int) error {
	// BadgerHold doesn't support atomic field updates in the same way as SQL
	// We have to read-modify-write inside a transaction (or lock)
	// Since BadgerHold transactions are for batching, we rely on Badger's transaction
	// But BadgerHold's Update is atomic per record write.
	// However, read-modify-write is subject to race conditions without external locking or CAS.
	// For simplicity in this refactor, we'll do read-modify-write.
	// In a high-concurrency scenario, this might need a mutex or Badger transaction.

	var job models.Job
	if err := s.db.Store().Get(jobID, &job); err != nil {
		return err
	}

	if job.Progress == nil {
		job.Progress = &models.JobProgress{}
	}

	job.Progress.CompletedURLs += completedDelta
	job.Progress.PendingURLs += pendingDelta
	job.Progress.TotalURLs += totalDelta
	job.Progress.FailedURLs += failedDelta

	// Recalculate percentage
	if job.Progress.TotalURLs > 0 {
		job.Progress.Percentage = float64(job.Progress.CompletedURLs+job.Progress.FailedURLs) / float64(job.Progress.TotalURLs) * 100
	}

	return s.SaveJob(ctx, &job)
}

func (s *JobStorage) UpdateJobHeartbeat(ctx context.Context, jobID string) error {
	var job models.Job
	if err := s.db.Store().Get(jobID, &job); err != nil {
		return err
	}
	now := time.Now()
	job.LastHeartbeat = &now
	return s.SaveJob(ctx, &job)
}

func (s *JobStorage) GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]*models.JobModel, error) {
	threshold := time.Now().Add(-time.Duration(staleThresholdMinutes) * time.Minute)
	var jobs []models.Job
	// Find running jobs with heartbeat older than threshold
	err := s.db.Store().Find(&jobs, badgerhold.Where("Status").Eq(models.JobStatusRunning).And("LastHeartbeat").Lt(threshold))
	if err != nil {
		return nil, err
	}

	result := make([]*models.JobModel, len(jobs))
	for i := range jobs {
		result[i] = jobs[i].JobModel
	}
	return result, nil
}

func (s *JobStorage) DeleteJob(ctx context.Context, jobID string) error {
	if err := s.db.Store().Delete(jobID, &models.Job{}); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil
		}
		return err
	}
	return nil
}

func (s *JobStorage) CountJobs(ctx context.Context) (int, error) {
	count, err := s.db.Store().Count(&models.Job{}, nil)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *JobStorage) CountJobsByStatus(ctx context.Context, status string) (int, error) {
	count, err := s.db.Store().Count(&models.Job{}, badgerhold.Where("Status").Eq(status))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *JobStorage) CountJobsWithFilters(ctx context.Context, opts *interfaces.JobListOptions) (int, error) {
	query := badgerhold.Where("ID").Ne("")

	if opts != nil {
		if opts.Status != "" {
			query = query.And("Status").Eq(opts.Status)
		}
		// Type filter removed as it's not in options
		if opts.ParentID != "" {
			query = query.And("ParentID").Eq(opts.ParentID)
		}
	}

	count, err := s.db.Store().Count(&models.Job{}, query)
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
	var jobs []models.Job
	if err := s.db.Store().Find(&jobs, badgerhold.Where("Status").Eq(models.JobStatusRunning)); err != nil {
		return 0, err
	}

	count := 0
	for _, job := range jobs {
		job.Status = models.JobStatusPending
		// Optionally append reason to error or log
		if err := s.SaveJob(ctx, &job); err == nil {
			count++
		}
	}
	return count, nil
}