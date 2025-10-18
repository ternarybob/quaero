package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

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

	crawlJob, ok := job.(*crawler.CrawlJob)
	if !ok {
		return fmt.Errorf("invalid job type: expected *crawler.CrawlJob")
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
			id, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
			status, progress_json, created_at, started_at, completed_at, error, result_count, failed_count, seed_urls
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
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

	s.logger.Debug().Str("job_id", crawlJob.ID).Str("status", string(crawlJob.Status)).Msg("Job saved")
	return nil
}

// GetJob retrieves a job by ID
func (s *JobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	query := `
		SELECT id, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
		       status, progress_json, created_at, started_at, completed_at, error, result_count, failed_count, seed_urls
		FROM crawl_jobs
		WHERE id = ?
	`

	row := s.db.db.QueryRowContext(ctx, query, jobID)
	return s.scanJob(row)
}

// ListJobs lists jobs with pagination and filters
func (s *JobStorage) ListJobs(ctx context.Context, opts *interfaces.ListOptions) ([]interface{}, error) {
	query := `
		SELECT id, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
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
func (s *JobStorage) GetJobsByStatus(ctx context.Context, status string) ([]interface{}, error) {
	query := `
		SELECT id, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source,
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

	// Determine completed_at based on status
	query := `
		UPDATE crawl_jobs
		SET status = ?, error = ?, completed_at = ?
		WHERE id = ?
	`

	var completedAt sql.NullInt64
	if status == string(crawler.JobStatusCompleted) || status == string(crawler.JobStatusFailed) || status == string(crawler.JobStatusCancelled) {
		completedAt.Valid = true
		completedAt.Int64 = sql.NullInt64{}.Int64 // Current time will be set by trigger or we set it here
		// For simplicity, we'll use a separate query to set completed_at to current time
		query = `
			UPDATE crawl_jobs
			SET status = ?, error = ?, completed_at = strftime('%s', 'now')
			WHERE id = ?
		`
		_, err := s.db.db.ExecContext(ctx, query, status, errorMsg, jobID)
		return err
	}

	_, err := s.db.db.ExecContext(ctx, query, status, errorMsg, completedAt, jobID)
	return err
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
		id, sourceType, entityType, configJSON, status, progressJSON string
		sourceConfigSnapshot, authSnapshot                           sql.NullString
		refreshSource                                                int
		errorMsg                                                     sql.NullString
		createdAt                                                    int64
		startedAt, completedAt                                       sql.NullInt64
		resultCount, failedCount                                     int
		seedURLsJSON                                                 sql.NullString
	)

	err := row.Scan(
		&id, &sourceType, &entityType, &configJSON, &sourceConfigSnapshot, &authSnapshot, &refreshSource,
		&status, &progressJSON, &createdAt, &startedAt, &completedAt, &errorMsg, &resultCount, &failedCount, &seedURLsJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}

	// Deserialize config and progress
	config, err := crawler.FromJSONCrawlConfig(configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize config: %w", err)
	}

	progress, err := crawler.FromJSONCrawlProgress(progressJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize progress: %w", err)
	}

	// Build CrawlJob
	job := &crawler.CrawlJob{
		ID:                   id,
		SourceType:           sourceType,
		EntityType:           entityType,
		Config:               *config,
		SourceConfigSnapshot: "",
		AuthSnapshot:         "",
		RefreshSource:        refreshSource != 0,
		Status:               crawler.JobStatus(status),
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
func (s *JobStorage) scanJobs(rows *sql.Rows) ([]interface{}, error) {
	var jobs []interface{}

	for rows.Next() {
		var (
			id, sourceType, entityType, configJSON, status, progressJSON string
			sourceConfigSnapshot, authSnapshot                           sql.NullString
			refreshSource                                                int
			errorMsg                                                     sql.NullString
			createdAt                                                    int64
			startedAt, completedAt                                       sql.NullInt64
			resultCount, failedCount                                     int
			seedURLsJSON                                                 sql.NullString
		)

		err := rows.Scan(
			&id, &sourceType, &entityType, &configJSON, &sourceConfigSnapshot, &authSnapshot, &refreshSource,
			&status, &progressJSON, &createdAt, &startedAt, &completedAt, &errorMsg, &resultCount, &failedCount, &seedURLsJSON,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		// Deserialize config and progress
		config, err := crawler.FromJSONCrawlConfig(configJSON)
		if err != nil {
			s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize config, skipping job")
			continue
		}

		progress, err := crawler.FromJSONCrawlProgress(progressJSON)
		if err != nil {
			s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize progress, skipping job")
			continue
		}

		// Build CrawlJob
		job := &crawler.CrawlJob{
			ID:                   id,
			SourceType:           sourceType,
			EntityType:           entityType,
			Config:               *config,
			SourceConfigSnapshot: "",
			AuthSnapshot:         "",
			RefreshSource:        refreshSource != 0,
			Status:               crawler.JobStatus(status),
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
