package models

// JobLogEntry represents a single log entry for a crawler job.
//
// Purpose:
//   - Provides database-persisted logging for crawler job lifecycle events
//   - Enables post-mortem analysis and troubleshooting via job logs UI
//   - Complements console logging with persistent, queryable logs
//
// Truncation Behavior:
//   - Job logs are automatically limited to the most recent 100 entries
//   - When AppendJobLog is called on a job with 100+ entries, the oldest entry is removed
//   - This prevents unbounded growth of logs in the database
//   - Truncation is transparent and automatic (no manual cleanup required)
//
// Timestamp Format:
//   - Uses "15:04:05.000" format (HH:MM:SS.mmm, 24-hour clock) for display with milliseconds
//   - Includes FullTimestamp (RFC3339Nano) for accurate chronological sorting with nanosecond precision
//   - Milliseconds are critical for fast jobs that complete in under 1 second
//   - Example: "14:23:45.123" for display, "2025-11-01T14:23:45.123456789Z" for sorting
//
// Log Levels:
//   - "info":  Normal operational events (job started, progress milestones, completion)
//   - "warn":  Non-critical issues (missing auth, all links filtered, job cancelled)
//   - "error": Failures requiring attention (request failures, scraping errors, job failures)
//   - "debug": Detailed diagnostic information (link discovery, configuration)
//
// Usage Example:
//
//	logEntry := models.JobLogEntry{
//	    Timestamp:      time.Now().Format("15:04:05.000"),
//	    FullTimestamp:  time.Now().Format(time.RFC3339Nano),
//	    Level:     "info",
//	    Message:   "Job started: source=jira/issues, seeds=5, max_depth=3",
//	}
//	if err := jobStorage.AppendJobLog(ctx, jobID, logEntry); err != nil {
//	    logger.Warn().Err(err).Msg("Failed to append log")
//	}
type JobLogEntry struct {
	Timestamp       string `json:"timestamp"`             // HH:MM:SS.mmm format (24-hour clock), e.g., "14:23:45.123"
	FullTimestamp   string `json:"full_timestamp"`        // RFC3339Nano format for accurate sorting, e.g., "2025-11-01T14:23:45.123456789Z"
	Level           string `json:"level"`                 // Log level: "info", "warn", "error", "debug"
	Message         string `json:"message"`               // Human-readable log message with structured data
	AssociatedJobID string `json:"job_id"`                // ID of the job that generated this log (populated during aggregation)
	StepName        string `json:"step_name,omitempty"`   // Name of the step that generated this log (for multi-step jobs)
	SourceType      string `json:"source_type,omitempty"` // Type of worker (agent, places_search, web_search, etc.)
	Originator      string `json:"originator,omitempty"`  // Log originator: "manager", "step", "worker" - identifies the architectural layer
	Phase           string `json:"phase,omitempty"`       // Execution phase: "init", "run", "orchestrator" - identifies the execution context
}
