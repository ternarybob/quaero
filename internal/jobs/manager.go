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
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// Manager handles job metadata and lifecycle.
// It does NOT manage the queue - that's goqite's job.
type Manager struct {
	db           *sql.DB
	queue        *queue.Manager
	eventService interfaces.EventService // Optional: may be nil for testing
}

func NewManager(db *sql.DB, queue *queue.Manager, eventService interfaces.EventService) *Manager {
	return &Manager{
		db:           db,
		queue:        queue,
		eventService: eventService,
	}
}

// Job represents job metadata
type Job struct {
	ID              string     `json:"id"`
	ParentID        *string    `json:"parent_id,omitempty"`
	Type            string     `json:"job_type"`
	Name            string     `json:"name"` // Human-readable job name
	Phase           string     `json:"phase"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"` // Set when job AND all children complete or timeout
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

// CreateJobRecord creates a new job record without enqueueing (for tracking only)
func (m *Manager) CreateJobRecord(ctx context.Context, job *Job) error {
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

	// Create job record in jobs table with retry on SQLITE_BUSY
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO jobs (
				id, parent_id, job_type, name, description,
				config_json, metadata_json,
				status, progress_json,
				created_at, result_count, failed_count
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)
		`, job.ID, job.ParentID, job.Type, job.Name, job.Name,
			string(configJSONBytes), string(metadataJSON),
			job.Status, string(progressJSONBytes), timeToUnix(job.CreatedAt))
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

	// Create metadata JSON with phase and document_count initialization
	metadata := map[string]interface{}{
		"phase":          "core",
		"document_count": 0,
	}
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

	// Create job record in jobs table with retry on SQLITE_BUSY
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO jobs (
				id, parent_id, job_type, name, description,
				config_json, metadata_json,
				status, progress_json,
				created_at, result_count, failed_count
			)
			VALUES (?, NULL, ?, '', '', ?, ?, 'pending', ?, ?, 0, 0)
		`, jobID, jobType, string(payloadJSON), string(metadataJSON), string(progressJSONBytes), timeToUnix(now))
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

	// Create job record in jobs table with retry on SQLITE_BUSY
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO jobs (
				id, parent_id, job_type, name, description,
				config_json, metadata_json,
				status, progress_json,
				created_at, result_count, failed_count
			)
			VALUES (?, ?, ?, '', '', ?, ?, 'pending', ?, ?, 0, 0)
		`, jobID, parentID, jobType, string(payloadJSON), string(metadataJSON), string(progressJSONBytes), timeToUnix(now))
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

// GetJobInternal retrieves a job by ID (internal jobs.Job type)
func (m *Manager) GetJobInternal(ctx context.Context, jobID string) (*Job, error) {
	var job Job
	var parentID sql.NullString
	var startedAt, completedAt, finishedAt sql.NullInt64
	var errMsg sql.NullString
	var createdAtUnix int64
	var configJSON, metadataStr, progressStr string

	row := m.db.QueryRowContext(ctx, `
		SELECT id, parent_id, job_type, status, created_at, started_at,
		       completed_at, finished_at, config_json, metadata_json, error, progress_json
		FROM jobs
		WHERE id = ?
	`, jobID)

	if err := row.Scan(
		&job.ID, &parentID, &job.Type, &job.Status,
		&createdAtUnix, &startedAt, &completedAt, &finishedAt,
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
	job.FinishedAt = unixToTimePtr(finishedAt)
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
		SELECT id, job_type, status, created_at, started_at, completed_at, finished_at,
		       metadata_json, progress_json, error
		FROM jobs
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
		var startedAt, completedAt, finishedAt sql.NullInt64
		var errorMsg sql.NullString
		var createdAtUnix int64
		var metadataStr, progressStr string

		if err := rows.Scan(
			&job.ID, &job.Type, &job.Status,
			&createdAtUnix, &startedAt, &completedAt, &finishedAt,
			&metadataStr, &progressStr, &errorMsg,
		); err != nil {
			return nil, err
		}

		job.CreatedAt = unixToTime(createdAtUnix)
		job.StartedAt = unixToTimePtr(startedAt)
		job.CompletedAt = unixToTimePtr(completedAt)
		job.FinishedAt = unixToTimePtr(finishedAt)
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
		       completed_at, finished_at, metadata_json, progress_json, error
		FROM jobs
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
		var startedAt, completedAt, finishedAt sql.NullInt64
		var errorMsg sql.NullString
		var createdAtUnix int64
		var metadataStr, progressStr string

		if err := rows.Scan(
			&job.ID, &parentIDStr, &job.Type, &job.Status,
			&createdAtUnix, &startedAt, &completedAt, &finishedAt,
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
		job.FinishedAt = unixToTimePtr(finishedAt)
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
	// Get job details before update to access parent_id and job_type
	var parentID sql.NullString
	var jobType string
	err := m.db.QueryRowContext(ctx, `
		SELECT parent_id, job_type FROM jobs WHERE id = ?
	`, jobID).Scan(&parentID, &jobType)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	now := time.Now()
	nowUnix := timeToUnix(now)

	query := "UPDATE jobs SET status = ?, last_heartbeat = ?"
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
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, query, args...)
		return err
	})

	if err != nil {
		return err
	}

	// Add job log for status change
	logMessage := fmt.Sprintf("Status changed: %s", status)
	if err := m.AddJobLog(ctx, jobID, "info", logMessage); err != nil {
		// Log error but don't fail the status update (logging is non-critical)
	}

	// Publish job status change event for parent job monitoring
	// Only publish if eventService is available (optional dependency)
	if m.eventService != nil {
		payload := map[string]interface{}{
			"job_id":    jobID,
			"status":    status,
			"job_type":  jobType,
			"timestamp": now.Format(time.RFC3339),
		}

		// Include parent_id if this is a child job
		if parentID.Valid {
			payload["parent_id"] = parentID.String
		}

		event := interfaces.Event{
			Type:    interfaces.EventJobStatusChange,
			Payload: payload,
		}

		// Publish asynchronously to avoid blocking status updates
		go func() {
			if err := m.eventService.Publish(ctx, event); err != nil {
				// Log error but don't fail the status update
				// EventService will handle logging via its subscribers
			}
		}()
	}

	return nil
}

// UpdateJobProgress updates job progress
func (m *Manager) UpdateJobProgress(ctx context.Context, jobID string, current, total int) error {
	progress := progressJSON{Current: current, Total: total}
	progressJSONBytes, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}

	_, err = m.db.ExecContext(ctx, `
		UPDATE jobs SET progress_json = ?
		WHERE id = ?
	`, string(progressJSONBytes), jobID)
	return err
}

// SetJobError sets job error message and marks as failed
func (m *Manager) SetJobError(ctx context.Context, jobID string, errorMsg string) error {
	now := time.Now()
	_, err := m.db.ExecContext(ctx, `
		UPDATE jobs SET status = 'failed', error = ?, completed_at = ?
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
		SELECT metadata_json FROM jobs WHERE id = ?
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
		UPDATE jobs SET metadata_json = ? WHERE id = ?
	`, string(updatedMetadata), jobID)
	return err
}

// IncrementDocumentCount increments the document_count in job metadata
// This is used to track the number of documents saved by child jobs for a parent job
func (m *Manager) IncrementDocumentCount(ctx context.Context, jobID string) error {
	// Read current metadata
	var metadataStr string
	err := m.db.QueryRowContext(ctx, `
		SELECT metadata_json FROM jobs WHERE id = ?
	`, jobID).Scan(&metadataStr)
	if err != nil {
		return fmt.Errorf("failed to get job metadata: %w", err)
	}

	// Parse metadata
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Increment document_count (default 0 if not exists)
	currentCount := 0
	if count, ok := metadata["document_count"].(float64); ok {
		currentCount = int(count)
	} else if count, ok := metadata["document_count"].(int); ok {
		currentCount = count
	}
	metadata["document_count"] = currentCount + 1

	// Save updated metadata
	updatedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Use retry logic for write contention
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			UPDATE jobs SET metadata_json = ? WHERE id = ?
		`, string(updatedMetadata), jobID)
		return err
	})

	return err
}

