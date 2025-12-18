# ARCHITECT Analysis: Step Status Not Updating to "Running"

## Problem Statement

The step status badge in the UI tree view shows "pending" even when logs show the step is running ("Starting Step 3/5", "Initializing worker...", etc.). The status should update from "pending" to "running" in real-time.

## Root Cause Analysis

### Two Separate Update Paths

1. **`job_step_progress` events** (from `EventJobProgress`):
   - Published by orchestrator when step starts running (line 237)
   - Handled by `updateJobProgress()` in queue.html
   - Updates: `job.status_report.step_status`, `job.metadata.current_step_status`
   - **MISSING**: Does NOT update `jobTreeData[job_id].steps[stepIdx].status`

2. **`job_update` events** (from `EventJobUpdate`):
   - Published by orchestrator only when step **completes** (line 682)
   - Handled by `handleJobUpdate()` in queue.html
   - **Does** update `jobTreeData[job_id].steps[stepIdx].status` (line 4526)

### The Gap

When a step starts running:
- Orchestrator publishes `EventJobProgress` with `step_status: "running"` ✓
- WebSocket broadcasts as `job_step_progress` ✓
- `updateJobProgress()` updates `job.metadata.current_step_status` ✓
- **BUT** tree view data `jobTreeData[*].steps[*].status` is NOT updated ✗

The tree view badge renders from `step.status` (queue.html line 675), which comes from `jobTreeData`, not from `job.metadata`.

## Solution Options

### Option A: Backend Fix (Recommended)

Add `EventJobUpdate` publication when step starts running in orchestrator.go.

**Location**: `internal/queue/orchestrator.go` after line 244

```go
// Also publish job_update event for direct UI status sync (step starting)
jobUpdatePayload := map[string]interface{}{
    "context":   "job_step",
    "job_id":    managerID,
    "step_name": step.Name,
    "status":    "running",
    "timestamp": time.Now().Format(time.RFC3339),
}
jobUpdateEvent := interfaces.Event{
    Type:    interfaces.EventJobUpdate,
    Payload: jobUpdatePayload,
}
go func() {
    if err := o.eventService.Publish(ctx, jobUpdateEvent); err != nil {
        // Log but don't fail
    }
}()
```

**Pros**:
- Follows existing pattern (same as step completion at line 671-689)
- UI code already handles this event type correctly
- Server-driven status (as user requested)

### Option B: Frontend Fix (Not Recommended)

Update `updateJobProgress()` to also update `jobTreeData`.

**Why not**: User explicitly stated "UI should NOT assume status in anyway" and wants server-driven updates.

## Recommendation

**Option A: Backend Fix** - Add `EventJobUpdate` publication when step starts running.

This ensures:
1. Server-driven status updates (as requested)
2. Consistent with existing pattern (uses same event type as step completion)
3. UI's `handleJobUpdate()` already handles this correctly
4. Single source of truth from backend

## Files to Modify

1. `internal/queue/orchestrator.go` - Add `EventJobUpdate` after step starts running event (after line 244)

## Build Verification Required

After modification, run: `./scripts/build.sh`
