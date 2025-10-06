package llm

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// AuditLog represents a log entry for LLM operations
type AuditLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Mode      string    `json:"mode"`
	Operation string    `json:"operation"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Duration  int64     `json:"duration_ms"`
	QueryText string    `json:"query_text,omitempty"`
}

// AuditLogger defines the interface for LLM audit logging
type AuditLogger interface {
	LogEmbed(mode interfaces.LLMMode, success bool, duration time.Duration, err error, queryText string) error
	LogChat(mode interfaces.LLMMode, success bool, duration time.Duration, err error, queryText string) error
	GetLogs(limit int) ([]AuditLog, error)
	ExportToJSON(w io.Writer) error
	Close() error
}

// SQLiteAuditLogger implements AuditLogger using SQLite database
type SQLiteAuditLogger struct {
	db         *sql.DB
	logQueries bool
	logger     arbor.ILogger
}

// NewSQLiteAuditLogger creates a new SQLite-based audit logger
func NewSQLiteAuditLogger(db *sql.DB, logQueries bool, logger arbor.ILogger) *SQLiteAuditLogger {
	return &SQLiteAuditLogger{
		db:         db,
		logQueries: logQueries,
		logger:     logger,
	}
}

// LogEmbed logs an embedding operation
func (l *SQLiteAuditLogger) LogEmbed(mode interfaces.LLMMode, success bool, duration time.Duration, err error, queryText string) error {
	return l.logOperation("embed", mode, success, duration, err, queryText)
}

// LogChat logs a chat operation
func (l *SQLiteAuditLogger) LogChat(mode interfaces.LLMMode, success bool, duration time.Duration, err error, queryText string) error {
	return l.logOperation("chat", mode, success, duration, err, queryText)
}

// logOperation handles the common logic for logging operations
func (l *SQLiteAuditLogger) logOperation(operation string, mode interfaces.LLMMode, success bool, duration time.Duration, opErr error, queryText string) error {
	timestamp := time.Now().Format(time.RFC3339)
	modeStr := string(mode)
	durationMs := duration.Milliseconds()

	var errorMsg string
	if opErr != nil {
		errorMsg = opErr.Error()
	}

	var query string
	if l.logQueries {
		query = queryText
	}

	l.logger.Debug().
		Str("operation", operation).
		Str("mode", modeStr).
		Str("success", fmt.Sprintf("%v", success)).
		Int64("duration_ms", durationMs).
		Msg("Logging LLM operation")

	insertSQL := `
		INSERT INTO llm_audit_log (timestamp, mode, operation, success, error, duration, query_text)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := l.db.Exec(insertSQL, timestamp, modeStr, operation, success, errorMsg, durationMs, query)
	if err != nil {
		l.logger.Error().
			Err(err).
			Str("operation", operation).
			Str("mode", modeStr).
			Msg("Failed to insert audit log entry")
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

// GetLogs retrieves recent audit logs with the specified limit
func (l *SQLiteAuditLogger) GetLogs(limit int) ([]AuditLog, error) {
	query := `
		SELECT id, timestamp, mode, operation, success, error, duration, query_text
		FROM llm_audit_log
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := l.db.Query(query, limit)
	if err != nil {
		l.logger.Error().Err(err).Int("limit", limit).Msg("Failed to query audit logs")
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var timestampStr string
		var errorMsg sql.NullString
		var queryText sql.NullString

		err := rows.Scan(
			&log.ID,
			&timestampStr,
			&log.Mode,
			&log.Operation,
			&log.Success,
			&errorMsg,
			&log.Duration,
			&queryText,
		)
		if err != nil {
			l.logger.Error().Err(err).Msg("Failed to scan audit log row")
			return nil, fmt.Errorf("failed to scan audit log row: %w", err)
		}

		log.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			l.logger.Error().Err(err).Str("timestamp", timestampStr).Msg("Failed to parse timestamp")
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}

		if errorMsg.Valid {
			log.Error = errorMsg.String
		}

		if queryText.Valid {
			log.QueryText = queryText.String
		}

		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		l.logger.Error().Err(err).Msg("Error iterating audit log rows")
		return nil, fmt.Errorf("error iterating audit log rows: %w", err)
	}

	l.logger.Debug().Int("count", len(logs)).Int("limit", limit).Msg("Retrieved audit logs")
	return logs, nil
}

// ExportToJSON exports all audit logs to JSON format
func (l *SQLiteAuditLogger) ExportToJSON(w io.Writer) error {
	query := `
		SELECT id, timestamp, mode, operation, success, error, duration, query_text
		FROM llm_audit_log
		ORDER BY timestamp ASC
	`

	rows, err := l.db.Query(query)
	if err != nil {
		l.logger.Error().Err(err).Msg("Failed to query audit logs for export")
		return fmt.Errorf("failed to query audit logs for export: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var timestampStr string
		var errorMsg sql.NullString
		var queryText sql.NullString

		err := rows.Scan(
			&log.ID,
			&timestampStr,
			&log.Mode,
			&log.Operation,
			&log.Success,
			&errorMsg,
			&log.Duration,
			&queryText,
		)
		if err != nil {
			l.logger.Error().Err(err).Msg("Failed to scan audit log row for export")
			return fmt.Errorf("failed to scan audit log row for export: %w", err)
		}

		log.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			l.logger.Error().Err(err).Str("timestamp", timestampStr).Msg("Failed to parse timestamp for export")
			return fmt.Errorf("failed to parse timestamp for export: %w", err)
		}

		if errorMsg.Valid {
			log.Error = errorMsg.String
		}

		if queryText.Valid {
			log.QueryText = queryText.String
		}

		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		l.logger.Error().Err(err).Msg("Error iterating audit log rows for export")
		return fmt.Errorf("error iterating audit log rows for export: %w", err)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(logs); err != nil {
		l.logger.Error().Err(err).Msg("Failed to encode audit logs to JSON")
		return fmt.Errorf("failed to encode audit logs to JSON: %w", err)
	}

	l.logger.Info().Int("count", len(logs)).Msg("Exported audit logs to JSON")
	return nil
}

// Close cleans up resources (no-op for SQLite)
func (l *SQLiteAuditLogger) Close() error {
	return nil
}

// NullAuditLogger is a no-op implementation of AuditLogger used when auditing is disabled
type NullAuditLogger struct{}

// NewNullAuditLogger creates a new null audit logger
func NewNullAuditLogger() *NullAuditLogger {
	return &NullAuditLogger{}
}

// LogEmbed does nothing (no-op)
func (l *NullAuditLogger) LogEmbed(mode interfaces.LLMMode, success bool, duration time.Duration, err error, queryText string) error {
	return nil
}

// LogChat does nothing (no-op)
func (l *NullAuditLogger) LogChat(mode interfaces.LLMMode, success bool, duration time.Duration, err error, queryText string) error {
	return nil
}

// GetLogs returns an empty slice (no-op)
func (l *NullAuditLogger) GetLogs(limit int) ([]AuditLog, error) {
	return []AuditLog{}, nil
}

// ExportToJSON writes empty JSON array (no-op)
func (l *NullAuditLogger) ExportToJSON(w io.Writer) error {
	_, err := w.Write([]byte("[]"))
	return err
}

// Close does nothing (no-op)
func (l *NullAuditLogger) Close() error {
	return nil
}