// SetJobFinished sets the finished_at timestamp for a job
// This should be called when a job AND all its spawned children complete or timeout
func (m *Manager) SetJobFinished(ctx context.Context, jobID string) error {
	now := time.Now()
	_, err := m.db.ExecContext(ctx, `
		UPDATE jobs SET finished_at = ?
		WHERE id = ?
	`, timeToUnix(now), jobID)
	return err
}

// UpdateJobConfig updates the job configuration in the database
func (m *Manager) UpdateJobConfig(ctx context.Context, jobID string, config map[string]interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	_, err = m.db.ExecContext(ctx, `
		UPDATE jobs SET config_json = ?
		WHERE id = ?
	`, string(configJSON), jobID)
	return err
}

// UpdateJobMetadata updates the job metadata in the database
// This method merges new metadata with existing metadata to preserve fields like phase
func (m *Manager) UpdateJobMetadata(ctx context.Context, jobID string, metadata map[string]interface{}) error {
	// Read existing metadata
	var existingMetadataJSON string
	err := m.db.QueryRowContext(ctx, `
		SELECT metadata_json FROM jobs WHERE id = ?
	`, jobID).Scan(&existingMetadataJSON)
	if err != nil {
		return fmt.Errorf("failed to read existing metadata: %w", err)
	}

	// Unmarshal existing metadata
	var existingMetadata map[string]interface{}
	if err := json.Unmarshal([]byte(existingMetadataJSON), &existingMetadata); err != nil {
		// If unmarshal fails, start with empty map
		existingMetadata = make(map[string]interface{})
	}

	// Merge new metadata into existing metadata (new values override existing)
	for key, value := range metadata {
		existingMetadata[key] = value
	}

	// Marshal merged metadata
	mergedMetadataJSON, err := json.Marshal(existingMetadata)
	if err != nil {
		return fmt.Errorf("marshal merged metadata: %w", err)
	}

	// Update database with merged metadata using retry logic for write contention
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			UPDATE jobs SET metadata_json = ?
			WHERE id = ?
		`, string(mergedMetadataJSON), jobID)
		return err
	})

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
	ParentJob       *Job    `json:"parent_job"`
	TotalChildren   int     `json:"total_children"`
	CompletedCount  int     `json:"completed_count"`
	FailedCount     int     `json:"failed_count"`
	RunningCount    int     `json:"running_count"`
	PendingCount    int     `json:"pending_count"`
	CancelledCount  int     `json:"cancelled_count"`
	OverallProgress float64 `json:"overall_progress"`            // 0.0 to 1.0
	EstimatedTime   *int64  `json:"estimated_time_ms,omitempty"` // Estimated milliseconds to completion
}

// GetJobTreeStatus retrieves aggregated status for a parent job and all its children
// This provides efficient status reporting for hierarchical job execution
func (m *Manager) GetJobTreeStatus(ctx context.Context, parentJobID string) (*JobTreeStatus, error) {
	// Get parent job using internal method
	parentJobInternal, err := m.GetJobInternal(ctx, parentJobID)
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
		FROM jobs
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
		if parentJobInternal.ProgressTotal > 0 {
			overallProgress = float64(parentJobInternal.ProgressCurrent) / float64(parentJobInternal.ProgressTotal)
		}
	}

	// Estimate time to completion (simple linear extrapolation)
	var estimatedTime *int64
	if runningCount > 0 && parentJobInternal.StartedAt != nil {
		elapsed := time.Since(*parentJobInternal.StartedAt)
		if overallProgress > 0 && overallProgress < 1.0 {
			totalEstimated := float64(elapsed) / overallProgress
			remaining := totalEstimated - float64(elapsed)
			remainingMS := int64(time.Duration(remaining) / time.Millisecond)
			estimatedTime = &remainingMS
		}
	}

	status := &JobTreeStatus{
		ParentJob:       parentJobInternal,
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
		FROM jobs
		WHERE parent_id = ? AND status = 'failed'
	`, parentJobID).Scan(&failedCount)

	if err != nil {
		return 0, fmt.Errorf("failed to query failed job count: %w", err)
	}

	return failedCount, nil
}

