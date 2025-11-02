// Package types provides JobLogger for correlation-based job logging.
//
// JobLogger Correlation Strategy:
//
// JobLogger wraps arbor.ILogger and adds correlation context for parent-child log aggregation.
// All logs from a job family (parent + children) share the same CorrelationID.
//
// Correlation Rules:
//   - Parent jobs: Use own jobID as CorrelationID
//   - Child jobs: Use parent's jobID as CorrelationID (inherited)
//
// This creates a flat log hierarchy where all logs for a job family can be queried by a single ID.
//
// Example:
//   Parent Job (jobID="parent-123"):
//     - CorrelationID = "parent-123"
//     - Logs: "Job started", "Spawned 5 children", "Job completed"
//
//   Child Job 1 (jobID="child-456", parentID="parent-123"):
//     - CorrelationID = "parent-123" (inherited from parent)
//     - Logs: "Processing URL: https://example.com"
//
//   Child Job 2 (jobID="child-789", parentID="parent-123"):
//     - CorrelationID = "parent-123" (inherited from parent)
//     - Logs: "Processing URL: https://example.com/page2"
//
// Log Flow:
//   1. JobLogger emits log via Arbor with CorrelationID
//   2. Arbor sends log to context channel (configured in app.go)
//   3. LogService consumes log from channel
//   4. LogService extracts jobID from CorrelationID
//   5. LogService dispatches to database (job_logs table) and WebSocket
//
// Querying Aggregated Logs:
//   - Query by parent jobID to get all logs (parent + children)
//   - LogService.GetAggregatedLogs(parentJobID) returns merged logs
//   - UI displays unified log stream for entire job family
//
// Structured Logging Helpers:
//
// JobLogger provides helper methods for consistent job lifecycle logging:
//   - LogJobStart(name, sourceType, config)
//   - LogJobProgress(completed, total, message)
//   - LogJobComplete(duration, resultCount)
//   - LogJobError(err, context)
//   - LogJobCancelled(reason)
//
// These helpers ensure consistent log format across all job types.
// PREFER these helpers over raw Arbor methods (Info(), Warn(), Error(), Debug()).
package types

import (
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
)

// JobLogger wraps arbor.ILogger with correlation context for job logging
type JobLogger struct {
	logger  arbor.ILogger
	jobID   string
	parentID string
}

// NewJobLogger creates a new JobLogger with correlation context.
//
// For child jobs (parentID not empty), uses parent's jobID as CorrelationID for log aggregation.
// For parent jobs (parentID empty), uses own jobID as CorrelationID.
//
// Example:
//   // Parent job
//   parentLogger := NewJobLogger(baseLogger, "parent-123", "")
//   parentLogger.LogJobStart("Crawl Jira Issues", "jira", config)
//   // Logs with CorrelationID="parent-123"
//
//   // Child job
//   childLogger := NewJobLogger(baseLogger, "child-456", "parent-123")
//   childLogger.LogJobStart("Process URL", "jira", config)
//   // Logs with CorrelationID="parent-123" (inherited from parent)
//
// All logs from parent and children can be queried by "parent-123".
func NewJobLogger(baseLogger arbor.ILogger, jobID string, parentID string) *JobLogger {
	correlationID := jobID
	if parentID != "" {
		// Child job: inherit parent's correlation context for log aggregation
		correlationID = parentID
	}

	correlatedLogger := baseLogger.WithCorrelationId(correlationID)

	return &JobLogger{
		logger:  correlatedLogger,
		jobID:   jobID,
		parentID: parentID,
	}
}

// Info returns an ILogEvent for info level logging.
// PREFER: Use LogJobStart(), LogJobProgress(), or LogJobComplete() for job lifecycle events.
// USE: For operational details that don't fit structured helpers (e.g., "Enqueueing child job").
func (jl *JobLogger) Info() arbor.ILogEvent {
	return jl.logger.Info()
}

// Warn returns an ILogEvent for warn level logging.
// PREFER: Use LogJobError() for job failures.
// USE: For non-critical warnings (e.g., "Failed to enqueue child job, continuing").
func (jl *JobLogger) Warn() arbor.ILogEvent {
	return jl.logger.Warn()
}

// Error returns an ILogEvent for error level logging.
// PREFER: Use LogJobError() for job failures with context.
// USE: For errors that don't fail the entire job (e.g., "Failed to update progress").
func (jl *JobLogger) Error() arbor.ILogEvent {
	return jl.logger.Error()
}

// Debug returns an ILogEvent for debug level logging.
// USE: For detailed operational information (e.g., "Depth limit check: 3 > 2").
func (jl *JobLogger) Debug() arbor.ILogEvent {
	return jl.logger.Debug()
}

// LogJobStart logs job initialization with structured fields
func (jl *JobLogger) LogJobStart(name string, sourceType string, config interface{}) {
	jl.Info().
		Str("job_id", jl.jobID).
		Str("name", name).
		Str("source_type", sourceType).
		Str("config", fmt.Sprintf("%+v", config)).
		Msg("Job started")
}

// LogJobProgress logs progress updates with structured fields
func (jl *JobLogger) LogJobProgress(completed int, total int, message string) {
	progressPct := 0.0
	if total > 0 {
		progressPct = float64(completed) / float64(total) * 100
	}

	jl.Info().
		Str("job_id", jl.jobID).
		Int("completed", completed).
		Int("total", total).
		Float64("progress_pct", progressPct).
		Msg(message)
}

// LogJobComplete logs successful job completion
func (jl *JobLogger) LogJobComplete(duration time.Duration, resultCount int) {
	jl.Info().
		Str("job_id", jl.jobID).
		Float64("duration_sec", duration.Seconds()).
		Int("result_count", resultCount).
		Msg("Job completed successfully")
}

// LogJobError logs job failure with error details
func (jl *JobLogger) LogJobError(err error, context string) {
	jl.Error().
		Str("job_id", jl.jobID).
		Str("error", err.Error()).
		Str("context", context).
		Msg("Job failed")
}

// LogJobCancelled logs job cancellation
func (jl *JobLogger) LogJobCancelled(reason string) {
	jl.Warn().
		Str("job_id", jl.jobID).
		Str("reason", reason).
		Msg("Job cancelled")
}

// GetJobID returns the job ID
func (jl *JobLogger) GetJobID() string {
	return jl.jobID
}

// GetParentID returns the parent job ID
func (jl *JobLogger) GetParentID() string {
	return jl.parentID
}
