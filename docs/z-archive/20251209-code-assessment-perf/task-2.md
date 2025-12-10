# Task 2: Add event batching for WebSocket broadcasts

Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Events should be buffered so workers aren't blocked by event publishing.

## Skill Patterns to Apply
- Use channels for async communication
- Use context for cancellation

## Do
- Add a buffered event channel in WebSocket handler
- Batch events and broadcast periodically (every 100ms or 10 events, whichever first)
- Ensure graceful shutdown drains the buffer

## Accept
- [ ] Events are batched before broadcast
- [ ] Workers are not blocked by slow clients
- [ ] Build compiles
