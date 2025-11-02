package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// JobLogStorage handles job log persistence in SQLite
type JobLogStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
}

// NewJobLogStorage creates a new job log storage
func NewJobLogStorage(db *SQLiteDB, logger arbor.ILogger) *JobLogStorage {
	return &JobLogStorage{
		db:     db,
		logger: logger,
	}
}

// AppendLog appends a single log entry to a job
func (s *JobLogStorage) AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry) error {
	query := `
		INSERT INTO job_logs (job_id, timestamp, level, message, created_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.db.ExecContext(ctx, query,
		jobID,
		entry.Timestamp,
		entry.Level,
		entry.Message,
		time.Now().Unix(),
	)

	if err != nil {
		return fmt.Errorf("failed to append log: %w", err)
	}

	return nil
}

// AppendLogs appends multiple log entries to a job (batch operation)
func (s *JobLogStorage) AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Begin transaction
	tx, err := s.db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO job_logs (job_id, timestamp, level, message, created_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert all entries
	now := time.Now().Unix()
	for _, entry := range entries {
		_, err := stmt.ExecContext(ctx,
			jobID,
			entry.Timestamp,
			entry.Level,
			entry.Message,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert log entry: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetLogs retrieves log entries for a job with optional limit
// ORDERING: Returns logs in newest-first order (ORDER BY created_at DESC).
// This matches typical web UI expectations where recent logs appear at the top.
// For chronological (oldest-first) display, reverse the slice after retrieval.
func (s *JobLogStorage) GetLogs(ctx context.Context, jobID string, limit int) ([]models.JobLogEntry, error) {
	query := `
		SELECT timestamp, level, message
		FROM job_logs
		WHERE job_id = ?
		ORDER BY created_at DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []models.JobLogEntry
	for rows.Next() {
		var log models.JobLogEntry
		if err := rows.Scan(&log.Timestamp, &log.Level, &log.Message); err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating logs: %w", err)
	}

	return logs, nil
}

// GetLogsByLevel retrieves log entries filtered by level
// ORDERING: Returns logs in newest-first order (ORDER BY created_at DESC).
// This matches the behavior of GetLogs() for consistency across the API.
func (s *JobLogStorage) GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.JobLogEntry, error) {
	query := `
		SELECT timestamp, level, message
		FROM job_logs
		WHERE job_id = ? AND level = ?
		ORDER BY created_at DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.db.QueryContext(ctx, query, jobID, level)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs by level: %w", err)
	}
	defer rows.Close()

	var logs []models.JobLogEntry
	for rows.Next() {
		var log models.JobLogEntry
		if err := rows.Scan(&log.Timestamp, &log.Level, &log.Message); err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating logs: %w", err)
	}

	return logs, nil
}

// DeleteLogs deletes all log entries for a job
func (s *JobLogStorage) DeleteLogs(ctx context.Context, jobID string) error {
	query := `DELETE FROM job_logs WHERE job_id = ?`

	result, err := s.db.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	s.logger.Debug().
		Str("job_id", jobID).
		Int64("rows_affected", rowsAffected).
		Msg("Deleted job logs")

	return nil
}

// CountLogs returns the number of log entries for a job
func (s *JobLogStorage) CountLogs(ctx context.Context, jobID string) (int, error) {
	query := `SELECT COUNT(*) FROM job_logs WHERE job_id = ?`

	var count int
	err := s.db.db.QueryRowContext(ctx, query, jobID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count logs: %w", err)
	}

	return count, nil
}

// GetLogsWithOffset retrieves log entries for a job with offset-based pagination
// ORDERING: Returns logs in newest-first order (ORDER BY created_at DESC).
// offset is the number of most recent logs to skip (e.g., offset=10 skips the 10 most recent logs)
func (s *JobLogStorage) GetLogsWithOffset(ctx context.Context, jobID string, limit int, offset int) ([]models.JobLogEntry, error) {
	query := `
		SELECT timestamp, level, message
		FROM job_logs
		WHERE job_id = ?
		ORDER BY created_at DESC
	`

	clauses := []string{}
	if limit > 0 {
		clauses = append(clauses, fmt.Sprintf("LIMIT %d", limit))
	}
	if offset > 0 {
		clauses = append(clauses, fmt.Sprintf("OFFSET %d", offset))
	}
	if len(clauses) > 0 {
		query += " " + strings.Join(clauses, " ")
	}

	rows, err := s.db.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs with offset: %w", err)
	}
	defer rows.Close()

	var logs []models.JobLogEntry
	for rows.Next() {
		var log models.JobLogEntry
		if err := rows.Scan(&log.Timestamp, &log.Level, &log.Message); err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating logs: %w", err)
	}

	return logs, nil
}

// GetLogsByLevelWithOffset retrieves log entries filtered by level with offset-based pagination
// ORDERING: Returns logs in newest-first order (ORDER BY created_at DESC).
// offset is the number of most recent logs to skip
func (s *JobLogStorage) GetLogsByLevelWithOffset(ctx context.Context, jobID string, level string, limit int, offset int) ([]models.JobLogEntry, error) {
	query := `
		SELECT timestamp, level, message
		FROM job_logs
		WHERE job_id = ? AND level = ?
		ORDER BY created_at DESC
	`

	clauses := []string{}
	if limit > 0 {
		clauses = append(clauses, fmt.Sprintf("LIMIT %d", limit))
	}
	if offset > 0 {
		clauses = append(clauses, fmt.Sprintf("OFFSET %d", offset))
	}
	if len(clauses) > 0 {
		query += " " + strings.Join(clauses, " ")
	}

	rows, err := s.db.db.QueryContext(ctx, query, jobID, level)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs by level with offset: %w", err)
	}
	defer rows.Close()

	var logs []models.JobLogEntry
	for rows.Next() {
		var log models.JobLogEntry
		if err := rows.Scan(&log.Timestamp, &log.Level, &log.Message); err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating logs: %w", err)
	}

	return logs, nil
}
