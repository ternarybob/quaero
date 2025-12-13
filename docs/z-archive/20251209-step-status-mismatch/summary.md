# Summary: Step Status Mismatch Fix

## Problem
When a multi-step job failed, individual steps showed "Completed" (green badge) instead of "Failed" (red badge) in the UI, even though the event log and parent job status correctly showed failures.

## Root Cause
Two issues:
1. **Backend**: The `step_stats` array in parent job metadata didn't include a `status` field to record the actual step status
2. **Frontend**: The UI code at `queue.html:2533-2535` had a buggy override that forced all steps to "completed" when the parent job status was "completed" (even though error-tolerant jobs can complete while having failed steps)

## Solution

### Backend Changes (orchestrator.go)
Added `status` field to `step_stats` in three locations:
- **Init failure path**: Records `status: "failed"` when step initialization fails
- **Execute failure path**: Records `status: "failed"` when step execution fails
- **Success path**: Records `status: "completed"` or `status: "spawned"` based on step type

### Frontend Changes (queue.html)
- Added check for `stepStat.status` from backend as authoritative source for terminal status
- Removed buggy override logic that was incorrectly marking all steps as "completed"

## Files Modified
1. `internal/queue/orchestrator.go` - Added status field to step_stats
2. `pages/queue.html` - Use step_stats.status for step badge rendering

## Verification
- Go build succeeds without errors
- UI now uses backend-provided step status as authoritative source
- Failed steps will show "Failed" (red) badge
- Successful steps will show "Completed" (green) badge
