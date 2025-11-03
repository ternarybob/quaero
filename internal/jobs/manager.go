package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/quaero/internal/queue"
)

// Manager handles job metadata and lifecycle.
// It does NOT manage the queue - that's goqite's job.
type Manager struct {
	db    *sql.DB
	queue *queue.Manager
}

func NewManager(db *sql.DB, queue *queue.Manager) *Manager {
	return &Manager{
		db:    db,
		queue: queue,
	}
}

// Job represents job metadata
type Job struct {
	ID              string     `json:"id"`
	ParentID        *string    `json:"parent_id,omitempty"`
	Type            string     `json:"job_type"`
	Phase           string     `json:"phase"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Payload         string     `json:"payload,omitempty"`
	Result          string     `json:"result,omitempty"`
	Error           *string    `json:"error,omitempty"`
	ProgressCurrent int        `json:"progress_current"`
	ProgressTotal   int        `json:"progress_total"`
}

// JobLog represents a job log entry
type JobLog struct {
	ID        int       `json:"id"`
	JobID     string    `json:"job_id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// Helper types for JSON field mapping
type metadataJSON struct {
	Phase  string `json:"phase,omitempty"`
	Result string `json:"result,omitempty"`
}

type progressJSON struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

// retryOnBusy retries a database operation with exponential backoff on SQLITE_BUSY errors
// This is critical for handling write contention in high-concurrency job processing
func retryOnBusy(ctx context.Context, operation func() error) error {
	const maxRetries = 5
	const baseDelay = 50 * time.Millisecond // Start with 50ms

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Check if error is SQLITE_BUSY
		errMsg := err.Error()
		if !strings.Contains(errMsg, "database is locked") && !strings.Contains(errMsg, "SQLITE_BUSY") {
			// Not a busy error, fail immediately
			return err
		}

		lastErr = err

		// Check context cancellation
		if ctx.Err() != nil {
			return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		}

		// Last attempt failed, don't sleep
		if attempt == maxRetries-1 {
			break
		}

		// Exponential backoff: 50ms, 100ms, 200ms, 400ms, 800ms
		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		time.Sleep(delay)
	}

	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

// Helper functions for time conversions
func timeToUnix(t time.Time) int64 {
	return t.Unix()
}

func timeToUnixPtr(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

func unixToTime(unix int64) time.Time {
	return time.Unix(unix, 0)
}

func unixToTimePtr(unix sql.NullInt64) *time.Time {
	if !unix.Valid {
		return nil
	}
	t := time.Unix(unix.Int64, 0)
	return &t
}

// CreateJob creates a new job record without enqueueing (for tracking only)
func (m *Manager) CreateJob(ctx context.Context, job *Job) error {
	// Create metadata JSON with phase
	metadata := metadataJSON{Phase: job.Phase}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	// Create progress JSON
	progress := progressJSON{Current: job.ProgressCurrent, Total: job.ProgressTotal}
	progressJSONBytes, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}

	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}

	// Create empty config JSON for parent jobs
	emptyConfig := make(map[string]interface{})
	configJSONBytes, err := json.Marshal(emptyConfig)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Create job record in crawl_jobs table with retry on SQLITE_BUSY
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO crawl_jobs (
				id, parent_id, job_type, name, description,
				source_type, entity_type, config_json,
				status, progress_json, metadata,
				created_at, refresh_source, result_count, failed_count
			)
			VALUES (?, ?, ?, ?, ?, 'job_definition', 'job', ?, ?, ?, ?, ?, 0, 0, 0)
		`, job.ID, job.ParentID, job.Type, job.Type, job.Type,
			string(configJSONBytes), job.Status, string(progressJSONBytes), string(metadataJSON), timeToUnix(job.CreatedAt))
		return err
	})

	if err != nil {
		return fmt.Errorf("create job record: %w", err)
	}

	return nil
}

// CreateParentJob creates a new parent job and enqueues it
func (m *Manager) CreateParentJob(ctx context.Context, jobType string, payload interface{}) (string, error) {
	jobID := uuid.New().String()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Create metadata JSON with phase
	metadata := metadataJSON{Phase: "core"}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}

	// Create empty progress JSON
	progress := progressJSON{Current: 0, Total: 0}
	progressJSONBytes, err := json.Marshal(progress)
	if err != nil {
		return "", fmt.Errorf("marshal progress: %w", err)
	}

	now := time.Now()

	// Create job record in crawl_jobs table with retry on SQLITE_BUSY
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO crawl_jobs (
				id, parent_id, job_type, name, description,
				source_type, entity_type, config_json,
				status, progress_json, metadata,
				created_at, refresh_source, result_count, failed_count
			)
			VALUES (?, NULL, ?, '', '', 'queue', 'job', ?, 'pending', ?, ?, ?, 0, 0, 0)
		`, jobID, jobType, string(payloadJSON), string(progressJSONBytes), string(metadataJSON), timeToUnix(now))
		return err
	})

	if err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, queue.Message{
		JobID:   jobID,
		Type:    jobType,
		Payload: payloadJSON,
	}); err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}

	return jobID, nil
}