// ============================================================================
// Interface Adapter Methods (interfaces.JobManager)
// ============================================================================

// CreateJob implements interfaces.JobManager.CreateJob
// Creates a new job with the given source type, source ID, and configuration
func (m *Manager) CreateJob(ctx context.Context, sourceType, sourceID string, config map[string]interface{}) (string, error) {
	// Create job model
	jobType := sourceType // Use source type as job type
	name := fmt.Sprintf("%s job for %s", sourceType, sourceID)

	metadata := map[string]interface{}{
		"source_id": sourceID,
	}

	jobModel := models.NewJobModel(jobType, name, config, metadata)

	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	// Serialize metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Serialize progress
	progress := &models.JobProgress{
		TotalURLs:     0,
		CompletedURLs: 0,
		FailedURLs:    0,
		PendingURLs:   0,
		Percentage:    0.0,
	}
	progressJSON, err := json.Marshal(progress)
	if err != nil {
		return "", fmt.Errorf("failed to marshal progress: %w", err)
	}

	now := time.Now()

	// Create job record in jobs table
	err = retryOnBusy(ctx, func() error {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO jobs (
				id, parent_id, job_type, name, description,
				config_json, metadata_json,
				status, progress_json,
				created_at, result_count, failed_count
			)
			VALUES (?, NULL, ?, ?, '', ?, ?, 'pending', ?, ?, 0, 0)
		`, jobModel.ID, jobType, name, string(configJSON), string(metadataJSON), string(progressJSON), timeToUnix(now))
		return err
	})

	if err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, queue.Message{
		JobID:   jobModel.ID,
		Type:    jobType,
		Payload: configJSON,
	}); err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}

	return jobModel.ID, nil
}

// GetJob implements interfaces.JobManager.GetJob
// Returns interface{} to match the interface, but the actual type is *models.Job
func (m *Manager) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	// Query the jobs table
	var job models.Job
	var jobModel models.JobModel
	var parentID, errorMsg sql.NullString
	var startedAt, completedAt, finishedAt, lastHeartbeat sql.NullInt64
	var configJSON, metadataJSON, progressJSON string
	var createdAtUnix int64
	var depth sql.NullInt64

	row := m.db.QueryRowContext(ctx, `
		SELECT id, parent_id, job_type, name, description, config_json, metadata_json,
		       status, progress_json, created_at, started_at, completed_at, finished_at,
		       last_heartbeat, error, result_count, failed_count, depth
		FROM jobs
		WHERE id = ?
	`, jobID)

	err := row.Scan(
		&jobModel.ID, &parentID, &jobModel.Type, &jobModel.Name, &sql.NullString{}, // description not in JobModel
		&configJSON, &metadataJSON,
		&job.Status, &progressJSON,
		&createdAtUnix, &startedAt, &completedAt, &finishedAt,
		&lastHeartbeat, &errorMsg, &job.ResultCount, &job.FailedCount, &depth,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job not found: %s", jobID)
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Map fields
	if parentID.Valid {
		jobModel.ParentID = &parentID.String
	}
	jobModel.CreatedAt = unixToTime(createdAtUnix)
	if depth.Valid {
		jobModel.Depth = int(depth.Int64)
	}

	// Parse config JSON
	if err := json.Unmarshal([]byte(configJSON), &jobModel.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Parse metadata JSON
	if err := json.Unmarshal([]byte(metadataJSON), &jobModel.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Parse progress JSON
	var progress models.JobProgress
	if err := json.Unmarshal([]byte(progressJSON), &progress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal progress: %w", err)
	}
	job.Progress = &progress

	// Map timestamps
	job.StartedAt = unixToTimePtr(startedAt)
	job.CompletedAt = unixToTimePtr(completedAt)
	job.FinishedAt = unixToTimePtr(finishedAt)
	job.LastHeartbeat = unixToTimePtr(lastHeartbeat)

	if errorMsg.Valid {
		job.Error = errorMsg.String
	}

	// Embed JobModel into Job
	job.JobModel = &jobModel

	return &job, nil
}

// ListJobs implements interfaces.JobManager.ListJobs
// Converts internal Job type to models.Job for compatibility
func (m *Manager) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.Job, error) {
	// Build query based on options
	query := `
		SELECT id, parent_id, job_type, name, description, config_json, metadata_json,
		       status, progress_json, created_at, started_at, completed_at, finished_at,
		       last_heartbeat, error, result_count, failed_count, depth
		FROM jobs
		WHERE 1=1
	`
	args := []interface{}{}

	if opts != nil {
		if opts.Status != "" {
			// Support comma-separated status values (e.g., "pending,running,completed")
			statuses := strings.Split(opts.Status, ",")
			if len(statuses) == 1 {
				// Single status value
				query += " AND status = ?"
				args = append(args, strings.TrimSpace(statuses[0]))
			} else {
				// Multiple status values - use IN clause
				placeholders := strings.Repeat("?,", len(statuses))
				placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma
				query += fmt.Sprintf(" AND status IN (%s)", placeholders)
				for _, s := range statuses {
					args = append(args, strings.TrimSpace(s))
				}
			}
		}
		if opts.ParentID != "" {
			if opts.ParentID == "root" {
				query += " AND parent_id IS NULL"
			} else {
				query += " AND parent_id = ?"
				args = append(args, opts.ParentID)
			}
		}

		// Ordering
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

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	jobs := []*models.Job{}
	for rows.Next() {
		var job models.Job
		var jobModel models.JobModel
		var parentID, errorMsg sql.NullString
		var startedAt, completedAt, finishedAt, lastHeartbeat sql.NullInt64
		var configJSON, metadataJSON, progressJSON string
		var createdAtUnix int64
		var depth sql.NullInt64

		var description sql.NullString
		if err := rows.Scan(
			&jobModel.ID, &parentID, &jobModel.Type, &jobModel.Name, &description,
			&configJSON, &metadataJSON,
			&job.Status, &progressJSON,
			&createdAtUnix, &startedAt, &completedAt, &finishedAt, &lastHeartbeat,
			&errorMsg, &job.ResultCount, &job.FailedCount, &depth,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		// Parse timestamps
		jobModel.CreatedAt = unixToTime(createdAtUnix)
		if startedAt.Valid {
			t := unixToTime(startedAt.Int64)
			job.StartedAt = &t
		}
		if completedAt.Valid {
			t := unixToTime(completedAt.Int64)
			job.CompletedAt = &t
		}
		if finishedAt.Valid {
			t := unixToTime(finishedAt.Int64)
			job.FinishedAt = &t
		}
		if lastHeartbeat.Valid {
			t := unixToTime(lastHeartbeat.Int64)
			job.LastHeartbeat = &t
		}

		// Parse JSON fields
		if err := json.Unmarshal([]byte(configJSON), &jobModel.Config); err != nil {
			jobModel.Config = make(map[string]interface{})
		}
		if err := json.Unmarshal([]byte(metadataJSON), &jobModel.Metadata); err != nil {
			jobModel.Metadata = make(map[string]interface{})
		}
		if err := json.Unmarshal([]byte(progressJSON), &job.Progress); err != nil {
			job.Progress = &models.JobProgress{}
		}

		// Set optional fields
		if parentID.Valid {
			jobModel.ParentID = &parentID.String
		}
		if errorMsg.Valid {
			job.Error = errorMsg.String
		}

		// Embed JobModel into Job
		job.JobModel = &jobModel
		jobs = append(jobs, &job)
	}

	return jobs, rows.Err()
}

// CountJobs implements interfaces.JobManager.CountJobs
func (m *Manager) CountJobs(ctx context.Context, opts *interfaces.JobListOptions) (int, error) {
	query := "SELECT COUNT(*) FROM jobs WHERE 1=1"
	args := []interface{}{}

	if opts != nil {
		if opts.Status != "" {
			// Support comma-separated status values (e.g., "pending,running,completed")
			statuses := strings.Split(opts.Status, ",")
			if len(statuses) == 1 {
				// Single status value
				query += " AND status = ?"
				args = append(args, strings.TrimSpace(statuses[0]))
			} else {
				// Multiple status values - use IN clause
				placeholders := strings.Repeat("?,", len(statuses))
				placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma
				query += fmt.Sprintf(" AND status IN (%s)", placeholders)
				for _, s := range statuses {
					args = append(args, strings.TrimSpace(s))
				}
			}
		}
		if opts.ParentID != "" {
			if opts.ParentID == "root" {
				query += " AND parent_id IS NULL"
			} else {
				query += " AND parent_id = ?"
				args = append(args, opts.ParentID)
			}
		}
	}

	var count int
	err := m.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return count, nil
}

// UpdateJob implements interfaces.JobManager.UpdateJob
func (m *Manager) UpdateJob(ctx context.Context, job interface{}) error {
	// Type assert to *models.Job
	modelJob, ok := job.(*models.Job)
	if !ok {
		return fmt.Errorf("invalid job type: expected *models.Job, got %T", job)
	}

	// Serialize JSON fields
	configJSON, err := json.Marshal(modelJob.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	metadataJSON, err := json.Marshal(modelJob.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	progressJSON, err := json.Marshal(modelJob.Progress)
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	// Build update query
	query := `
		UPDATE jobs SET
			name = ?, config_json = ?, metadata_json = ?,
			status = ?, progress_json = ?, error = ?,
			result_count = ?, failed_count = ?
		WHERE id = ?
	`

	_, err = m.db.ExecContext(ctx, query,
		modelJob.Name, string(configJSON), string(metadataJSON),
		modelJob.Status, string(progressJSON), modelJob.Error,
		modelJob.ResultCount, modelJob.FailedCount,
		modelJob.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return nil
}

// DeleteJob implements interfaces.JobManager.DeleteJob
func (m *Manager) DeleteJob(ctx context.Context, jobID string) (int, error) {
	// Count children before deletion (CASCADE will delete them)
	var childCount int
	err := m.db.QueryRowContext(ctx, `
		WITH RECURSIVE job_tree AS (
			SELECT id FROM jobs WHERE id = ?
			UNION ALL
			SELECT j.id FROM jobs j
			INNER JOIN job_tree jt ON j.parent_id = jt.id
		)
		SELECT COUNT(*) - 1 FROM job_tree
	`, jobID).Scan(&childCount)

	if err != nil {
		return 0, fmt.Errorf("failed to count child jobs: %w", err)
	}

	// Delete job (CASCADE will delete children and related records)
	result, err := m.db.ExecContext(ctx, "DELETE FROM jobs WHERE id = ?", jobID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return childCount, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return 0, fmt.Errorf("job not found: %s", jobID)
	}

	return childCount, nil
}

// CopyJob implements interfaces.JobManager.CopyJob
func (m *Manager) CopyJob(ctx context.Context, jobID string) (string, error) {
	// Get original job
	var jobType, name, description, configJSON, metadataJSON string
	var parentID sql.NullString

	err := m.db.QueryRowContext(ctx, `
		SELECT parent_id, job_type, name, description, config_json, metadata_json
		FROM jobs WHERE id = ?
	`, jobID).Scan(&parentID, &jobType, &name, &description, &configJSON, &metadataJSON)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("job not found: %s", jobID)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get job: %w", err)
	}

	// Create new job with same configuration
	newJobID := uuid.New().String()
	now := timeToUnix(time.Now())

	// Default progress JSON
	progressJSON := `{"current":0,"total":0,"message":""}`

	_, err = m.db.ExecContext(ctx, `
		INSERT INTO jobs (
			id, parent_id, job_type, name, description,
			config_json, metadata_json,
			status, progress_json,
			created_at, result_count, failed_count
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, 0, 0)
	`, newJobID, parentID, jobType, name+" (Copy)", description,
		configJSON, metadataJSON, progressJSON, now)

	if err != nil {
		return "", fmt.Errorf("failed to create job copy: %w", err)
	}

	return newJobID, nil
}

// GetJobChildStats implements interfaces.JobManager.GetJobChildStats
func (m *Manager) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	if len(parentIDs) == 0 {
		return make(map[string]*interfaces.JobChildStats), nil
	}

	// Build IN clause
	placeholders := make([]string, len(parentIDs))
	args := make([]interface{}, len(parentIDs))
	for i, id := range parentIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT parent_id,
		       COUNT(*) as child_count,
		       SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_children,
		       SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_children,
		       SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) as cancelled_children,
		       SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_children,
		       SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running_children
		FROM jobs
		WHERE parent_id IN (%s)
		GROUP BY parent_id
	`, strings.Join(placeholders, ","))

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query child stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]*interfaces.JobChildStats)
	for rows.Next() {
		var parentID string
		var stat interfaces.JobChildStats

		if err := rows.Scan(&parentID, &stat.ChildCount, &stat.CompletedChildren, &stat.FailedChildren, &stat.CancelledChildren, &stat.PendingChildren, &stat.RunningChildren); err != nil {
			return nil, fmt.Errorf("failed to scan child stats: %w", err)
		}

		stats[parentID] = &stat
	}

	return stats, rows.Err()
}

