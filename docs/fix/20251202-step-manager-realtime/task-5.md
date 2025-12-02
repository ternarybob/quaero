# Task 5: Add step_progress event when child job status changes
Depends: 2 | Critical: no | Model: sonnet

## Addresses User Intent
Provides instant real-time feedback in the step events panel when child jobs complete, rather than waiting for the 5-second StepMonitor poll.

## Do
1. In monitor.go, find where step status changes are detected
2. When a child job completes (status changes to completed/failed/cancelled):
   - Immediately publish a step_progress event
   - Include updated counts (completed_jobs, failed_jobs, etc.)
3. This supplements the 5-second polling with immediate updates

## Alternative Approach
Instead of modifying monitor.go, we could:
- Have the agent worker publish a "child_completed" event
- Have the WebSocket handler forward this to trigger UI step progress update

## Files to Review/Modify
- `internal/queue/state/monitor.go`
- OR `internal/queue/state/step_monitor.go`

## Accept
- [ ] step_progress events are published immediately when child jobs complete
- [ ] UI receives real-time step progress updates (not just 5-second polls)
- [ ] Progress bar updates as each child completes
