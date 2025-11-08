# Parent Job Progress Tracking - Plan Summary

## Overview

**Task**: Implement real-time event-driven progress tracking for parent crawler jobs

**Problem**: UI shows progress ("66 pending, 1 running, 41 completed") but backend lacks real-time event publishing. Current polling-based approach has 5-second delay.

**Solution**: Event-driven architecture where child job status changes trigger immediate parent progress updates via WebSocket.

## Key Changes

### Architecture Shift
- **From**: Polling every 5 seconds
- **To**: Event-driven on status changes + backup polling every 30 seconds

### New Event Flow
```
Child Job Status Change
  ↓
Manager publishes "job_status_change"
  ↓
ParentJobExecutor subscribes & calculates stats
  ↓
ParentJobExecutor publishes "parent_job_progress"
  ↓
WebSocket broadcasts to UI
```

## Implementation Steps (8 Total)

1. ✅ Add `EventJobStatusChange` event type
2. ✅ Publish events from `Manager.UpdateJobStatus()`
3. ✅ Add EventService dependency to Manager
4. ✅ Subscribe to status changes in ParentJobExecutor
5. ✅ Subscribe to progress events in WebSocket handler
6. ✅ Update app initialization
7. ✅ Add logging for status changes
8. ⚠️ Optional: Reduce polling frequency (30s backup)

## Files Modified

1. `internal/interfaces/event_service.go` - Event type definition
2. `internal/jobs/manager.go` - Event publishing + EventService
3. `internal/jobs/processor/parent_job_executor.go` - Event subscription
4. `internal/handlers/websocket.go` - WebSocket subscription
5. `internal/app/app.go` - Dependency injection

## Progress Format

**Required**: `"X pending, Y running, Z completed, W failed"`

**Example**: `"66 pending, 1 running, 41 completed, 0 failed"`

## Breaking Changes

- Manager constructor signature changes
- Requires updating `NewManager()` calls in `app.go`
- Graceful degradation if EventService is nil

## Success Criteria

✅ Real-time updates (< 1 second latency)
✅ Progress text format matches requirement exactly
✅ WebSocket receives updates keyed by job_id
✅ Job logs show child status transitions
✅ No WebSocket flooding (only on status changes)

## Estimated Time

**3.5 hours** for full implementation and testing

## Edge Cases Handled

- Race conditions (concurrent child updates)
- Parent completion before children
- Multiple child failures
- EventService unavailable (graceful degradation)
- WebSocket disconnection
- Long-running jobs (no memory accumulation)

## Testing Required

### Unit Tests
- Manager event publishing
- ParentJobExecutor subscription
- Progress text formatting
- Overall status calculation

### Integration Tests
- End-to-end progress flow
- Concurrent child updates
- WebSocket message delivery

### Manual Testing
- UI real-time updates
- DevTools WebSocket monitoring
- Multiple concurrent jobs

## Rollback Plan

**Level 1**: Set `EventService = nil` (falls back to polling)
**Level 2**: Disable subscriptions (remove event handlers)
**Level 3**: Full revert (remove all changes)

## Next Agent Tasks

**Agent 2 (Implementer)**:
1. Implement steps 1-7 sequentially
2. Skip step 8 (polling optimization) initially
3. Write unit tests after each step
4. Validate WebSocket output format
5. Report completion to Agent 3

**Agent 3 (Validator)**:
1. Review code changes
2. Run integration tests
3. Validate WebSocket messages
4. Check progress text format
5. Approve or request fixes

## Documentation

**Plan Location**: `C:\development\quaero\docs\parent-job-progress-tracking\plan.md`

**Screenshot Reference**: `C:/Users/bobmc/Pictures/Screenshots/ksnip_20251108-173620.png`

---

**Status**: ✅ Plan Complete - Ready for Implementation
**Date**: 2025-11-08
**Complexity**: Medium
**Priority**: High
