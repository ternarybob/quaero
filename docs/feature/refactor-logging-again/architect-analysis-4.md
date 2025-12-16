# ARCHITECT Analysis - Real-time SSE Log Streaming Issue (Iteration 4)

## Issue Summary

Screenshot shows:
- `high_volume_generator` step: "completed" status but "No logs for this step"
- `slow_generator` step: "pending" status with "No logs for this step"
- Parent job logs ARE appearing (lines 254-257 show child status updates)

## Architecture Analysis

### Two Log Display Paths

1. **Initial Load Path**:
   - `loadJobTreeData()` fetches tree structure
   - `fetchStepLogs()` fetches initial logs via `/api/logs?step=<name>`
   - Logs stored in `jobTreeData[jobId].steps[idx].logs`

2. **Real-time SSE Path**:
   - `connectJobSSE()` connects to `/api/logs/stream?job_id=<manager_id>`
   - SSE sends `logs` events with `{logs: [...], meta: {...}}`
   - `handleSSELogs()` groups by `step_name` and updates `jobTreeData`

### Verified Working Components

1. **SSE Handler Backend** (`sse_logs_handler.go`):
   - `routeJobLogFromLogEvent()` routes logs to manager subscribers ✓
   - `handleJobLogEvent()` routes `EventJobLog` events ✓
   - `sendJobLogBatch()` includes `step_name` field ✓

2. **Worker Metadata** (`test_job_generator_worker.go`):
   - Workers have `step_name`, `step_id`, `manager_id` in metadata ✓
   - `AddJobLog()` resolves hierarchy via `resolveJobHierarchy()` ✓

3. **API Endpoint** (fixed previously):
   - `getStepGroupedLogs()` now uses `includeChildren=true` ✓

### Root Cause Hypothesis

The issue is likely that **SSE logs are not reaching the frontend** because:

1. **EventJobLog vs log_event**: Workers call `AddJobLog()` which publishes `EventJobLog`. The SSE handler subscribes to BOTH `EventJobLog` AND `log_event`. However:
   - `EventJobLog` → `handleJobLogEvent()` - routes to job subscribers
   - `log_event` → `handleServiceLogEvent()` → `routeJobLogFromLogEvent()` - routes job logs

2. **Possible Issue**: The frontend SSE connection subscribes with the **manager job ID**, but `handleJobLogEvent` only routes to subscribers if `matchingJobIDs` includes the manager ID. This requires `manager_id` to be in the event payload.

3. **Frontend Not Receiving Updates**: If SSE events aren't arriving, `handleSSELogs` never gets called, and `step.logs` stays empty.

## Debug Strategy

1. Add console logging to `handleSSELogs` to verify if events are received
2. Check browser console for SSE connection status
3. Verify `manager_id` is present in EventJobLog payload for worker logs

## Proposed Fix

The most likely issue is the **API polling is still being used for log refresh** instead of relying on SSE. The prompt specifically says:
> "Remove obsolete API polling - The queue.html still has many /api/logs fetch calls that should be removed since SSE streaming now works"

The initial API fetch is correct, but if additional polling/refresh calls exist, they may be conflicting with SSE updates or causing state overwrites.

## Files to Modify

1. `pages/queue.html` - Remove API polling for logs after initial load
2. Verify SSE is sending logs in real-time (may need debug logging)

## Anti-Creation Assessment

- **No new files needed** - MODIFY existing code
- Remove obsolete code, don't add new mechanisms
