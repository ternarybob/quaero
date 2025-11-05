package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

var (
	// ErrJobNotFound is returned when a job is not found
	ErrJobNotFound = errors.New("job not found")
)

// JobStorage implements interfaces.JobStorage for the new Job model architecture
type JobStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
	mu     sync.Mutex
}

// NewJobStorage creates a new job storage instance
func NewJobStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.JobStorage {
	return &JobStorage{
		db:     db,
		logger: logger,
	}
}

// SaveJob creates or updates a job in the database
func (s *JobStorage) SaveJob(ctx context.Context, job interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cast to *models.Job
	jobModel, ok := job.(*models.Job)
	if !ok {
		return fmt.Errorf("invalid job type: expected *models.Job, got %T", job)
	}

	// Serialize config and metadata to JSON
	configJSON, err := json.Marshal(jobModel.Config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	metadataJSON := "{}"
	if jobModel.Metadata != nil && len(jobModel.Metadata) > 0 {
		metadataBytes, err := json.Marshal(jobModel.Metadata)
		if err != nil {
			return fmt.Errorf("failed to serialize metadata: %w", err)
		}
		metadataJSON = string(metadataBytes)
	}

	// Serialize progress to JSON
	progressJSON := "{}"
	if jobModel.Progress != nil {
		progressBytes, err := json.Marshal(jobModel.Progress)
		if err != nil {
			return fmt.Errorf("failed to serialize progress: %w", err)
		}
		progressJSON = string(progressBytes)
	}

	// Convert timestamps to Unix (SQLite integer)
	createdAt := jobModel.CreatedAt.Unix()
	var startedAt, completedAt, finishedAt, lastHeartbeat sql.NullInt64

	if jobModel.StartedAt != nil {
		startedAt.Valid = true
		startedAt.Int64 = jobModel.StartedAt.Unix()
	}

	if jobModel.CompletedAt != nil {
		completedAt.Valid = true
		completedAt.Int64 = jobModel.CompletedAt.Unix()
	}

	if jobModel.FinishedAt != nil {
		finishedAt.Valid = true
		finishedAt.Int64 = jobModel.FinishedAt.Unix()
	}

	if jobModel.LastHeartbeat != nil {
		lastHeartbeat.Valid = true
		lastHeartbeat.Int64 = jobModel.LastHeartbeat.Unix()
	}

	// Handle nullable parent_id
	var parentID sql.NullString
	if jobModel.ParentID != nil && *jobModel.ParentID != "" {
		parentID.Valid = true
		parentID.String = *jobModel.ParentID
	}

	query := `
		INSERT INTO jobs (
			id, parent_id, job_type, name, description, config_json, metadata_json,
			status, progress_json, created_at, started_at, completed_at, finished_at,
			last_heartbeat, error, result_count, failed_count, depth
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			parent_id = excluded.parent_id,
			job_type = excluded.job_type,
			name = excluded.name,
			description = excluded.description,
			config_json = excluded.config_json,
			metadata_json = excluded.metadata_json,
			status = excluded.status,
			progress_json = excluded.progress_json,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			finished_at = excluded.finished_at,
			last_heartbeat = excluded.last_heartbeat,
			error = excluded.error,
			result_count = excluded.result_count,
			failed_count = excluded.failed_count,
			depth = excluded.depth
	`

	_, err = s.db.db.ExecContext(ctx, query,
		jobModel.ID,
		parentID,
		jobModel.Type,
		jobModel.Name,
		"", // description - empty for now
		string(configJSON),
		metadataJSON,
		string(jobModel.Status),
		progressJSON,
		createdAt,
		startedAt,
		completedAt,
		finishedAt,
		lastHeartbeat,
		jobModel.Error,
		jobModel.ResultCount,
		jobModel.FailedCount,
		jobModel.Depth,
	)

	if err != nil {
		s.logger.Error().Err(err).Str("job_id", jobModel.ID).Msg("Failed to save job")
		return fmt.Errorf("failed to save job: %w", err)
	}

	s.logger.Debug().Str("job_id", jobModel.ID).Str("status", string(jobModel.Status)).Msg("Job saved")
	return nil
}

// GetJob retrieves a job by ID
func (s *JobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	query := `
		SELECT id, parent_id, job_type, name, description, config_json, metadata_json,
		       status, progress_json, created_at, started_at, completed_at, finished_at,
		       last_heartbeat, error, result_count, failed_count, depth
		FROM jobs
		WHERE id = ?
	`

	row := s.db.db.QueryRowContext(ctx, query, jobID)
	return s.scanJob(row)
}

// scanJob scans a single job row
func (s *JobStorage) scanJob(row *sql.Row) (*models.Job, error) {
	var (
		id, jobType, name, description, configJSON, metadataJSON, status, progressJSON, errorMsg string
		parentID                                                                                 sql.NullString
		createdAt                                                                                int64
		startedAt, completedAt, finishedAt, lastHeartbeat                                        sql.NullInt64
		resultCount, failedCount, depth                                                          int
	)

	err := row.Scan(
		&id, &parentID, &jobType, &name, &description, &configJSON, &metadataJSON,
		&status, &progressJSON, &createdAt, &startedAt, &completedAt, &finishedAt,
		&lastHeartbeat, &errorMsg, &resultCount, &failedCount, &depth,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}

	// Deserialize config
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to deserialize config: %w", err)
	}

	// Deserialize metadata
	var metadata map[string]interface{}
	if metadataJSON != "" && metadataJSON != "{}" {
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return nil, fmt.Errorf("failed to deserialize metadata: %w", err)
		}
	}
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Deserialize progress
	var progress *models.JobProgress
	if progressJSON != "" && progressJSON != "{}" {
		progress = &models.JobProgress{}
		if err := json.Unmarshal([]byte(progressJSON), progress); err != nil {
			return nil, fmt.Errorf("failed to deserialize progress: %w", err)
		}
	}

	// Build JobModel
	var parentIDPtr *string
	if parentID.Valid && parentID.String != "" {
		parentIDPtr = &parentID.String
	}

	jobModel := &models.JobModel{
		ID:        id,
		ParentID:  parentIDPtr,
		Type:      jobType,
		Name:      name,
		Config:    config,
		Metadata:  metadata,
		CreatedAt: time.Unix(createdAt, 0),
		Depth:     depth,
	}

	// Build Job with runtime state
	job := &models.Job{
		JobModel:    jobModel,
		Status:      models.JobStatus(status),
		Progress:    progress,
		Error:       errorMsg,
		ResultCount: resultCount,
		FailedCount: failedCount,
	}

	// Convert timestamps
	if startedAt.Valid {
		t := time.Unix(startedAt.Int64, 0)
		job.StartedAt = &t
	}
	if completedAt.Valid {
		t := time.Unix(completedAt.Int64, 0)
		job.CompletedAt = &t
	}
	if finishedAt.Valid {
		t := time.Unix(finishedAt.Int64, 0)
		job.FinishedAt = &t
	}
	if lastHeartbeat.Valid {
		t := time.Unix(lastHeartbeat.Int64, 0)
		job.LastHeartbeat = &t
	}

	return job, nil
}

