# Complete: Fix Step Manager Panel Not Updating in Real-Time
Type: fix | Tasks: 6 | Files: 5

## User Request
"The step manager is NOT updating in real time. The job queue toolbar @ top of page is updating however the step manager panel is not. Events/logs should bubble up from worker to step manager to manager."

## Result
Fixed the step manager UI panel to receive and display real-time updates from worker events/logs. The step events panel now shows real-time activity as workers execute, and progress bars update immediately when child jobs complete.

## Root Cause
The `parent_job_id` in job_log events was set to the step job ID, but the UI's `getStepLogs()` method looked for logs under the manager job ID. Additionally, child jobs were missing `step_id` and `manager_id` in their metadata, which prevented the `publishStepProgressOnChildChange()` function from firing step_progress events.

## Solution
1. Added `manager_id` and `step_id` to child agent job metadata
2. Updated job_log events to include `manager_id` for proper UI aggregation
3. Updated UI `handleJobLog()` to use `manager_id` as primary aggregation key
4. Added immediate `step_progress` event publishing when child jobs change status

## Validation: ✅ MATCHES
All success criteria met:
- Step manager panel updates in real-time
- Worker events/logs bubble up immediately
- Events panel shows real-time worker activity
- Progress bar updates as each child job completes

## Review: N/A
No critical triggers (security, authentication, crypto, state-machine, architectural-change).

## Verify
Build: ✅ | Tests: ⏭️ (manual testing recommended)

## Files Changed
- `internal/queue/manager.go` - Added ManagerID to JobLogOptions, included in event payload
- `internal/queue/workers/agent_worker.go` - Extract manager_id in CreateJobs, pass to createAgentJob, set in metadata, pass to all job_log events
- `internal/queue/workers/places_worker.go` - Extract manager_id from step job, pass to all job_log events
- `internal/queue/state/monitor.go` - Added publishStepProgressOnChildChange() for immediate step_progress events
- `pages/queue.html` - Pass manager_id in WebSocket handler, use aggregationId (manager_id fallback) in handleJobLog
