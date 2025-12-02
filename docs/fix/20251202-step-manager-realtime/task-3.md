# Task 3: Update UI handleJobLog to use manager_id for log aggregation
Depends: 2 | Critical: no | Model: sonnet

## Addresses User Intent
This ensures the UI properly stores and retrieves logs so the step events panel displays real-time updates.

## Do
1. In `queue.html` WebSocket message handler for `job_log`:
   - Extract `manager_id` from the event payload
   - Pass `manager_id` to the custom event detail

2. In `handleJobLog()` method:
   - Use `manager_id` as the primary key for storing logs (fallback to parent_job_id for backwards compatibility)
   - This ensures `getStepLogs(manager_job_id, step_name)` finds the logs

## Files to Modify
- `pages/queue.html`

## Accept
- [ ] job_log WebSocket handler extracts manager_id
- [ ] handleJobLog stores logs under manager_id when available
- [ ] getStepLogs finds logs for manager jobs
- [ ] Backwards compatible with events missing manager_id
