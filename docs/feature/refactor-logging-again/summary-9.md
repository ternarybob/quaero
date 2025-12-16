# Summary: Fix Timestamp Format Inconsistency

## Issue
Service Logs showed two different timestamp formats:
- `[21:08:27.339]` - With milliseconds (arbor logs via consumer.go)
- `[21:15:58]` - Without milliseconds (job logs via job_manager.go/runtime.go)

## Root Cause
Three files format timestamps for log entries, but only one was updated to include milliseconds:
- `consumer.go` used `"15:04:05.000"` (with ms)
- `job_manager.go` used `"15:04:05"` (without ms)
- `runtime.go` used `"15:04:05"` (without ms)

## Fix Applied
Updated two files to use consistent `"15:04:05.000"` format:

1. **`internal/queue/job_manager.go:867`**
2. **`internal/queue/state/runtime.go:201`**

## Result
All log timestamps now display with consistent millisecond precision:
- `[21:08:27.339]` format throughout the UI
- Enables proper ordering for fast jobs completing under 1 second

## Build Status
✅ Build passes
