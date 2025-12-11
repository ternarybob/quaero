# Task 4: Update UI to use new endpoint and message format
Depends: 3 | Critical: no | Model: sonnet | Skill: -

## Addresses User Intent
Makes UI correctly update job and step statuses in real-time by consuming the new WebSocket message format and calling the simplified structure endpoint.

## Skill Patterns to Apply
N/A - no skill for this task

## Do
1. In queue.html, add handler for `job_update` WebSocket message type
2. When `job_update` received:
   - If `context=job`, update the job's status in allJobs array
   - If `context=job_step`, update the step's status in jobTreeData
   - If `refresh_logs=true` and step is expanded, fetch logs for that step
3. Add `fetchJobStructure(jobId)` function that calls `/api/jobs/{id}/structure`
4. On receiving `job_update` with `refresh_logs=true`, call `fetchJobStructure` and update UI
5. Remove or reduce reliance on the old `step_progress` message for status updates

## Accept
- [ ] UI listens for `job_update` messages
- [ ] Job status updates immediately when `job_update` with `context=job` received
- [ ] Step status updates immediately when `job_update` with `context=job_step` received
- [ ] Logs only fetched for expanded steps
- [ ] No duplicate fetches or infinite loops
