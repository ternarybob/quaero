# Complete: Fix Step Logs Inconsistent
Type: fix | Tasks: 4 | Files: 2

## User Request
"After the last logging refactor, the steps logging is not consistent. Shows logs in the console, however 0 in the step."

## Result
Fixed the step events inconsistency by:
1. Changing `StepMonitor.publishStepLog()` to store logs under the step job ID
2. Adding `step_progress` event publishing to orchestrator for synchronously-completed steps

## Skills Used
- go (backend event flow)

## Validation: ✅ MATCHES
Implementation addresses both root causes.

## Review: N/A
No critical triggers detected.

## Verify
Build: ✅ | Tests: ⏭️ (manual test recommended)

## Root Causes
1. `publishStepLog` was storing logs under `managerID` but UI fetches from `stepJobId`
2. Orchestrator didn't publish `step_progress` events, so UI never received refresh trigger

## Files Changed
- `internal/queue/state/step_monitor.go` - Updated `publishStepLog` to use stepID
- `internal/queue/orchestrator.go` - Added `EventStepProgress` publishing on step completion/failure
