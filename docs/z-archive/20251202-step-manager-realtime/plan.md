# Plan: Fix Step Manager Panel Not Updating in Real-Time
Type: fix | Workdir: ./docs/fix/20251202-step-manager-realtime/

## User Intent (from manifest)
Fix the step manager UI panel to receive and display real-time updates from worker events/logs. Currently, the job toolbar updates correctly but the step manager panel (showing step progress, worker status, etc.) stops updating during job execution until the job completes.

## Root Cause Analysis

The issue is a **parent_job_id mismatch** in the job_log events:

### Job Hierarchy
```
Manager Job (07ae3cfc-...)        <- This ID is used by UI to lookup logs
├── Step 1 (bc2ab382-...)         <- search_nearby_restaurants
│   └── Worker jobs (child jobs)
└── Step 2 (2c2e02d9-...)         <- extract_keywords
    └── Agent jobs (child jobs)   <- These publish job_log events
```

### Current Behavior
1. Agent worker creates a job with `parent_id = step_job_id` (2c2e02d9)
2. In `Execute()`, `parentID := job.GetParentID()` gets the step job ID
3. `publishAgentJobLog()` sends event with `parent_job_id = step_job_id`
4. UI stores logs under `jobLogs[step_job_id]`
5. UI calls `getStepLogs(manager_job_id, step_name)` - **looks up wrong key!**

### Expected Behavior
1. Job logs should include `manager_id` field (the top-level manager job ID)
2. UI should store/lookup logs using manager_id (or step_id should work too)
3. Logs should bubble up from worker → step → manager hierarchy

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Add manager_id to agent job metadata when creating child jobs | - | no | sonnet |
| 2 | Update agent worker to publish job_log events with manager_id | 1 | no | sonnet |
| 3 | Update UI handleJobLog to use manager_id for log aggregation | 2 | no | sonnet |
| 4 | Verify places_worker publishes correct parent/manager IDs | 1 | no | sonnet |
| 5 | Add step_progress event when child job status changes | 2 | no | sonnet |
| 6 | Build and test fix | 1,2,3,4,5 | no | sonnet |

## Order
[1] → [2,3,4] → [5] → [6]

## Technical Details

### Task 1: Add manager_id to agent job metadata
In `agent_worker.go` `CreateJobs()`, when creating child agent jobs, add `manager_id` to the job metadata. The step job should have `parent_id` = manager_id, so we can get it from there.

### Task 2: Update agent worker to publish with manager_id
In `publishAgentJobLog()`, include `manager_id` in the event payload so UI can aggregate logs at the manager level.

### Task 3: Update UI handleJobLog
In `handleJobLog()` in queue.html:
- Check for `manager_id` in the event payload
- Use `manager_id` as the key for storing logs (not parent_job_id which is the step)
- This allows logs to be found when UI calls `getStepLogs(manager_job_id, step_name)`

### Task 4: Verify places_worker
Check `places_worker.go` to ensure it also includes the correct manager_id in its job_log events.

### Task 5: Real-time step_progress on child completion
Currently step_progress events only come from StepMonitor's 5-second poll. Add immediate step_progress event when a child job completes to provide instant UI feedback.
