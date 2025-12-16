# Summary: Step Logs Not Displaying Worker Child Logs

## Problem

Steps in the queue UI showed "No logs for this step" even after completion:
1. Steps auto-expanded correctly with status updates
2. But logs array was empty during execution
3. Network tab showed `/api/logs` API calls but no data returned
4. Only after job completion did some logs appear

## Investigation

### Previous Fix (Already Applied)
The SSE routing fix from the previous iteration was verified to be in place:
- `handleServiceLogEvent` at lines 135-138 correctly routes job logs (with `job_id`) to `routeJobLogFromLogEvent`
- This handles real-time streaming of new log events

### Root Cause Found
The API endpoint `/api/logs?scope=job&step=<name>` (used for initial log fetch and refresh) had a bug:

In `internal/handlers/unified_logs_handler.go`, the `getStepGroupedLogs` function was hardcoded:
```go
logEntries, _, _, err := h.logService.GetAggregatedLogs(ctx, jobID, false, ...)
//                                                               ^^^^^^
//                                       includeChildren=false hardcoded!
```

For `test_job_generator`:
- Step jobs (e.g., `high_volume_generator`) spawn worker jobs
- Worker jobs write logs under their own job IDs
- With `includeChildren=false`, only the step job's own logs were returned (often empty)
- Worker logs were excluded, causing "No logs for this step"

## Fix Applied

Modified `internal/handlers/unified_logs_handler.go`:

1. **Line 261**: Pass `includeChildren` parameter to `getStepGroupedLogs`
2. **Lines 521-530**: Updated function signature to accept `includeChildren bool`
3. **Lines 541-553**: Updated count queries to use `includeChildren`

The default `includeChildren=true` (line 248) now applies, including worker logs.

## Testing

1. Build passes ✅
2. User should:
   - Restart the service
   - Create a Test Job Generator job
   - Verify logs appear in real-time as steps execute
   - Verify step logs include worker child logs

## Architecture Note

The system has two log paths:
1. **SSE streaming** (`/api/logs/stream`): Real-time updates via EventSource
2. **API fetch** (`/api/logs`): Initial state and manual refresh

Both paths now correctly include child logs when querying step-level logs.
