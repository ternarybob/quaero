# Task 2: Update agent worker to publish job_log events with manager_id
Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent
This ensures job_log events include the manager_id so the UI can properly aggregate logs at the manager level for display in the step events panel.

## Do
1. In `agent_worker.go` `Execute()` method:
   - Extract `manager_id` from job metadata (set by CreateJobs in Task 1)
   - Pass `manager_id` to `publishAgentJobLog()` calls

2. Update `publishAgentJobLog()` function:
   - Add `managerID` parameter
   - Include `manager_id` field in the event payload

3. Update `JobLogOptions` struct in manager.go if needed:
   - Add `ManagerID` field

4. Update `AddJobLogWithEvent()` in manager.go:
   - Include `manager_id` in the published event payload

## Files to Modify
- `internal/queue/workers/agent_worker.go`
- `internal/queue/manager.go` (JobLogOptions and AddJobLogWithEvent)

## Accept
- [ ] job_log events include `manager_id` field
- [ ] Agent worker extracts manager_id from job metadata
- [ ] All publishAgentJobLog calls pass manager_id
