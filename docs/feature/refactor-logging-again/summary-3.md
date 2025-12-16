# Summary: Fix Real-Time SSE Log Streaming (Iteration 3)

## Issue

Logs were not appearing in step panels while job was running:
- Steps showed status updates correctly (e.g., "completed", "pending")
- But logs showed "No logs for this step" during execution
- Network tab showed many `/api/logs` polling calls

## Root Cause Analysis

### Finding 1: Payload String Extraction Bug

In `sse_logs_handler.go`, the `handleJobLogEvent` function used:
```go
StepName: fmt.Sprintf("%v", payload["step_name"])
```

This converts `nil` to the literal string `"<nil>"` instead of empty string. When frontend groups logs by `step_name`, logs with `"<nil>"` don't match any tree step.

### Finding 2: API Not Including Child Logs (Fixed Previously)

The `getStepGroupedLogs` function was using `includeChildren=false`, which excluded worker job logs from step log queries.

## Fixes Applied

### 1. SSE Payload Extraction (`internal/handlers/sse_logs_handler.go`)

**Before:**
```go
entry := jobLogEntry{
    Timestamp:  fmt.Sprintf("%v", payload["timestamp"]),
    Level:      fmt.Sprintf("%v", payload["level"]),
    Message:    fmt.Sprintf("%v", payload["message"]),
    StepName:   fmt.Sprintf("%v", payload["step_name"]),
    ...
}
```

**After:**
```go
timestamp, _ := payload["timestamp"].(string)
level, _ := payload["level"].(string)
message, _ := payload["message"].(string)
stepName, _ := payload["step_name"].(string)

entry := jobLogEntry{
    Timestamp:  timestamp,
    Level:      level,
    Message:    message,
    StepName:   stepName,
    ...
}
```

### 2. Debug Logging (`pages/queue.html`)

Added console.log statements in `handleSSELogs`:
- When SSE logs are received
- Each log entry's step_name
- Step grouping results
- Merge operation results

### 3. New Test File (`test/ui/job_test_generator_streaming_test.go`)

Created tests for:
- `TestTestJobGeneratorSSEStreaming`: Verifies logs appear while job is running
- `TestTestJobGeneratorStepAutoExpand`: Verifies steps auto-expand

## Files Modified

1. `internal/handlers/sse_logs_handler.go` - Fixed string extraction (lines 242-267)
2. `internal/handlers/unified_logs_handler.go` - Fixed includeChildren (previous iteration)
3. `pages/queue.html` - Added debug logging (lines 4830, 4839, 4847, 4868)

## Files Created

1. `test/ui/job_test_generator_streaming_test.go` - SSE streaming tests

## Testing

To verify the fix:
1. Restart the server
2. Create a Test Job Generator job
3. Open browser console (F12)
4. Watch for `[Queue] SSE logs received` messages
5. Verify logs appear in step panels in real-time

## Build Status

✅ All packages compile successfully
