# ARCHITECT Analysis - SSE Log Streaming Issue (Updated)

## Issue Summary

Screenshots show:
1. Steps auto-expand correctly showing status (e.g., `high_volume_generator` as "completed")
2. BUT logs show "No logs for this step" even though step is completed
3. Network tab shows many `/api/logs` polling calls during execution

## Root Cause Analysis

### Log Event Flow Traced

The system has TWO log publication paths:

**Path 1: AddJobLog → EventJobLog → SSE handleJobLogEvent**
- Worker calls `w.jobMgr.AddJobLog(ctx, job.ID, level, message)`
- `AddJobLogFull` resolves hierarchy via `resolveJobHierarchy()`
- Gets `managerID`, `stepID`, `parentID` from job metadata
- Publishes `EventJobLog` with all IDs in payload
- SSE handler receives and routes to subscribers matching `jobID`, `managerID`, or `parentID`

**Path 2: arbor Logger → log_event → SSE handleServiceLogEvent → routeJobLogFromLogEvent**
- Worker uses `jobLogger.Debug().Msg(...)` with arbor logger
- arbor publishes to `log_event` via consumer.go
- Fields extracted from arbor context into payload
- SSE handler routes to job subscribers if `job_id` present

### Verified Working Components

1. **EventJobLog routing** (sse_logs_handler.go:231-299): Correctly builds `matchingJobIDs` and routes to manager subscribers
2. **Job metadata** (test_job_generator_worker.go:387-391): Sets `step_name`, `step_id`, `manager_id` on workers
3. **Hierarchy resolution** (job_manager.go:955-1012): Correctly resolves full hierarchy from metadata

### Probable Root Cause

The issue appears to be in the **frontend handling**, not backend routing.

Looking at `handleSSELogs` (queue.html:4829-4881):
```javascript
handleSSELogs(jobId, data) {
    if (!data.logs || !Array.isArray(data.logs) || data.logs.length === 0) {
        return;  // <-- Early return if no logs
    }
    // ... groups by step_name and updates tree
}
```

If the SSE event has `logs: []` (empty array), nothing happens. But the **initial logs** come from the API, not SSE.

The real issue: **The API endpoint `/api/logs` with `step` parameter may not be returning logs correctly**.

Looking at `fetchStepLogs` (queue.html:4667-4668):
```javascript
const response = await fetch(`/api/logs?scope=job&job_id=${stepJobId}&step=${encodeURIComponent(stepName)}&limit=${limit}&level=...`);
```

It fetches using `stepJobId` (the step job ID), NOT the manager ID. And it filters by `step=${stepName}`.

**Hypothesis**: The API endpoint may be querying logs for step job only (not including worker logs from children).

## Action Plan

1. **Test API directly** - Call `/api/logs?scope=job&job_id=<step_id>&step=<step_name>` and verify it returns worker logs
2. **If API returns empty** - The issue is in log storage/query (not SSE)
3. **If API returns logs** - The issue is in frontend state management (SSE vs API conflict)

## Files to Modify

Based on analysis, likely changes needed in:
- `internal/logs/service.go` - If API query doesn't include child job logs
- OR `pages/queue.html` - If frontend isn't handling the API response correctly

## Anti-Creation Assessment

- **No new files needed** - MODIFY existing code
- **Pattern exists** - SSE handler and API endpoint already present
- **Focus**: Debug why logs aren't appearing, not create new infrastructure