// UpdateJob updates a job's metadata fields like name and description
func (s *JobStorage) UpdateJob(ctx context.Context, job interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobModel, ok := job.(*models.Job)
	if !ok {
		return fmt.Errorf("invalid job type: expected *models.Job")
	}

	query := `
		UPDATE jobs
		SET name = ?, description = ?
		WHERE id = ?
	`

	_, err := s.db.db.ExecContext(ctx, query, jobModel.Name, "", jobModel.ID)
	if err != nil {
		s.logger.Error().Err(err).Str("job_id", jobModel.ID).Msg("Failed to update job")
		return fmt.Errorf("failed to update job: %w", err)
	}

	s.logger.Debug().Str("job_id", jobModel.ID).Str("name", jobModel.Name).Msg("Job updated")
	return nil
}

// ListJobs lists jobs with pagination and filters
func (s *JobStorage) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.JobModel, error) {
	query := `
		SELECT id, parent_id, job_type, name, description, config_json, metadata_json,
		       status, progress_json, created_at, started_at, completed_at, finished_at,
		       last_heartbeat, error, result_count, failed_count, depth
		FROM jobs
		WHERE 1=1
	`

	args := []interface{}{}

	// Apply filters
	if opts != nil {
		if opts.Status != "" {
			// Support comma-separated status values
			statuses := strings.Split(opts.Status, ",")
			if len(statuses) == 1 {
				query += " AND status = ?"
				args = append(args, strings.TrimSpace(statuses[0]))
			} else {
				placeholders := strings.Repeat("?,", len(statuses))
				placeholders = placeholders[:len(placeholders)-1]
				query += fmt.Sprintf(" AND status IN (%s)", placeholders)
				for _, s := range statuses {
					args = append(args, strings.TrimSpace(s))
				}
			}
		}

		if opts.ParentID != "" {
			if opts.ParentID == "root" {
				query += " AND (parent_id IS NULL OR parent_id = '')"
			} else {
				query += " AND parent_id = ?"
				args = append(args, opts.ParentID)
			}
		}

		// Order by
		orderBy := "created_at"
		if opts.OrderBy != "" {
			orderBy = opts.OrderBy
		}
		orderDir := "DESC"
		if opts.OrderDir != "" {
			orderDir = opts.OrderDir
		}
		query += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir)

		// Pagination
		if opts.Limit > 0 {
			query += " LIMIT ?"
			args = append(args, opts.Limit)
			if opts.Offset > 0 {
				query += " OFFSET ?"
				args = append(args, opts.Offset)
			}
		}
	} else {
		query += " ORDER BY created_at DESC"
	}

	rows, err := s.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	return s.scanJobs(rows)
}

