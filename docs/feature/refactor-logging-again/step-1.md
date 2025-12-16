# WORKER Step 1: Fix Step Logs Not Including Worker Children

## Problem

The `getStepGroupedLogs` function in `unified_logs_handler.go` was hardcoded to use `includeChildren=false`, which prevented worker logs from appearing under step logs.

When `test_job_generator` runs:
1. Manager job creates step jobs (e.g., `fast_generator`, `high_volume_generator`)
2. Step jobs spawn worker jobs that do the actual work
3. Worker jobs write logs under their own job IDs
4. UI queries step logs via `/api/logs?scope=job&job_id=<step_id>&step=<step_name>`
5. **Bug**: The query only returned logs from the step job, not from worker children

## Root Cause

In `internal/handlers/unified_logs_handler.go`:

```go
// Line 530 (before fix):
logEntries, _, _, err := h.logService.GetAggregatedLogs(ctx, jobID, false, level, limit, "", order)
```

The `false` argument hardcoded `includeChildren=false`, overriding the query parameter default.

## Fix Applied

1. **Pass `includeChildren` parameter through to `getStepGroupedLogs`**:
   - Changed function signature to accept `includeChildren bool`
   - Call site now passes the parsed query parameter
   - Default is `true` (line 248), so child logs are included unless explicitly disabled

2. **Updated count queries** to also respect `includeChildren`:
   - `CountAggregatedLogs` calls now use `includeChildren` instead of hardcoded `false`

## Files Modified

- `internal/handlers/unified_logs_handler.go`:
  - Line 261: Pass `includeChildren` to `getStepGroupedLogs`
  - Lines 521-530: Updated function signature and log query
  - Lines 541-553: Updated count queries

## Testing

1. Start the server
2. Create a Test Job Generator job
3. Expand a step (e.g., `high_volume_generator`)
4. Verify logs now appear under the step

## Performance Note

The `includeChildren=true` default may be slower for steps with hundreds of workers due to the k-way merge algorithm. If performance issues arise for specific job types, the frontend can explicitly pass `include_children=false`.