// StopAllChildJobs implements interfaces.JobManager.StopAllChildJobs
func (m *Manager) StopAllChildJobs(ctx context.Context, parentID string) (int, error) {
	// Update all running/pending child jobs to cancelled
	result, err := m.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'cancelled', completed_at = ?
		WHERE parent_id = ? AND status IN ('running', 'pending')
	`, timeToUnix(time.Now()), parentID)

	if err != nil {
		return 0, fmt.Errorf("failed to stop child jobs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// ============================================================================
// Real-Time Progress Tracking Methods (Task 4.1)
// ============================================================================

// CrawlerProgressStats represents comprehensive progress statistics for crawler jobs
type CrawlerProgressStats struct {
	// Basic job information
	JobID    string `json:"job_id"`
	ParentID string `json:"parent_id,omitempty"`
	Status   string `json:"status"`
	JobType  string `json:"job_type"`

	// Child job statistics
	TotalChildren     int `json:"total_children"`
	CompletedChildren int `json:"completed_children"`
	FailedChildren    int `json:"failed_children"`
	RunningChildren   int `json:"running_children"`
	PendingChildren   int `json:"pending_children"`
	CancelledChildren int `json:"cancelled_children"`

	// Progress calculation
	OverallProgress float64 `json:"overall_progress"` // 0.0 to 1.0
	ProgressText    string  `json:"progress_text"`    // Human-readable progress

	// Link following statistics (crawler-specific)
	LinksFound    int `json:"links_found"`
	LinksFiltered int `json:"links_filtered"`
	LinksFollowed int `json:"links_followed"`
	LinksSkipped  int `json:"links_skipped"`

	// Timing information
	StartedAt    *time.Time `json:"started_at,omitempty"`
	EstimatedEnd *time.Time `json:"estimated_end,omitempty"`
	Duration     *float64   `json:"duration_seconds,omitempty"`

	// Error information
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// GetCrawlerProgressStats retrieves comprehensive progress statistics for a crawler job
// This method calculates parent job progress from child job statistics and includes
// link following metrics for real-time monitoring
func (m *Manager) GetCrawlerProgressStats(ctx context.Context, jobID string) (*CrawlerProgressStats, error) {
	// Get the job details
	job, err := m.GetJobInternal(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	stats := &CrawlerProgressStats{
		JobID:     job.ID,
		Status:    job.Status,
		JobType:   job.Type,
		StartedAt: job.StartedAt,
	}

	if job.ParentID != nil {
		stats.ParentID = *job.ParentID
	}

	// Calculate duration if job has started
	if job.StartedAt != nil {
		var endTime time.Time
		if job.CompletedAt != nil {
			endTime = *job.CompletedAt
		} else {
			endTime = time.Now()
		}
		duration := endTime.Sub(*job.StartedAt).Seconds()
		stats.Duration = &duration
	}

	// Get child job statistics
	childStats, err := m.getChildJobStatistics(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child statistics: %w", err)
	}

	stats.TotalChildren = childStats.TotalChildren
	stats.CompletedChildren = childStats.CompletedChildren
	stats.FailedChildren = childStats.FailedChildren
	stats.RunningChildren = childStats.RunningChildren
	stats.PendingChildren = childStats.PendingChildren
	stats.CancelledChildren = childStats.CancelledChildren

	// Calculate overall progress
	if stats.TotalChildren > 0 {
		terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
		stats.OverallProgress = float64(terminalCount) / float64(stats.TotalChildren)
	} else {
		// No children yet, use parent job progress if available
		if job.ProgressTotal > 0 {
			stats.OverallProgress = float64(job.ProgressCurrent) / float64(job.ProgressTotal)
		}
	}

	// Generate progress text
	stats.ProgressText = m.generateProgressText(stats)

	// Get link following statistics from crawler metadata
	linkStats, err := m.getLinkFollowingStats(ctx, jobID)
	if err == nil {
		stats.LinksFound = linkStats.LinksFound
		stats.LinksFiltered = linkStats.LinksFiltered
		stats.LinksFollowed = linkStats.LinksFollowed
		stats.LinksSkipped = linkStats.LinksSkipped
	}

	// Estimate completion time
	if stats.OverallProgress > 0 && stats.OverallProgress < 1.0 && stats.StartedAt != nil && stats.RunningChildren > 0 {
		elapsed := time.Since(*stats.StartedAt)
		totalEstimated := float64(elapsed) / stats.OverallProgress
		remaining := totalEstimated - float64(elapsed)
		estimatedEnd := time.Now().Add(time.Duration(remaining))
		stats.EstimatedEnd = &estimatedEnd
	}

	// Extract errors and warnings
	if job.Error != nil && *job.Error != "" {
		stats.Errors = []string{*job.Error}
	}

	return stats, nil
}

// childJobStatistics holds detailed child job statistics
type childJobStatistics struct {
	TotalChildren     int
	CompletedChildren int
	FailedChildren    int
	RunningChildren   int
	PendingChildren   int
	CancelledChildren int
}

// getChildJobStatistics retrieves detailed child job statistics
func (m *Manager) getChildJobStatistics(ctx context.Context, parentJobID string) (*childJobStatistics, error) {
	var stats childJobStatistics

	row := m.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) as cancelled
		FROM jobs
		WHERE parent_id = ?
	`, parentJobID)

	if err := row.Scan(
		&stats.TotalChildren,
		&stats.CompletedChildren,
		&stats.FailedChildren,
		&stats.RunningChildren,
		&stats.PendingChildren,
		&stats.CancelledChildren,
	); err != nil {
		return nil, fmt.Errorf("failed to aggregate child statistics: %w", err)
	}

	return &stats, nil
}

