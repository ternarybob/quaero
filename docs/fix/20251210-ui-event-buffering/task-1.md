# Task 1: Modify queue.html refreshStepEvents to only fetch on START/COMPLETE
Depends: - | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
Step Events panel should only fetch on step START and step COMPLETE, not during execution.

## Skill Patterns to Apply
- Alpine.js reactive data binding
- WebSocket event handling
- API fetch with throttling

## Do
1. In `refreshStepEvents()` function:
   - Track which steps have already been fetched for START (use `_stepEventsFetched` map)
   - On first trigger for a step (not in map) -> fetch initial events (START condition)
   - On `finished=true` -> always fetch final events (COMPLETE condition)
   - On subsequent triggers during execution (in map, not finished) -> SKIP fetch

2. Clear the `_stepEventsFetched` map when step tracking is reset

## Accept
- [ ] `refreshStepEvents()` only fetches on START (first trigger)
- [ ] `refreshStepEvents()` only fetches on COMPLETE (finished=true)
- [ ] Middle-of-execution triggers are skipped
- [ ] Console logs show skip behavior for debugging
