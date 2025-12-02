# Task 4: Verify places_worker publishes correct parent/manager IDs
Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent
Ensures consistency across all workers so the step events panel works for all job types.

## Do
1. Review `places_worker.go` to check how it publishes job_log events
2. If places_worker creates child jobs, ensure they have manager_id in metadata
3. Ensure job_log events from places_worker include manager_id

## Files to Review/Modify
- `internal/queue/workers/places_worker.go`

## Accept
- [ ] places_worker job_log events include manager_id where applicable
- [ ] places_worker child jobs have manager_id in metadata
- [ ] Consistent event format across agent_worker and places_worker
