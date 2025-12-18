# Summary: Step Status Not Updating to "Running"

## Issue

Step status badge in the job tree view remained "pending" (yellow) even when logs showed the step was running ("Starting Step 3/5", "Initializing worker...", etc.). The status should update to "running" (blue) in real-time.

## Root Cause

When a step started running, the orchestrator published `EventJobProgress` but NOT `EventJobUpdate`. The UI's tree view status updates via `handleJobUpdate()` only, which handles `EventJobUpdate` events with `context: "job_step"`.

**Before Fix:**
- Step starts → `EventJobProgress` (handled by `updateJobProgress()`)
- `updateJobProgress()` updates `job.metadata.current_step_status`
- BUT tree view renders from `jobTreeData[*].steps[*].status` (NOT updated)

**After Fix:**
- Step starts → `EventJobProgress` + `EventJobUpdate`
- `handleJobUpdate()` updates `jobTreeData[*].steps[*].status`
- Tree badge correctly shows "running"

## Fix Applied

Added `EventJobUpdate` publication when a step starts running.

**File Modified:** `internal/queue/orchestrator.go` (lines 246-263)

```go
// Also publish job_update event for direct UI tree status sync (step starting)
// This matches the pattern used for step completion (line ~682)
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

## Design Principle

**Server-Driven Status**: As requested, the UI does not assume status. All status updates are pushed from the backend via events.

## Validation

- **Build**: PASSED ✓
- **Pattern Consistency**: Matches existing step completion event pattern ✓
- **Skill Compliance**: Extended existing pattern, no new files/functions ✓

## Workdir

`.claude/workdir/step-status-not-updating-2025-12-19/`
