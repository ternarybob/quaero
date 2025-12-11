# Step 3: Update StepMonitor to broadcast step status directly
Model: opus | Skill: go | Status: ✅

## Done
- Added `EventJobUpdate` event type to interfaces/event_service.go
- Modified `StepMonitor.publishStepProgress()` to also publish `EventJobUpdate`
- Added `StepMonitor.publishJobUpdate()` helper method
- Subscribed to `EventJobUpdate` in WebSocketHandler to call `BroadcastJobUpdate`

## Files Changed
- `internal/interfaces/event_service.go` - Added EventJobUpdate event type (lines 287-298)
- `internal/queue/state/step_monitor.go` - Added publishJobUpdate method and call from publishStepProgress (lines 350-385)
- `internal/handlers/websocket.go` - Added EventJobUpdate subscription (lines 1509-1535)

## Skill Compliance
- [x] Dependency injection: Uses existing eventService pattern
- [x] Status change detection: Broadcasts on every publishStepProgress call with isTerminal flag
- [x] Non-blocking: Uses goroutines for event publishing

## Build Check
Build: ✅ | Tests: ⏭️