// CreateChildJob creates a child job and enqueues it
func (m *Manager) CreateChildJob(ctx context.Context, parentID, jobType, phase string, payload interface{}) (string, error) {
	jobID := uuid.New().String()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Create metadata JSON with phase
	metadata := metadataJSON{Phase: phase}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}

	// Create empty progress JSON
	progress := progressJSON{Current: 0, Total: 0}
	progressJSONBytes, err := json.Marshal(progress)
	if err != nil {
		return "", fmt.Errorf("marshal progress: %w", err)
	}

	now := time.Now()

	// Create job record in crawl_jobs table with retry on SQLITE_BUSY
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO crawl_jobs (
				id, parent_id, job_type, name, description,
				source_type, entity_type, config_json,
				status, progress_json, metadata,
				created_at, refresh_source, result_count, failed_count
			)
			VALUES (?, ?, ?, '', '', 'queue', 'job', ?, 'pending', ?, ?, ?, 0, 0, 0)
		`, jobID, parentID, jobType, string(payloadJSON), string(progressJSONBytes), string(metadataJSON), timeToUnix(now))
		return err
	})

	if err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, queue.Message{
		JobID:   jobID,
		Type:    jobType,
		Payload: payloadJSON,
	}); err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}

	return jobID, nil
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(ctx context.Context, jobID string) (*Job, error) {
	var job Job
	var parentID sql.NullString
	var startedAt, completedAt sql.NullInt64
	var errMsg sql.NullString
	var createdAtUnix int64
	var configJSON, metadataStr, progressStr string

	row := m.db.QueryRowContext(ctx, `
		SELECT id, parent_id, job_type, status, created_at, started_at,
		       completed_at, config_json, metadata, error, progress_json
		FROM crawl_jobs
		WHERE id = ?
	`, jobID)

	if err := row.Scan(
		&job.ID, &parentID, &job.Type, &job.Status,
		&createdAtUnix, &startedAt, &completedAt,
		&configJSON, &metadataStr, &errMsg, &progressStr,
	); err != nil {
		return nil, err
	}

	// Map fields
	if parentID.Valid {
		job.ParentID = &parentID.String
	}
	job.CreatedAt = unixToTime(createdAtUnix)
	job.StartedAt = unixToTimePtr(startedAt)
	job.CompletedAt = unixToTimePtr(completedAt)
	job.Payload = configJSON
	if errMsg.Valid {
		job.Error = &errMsg.String
	}

	// Parse metadata JSON for phase and result
	var metadata metadataJSON
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err == nil {
		job.Phase = metadata.Phase
		job.Result = metadata.Result
	}

	// Parse progress JSON
	var progress progressJSON
	if err := json.Unmarshal([]byte(progressStr), &progress); err == nil {
		job.ProgressCurrent = progress.Current
		job.ProgressTotal = progress.Total
	}

	return &job, nil
}

// ListParentJobs returns all parent jobs (parent_id IS NULL)
func (m *Manager) ListParentJobs(ctx context.Context, limit, offset int) ([]Job, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, job_type, status, created_at, started_at, completed_at,
		       metadata, progress_json, error
		FROM crawl_jobs
		WHERE parent_id IS NULL
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var startedAt, completedAt sql.NullInt64
		var errorMsg sql.NullString
		var createdAtUnix int64
		var metadataStr, progressStr string

		if err := rows.Scan(
			&job.ID, &job.Type, &job.Status,
			&createdAtUnix, &startedAt, &completedAt,
			&metadataStr, &progressStr, &errorMsg,
		); err != nil {
			return nil, err
		}

		job.CreatedAt = unixToTime(createdAtUnix)
		job.StartedAt = unixToTimePtr(startedAt)
		job.CompletedAt = unixToTimePtr(completedAt)
		if errorMsg.Valid {
			job.Error = &errorMsg.String
		}

		// Parse metadata for phase
		var metadata metadataJSON
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err == nil {
			job.Phase = metadata.Phase
		}

		// Parse progress JSON
		var progress progressJSON
		if err := json.Unmarshal([]byte(progressStr), &progress); err == nil {
			job.ProgressCurrent = progress.Current
			job.ProgressTotal = progress.Total
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// ListChildJobs returns all child jobs for a parent
func (m *Manager) ListChildJobs(ctx context.Context, parentID string) ([]Job, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, parent_id, job_type, status, created_at, started_at,
		       completed_at, metadata, progress_json, error
		FROM crawl_jobs
		WHERE parent_id = ?
		ORDER BY created_at ASC
	`, parentID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var parentIDStr sql.NullString
		var startedAt, completedAt sql.NullInt64
		var errorMsg sql.NullString
		var createdAtUnix int64
		var metadataStr, progressStr string

		if err := rows.Scan(
			&job.ID, &parentIDStr, &job.Type, &job.Status,
			&createdAtUnix, &startedAt, &completedAt,
			&metadataStr, &progressStr, &errorMsg,
		); err != nil {
			return nil, err
		}

		if parentIDStr.Valid {
			job.ParentID = &parentIDStr.String
		}
		job.CreatedAt = unixToTime(createdAtUnix)
		job.StartedAt = unixToTimePtr(startedAt)
		job.CompletedAt = unixToTimePtr(completedAt)
		if errorMsg.Valid {
			job.Error = &errorMsg.String
		}

		// Parse metadata for phase
		var metadata metadataJSON
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err == nil {
			job.Phase = metadata.Phase
		}

		// Parse progress JSON
		var progress progressJSON
		if err := json.Unmarshal([]byte(progressStr), &progress); err == nil {
			job.ProgressCurrent = progress.Current
			job.ProgressTotal = progress.Total
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// UpdateJobStatus updates the job status
func (m *Manager) UpdateJobStatus(ctx context.Context, jobID, status string) error {
	now := time.Now()
	nowUnix := timeToUnix(now)

	query := "UPDATE crawl_jobs SET status = ?, last_heartbeat = ?"
	args := []interface{}{status, nowUnix}

	if status == "running" {
		query += ", started_at = ?"
		args = append(args, nowUnix)
	} else if status == "completed" || status == "failed" || status == "cancelled" {
		query += ", completed_at = ?"
		args = append(args, nowUnix)
	}

	query += " WHERE id = ?"
	args = append(args, jobID)

	// Use retry logic for status updates to handle write contention
	return retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, query, args...)
		return err
	})
}

