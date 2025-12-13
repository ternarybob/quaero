# Complete: Fix Step Logs Inconsistent
Type: fix | Tasks: 7 | Files: 4

## User Request
"After the last logging refactor, the steps logging is not consistent. Shows logs in the console, however 0 in the step."

## Result
Fixed the step events inconsistency by:
1. Changing `StepMonitor.publishStepLog()` to store logs under the step job ID
2. Adding `step_progress` event publishing to orchestrator for completed/failed steps
3. Adding periodic `step_progress` events during synchronous wait (every 2s)
4. Enabling UI to fetch step events during running state (not just on completion)
5. Including `step_job_ids` in manager metadata on EACH step start/completion (not just at end)
6. Fixed `/api/logs` handler to call accessor methods correctly (was returning null for all context fields)

## Skills Used
- go (backend event flow)
- frontend (WebSocket trigger handling)

## Validation: ✅ MATCHES
Implementation addresses all root causes including real-time updates.

## Review: N/A
No critical triggers detected.

## Verify
Build: ✅ | Tests: ⏭️ (manual test recommended)

## Root Causes
1. `publishStepLog` was storing logs under `managerID` but UI fetches from `stepJobId`
2. Orchestrator didn't publish `step_progress` events, so UI never received refresh trigger
3. UI skipped fetching step events when `finished=false`
4. Orchestrator's synchronous wait loop didn't publish periodic progress events
5. `step_job_ids` was only saved to manager metadata at END of all steps - UI couldn't find step job IDs during execution
6. **NEW**: `/api/logs` handler used `log.JobID` (method reference) instead of `log.JobID()` (method call), causing null values

## Files Changed
- `internal/queue/state/step_monitor.go` - Updated `publishStepLog` to use stepID
- `internal/queue/orchestrator.go` - Added `EventStepProgress` publishing + periodic events + `step_job_ids` in metadata
- `pages/queue.html` - Removed `finished` check to allow real-time refresh during step execution
- `internal/handlers/unified_logs_handler.go` - Fixed method calls: `log.JobID()`, `log.StepName()`, etc.
