# WORKER Step 2: Fix SSE Log Event Payload Handling

## Problem

SSE logs were not displaying in real-time because:
1. `handleJobLogEvent` used `fmt.Sprintf("%v", payload["field"])` which converts `nil` to string `"<nil>"`
2. This caused `step_name` to be `"<nil>"` instead of empty string when not present
3. Frontend grouping by `step_name` couldn't match tree steps

## Fix Applied

Modified `internal/handlers/sse_logs_handler.go` line 242-267:

**Before:**
```go
entry := jobLogEntry{
    Timestamp:  fmt.Sprintf("%v", payload["timestamp"]),
    Level:      fmt.Sprintf("%v", payload["level"]),
    Message:    fmt.Sprintf("%v", payload["message"]),
    JobID:      jobID,
    StepName:   fmt.Sprintf("%v", payload["step_name"]),
    StepID:     stepID,
    LineNumber: lineNumber,
}
```

**After:**
```go
// Extract string fields with proper nil handling
timestamp, _ := payload["timestamp"].(string)
level, _ := payload["level"].(string)
message, _ := payload["message"].(string)

entry := jobLogEntry{
    Timestamp:  timestamp,
    Level:      level,
    Message:    message,
    JobID:      jobID,
    StepName:   stepName,
    StepID:     stepID,
    LineNumber: lineNumber,
}
```

## Debug Logging Added

Added console.log statements to `pages/queue.html` in `handleSSELogs`:
- Logs received event with count
- Logs each log entry's step_name
- Logs step grouping results
- Logs merge operation results

These will help diagnose if logs are:
1. Being received by SSE
2. Having correct step_name
3. Being matched to tree steps
4. Being merged into tree data

## Files Modified

- `internal/handlers/sse_logs_handler.go` - Fixed payload string extraction
- `pages/queue.html` - Added debug logging

## Testing

User should:
1. Restart the server
2. Create a Test Job Generator job
3. Open browser console
4. Look for `[Queue] SSE logs received` and related messages
5. Verify logs appear in step panels
