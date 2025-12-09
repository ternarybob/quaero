# Task 1: Identify and fix duplicate log sources

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Fixes duplicate log entries (e.g., "Starting 2 workers..." appearing twice) - User Intent #2

## Do

1. Examine `internal/queue/state/step_monitor.go` for duplicate publishing patterns
2. Check if `UpdateJobStatus()` in job_manager.go also logs when StepMonitor already logged
3. Look for patterns where both `publishStepLog()` AND `AddJobLog()` are called for same event
4. Fix any identified duplicate sources by ensuring single log path

## Accept

- [ ] No duplicate log entries when step starts or finishes
- [ ] "Starting N workers..." appears exactly once
- [ ] "Step finished successfully" appears exactly once
- [ ] Code compiles without errors