// UpdateJobProgress updates job progress
func (m *Manager) UpdateJobProgress(ctx context.Context, jobID string, current, total int) error {
	progress := progressJSON{Current: current, Total: total}
	progressJSONBytes, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}

	_, err = m.db.ExecContext(ctx, `
		UPDATE crawl_jobs SET progress_json = ?
		WHERE id = ?
	`, string(progressJSONBytes), jobID)
	return err
}

// SetJobError sets job error message and marks as failed
func (m *Manager) SetJobError(ctx context.Context, jobID string, errorMsg string) error {
	now := time.Now()
	_, err := m.db.ExecContext(ctx, `
		UPDATE crawl_jobs SET status = 'failed', error = ?, completed_at = ?
		WHERE id = ?
	`, errorMsg, timeToUnix(now), jobID)
	return err
}

// SetJobResult sets job result data
func (m *Manager) SetJobResult(ctx context.Context, jobID string, result interface{}) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	// Read existing metadata to preserve phase
	var existingMetadata string
	err = m.db.QueryRowContext(context.Background(), `
		SELECT metadata FROM crawl_jobs WHERE id = ?
	`, jobID).Scan(&existingMetadata)
	if err != nil {
		return fmt.Errorf("read existing metadata: %w", err)
	}

	// Parse existing metadata
	var metadata metadataJSON
	if err := json.Unmarshal([]byte(existingMetadata), &metadata); err != nil {
		metadata = metadataJSON{} // Start fresh if parse fails
	}

	// Update result field
	metadata.Result = string(resultJSON)

	// Marshal updated metadata
	updatedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal updated metadata: %w", err)
	}

	_, err = m.db.ExecContext(context.Background(), `
		UPDATE crawl_jobs SET metadata = ? WHERE id = ?
	`, string(updatedMetadata), jobID)
	return err
}

