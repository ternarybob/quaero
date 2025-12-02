# Step 2: Update agent worker job_log events with manager_id
Model: sonnet | Status: ✅

## Done
- Added `ManagerID` field to `JobLogOptions` struct in manager.go
- Updated `AddJobLogWithEvent()` to include manager_id in event payload
- Updated `Execute()` in agent_worker.go to extract manager_id from job metadata
- Updated `publishAgentJobLog()` to accept and pass manager_id
- Updated all 5 calls to publishAgentJobLog to pass manager_id

## Files Changed
- `internal/queue/manager.go` - Added ManagerID to JobLogOptions, included in event payload
- `internal/queue/workers/agent_worker.go` - Extract and pass manager_id in all log events

## Build Check
Build: ✅ | Tests: ⏭️