// scanJobs scans multiple job rows
func (s *JobStorage) scanJobs(rows *sql.Rows) ([]*models.JobModel, error) {
	jobs := []*models.JobModel{}

	for rows.Next() {
		var (
			id, jobType, name, description, configJSON, metadataJSON, status, progressJSON, errorMsg string
			parentID                                                                                 sql.NullString
			createdAt                                                                                int64
			startedAt, completedAt, finishedAt, lastHeartbeat                                        sql.NullInt64
			resultCount, failedCount, depth                                                          int
		)

		err := rows.Scan(
			&id, &parentID, &jobType, &name, &description, &configJSON, &metadataJSON,
			&status, &progressJSON, &createdAt, &startedAt, &completedAt, &finishedAt,
			&lastHeartbeat, &errorMsg, &resultCount, &failedCount, &depth,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}

		// Deserialize config
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize config, using empty map")
			config = make(map[string]interface{})
		}

		// Deserialize metadata
		var metadata map[string]interface{}
		if metadataJSON != "" && metadataJSON != "{}" {
			if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
				s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize metadata, using empty map")
				metadata = make(map[string]interface{})
			}
		}
		if metadata == nil {
			metadata = make(map[string]interface{})
		}

		// Build JobModel
		var parentIDPtr *string
		if parentID.Valid && parentID.String != "" {
			parentIDPtr = &parentID.String
		}

		jobModel := &models.JobModel{
			ID:        id,
			ParentID:  parentIDPtr,
			Type:      jobType,
			Name:      name,
			Config:    config,
			Metadata:  metadata,
			CreatedAt: time.Unix(createdAt, 0),
			Depth:     depth,
		}

		jobs = append(jobs, jobModel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating job rows: %w", err)
	}

	return jobs, nil
}

// GetChildJobs retrieves all child jobs for a given parent job ID
func (s *JobStorage) GetChildJobs(ctx context.Context, parentID string) ([]*models.JobModel, error) {
	query := `
		SELECT id, parent_id, job_type, name, description, config_json, metadata_json,
		       status, progress_json, created_at, started_at, completed_at, finished_at,
		       last_heartbeat, error, result_count, failed_count, depth
		FROM jobs
		WHERE parent_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child jobs: %w", err)
	}
	defer rows.Close()

	jobs, err := s.scanJobs(rows)
	if err != nil {
		return nil, err
	}

	s.logger.Debug().Str("parent_id", parentID).Int("child_count", len(jobs)).Msg("Retrieved child jobs")
	return jobs, nil
}

// GetJobsByStatus filters jobs by status
func (s *JobStorage) GetJobsByStatus(ctx context.Context, status string) ([]*models.JobModel, error) {
	query := `
		SELECT id, parent_id, job_type, name, description, config_json, metadata_json,
		       status, progress_json, created_at, started_at, completed_at, finished_at,
		       last_heartbeat, error, result_count, failed_count, depth
		FROM jobs
		WHERE status = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs by status: %w", err)
	}
	defer rows.Close()

	return s.scanJobs(rows)
}

