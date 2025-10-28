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

// ErrJobNotFound is returned when a job is not found in the database
var ErrJobNotFound = errors.New("job not found")

// unixToTime converts Unix timestamp to time.Time
func unixToTime(unix int64) time.Time {
	return time.Unix(unix, 0)
}

// splitAndTrim splits a string by delimiter and trims whitespace from each part
func splitAndTrim(s string, delimiter string) []string {
	parts := strings.Split(s, delimiter)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// JobStorage implements SQLite storage for crawler jobs
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

	crawlJob, ok := job.(*models.CrawlJob)
	if !ok {
		return fmt.Errorf("invalid job type: expected *models.CrawlJob")
	}

	// Serialize config and progress to JSON
	configJSON, err := crawlJob.Config.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	progressJSON, err := crawlJob.Progress.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize progress: %w", err)
	}

	// Convert timestamps to Unix (SQLite integer)
	createdAt := crawlJob.CreatedAt.Unix()
	var startedAt, completedAt sql.NullInt64

	if !crawlJob.StartedAt.IsZero() {
		startedAt.Valid = true
		startedAt.Int64 = crawlJob.StartedAt.Unix()
	}

	if !crawlJob.CompletedAt.IsZero() {
		completedAt.Valid = true
		completedAt.Int64 = crawlJob.CompletedAt.Unix()
	}

	// Convert RefreshSource bool to integer for SQLite
	refreshSource := 0
	if crawlJob.RefreshSource {
		refreshSource = 1
	}

	// Serialize seed URLs to JSON
	seedURLsJSON := "[]"
	if len(crawlJob.SeedURLs) > 0 {
		seedURLsBytes, err := json.Marshal(crawlJob.SeedURLs)
		if err != nil {
			return fmt.Errorf("failed to serialize seed URLs: %w", err)
		}
		seedURLsJSON = string(seedURLsBytes)
	}

	// Handle nullable snapshot fields
	var sourceConfigSnapshot, authSnapshot sql.NullString
	if crawlJob.SourceConfigSnapshot != "" {
		sourceConfigSnapshot.Valid = true
		sourceConfigSnapshot.String = crawlJob.SourceConfigSnapshot
	}
	if crawlJob.AuthSnapshot != "" {
		authSnapshot.Valid = true
		authSnapshot.String = crawlJob.AuthSnapshot
	}

	query := `
		INSERT INTO crawl_jobs (
			id, name, description, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
			status, progress_json, created_at, started_at, completed_at, error, result_count, failed_count, seed_urls
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			status = excluded.status,
			progress_json = excluded.progress_json,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			error = excluded.error,
			result_count = excluded.result_count,
			failed_count = excluded.failed_count,
			source_config_snapshot = excluded.source_config_snapshot,
			auth_snapshot = excluded.auth_snapshot,
			refresh_source = excluded.refresh_source,
			seed_urls = excluded.seed_urls
	`

	_, err = s.db.db.ExecContext(ctx, query,
		crawlJob.ID,
		crawlJob.Name,
		crawlJob.Description,
		crawlJob.SourceType,
		crawlJob.EntityType,
		configJSON,
		sourceConfigSnapshot,
		authSnapshot,
		refreshSource,
		string(crawlJob.Status),
		progressJSON,
		createdAt,
		startedAt,
		completedAt,
		crawlJob.Error,
		crawlJob.ResultCount,
		crawlJob.FailedCount,
		seedURLsJSON,
	)

	if err != nil {
		s.logger.Error().Err(err).Str("job_id", crawlJob.ID).Msg("Failed to save job")
		return fmt.Errorf("failed to save job: %w", err)
	}

	// Validate result count matches progress for completed jobs
	if crawlJob.Status == models.JobStatusCompleted || crawlJob.Status == models.JobStatusFailed || crawlJob.Status == models.JobStatusCancelled {
		expectedResultCount := crawlJob.Progress.CompletedURLs
		expectedFailedCount := crawlJob.Progress.FailedURLs

		if crawlJob.ResultCount != expectedResultCount {
			s.logger.Warn().
				Str("job_id", crawlJob.ID).
				Str("status", string(crawlJob.Status)).
				Int("stored_result_count", crawlJob.ResultCount).
				Int("progress_completed_urls", expectedResultCount).
				Int("mismatch", crawlJob.ResultCount-expectedResultCount).
				Msg("Result count mismatch detected")
		}

		if crawlJob.FailedCount != expectedFailedCount {
			s.logger.Warn().
				Str("job_id", crawlJob.ID).
				Str("status", string(crawlJob.Status)).
				Int("stored_failed_count", crawlJob.FailedCount).
				Int("progress_failed_urls", expectedFailedCount).
				Int("mismatch", crawlJob.FailedCount-expectedFailedCount).
				Msg("Failed count mismatch detected")
		}
	}

	s.logger.Debug().Str("job_id", crawlJob.ID).Str("status", string(crawlJob.Status)).Msg("Job saved")
	return nil
}

