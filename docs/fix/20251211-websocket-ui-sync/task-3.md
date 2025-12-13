# Task 3: Update StepMonitor to broadcast step status directly
Depends: 2 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Ensures step status changes are broadcast immediately to UI without going through the log aggregator, fixing the delayed/missing updates issue.

## Skill Patterns to Apply
- Dependency injection: Pass WebSocketHandler reference to StepMonitor or use event service
- Status change detection: Only broadcast when status actually changes
- Non-blocking: Use goroutines for broadcasts

## Do
1. Add `EventJobUpdate` event type to event_service.go
2. Modify `StepMonitor.publishStepProgress()` to also publish `EventJobUpdate` when status changes
3. Subscribe to `EventJobUpdate` in WebSocketHandler to call `BroadcastJobUpdate`
4. Ensure broadcasts happen for: running → completed, running → failed, pending → running

## Accept
- [ ] New event type `EventJobUpdate` exists
- [ ] StepMonitor publishes job_update events on step status transitions
- [ ] WebSocketHandler subscribes and broadcasts to clients
- [ ] Status changes are immediate (not delayed by aggregator)
- [ ] Existing step_progress events still work for backward compatibility
