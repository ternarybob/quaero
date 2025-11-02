package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

// CreateParentJob creates a new parent job and enqueues it
func (m *Manager) CreateParentJob(ctx context.Context, jobType string, payload interface{}) (string, error) {
	jobID := uuid.New().String()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Create job record
	_, err = m.db.ExecContext(ctx, `
		INSERT INTO jobs (id, parent_id, job_type, phase, status, created_at, payload)
		VALUES (?, NULL, ?, 'core', 'pending', ?, ?)
	`, jobID, jobType, time.Now(), string(payloadJSON))

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

	// Create job record
	_, err = m.db.ExecContext(ctx, `
		INSERT INTO jobs (id, parent_id, job_type, phase, status, created_at, payload)
		VALUES (?, ?, ?, ?, 'pending', ?, ?)
	`, jobID, parentID, jobType, phase, time.Now(), string(payloadJSON))

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
	var startedAt, completedAt sql.NullTime
	var errMsg, result sql.NullString

	row := m.db.QueryRowContext(ctx, `
		SELECT id, parent_id, job_type, phase, status, created_at, started_at,
		       completed_at, payload, result, error, progress_current, progress_total
		FROM jobs
		WHERE id = ?
	`, jobID)

	if err := row.Scan(
		&job.ID, &parentID, &job.Type, &job.Phase, &job.Status,
		&job.CreatedAt, &startedAt, &completedAt,
		&job.Payload, &result, &errMsg,
		&job.ProgressCurrent, &job.ProgressTotal,
	); err != nil {
		return nil, err
	}

	if parentID.Valid {
		job.ParentID = &parentID.String
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}
	if result.Valid {
		job.Result = result.String
	}
	if errMsg.Valid {
		job.Error = &errMsg.String
	}

	return &job, nil
}

// ListParentJobs returns all parent jobs (parent_id IS NULL)
func (m *Manager) ListParentJobs(ctx context.Context, limit, offset int) ([]Job, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, job_type, phase, status, created_at, started_at, completed_at,
		       progress_current, progress_total, error
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
		var startedAt, completedAt sql.NullTime
		var errorMsg sql.NullString

		if err := rows.Scan(
			&job.ID, &job.Type, &job.Phase, &job.Status,
			&job.CreatedAt, &startedAt, &completedAt,
			&job.ProgressCurrent, &job.ProgressTotal, &errorMsg,
		); err != nil {
			return nil, err
		}

		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}
		if errorMsg.Valid {
			job.Error = &errorMsg.String
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// ListChildJobs returns all child jobs for a parent
func (m *Manager) ListChildJobs(ctx context.Context, parentID string) ([]Job, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, parent_id, job_type, phase, status, created_at, started_at,
		       completed_at, progress_current, progress_total, error
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
		var parentID sql.NullString
		var startedAt, completedAt sql.NullTime
		var errorMsg sql.NullString

		if err := rows.Scan(
			&job.ID, &parentID, &job.Type, &job.Phase, &job.Status,
			&job.CreatedAt, &startedAt, &completedAt,
			&job.ProgressCurrent, &job.ProgressTotal, &errorMsg,
		); err != nil {
			return nil, err
		}

		if parentID.Valid {
			job.ParentID = &parentID.String
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}
		if errorMsg.Valid {
			job.Error = &errorMsg.String
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// UpdateJobStatus updates the job status
func (m *Manager) UpdateJobStatus(ctx context.Context, jobID, status string) error {
	now := time.Now()

	query := "UPDATE jobs SET status = ?"
	args := []interface{}{status}

	if status == "running" {
		query += ", started_at = ?"
		args = append(args, now)
	} else if status == "completed" || status == "failed" || status == "cancelled" {
		query += ", completed_at = ?"
		args = append(args, now)
	}

	query += " WHERE id = ?"
	args = append(args, jobID)

	_, err := m.db.ExecContext(ctx, query, args...)
	return err
}

// UpdateJobProgress updates job progress
func (m *Manager) UpdateJobProgress(ctx context.Context, jobID string, current, total int) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE jobs SET progress_current = ?, progress_total = ?
		WHERE id = ?
	`, current, total, jobID)
	return err
}

// SetJobError sets job error message and marks as failed
func (m *Manager) SetJobError(ctx context.Context, jobID string, errorMsg string) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE jobs SET status = 'failed', error = ?, completed_at = ?
		WHERE id = ?
	`, errorMsg, time.Now(), jobID)
	return err
}

// SetJobResult sets job result data
func (m *Manager) SetJobResult(ctx context.Context, jobID string, result interface{}) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	_, err = m.db.ExecContext(ctx, `
		UPDATE jobs SET result = ? WHERE id = ?
	`, string(resultJSON), jobID)
	return err
}

// AddJobLog adds a log entry for a job
func (m *Manager) AddJobLog(ctx context.Context, jobID, level, message string) error {
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO job_logs (job_id, timestamp, level, message)
		VALUES (?, ?, ?, ?)
	`, jobID, time.Now(), level, message)
	return err
}

// GetJobLogs retrieves logs for a job
func (m *Manager) GetJobLogs(ctx context.Context, jobID string, limit int) ([]JobLog, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, job_id, timestamp, level, message
		FROM job_logs
		WHERE job_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, jobID, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []JobLog
	for rows.Next() {
		var log JobLog
		if err := rows.Scan(&log.ID, &log.JobID, &log.Timestamp, &log.Level, &log.Message); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
}
