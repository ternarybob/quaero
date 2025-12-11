# Task 3: Add Live Tree Expansion with WebSocket Integration

Depends: 1 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent

Implements "Live Tree Expansion" - tree view expands as events are received, updating in real-time.

## Skill Patterns to Apply

- Alpine.js reactive data patterns
- WebSocket event handling (existing pattern in codebase)
- Debounced updates to prevent UI thrashing

## Do

1. Subscribe to step-related WebSocket events in `pages/queue.html`:
   - `job_step_started` - mark step as running, expand tree
   - `job_step_completed` - update step status
   - `job_step_failed` - update step status, auto-expand failed step
   - `job_log` - add log to appropriate step

2. Auto-expand tree for running jobs:
   - When a job transitions to "running", expand its tree view
   - When a step starts, expand that step's log panel
   - Keep failed steps expanded

3. Update step data in real-time:
   - Update `jobTreeData[jobId].steps[stepIndex].status`
   - Add new logs to `jobTreeData[jobId].steps[stepIndex].logs`
   - Update ChildSummary counts as grandchild jobs complete

4. Add debouncing:
   - Batch rapid updates (e.g., multiple logs in quick succession)
   - Use requestAnimationFrame for smooth UI updates

## Accept

- [ ] Tree view auto-expands when job starts running
- [ ] Step panels auto-expand when step starts
- [ ] Step status updates in real-time without page refresh
- [ ] New logs appear as they're received
- [ ] ChildSummary counts update as jobs complete
- [ ] UI remains responsive during rapid updates
