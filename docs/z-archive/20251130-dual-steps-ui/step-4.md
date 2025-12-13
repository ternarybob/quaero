# Step 4: Update Queue UI for step progress
- Task: task-4.md | Group: 4 | Model: sonnet

## Actions
1. Added handler for `job_step_progress` WebSocket message type
2. Format progress_text as "Step X/Y: step_name (status)"
3. Store step fields in job.status_report (current_step, total_steps, step_name, step_type, step_status)
4. Trigger re-render via throttledRenderJobs()

## Files
- `pages/queue.html` - lines 1120-1136: message handler
- `pages/queue.html` - lines 2700-2707: step field storage

## Decisions
- Use existing progress_text display: Leverages existing UI component
- Store in status_report: Consistent with other progress data

## Verify
Compile: ✅ | Tests: ⚙️

## Status: ✅ COMPLETE
