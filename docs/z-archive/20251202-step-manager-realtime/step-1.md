# Step 1: Add manager_id to agent job metadata
Model: sonnet | Status: ✅

## Done
- Modified `CreateJobs()` in agent_worker.go to extract manager_id from step job's ParentID
- Updated `createAgentJob()` function to accept managerID parameter
- Added `manager_id` and `step_id` fields to child job metadata

## Files Changed
- `internal/queue/workers/agent_worker.go` - Added manager_id extraction and propagation to child jobs

## Build Check
Build: ✅ | Tests: ⏭️
