package logging

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// ContextLogger wraps arbor.ILogger and sends logs to JobManager if context contains job info
type ContextLogger struct {
	logger     arbor.ILogger
	jobManager interfaces.JobStatusManager
}

// NewContextLogger creates a new ContextLogger
func NewContextLogger(logger arbor.ILogger, jobManager interfaces.JobStatusManager) *ContextLogger {
	return &ContextLogger{
		logger:     logger,
		jobManager: jobManager,
	}
}

// Debug logs a debug message
func (l *ContextLogger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	contextStr := fmt.Sprintf("%v", keysAndValues)
	l.logger.Debug().Str("context", contextStr).Msg(msg)
	// We don't typically log debug to job log unless verbose?
	// For now, let's skip debug for job log to reduce noise, or make it configurable.
}

// Info logs an info message
func (l *ContextLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{}) {
	contextStr := fmt.Sprintf("%v", keysAndValues)
	l.logger.Info().Str("context", contextStr).Msg(msg)
	l.logToJob(ctx, "info", msg)
}

// Warn logs a warning message
func (l *ContextLogger) Warn(ctx context.Context, msg string, keysAndValues ...interface{}) {
	contextStr := fmt.Sprintf("%v", keysAndValues)
	l.logger.Warn().Str("context", contextStr).Msg(msg)
	l.logToJob(ctx, "warning", msg)
}

// Error logs an error message
func (l *ContextLogger) Error(ctx context.Context, msg string, keysAndValues ...interface{}) {
	contextStr := fmt.Sprintf("%v", keysAndValues)
	l.logger.Error().Str("context", contextStr).Msg(msg)
	l.logToJob(ctx, "error", msg)
}

// logToJob extracts job context and logs to JobManager
func (l *ContextLogger) logToJob(ctx context.Context, level string, msg string) {
	if l.jobManager == nil {
		return
	}

	// Extract job ID from context?
	// The plan says "AddJobLog should take explicit context or be called by a wrapper that has context".
	// But JobManager.AddJobLog ALREADY takes context and extracts metadata from it?
	// Wait, I simplified resolveJobContext in JobManager to use metadata.
	// But how does context get populated?
	// The context passed to workers usually has job ID?
	// Or does the worker need to pass job ID explicitly?
	// JobManager.AddJobLog takes `jobID` as argument!
	// `AddJobLog(ctx context.Context, jobID string, level string, message string) error`

	// So this wrapper needs to know the jobID.
	// Either it's in the context, or passed as arg.
	// If I want `log.Info(ctx, "msg")`, ctx must contain jobID.
	// I need a key for jobID in context.

	jobID, ok := ctx.Value("job_id").(string)
	if ok && jobID != "" {
		l.jobManager.AddJobLog(ctx, jobID, level, msg)
	}
}
