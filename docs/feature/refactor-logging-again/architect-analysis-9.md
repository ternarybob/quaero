# ARCHITECT Analysis - Timestamp Format Inconsistency

## Screenshot Analysis
Service Logs panel shows two different timestamp formats:
- `[21:08:27.339]` - With milliseconds (from arbor log consumer)
- `[21:15:58]` - Without milliseconds (from job_manager/runtime)

## Root Cause

**Three different timestamp format usages:**

1. **`internal/logs/consumer.go:243`** (arbor log consumer):
   ```go
   formattedTime := event.Timestamp.Format("15:04:05.000")  // WITH milliseconds
   ```

2. **`internal/queue/job_manager.go:867`** (AddJobLogFull):
   ```go
   Timestamp: now.Format("15:04:05"),  // WITHOUT milliseconds
   ```

3. **`internal/queue/state/runtime.go:201`** (AddJobLog):
   ```go
   Timestamp: now.Format("15:04:05"),  // WITHOUT milliseconds
   ```

## Analysis

The documentation in `docs/z-archive/20251210-step-events-flow/summary.md` states:
> Display format updated from "15:04:05" to "15:04:05.000" with milliseconds

This was done to enable proper ordering for fast jobs. However, only `consumer.go` was updated - the job_manager and runtime still use the old format.

## Recommendation: MODIFY (not CREATE)

**Fix:** Update `job_manager.go:867` and `runtime.go:201` to use `"15:04:05.000"` format with milliseconds, matching `consumer.go`.

This is a simple format string change in 2 locations - no new code needed.

## Files to Modify
1. `internal/queue/job_manager.go` - Line 867: Change `"15:04:05"` to `"15:04:05.000"`
2. `internal/queue/state/runtime.go` - Line 201: Change `"15:04:05"` to `"15:04:05.000"`

## Existing Pattern
Follow `internal/logs/consumer.go:243` which already uses `"15:04:05.000"` format.