// GetJobChildStats retrieves aggregate statistics for parent jobs' children
func (s *JobStorage) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	if len(parentIDs) == 0 {
		return make(map[string]*interfaces.JobChildStats), nil
	}

	placeholders := strings.Repeat("?,", len(parentIDs))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
		SELECT
			parent_id,
			COUNT(*) as child_count,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_children,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_children
		FROM jobs
		WHERE parent_id IN (%s)
		GROUP BY parent_id
	`, placeholders)

	args := make([]interface{}, len(parentIDs))
	for i, id := range parentIDs {
		args[i] = id
	}

	rows, err := s.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get job child stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]*interfaces.JobChildStats)
	for rows.Next() {
		var parentID string
		var childCount, completedChildren, failedChildren int

		if err := rows.Scan(&parentID, &childCount, &completedChildren, &failedChildren); err != nil {
			return nil, fmt.Errorf("failed to scan child stats: %w", err)
		}

		stats[parentID] = &interfaces.JobChildStats{
			ChildCount:        childCount,
			CompletedChildren: completedChildren,
			FailedChildren:    failedChildren,
		}
	}

	return stats, nil
}

// UpdateJobStatus updates job status and error message
func (s *JobStorage) UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE jobs
		SET status = ?,
		    error = ?,
		    completed_at = CASE
		        WHEN ? IN ('completed', 'failed', 'cancelled')
		        THEN strftime('%s', 'now')
		        ELSE NULL
		    END
		WHERE id = ?
	`

	_, err := s.db.db.ExecContext(ctx, query, status, errorMsg, status, jobID)
	return err
}

// UpdateJobProgress updates job progress information
func (s *JobStorage) UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE jobs
		SET progress_json = ?
		WHERE id = ?
	`

	_, err := s.db.db.ExecContext(ctx, query, progressJSON, jobID)
	return err
}

// UpdateProgressCountersAtomic atomically updates progress counters
func (s *JobStorage) UpdateProgressCountersAtomic(ctx context.Context, jobID string, completedDelta, pendingDelta, totalDelta, failedDelta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current progress
	var progressJSON string
	err := s.db.db.QueryRowContext(ctx, "SELECT progress_json FROM jobs WHERE id = ?", jobID).Scan(&progressJSON)
	if err != nil {
		return fmt.Errorf("failed to get current progress: %w", err)
	}

	// Deserialize current progress
	var progress models.JobProgress
	if progressJSON != "" && progressJSON != "{}" {
		if err := json.Unmarshal([]byte(progressJSON), &progress); err != nil {
			return fmt.Errorf("failed to deserialize progress: %w", err)
		}
	}

	// Update counters
	progress.CompletedURLs += completedDelta
	progress.FailedURLs += failedDelta
	progress.PendingURLs += pendingDelta
	progress.TotalURLs += totalDelta

	// Calculate percentage
	if progress.TotalURLs > 0 {
		progress.Percentage = float64(progress.CompletedURLs+progress.FailedURLs) / float64(progress.TotalURLs) * 100
	}

	// Serialize updated progress
	updatedProgressJSON, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("failed to serialize updated progress: %w", err)
	}

	// Update database
	query := `
		UPDATE jobs
		SET progress_json = ?,
		    result_count = ?,
		    failed_count = ?
		WHERE id = ?
	`

	_, err = s.db.db.ExecContext(ctx, query, string(updatedProgressJSON), progress.CompletedURLs, progress.FailedURLs, jobID)
	return err
}

// UpdateJobHeartbeat updates the last heartbeat timestamp
func (s *JobStorage) UpdateJobHeartbeat(ctx context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE jobs
		SET last_heartbeat = strftime('%s', 'now')
		WHERE id = ?
	`

	_, err := s.db.db.ExecContext(ctx, query, jobID)
	return err
}

// GetStaleJobs retrieves jobs that haven't had a heartbeat in the specified threshold
func (s *JobStorage) GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]*models.JobModel, error) {
	query := `
		SELECT id, parent_id, job_type, name, description, config_json, metadata_json,
		       status, progress_json, created_at, started_at, completed_at, finished_at,
		       last_heartbeat, error, result_count, failed_count, depth
		FROM jobs
		WHERE status = 'running'
		  AND last_heartbeat IS NOT NULL
		  AND last_heartbeat < strftime('%s', 'now', '-' || ? || ' minutes')
		ORDER BY last_heartbeat ASC
	`

	rows, err := s.db.db.QueryContext(ctx, query, staleThresholdMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get stale jobs: %w", err)
	}
	defer rows.Close()

	return s.scanJobs(rows)
}

// DeleteJob deletes a job by ID
func (s *JobStorage) DeleteJob(ctx context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if job exists
	var count int
	err := s.db.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE id = ?", jobID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check job existence: %w", err)
	}

	if count == 0 {
		s.logger.Debug().Str("job_id", jobID).Msg("Job not found for deletion")
		return nil
	}

	// Delete job (CASCADE will delete children)
	_, err = s.db.db.ExecContext(ctx, "DELETE FROM jobs WHERE id = ?", jobID)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	s.logger.Debug().Str("job_id", jobID).Msg("Job deleted")
	return nil
}

