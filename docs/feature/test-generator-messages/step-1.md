# Step 1: Worker Implementation

## Changes Made

**File:** `internal/queue/workers/test_job_generator_worker.go`

### Task 1: Add [SIMULATED] prefix to all log messages

All log messages from the test job generator now have `[SIMULATED]` prefix to clearly indicate these are fake test messages, not real errors:

- `[SIMULATED] Starting: 50 logs, 10ms delay, 10% failure rate`
- `[SIMULATED] Processing item 1/50`
- `[SIMULATED] Warning at item 5 (simulated resource usage high)`
- `[SIMULATED] Error at item 10 (simulated operation failed)`
- `[SIMULATED] Log summary: INF=40, WRN=7, ERR=3`
- `[SIMULATED] Failure triggered by failure_rate configuration`
- `[SIMULATED] Completed successfully: 0 children spawned`
- `[SIMULATED] Job cancelled`
- `[SIMULATED] Spawned 2 child jobs`

### Task 2: Add job/step identification to log messages

Added context extraction from job metadata (lines 97-107):
```go
jobName := job.Name
if jobName == "" {
    jobName = job.ID[:8]
}
stepName := ""
if job.Metadata != nil {
    if sn, ok := job.Metadata["step_name"].(string); ok {
        stepName = sn
    }
}
```

Built context prefix (lines 117-123):
```go
contextPrefix := "[SIMULATED]"
if stepName != "" {
    contextPrefix = fmt.Sprintf("[SIMULATED] %s/%s:", stepName, jobName)
} else {
    contextPrefix = fmt.Sprintf("[SIMULATED] %s:", jobName)
}
```

### Example Output (Before vs After)

**Before:**
```
[WRN] Warning at item 219: resource usage high
[ERR] Error at item 220: operation failed
```

**After:**
```
[WRN] [SIMULATED] slow_generator/Test Job Generator Worker 1: Warning at item 219 (simulated resource usage high)
[ERR] [SIMULATED] slow_generator/Test Job Generator Worker 1: Error at item 220 (simulated operation failed)
```

## Build Status
**PASS** - Build completed successfully

## Files Modified
- `internal/queue/workers/test_job_generator_worker.go`
