# Complete: Job Queue Real-time Update Fix

## Classification
- Type: fix
- Location: ./docs/fixes/job-queue-realtime-update/

When a job completed, the Queue UI showed stale data (status "Running", "0 Documents") until the page was manually refreshed.

## Root Cause (ACTUAL)

The WebSocket event subscriber (`websocket_events.go`) subscribes to lifecycle events like `EventJobCompleted`, `EventJobFailed`, `EventJobCancelled`. However, the queue manager's `UpdateJobStatus()` function only published `EventJobStatusChange` - **it never published the specific lifecycle events**.

This caused:
1. Job status changes → `EventJobStatusChange` published
2. WebSocket handler waiting for `EventJobCompleted` → never receives it
3. Frontend never gets notified of job completion via WebSocket
4. UI stays stale until manual page refresh

## The Fix

Modified `UpdateJobStatus()` in both:
- `internal/queue/manager.go`
- `internal/queue/state/runtime.go`

To publish BOTH:
1. `EventJobStatusChange` (for monitor/internal tracking)
2. The specific lifecycle event (`EventJobCompleted`/`EventJobFailed`/`EventJobCancelled`) when transitioning to terminal states

## Stats
Tasks: 2 | Files: 3 | Duration: ~30 min
Models: Planning=opus, Workers=sonnet

## Files Changed
- `internal/queue/manager.go:501-537` - Added lifecycle event publishing
- `internal/queue/state/runtime.go:67-102` - Added lifecycle event publishing
- `internal/queue/state/monitor.go:203-226` - Uses stats-inclusive progress method on completion

## Verify
- go build: PASS

## Event Flow After Fix

```
1. Job completes
2. UpdateJobStatus(ctx, jobID, "completed") called
3. Publishes EventJobStatusChange (for monitor)
4. ALSO publishes EventJobCompleted (NEW)
5. WebSocket handler receives EventJobCompleted
6. Broadcasts "job_status_change" WebSocket message
7. Frontend receives message, updates UI immediately
```