// CountJobs counts total jobs
func (s *JobStorage) CountJobs(ctx context.Context) (int, error) {
	var count int
	err := s.db.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs").Scan(&count)
	return count, err
}

// CountJobsByStatus counts jobs by status
func (s *JobStorage) CountJobsByStatus(ctx context.Context, status string) (int, error) {
	var count int
	err := s.db.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE status = ?", status).Scan(&count)
	return count, err
}

// CountJobsWithFilters counts jobs with filters
func (s *JobStorage) CountJobsWithFilters(ctx context.Context, opts *interfaces.JobListOptions) (int, error) {
	query := "SELECT COUNT(*) FROM jobs WHERE 1=1"
	args := []interface{}{}

	if opts != nil {
		if opts.Status != "" {
			statuses := strings.Split(opts.Status, ",")
			if len(statuses) == 1 {
				query += " AND status = ?"
				args = append(args, strings.TrimSpace(statuses[0]))
			} else {
				placeholders := strings.Repeat("?,", len(statuses))
				placeholders = placeholders[:len(placeholders)-1]
				query += fmt.Sprintf(" AND status IN (%s)", placeholders)
				for _, s := range statuses {
					args = append(args, strings.TrimSpace(s))
				}
			}
		}

		if opts.ParentID != "" {
			if opts.ParentID == "root" {
				query += " AND (parent_id IS NULL OR parent_id = '')"
			} else {
				query += " AND parent_id = ?"
				args = append(args, opts.ParentID)
			}
		}
	}

	var count int
	err := s.db.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// AppendJobLog is deprecated - use LogService.AppendLog() instead
func (s *JobStorage) AppendJobLog(ctx context.Context, jobID string, logEntry models.JobLogEntry) error {
	s.logger.Warn().Msg("AppendJobLog is deprecated - use LogService.AppendLog() instead")
	return nil
}

// GetJobLogs is deprecated - use LogService.GetLogs() instead
func (s *JobStorage) GetJobLogs(ctx context.Context, jobID string) ([]models.JobLogEntry, error) {
	s.logger.Warn().Msg("GetJobLogs is deprecated - use LogService.GetLogs() instead")
	return []models.JobLogEntry{}, nil
}

// MarkRunningJobsAsPending marks all running jobs as pending (for graceful shutdown)
func (s *JobStorage) MarkRunningJobsAsPending(ctx context.Context, reason string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE jobs
		SET status = 'pending',
		    error = ?,
		    completed_at = NULL
		WHERE status = 'running'
	`

	result, err := s.db.db.ExecContext(ctx, query, reason)
	if err != nil {
		return 0, fmt.Errorf("failed to mark running jobs as pending: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if count > 0 {
		s.logger.Info().Int64("count", count).Str("reason", reason).Msg("Marked running jobs as pending")
	}

	return int(count), nil
}

// MarkURLSeen atomically records a URL as seen for a job
func (s *JobStorage) MarkURLSeen(ctx context.Context, jobID string, url string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if URL already exists
	var count int
	err := s.db.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM job_seen_urls WHERE job_id = ? AND url = ?",
		jobID, url).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check URL existence: %w", err)
	}

	if count > 0 {
		return false, nil // URL already seen
	}

	// Insert new URL
	_, err = s.db.db.ExecContext(ctx,
		"INSERT INTO job_seen_urls (job_id, url, seen_at) VALUES (?, ?, strftime('%s', 'now'))",
		jobID, url)
	if err != nil {
		return false, fmt.Errorf("failed to mark URL as seen: %w", err)
	}

	return true, nil // URL newly added
}

// IsURLSeen checks if a URL has been seen for a job
func (s *JobStorage) IsURLSeen(ctx context.Context, jobID string, url string) (bool, error) {
	var count int
	err := s.db.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM job_seen_urls WHERE job_id = ? AND url = ?",
		jobID, url).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check URL: %w", err)
	}

	return count > 0, nil
}

// GetSeenURLs retrieves all seen URLs for a job
func (s *JobStorage) GetSeenURLs(ctx context.Context, jobID string) ([]string, error) {
	rows, err := s.db.db.QueryContext(ctx,
		"SELECT url FROM job_seen_urls WHERE job_id = ? ORDER BY seen_at ASC",
		jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seen URLs: %w", err)
	}
	defer rows.Close()

	urls := []string{}
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("failed to scan URL: %w", err)
		}
		urls = append(urls, url)
	}

	return urls, nil
}
