# Task 1: Add manager_id to agent job metadata when creating child jobs
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
This task ensures child jobs have access to the top-level manager ID so events can be properly aggregated for the step events panel.

## Do
1. In `agent_worker.go` `CreateJobs()` method:
   - Get the step job and extract its parent_id (which is the manager_id)
   - When creating child agent jobs, add `manager_id` to the job metadata
   - Also ensure `step_id` is set in metadata

## Files to Modify
- `internal/queue/workers/agent_worker.go`

## Accept
- [ ] Child agent jobs have `manager_id` in their metadata
- [ ] Child agent jobs have `step_id` in their metadata
- [ ] Manager ID is correctly extracted from step job's parent_id