// linkFollowingStats holds link following statistics
type linkFollowingStats struct {
	LinksFound    int
	LinksFiltered int
	LinksFollowed int
	LinksSkipped  int
}

// getLinkFollowingStats retrieves link following statistics from crawler metadata
// This aggregates link statistics across all child jobs for a parent crawler job
func (m *Manager) getLinkFollowingStats(ctx context.Context, jobID string) (*linkFollowingStats, error) {
	// For now, return empty stats as this would require parsing crawler metadata
	// In a full implementation, this would query the documents table or job metadata
	// to aggregate link statistics from all child jobs
	return &linkFollowingStats{}, nil
}

// generateProgressText creates human-readable progress text
func (m *Manager) generateProgressText(stats *CrawlerProgressStats) string {
	if stats.TotalChildren == 0 {
		return "No child jobs spawned yet"
	}

	return fmt.Sprintf("%d URLs (%d completed, %d failed, %d running, %d pending)",
		stats.TotalChildren,
		stats.CompletedChildren,
		stats.FailedChildren,
		stats.RunningChildren,
		stats.PendingChildren,
	)
}

// GetJobTreeProgressStats retrieves progress statistics for multiple parent jobs
// This is optimized for bulk operations when displaying multiple jobs in the UI
func (m *Manager) GetJobTreeProgressStats(ctx context.Context, parentJobIDs []string) (map[string]*CrawlerProgressStats, error) {
	if len(parentJobIDs) == 0 {
		return make(map[string]*CrawlerProgressStats), nil
	}

	result := make(map[string]*CrawlerProgressStats)

	// Get all parent jobs in a single query
	placeholders := make([]string, len(parentJobIDs))
	args := make([]interface{}, len(parentJobIDs))
	for i, id := range parentJobIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, parent_id, job_type, status, started_at, completed_at, error,
		       progress_json, created_at
		FROM jobs
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query parent jobs: %w", err)
	}
	defer rows.Close()

	// Process each parent job
	for rows.Next() {
		var jobID, jobType, status string
		var parentID sql.NullString
		var startedAt, completedAt sql.NullInt64
		var errorMsg sql.NullString
		var progressJSON string
		var createdAtUnix int64

		if err := rows.Scan(&jobID, &parentID, &jobType, &status, &startedAt, &completedAt, &errorMsg, &progressJSON, &createdAtUnix); err != nil {
			continue // Skip invalid rows
		}

		stats := &CrawlerProgressStats{
			JobID:   jobID,
			Status:  status,
			JobType: jobType,
		}

		if parentID.Valid {
			stats.ParentID = parentID.String
		}

		if startedAt.Valid {
			t := unixToTime(startedAt.Int64)
			stats.StartedAt = &t
		}

		if errorMsg.Valid && errorMsg.String != "" {
			stats.Errors = []string{errorMsg.String}
		}

		// Get child statistics for this parent
		childStats, err := m.getChildJobStatistics(ctx, jobID)
		if err == nil {
			stats.TotalChildren = childStats.TotalChildren
			stats.CompletedChildren = childStats.CompletedChildren
			stats.FailedChildren = childStats.FailedChildren
			stats.RunningChildren = childStats.RunningChildren
			stats.PendingChildren = childStats.PendingChildren
			stats.CancelledChildren = childStats.CancelledChildren

			// Calculate progress
			if stats.TotalChildren > 0 {
				terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
				stats.OverallProgress = float64(terminalCount) / float64(stats.TotalChildren)
			}

			stats.ProgressText = m.generateProgressText(stats)
		}

		result[jobID] = stats
	}

	return result, rows.Err()
}

