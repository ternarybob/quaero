# WORKER Step 1: Add EventJobUpdate for Step Starting

## Change Made

Added `EventJobUpdate` publication when a step starts running in the orchestrator.

## Location

`internal/queue/orchestrator.go` lines 246-263 (after existing `EventJobProgress` publication)

## Code Added

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

## Why This Works

1. **Existing Pattern**: Follows same pattern as step completion (line ~700)
2. **UI Handler**: `handleJobUpdate()` in queue.html already handles this event type
3. **Tree Update**: The handler updates `jobTreeData[job_id].steps[stepIdx].status` (line 4526)
4. **Server-Driven**: Status comes from backend, UI doesn't assume state

## Event Flow

```
Orchestrator (step starts)
    │
    ├──► EventJobProgress (for progress tracking)
    │
    └──► EventJobUpdate (NEW - for tree status sync)
            │
            └──► WebSocket "job_update"
                    │
                    └──► handleJobUpdate()
                            │
                            └──► jobTreeData[*].steps[*].status = "running"
```

## Build Result

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

BUILD PASSED