// AddJobLog adds a log entry for a job
func (m *Manager) AddJobLog(ctx context.Context, jobID, level, message string) error {
	now := time.Now()
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO job_logs (job_id, timestamp, level, message, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, jobID, now.Format(time.RFC3339), level, message, timeToUnix(now))
	return err
}

// GetJobLogs retrieves logs for a job
func (m *Manager) GetJobLogs(ctx context.Context, jobID string, limit int) ([]JobLog, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, job_id, timestamp, level, message
		FROM job_logs
		WHERE job_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, jobID, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []JobLog
	for rows.Next() {
		var log JobLog
		var timestampStr string
		if err := rows.Scan(&log.ID, &log.JobID, &timestampStr, &log.Level, &log.Message); err != nil {
			return nil, err
		}

		// Parse RFC3339 timestamp
		if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			log.Timestamp = t
		}

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// JobTreeStatus represents aggregated status for a job tree (parent + children)
type JobTreeStatus struct {
	ParentJob       *Job      `json:"parent_job"`
	TotalChildren   int       `json:"total_children"`
	CompletedCount  int       `json:"completed_count"`
	FailedCount     int       `json:"failed_count"`
	RunningCount    int       `json:"running_count"`
	PendingCount    int       `json:"pending_count"`
	CancelledCount  int       `json:"cancelled_count"`
	OverallProgress float64   `json:"overall_progress"` // 0.0 to 1.0
	EstimatedTime   *int64    `json:"estimated_time_ms,omitempty"` // Estimated milliseconds to completion
}

// GetJobTreeStatus retrieves aggregated status for a parent job and all its children
// This provides efficient status reporting for hierarchical job execution
func (m *Manager) GetJobTreeStatus(ctx context.Context, parentJobID string) (*JobTreeStatus, error) {
	// Get parent job
	parentJob, err := m.GetJob(ctx, parentJobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent job: %w", err)
	}

	// Aggregate child job statuses with single SQL query
	var totalChildren, completedCount, failedCount, runningCount, pendingCount, cancelledCount int

	row := m.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) as cancelled
		FROM crawl_jobs
		WHERE parent_id = ?
	`, parentJobID)

	if err := row.Scan(&totalChildren, &completedCount, &failedCount, &runningCount, &pendingCount, &cancelledCount); err != nil {
		return nil, fmt.Errorf("failed to aggregate child statuses: %w", err)
	}

	// Calculate overall progress
	// Progress based on completed + failed (terminal states) vs total
	var overallProgress float64
	if totalChildren > 0 {
		terminalCount := completedCount + failedCount + cancelledCount
		overallProgress = float64(terminalCount) / float64(totalChildren)
	} else {
		// No children yet, use parent job progress if available
		if parentJob.ProgressTotal > 0 {
			overallProgress = float64(parentJob.ProgressCurrent) / float64(parentJob.ProgressTotal)
		}
	}

	// Estimate time to completion (simple linear extrapolation)
	var estimatedTime *int64
	if runningCount > 0 && parentJob.StartedAt != nil {
		elapsed := time.Since(*parentJob.StartedAt)
		if overallProgress > 0 && overallProgress < 1.0 {
			totalEstimated := float64(elapsed) / overallProgress
			remaining := totalEstimated - float64(elapsed)
			remainingMS := int64(time.Duration(remaining) / time.Millisecond)
			estimatedTime = &remainingMS
		}
	}

	status := &JobTreeStatus{
		ParentJob:       parentJob,
		TotalChildren:   totalChildren,
		CompletedCount:  completedCount,
		FailedCount:     failedCount,
		RunningCount:    runningCount,
		PendingCount:    pendingCount,
		CancelledCount:  cancelledCount,
		OverallProgress: overallProgress,
		EstimatedTime:   estimatedTime,
	}

	return status, nil
}

// GetFailedChildCount returns the count of failed child jobs for a parent job
func (m *Manager) GetFailedChildCount(ctx context.Context, parentJobID string) (int, error) {
	var failedCount int
	err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM crawl_jobs
		WHERE parent_id = ? AND status = 'failed'
	`, parentJobID).Scan(&failedCount)

	if err != nil {
		return 0, fmt.Errorf("failed to query failed job count: %w", err)
	}

	return failedCount, nil
}
