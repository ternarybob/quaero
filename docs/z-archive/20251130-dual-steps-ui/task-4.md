# Task 4: Update Queue UI for step progress
- Group: 4 | Mode: concurrent | Model: sonnet
- Skill: @frontend-developer | Critical: no | Depends: 3
- Sandbox: /tmp/3agents/task-4/ | Source: ./ | Output: docs/feature/20251130-dual-steps-ui/

## Files
- `pages/queue.html` - add step progress display

## Requirements
Update the Queue page to display step progress:
1. Handle `job_step_progress` WebSocket messages
2. Display progress as "Step X/Y: step_name (status)"
3. Store step data in `job.status_report`

## Acceptance
- [ ] WebSocket handler for step progress
- [ ] Progress text shows step info
- [ ] UI updates in real-time
