# Summary: Fix SSE "Job Not Found" Error Log Level

## Issue
Log file showed ERROR level for expected "job not found" condition:
```
level=ERR message="Failed to get initial job logs" error="job not found..."
```

## Root Cause
When browser connects to SSE with a stale job ID (from localStorage after server restart with database reset), `GetAggregatedLogs` returns "job not found". This is expected behavior, not an error.

## Fix Applied
Changed log level from `Error` to `Debug` in two locations:

**`internal/handlers/sse_logs_handler.go`:**
```go
// Line 769-770 (step-specific logs)
h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Failed to get initial job logs (job may not exist)")

// Line 792-793 (job logs with children)
h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Failed to get initial job logs (job may not exist)")
```

## Rationale
- "Job not found" after server restart is expected behavior
- Debug level is appropriate for expected conditions
- Error level should be reserved for unexpected failures
- Follows existing codebase pattern for "not found" handling

## Build Status
✅ Build passes
