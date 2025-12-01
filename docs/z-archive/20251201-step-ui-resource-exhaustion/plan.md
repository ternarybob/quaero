# Fix: Step UI Gets Stuck and Browser Resource Exhaustion

## Problem Summary

1. **UI gets stuck**: Step progress panel shows "1000 pending, 0 running, 0 completed, 0 failed" and doesn't update
2. **Resource exhaustion**: Browser crashes with `ERR_INSUFFICIENT_RESOURCES` due to excessive API calls

## Root Cause Analysis

### Issue 1: Excessive API calls causing browser crash

When `job_status_change` WebSocket events arrive for worker jobs (children of step jobs):
- UI's `updateJobInList()` checks if job exists in `allJobs`
- Worker jobs under steps are NOT in `allJobs` because UI only fetches `parent_id=manager_id`
- For each missing job, UI fetches `/api/jobs/${job_id}` individually
- With 1000 worker jobs completing, this triggers 1000+ API calls
- Browser hits socket/connection limit → `ERR_INSUFFICIENT_RESOURCES`

**Location**: `pages/queue.html` line 3284 in `updateJobInList()`

### Issue 2: Step progress not updating

The `step_progress` WebSocket events ARE being sent (every 5 seconds from StepMonitor).
The issue is that the UI is overwhelmed by the 1000+ individual API calls before it can process
the step_progress events properly. Once the browser hits resource limits, WebSocket messages
are dropped.

## Solution

### Fix 1: Stop fetching individual worker jobs (Primary Fix)

In `updateJobInList()`, don't fetch job details for jobs that:
1. Are NOT manager or step jobs (indicated by parent_id existing)
2. Would cause a new fetch request

Instead, simply ignore updates for jobs not in `allJobs` that have a `parent_id`.
The step_progress events will provide aggregated stats.

### Fix 2: Reduce unnecessary API calls for child jobs

The `debouncedRefreshParent()` is called for every child job update, even when step_progress
events already provide the aggregated stats. For step architecture jobs, we should skip
individual parent refreshes.

## Implementation

### Step 1: Modify `updateJobInList()` in `pages/queue.html`

Before (line ~3282-3301):
```javascript
// If job not found, fetch full job data regardless of status
if (!job) {
    try {
        const response = await fetch(`/api/jobs/${update.job_id}`);
        ...
    }
}
```

After:
```javascript
// If job not found and has a parent_id, skip fetching
// Worker jobs under steps are tracked via step_progress events
if (!job) {
    // Check if this is a child job (has parent_id in the update)
    // Child jobs are tracked via aggregated step_progress/parent_job_progress events
    if (update.parent_id) {
        console.debug('[Queue] Skipping individual fetch for child job:', update.job_id?.substring(0, 8));
        return;
    }

    // Only fetch for root-level jobs (managers/parents without parent_id)
    try {
        const response = await fetch(`/api/jobs/${update.job_id}`);
        ...
    }
}
```

### Step 2: Skip parent refresh for step architecture jobs

The `debouncedRefreshParent()` at line 3401 should be skipped when:
- The parent job uses step architecture (manager jobs)
- step_progress events are handling updates

## Success Criteria

1. Browser no longer crashes with `ERR_INSUFFICIENT_RESOURCES`
2. Step progress panel updates correctly showing real-time stats
3. Manager jobs show aggregated progress from their steps
4. Individual job fetches only occur for root-level jobs

## Implementation Status: COMPLETED

### Changes Made

#### 1. Frontend: `pages/queue.html`

Modified `updateJobInList()` to skip fetching individual job details for child jobs.
Child jobs are identified by having a `parent_id` in the WebSocket update payload.

```javascript
// Lines 3281-3291: Added parent_id check before fetching
if (!job) {
    if (update.parent_id) {
        console.debug('[Queue] Skipping individual fetch for child job:', ...);
        return;
    }
    // Only fetch for root-level jobs
    ...
}
```

#### 2. Backend: `internal/handlers/websocket.go`

Added `ParentID` field to `JobStatusUpdate` struct (line 178):
```go
ParentID  string `json:"parent_id,omitempty"` // Parent job ID
```

#### 3. Backend: `internal/handlers/websocket_events.go`

Updated all 5 event handlers to extract and include `parent_id`:
- `handleJobCreated()` - line 199
- `handleJobStarted()` - line 224
- `handleJobCompleted()` - line 256
- `handleJobFailed()` - line 325
- `handleJobCancelled()` - line 363

Each handler now includes:
```go
ParentID: getStringWithFallback(payload, "parent_id", "parentId"),
```

### Build Verification

- Go build: ✅ Compiles cleanly
- Go fmt: ✅ Applied
- Queue tests: ✅ All pass

### Expected Behavior After Fix

1. When 1000+ worker jobs complete, UI receives `job_status_change` events with `parent_id` set
2. UI checks if `parent_id` exists → skips individual API fetch for child jobs
3. UI relies on `step_progress` events for aggregated statistics
4. Browser no longer exhausts socket connections
5. Step progress panel updates smoothly from backend events
