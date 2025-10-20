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
//   - Uses "15:04:05" format (HH:MM:SS, 24-hour clock)
//   - Consistent across all log entries for easy visual scanning
//   - Example: "14:23:45" for 2:23:45 PM
//
// Log Levels:
//   - "info":  Normal operational events (job started, progress milestones, completion)
//   - "warn":  Non-critical issues (missing auth, all links filtered, job cancelled)
//   - "error": Failures requiring attention (request failures, scraping errors, job failures)
//   - "debug": Detailed diagnostic information (link discovery, configuration)
//
// Usage Example:
//   logEntry := models.JobLogEntry{
//       Timestamp: time.Now().Format("15:04:05"),
//       Level:     "info",
//       Message:   "Job started: source=jira/issues, seeds=5, max_depth=3",
//   }
//   if err := jobStorage.AppendJobLog(ctx, jobID, logEntry); err != nil {
//       logger.Warn().Err(err).Msg("Failed to append log")
//   }
type JobLogEntry struct {
	Timestamp string `json:"timestamp"` // HH:MM:SS format (24-hour clock), e.g., "14:23:45"
	Level     string `json:"level"`     // Log level: "info", "warn", "error", "debug"
	Message   string `json:"message"`   // Human-readable log message with structured data
}