// GetJob retrieves a job by ID
func (s *JobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	query := `
		SELECT id, name, description, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
		       status, progress_json, created_at, started_at, completed_at, error, result_count, failed_count, seed_urls
		FROM crawl_jobs
		WHERE id = ?
	`

	row := s.db.db.QueryRowContext(ctx, query, jobID)
	return s.scanJob(row)
}

// ListJobs lists jobs with pagination and filters
func (s *JobStorage) ListJobs(ctx context.Context, opts *interfaces.ListOptions) ([]*models.CrawlJob, error) {
	query := `
		SELECT id, name, description, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
		       status, progress_json, created_at, started_at, completed_at, error, result_count, failed_count, seed_urls
		FROM crawl_jobs
		WHERE 1=1
	`

	args := []interface{}{}

	// Apply filters
	if opts != nil {
		if opts.SourceType != "" {
			query += " AND source_type = ?"
			args = append(args, opts.SourceType)
		}
		if opts.Status != "" {
			// Support comma-separated status values: "pending,running" -> IN clause
			statuses := []string{}
			for _, s := range splitAndTrim(opts.Status, ",") {
				if s != "" {
					statuses = append(statuses, s)
				}
			}

			if len(statuses) == 1 {
				// Single status: use equality
				query += " AND status = ?"
				args = append(args, statuses[0])
			} else if len(statuses) > 1 {
				// Multiple statuses: use IN clause
				placeholders := ""
				for i := range statuses {
					if i > 0 {
						placeholders += ", "
					}
					placeholders += "?"
					args = append(args, statuses[i])
				}
				query += fmt.Sprintf(" AND status IN (%s)", placeholders)
			}
		}
		if opts.EntityType != "" {
			query += " AND entity_type = ?"
			args = append(args, opts.EntityType)
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
		// Default ordering
		query += " ORDER BY created_at DESC LIMIT 50"
	}

	rows, err := s.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	return s.scanJobs(rows)
}

// GetJobsByStatus filters jobs by status
func (s *JobStorage) GetJobsByStatus(ctx context.Context, status string) ([]*models.CrawlJob, error) {
	query := `
		SELECT id, name, description, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
		       status, progress_json, created_at, started_at, completed_at, error, result_count, failed_count, seed_urls
		FROM crawl_jobs
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

// UpdateJobStatus updates job status and error message
func (s *JobStorage) UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Single query with conditional completed_at handling
	// Terminal statuses set completed_at to NOW, non-terminal statuses clear it to NULL
	query := `
		UPDATE crawl_jobs
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

// MarkRunningJobsAsPending marks all running jobs as pending (for graceful shutdown)
// Returns the count of jobs marked as pending
func (s *JobStorage) MarkRunningJobsAsPending(ctx context.Context, reason string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE crawl_jobs
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

// UpdateJobProgress updates job progress information
func (s *JobStorage) UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// We need to extract result_count and failed_count from the progress JSON for efficient querying
	// For now, we'll just update the progress_json field
	query := `
		UPDATE crawl_jobs
		SET progress_json = ?
		WHERE id = ?
	`

	_, err := s.db.db.ExecContext(ctx, query, progressJSON, jobID)
	return err
}

// UpdateJobHeartbeat updates the last_heartbeat timestamp for a job
func (s *JobStorage) UpdateJobHeartbeat(ctx context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE crawl_jobs
		SET last_heartbeat = strftime('%s', 'now')
		WHERE id = ?
	`

	_, err := s.db.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job heartbeat: %w", err)
	}
	return nil
}

// GetStaleJobs returns jobs with stale heartbeats (older than threshold)
func (s *JobStorage) GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]*models.CrawlJob, error) {
	query := `
		SELECT id, name, description, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
		       status, progress_json, created_at, started_at, completed_at, error, result_count, failed_count, seed_urls
		FROM crawl_jobs
		WHERE status = 'running'
		  AND COALESCE(last_heartbeat, started_at, created_at) < strftime('%s', 'now', '-' || ? || ' minutes')
		ORDER BY COALESCE(last_heartbeat, started_at, created_at) ASC
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

	query := "DELETE FROM crawl_jobs WHERE id = ?"
	_, err := s.db.db.ExecContext(ctx, query, jobID)
	return err
}

// CountJobs returns total job count
func (s *JobStorage) CountJobs(ctx context.Context) (int, error) {
	query := "SELECT COUNT(*) FROM crawl_jobs"
	var count int
	err := s.db.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// CountJobsByStatus counts jobs by status
func (s *JobStorage) CountJobsByStatus(ctx context.Context, status string) (int, error) {
	query := "SELECT COUNT(*) FROM crawl_jobs WHERE status = ?"
	var count int
	err := s.db.db.QueryRowContext(ctx, query, status).Scan(&count)
	return count, err
}

// CountJobsWithFilters returns count of jobs matching filter criteria
func (s *JobStorage) CountJobsWithFilters(ctx context.Context, opts *interfaces.ListOptions) (int, error) {
	query := "SELECT COUNT(*) FROM crawl_jobs WHERE 1=1"
	args := []interface{}{}

	// Apply same filters as ListJobs
	if opts != nil {
		if opts.SourceType != "" {
			query += " AND source_type = ?"
			args = append(args, opts.SourceType)
		}
		if opts.Status != "" {
			// Support comma-separated status values: "pending,running" -> IN clause
			statuses := []string{}
			for _, s := range splitAndTrim(opts.Status, ",") {
				if s != "" {
					statuses = append(statuses, s)
				}
			}

			if len(statuses) == 1 {
				// Single status: use equality
				query += " AND status = ?"
				args = append(args, statuses[0])
			} else if len(statuses) > 1 {
				// Multiple statuses: use IN clause
				placeholders := ""
				for i := range statuses {
					if i > 0 {
						placeholders += ", "
					}
					placeholders += "?"
					args = append(args, statuses[i])
				}
				query += fmt.Sprintf(" AND status IN (%s)", placeholders)
			}
		}
		if opts.EntityType != "" {
			query += " AND entity_type = ?"
			args = append(args, opts.EntityType)
		}
	}

	var count int
	err := s.db.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// scanJob scans a single row into CrawlJob
func (s *JobStorage) scanJob(row *sql.Row) (interface{}, error) {
	var (
		id, name, description, sourceType, entityType, configJSON, status, progressJSON string
		sourceConfigSnapshot, authSnapshot                                              sql.NullString
		refreshSource                                                                   int
		errorMsg                                                                        sql.NullString
		createdAt                                                                       int64
		startedAt, completedAt                                                          sql.NullInt64
		resultCount, failedCount                                                        int
		seedURLsJSON                                                                    sql.NullString
	)

	err := row.Scan(
		&id, &name, &description, &sourceType, &entityType, &configJSON, &sourceConfigSnapshot, &authSnapshot, &refreshSource,
		&status, &progressJSON, &createdAt, &startedAt, &completedAt, &errorMsg, &resultCount, &failedCount, &seedURLsJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}

	// Deserialize config and progress
	config, err := models.FromJSONCrawlConfig(configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize config: %w", err)
	}

	progress, err := models.FromJSONCrawlProgress(progressJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize progress: %w", err)
	}

	// Build CrawlJob
	job := &models.CrawlJob{
		ID:                   id,
		Name:                 name,
		Description:          description,
		SourceType:           sourceType,
		EntityType:           entityType,
		Config:               *config,
		SourceConfigSnapshot: "",
		AuthSnapshot:         "",
		RefreshSource:        refreshSource != 0,
		Status:               models.JobStatus(status),
		Progress:             *progress,
		ResultCount:          resultCount,
		FailedCount:          failedCount,
	}

	// Handle nullable snapshots
	if sourceConfigSnapshot.Valid {
		job.SourceConfigSnapshot = sourceConfigSnapshot.String
	}
	if authSnapshot.Valid {
		job.AuthSnapshot = authSnapshot.String
	}

	// Convert timestamps
	job.CreatedAt = unixToTime(createdAt)
	if startedAt.Valid {
		job.StartedAt = unixToTime(startedAt.Int64)
	}
	if completedAt.Valid {
		job.CompletedAt = unixToTime(completedAt.Int64)
	}
	if errorMsg.Valid {
		job.Error = errorMsg.String
	}

	// Deserialize seed URLs
	if seedURLsJSON.Valid && seedURLsJSON.String != "" {
		var seedURLs []string
		if err := json.Unmarshal([]byte(seedURLsJSON.String), &seedURLs); err == nil {
			job.SeedURLs = seedURLs
		} else {
			s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize seed URLs")
		}
	}

	return job, nil
}

// scanJobs scans multiple rows into slice of CrawlJob
func (s *JobStorage) scanJobs(rows *sql.Rows) ([]*models.CrawlJob, error) {
	var jobs []*models.CrawlJob

	for rows.Next() {
		var (
			id, name, description, sourceType, entityType, configJSON, status, progressJSON string
			sourceConfigSnapshot, authSnapshot                                              sql.NullString
			refreshSource                                                                   int
			errorMsg                                                                        sql.NullString
			createdAt                                                                       int64
			startedAt, completedAt                                                          sql.NullInt64
			resultCount, failedCount                                                        int
			seedURLsJSON                                                                    sql.NullString
		)

		err := rows.Scan(
			&id, &name, &description, &sourceType, &entityType, &configJSON, &sourceConfigSnapshot, &authSnapshot, &refreshSource,
			&status, &progressJSON, &createdAt, &startedAt, &completedAt, &errorMsg, &resultCount, &failedCount, &seedURLsJSON,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		// Deserialize config and progress
		config, err := models.FromJSONCrawlConfig(configJSON)
		if err != nil {
			s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize config, skipping job")
			continue
		}

		progress, err := models.FromJSONCrawlProgress(progressJSON)
		if err != nil {
			s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize progress, skipping job")
			continue
		}

		// Build CrawlJob
		job := &models.CrawlJob{
			ID:                   id,
			Name:                 name,
			Description:          description,
			SourceType:           sourceType,
			EntityType:           entityType,
			Config:               *config,
			SourceConfigSnapshot: "",
			AuthSnapshot:         "",
			RefreshSource:        refreshSource != 0,
			Status:               models.JobStatus(status),
			Progress:             *progress,
			ResultCount:          resultCount,
			FailedCount:          failedCount,
		}

		// Handle nullable snapshots
		if sourceConfigSnapshot.Valid {
			job.SourceConfigSnapshot = sourceConfigSnapshot.String
		}
		if authSnapshot.Valid {
			job.AuthSnapshot = authSnapshot.String
		}

		// Convert timestamps
		job.CreatedAt = unixToTime(createdAt)
		if startedAt.Valid {
			job.StartedAt = unixToTime(startedAt.Int64)
		}
		if completedAt.Valid {
			job.CompletedAt = unixToTime(completedAt.Int64)
		}
		if errorMsg.Valid {
			job.Error = errorMsg.String
		}

		// Deserialize seed URLs
		if seedURLsJSON.Valid && seedURLsJSON.String != "" {
			var seedURLs []string
			if err := json.Unmarshal([]byte(seedURLsJSON.String), &seedURLs); err == nil {
				job.SeedURLs = seedURLs
			} else {
				s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize seed URLs")
			}
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// UpdateJob updates a job's metadata fields like name and description
func (s *JobStorage) UpdateJob(ctx context.Context, job interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	crawlJob, ok := job.(*models.CrawlJob)
	if !ok {
		return fmt.Errorf("invalid job type: expected *models.CrawlJob")
	}

	query := `
		UPDATE crawl_jobs
		SET name = ?, description = ?
		WHERE id = ?
	`

	_, err := s.db.db.ExecContext(ctx, query, crawlJob.Name, crawlJob.Description, crawlJob.ID)
	if err != nil {
		s.logger.Error().Err(err).Str("job_id", crawlJob.ID).Msg("Failed to update job")
		return fmt.Errorf("failed to update job: %w", err)
	}

	s.logger.Debug().Str("job_id", crawlJob.ID).Str("name", crawlJob.Name).Str("description", crawlJob.Description).Msg("Job updated")
	return nil
}

// AppendJobLog appends a single log entry to the job's logs array with automatic truncation.
//
// Truncation Mechanism:
//   - Automatically limits job logs to the most recent 100 entries
//   - When log count exceeds 100, the oldest entries are removed: logs = logs[len(logs)-100:]
//   - This prevents unbounded growth of the logs JSON column in the database
//   - Truncation is transparent and atomic (no manual cleanup required)
//
// Thread Safety:
//   - Uses s.mu.Lock() to ensure thread-safe read-modify-write operations
//   - Multiple concurrent AppendJobLog calls for the same job are serialized
//   - Lock is held for the entire operation (query, deserialize, append, serialize, update)
//   - Prevents race conditions when multiple workers log simultaneously
//
// Error Handling:
//   - Returns ErrJobNotFound if the job does not exist in the database
//   - Logs a warning and starts with an empty array if deserialization fails (resilient to corrupted logs)
//   - Returns an error if serialization or database update fails (critical failures)
//   - Non-critical errors (e.g., deserialization) are logged but do not block appending new logs
//
// Performance Considerations:
//   - Each call performs a full read-modify-write cycle (not optimized for high-frequency logging)
//   - For high-volume logging, consider batching log entries before calling this method
//   - The 100-entry limit keeps JSON payload size manageable (~10-20KB typical)
//
// Usage Example:
//
//	logEntry := models.JobLogEntry{
//	    Timestamp: time.Now().Format("15:04:05"),
//	    Level:     "info",
//	    Message:   "Job started: source=jira/issues, seeds=5",
//	}
//	if err := jobStorage.AppendJobLog(ctx, jobID, logEntry); err != nil {
//	    logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to append log")
//	    // Non-fatal: log the error but continue job execution
//	}
//
// Deprecated: Use LogService.AppendLog() instead.
// This method writes to the crawl_jobs.logs JSON column which has a 100-entry limit.
// The new LogService writes to the dedicated job_logs table with unlimited history.
// This method is kept for backward compatibility during migration.
func (s *JobStorage) AppendJobLog(ctx context.Context, jobID string, logEntry models.JobLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Warn().
		Str("job_id", jobID).
		Msg("AppendJobLog is deprecated - use LogService.AppendLog() instead")

	// Query current logs
	query := "SELECT logs FROM crawl_jobs WHERE id = ?"
	var logsJSON sql.NullString
	err := s.db.db.QueryRowContext(ctx, query, jobID).Scan(&logsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrJobNotFound
		}
		s.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to query job logs")
		return fmt.Errorf("failed to query job logs: %w", err)
	}

	// Deserialize existing logs
	var logs []models.JobLogEntry
	if logsJSON.Valid && logsJSON.String != "" && logsJSON.String != "[]" {
		if err := json.Unmarshal([]byte(logsJSON.String), &logs); err != nil {
			s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to deserialize logs, starting with empty array")
			logs = []models.JobLogEntry{}
		}
	}

	// Validate log level (Comment 6: validate level, default to info if invalid)
	validLevels := map[string]bool{
		"info":  true,
		"warn":  true,
		"error": true,
		"debug": true,
	}
	if !validLevels[logEntry.Level] {
		s.logger.Warn().
			Str("job_id", jobID).
			Str("invalid_level", logEntry.Level).
			Msg("Invalid log level, defaulting to info")
		logEntry.Level = "info"
	}

	// Validate message is not empty (Comment 1: guard against empty messages)
	if logEntry.Message == "" {
		s.logger.Warn().
			Str("job_id", jobID).
			Msg("Log message is empty, cannot append")
		return fmt.Errorf("log message cannot be empty")
	}

	// Append new log entry
	logs = append(logs, logEntry)

	// Limit to last 100 entries to prevent unbounded growth
	// Comment 6: warn on truncation for visibility
	truncated := false
	entriesTruncated := 0
	if len(logs) > 100 {
		entriesTruncated = len(logs) - 100
		logs = logs[len(logs)-100:]
		truncated = true
	}

	// Serialize back to JSON
	logsBytes, err := json.Marshal(logs)
	if err != nil {
		s.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to serialize logs")
		return fmt.Errorf("failed to serialize logs: %w", err)
	}

	// Update database
	updateQuery := "UPDATE crawl_jobs SET logs = ? WHERE id = ?"
	_, err = s.db.db.ExecContext(ctx, updateQuery, string(logsBytes), jobID)
	if err != nil {
		s.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job logs")
		return fmt.Errorf("failed to update job logs: %w", err)
	}

	// Log truncation warning if entries were removed (Comment 6)
	if truncated {
		s.logger.Warn().
			Str("job_id", jobID).
			Int("entries_truncated", entriesTruncated).
			Int("kept_entries", len(logs)).
			Msg("Job logs truncated to 100 most recent entries")
	}

	s.logger.Debug().Str("job_id", jobID).Int("log_count", len(logs)).Msg("Job log appended")
	return nil
}

// GetJobLogs retrieves all log entries for a job
//
// Deprecated: Use LogService.GetLogs() instead.
// This method reads from the crawl_jobs.logs JSON column (limited to 100 entries).
// The new LogService reads from the dedicated job_logs table with full history.
// This method is kept for backward compatibility during migration.
func (s *JobStorage) GetJobLogs(ctx context.Context, jobID string) ([]models.JobLogEntry, error) {
	s.logger.Warn().
		Str("job_id", jobID).
		Msg("GetJobLogs is deprecated - use LogService.GetLogs() instead")

	query := "SELECT logs FROM crawl_jobs WHERE id = ?"
	var logsJSON sql.NullString
	err := s.db.db.QueryRowContext(ctx, query, jobID).Scan(&logsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to query job logs: %w", err)
	}

	// Handle NULL/empty cases by returning empty array
	if !logsJSON.Valid || logsJSON.String == "" || logsJSON.String == "[]" {
		return []models.JobLogEntry{}, nil
	}

	// Deserialize JSON array
	var logs []models.JobLogEntry
	if err := json.Unmarshal([]byte(logsJSON.String), &logs); err != nil {
		s.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to deserialize logs")
		return nil, fmt.Errorf("failed to deserialize logs: %w", err)
	}

	return logs, nil
}

// MarkURLSeen atomically records a URL as seen for a job and returns whether it was newly added.
// VERIFICATION COMMENT 1: Concurrency-safe URL deduplication
//
// Thread Safety:
//   - Uses INSERT OR IGNORE for atomic duplicate detection at database level
//   - No locks required - SQLite handles atomicity via UNIQUE constraint
//   - Safe for concurrent workers processing the same job
//
// Returns:
//   - (true, nil) if URL was newly added (first worker to enqueue this URL wins)
//   - (false, nil) if URL was already seen (duplicate detected, skip enqueueing)
//   - (false, error) if database operation fails
//
// Performance:
//   - Single INSERT operation - no SELECT needed
//   - Indexed on (job_id, url) PRIMARY KEY for fast lookups
//   - Rows deleted CASCADE when job is deleted (no orphaned entries)
func (s *JobStorage) MarkURLSeen(ctx context.Context, jobID string, url string) (bool, error) {
	query := `
		INSERT OR IGNORE INTO job_seen_urls (job_id, url, created_at)
		VALUES (?, ?, ?)
	`

	result, err := s.db.db.ExecContext(ctx, query, jobID, url, time.Now().Unix())
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("job_id", jobID).
			Str("url", url).
			Msg("Failed to mark URL as seen")
		return false, fmt.Errorf("failed to mark URL as seen: %w", err)
	}

	// Check rows affected: 1 = newly added, 0 = already exists
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("job_id", jobID).
			Str("url", url).
			Msg("Failed to get rows affected")
		// Return true conservatively to avoid skipping URLs on error
		return true, nil
	}

	newlyAdded := rowsAffected > 0

	if newlyAdded {
		s.logger.Debug().
			Str("job_id", jobID).
			Str("url", url).
			Msg("URL marked as seen (newly added)")
	} else {
		s.logger.Debug().
			Str("job_id", jobID).
			Str("url", url).
			Msg("URL already seen (duplicate detected)")
	}

	return newlyAdded, nil
}
