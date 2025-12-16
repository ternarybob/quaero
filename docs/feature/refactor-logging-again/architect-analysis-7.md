# ARCHITECT Analysis - SSE Job Not Found Error

## Error from Log
```
time=20:58:12 level=ERR message="Failed to get initial job logs"
function=(*SSELogsHandler).sendInitialJobLogs
job_id=1fedcc3d-d6fc-4621-bec0-a3f06463726c
error="job not found: job not found: 1fedcc3d-d6fc-4621-bec0-a3f06463726c"
```

## Root Cause
1. Browser connects to SSE with a stale `job_id` from localStorage/sessionStorage
2. Server was restarted with `reset_on_startup=true` (line 7 of log)
3. Database was wiped - job no longer exists
4. `GetAggregatedLogs` returns "job not found" error
5. Error is logged, but this is expected behavior for stale job IDs

## Analysis
This is NOT a bug - it's expected behavior when:
1. User visits queue page with cached job IDs from previous session
2. Server was restarted with database reset
3. SSE tries to get logs for non-existent job

## Recommendation: MODIFY (not CREATE)
The error should be handled gracefully:
1. Change log level from `Error` to `Debug` for "job not found" errors
2. This is a normal case, not an error condition
3. The SSE stream should continue (which it does via `return`)

## Location
`internal/handlers/sse_logs_handler.go` lines 768-770 and 790-792

## Existing Pattern
Look at how other handlers deal with "not found" errors - they typically:
- Return 404 for HTTP endpoints
- Log at Debug level for expected "not found" cases
- Only log at Error for unexpected failures