// ChildJobStats represents statistics for child jobs of a parent job
type ChildJobStats struct {
	TotalChildren     int `json:"total_children"`
	CompletedChildren int `json:"completed_children"`
	FailedChildren    int `json:"failed_children"`
	CancelledChildren int `json:"cancelled_children"`
	RunningChildren   int `json:"running_children"`
	PendingChildren   int `json:"pending_children"`
}

// GetChildJobStats retrieves child job statistics for a single parent job
// This is used by the ParentJobOrchestrator to monitor child job progress
func (m *Manager) GetChildJobStats(ctx context.Context, parentJobID string) (*ChildJobStats, error) {
	var stats ChildJobStats

	query := `
		SELECT
			COUNT(*) as total_children,
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0) as completed_children,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) as failed_children,
			COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0) as cancelled_children,
			COALESCE(SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END), 0) as running_children,
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending_children
		FROM jobs
		WHERE parent_id = ?
	`

	err := m.db.QueryRowContext(ctx, query, parentJobID).Scan(
		&stats.TotalChildren,
		&stats.CompletedChildren,
		&stats.FailedChildren,
		&stats.CancelledChildren,
		&stats.RunningChildren,
		&stats.PendingChildren,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get child job stats: %w", err)
	}

	return &stats, nil
}

// GetQueue returns the queue manager for enqueueing jobs
func (m *Manager) GetQueue() *queue.Manager {
	return m.queue
}

// GetDocumentCount retrieves the document_count from job metadata
// Returns 0 if document_count is not present in metadata
func (m *Manager) GetDocumentCount(ctx context.Context, jobID string) (int, error) {
	var metadataStr string
	err := m.db.QueryRowContext(ctx, `
		SELECT metadata_json FROM jobs WHERE id = ?
	`, jobID).Scan(&metadataStr)

	if err != nil {
		return 0, fmt.Errorf("failed to query metadata: %w", err)
	}

	// Parse metadata JSON
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		return 0, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Extract document_count (handle both float64 and int types from JSON)
	if val, ok := metadata["document_count"]; ok {
		if floatVal, ok := val.(float64); ok {
			return int(floatVal), nil
		} else if intVal, ok := val.(int); ok {
			return intVal, nil
		}
	}

	// document_count not found in metadata, return 0
	return 0, nil
}
