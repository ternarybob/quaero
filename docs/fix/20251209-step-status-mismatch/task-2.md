# Task 2: Update UI to use step_stats.status for step display
Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent
Updates the UI rendering logic to use the persisted step status from backend instead of calculating/overriding it incorrectly.

## Do
1. Modify queue.html renderJobs() to check step_stats[index].status first
2. Remove the buggy override that marks all steps as completed when parent is completed
3. Keep real-time WebSocket updates as highest priority (for in-progress steps)

## Accept
- [ ] UI uses step_stats.status when available
- [ ] Failed steps show red "failed" badge
- [ ] Successful steps show green "completed" badge
- [ ] Real-time updates still work for in-progress steps
