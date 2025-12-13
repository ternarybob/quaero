# Task 5: Ensure UI filters events by step_name correctly

Depends: 4 | Critical: no | Model: sonnet

## Addresses User Intent

UI must only show events in the step panel they belong to. Events from step A must NOT appear in step B's panel.

## Do

1. Review pages/queue.html `getStepLogs()` function
2. Ensure filtering by step_name works correctly
3. Remove any fallback logic that shows all events when step_name is missing
4. Add logging/debugging to track event routing in UI
5. Verify WebSocket handler passes step_name to Alpine component

## Accept

- [ ] `getStepLogs()` only returns events with matching step_name
- [ ] No fallback that shows all events in all panels
- [ ] Events from one step do not appear in another step's panel
