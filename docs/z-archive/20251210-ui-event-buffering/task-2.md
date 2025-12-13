# Task 2: Create LogEventAggregator for service logs buffering
Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Service Logs panel should use websocket-triggered batching instead of real-time individual logs.

## Skill Patterns to Apply
- Aggregator pattern (similar to StepEventAggregator)
- WebSocket broadcast callback
- Structured logging with arbor
- Context everywhere

## Do
1. Create `LogEventAggregator` in `internal/services/events/log_aggregator.go`:
   - Similar structure to `StepEventAggregator`
   - Track pending log count (not individual logs)
   - Time-based triggering (default 1s)
   - OnTrigger callback to send `refresh_logs` WebSocket message

2. Integrate in `WebSocketHandler`:
   - Create aggregator on handler init
   - In `log_event` subscriber: call aggregator.RecordEvent() instead of broadcasting
   - Add `broadcastLogsRefreshTrigger()` callback to send `refresh_logs` message

## Accept
- [ ] LogEventAggregator created with time-based triggering
- [ ] WebSocketHandler integrates aggregator for log events
- [ ] `refresh_logs` WebSocket message sent on trigger
- [ ] No individual `log` messages broadcast during high volume
