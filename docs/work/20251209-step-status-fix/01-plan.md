# Implementation Plan: Fix Step Status Not Updating from "Spawned" to "Completed"

## Problem

Steps with child jobs (type="agent", "local_dir", "code_map", etc.) are getting stuck showing "Spawned" status in the UI, even after all child jobs complete and the step monitor marks the step as completed.

**Screenshot Evidence:** Steps 1-3 show "Spawned" badge even though logs show "Status changed: completed".

## Root Cause Analysis

1. **Orchestrator** (`orchestrator.go:471-483`): When a step spawns child jobs, it sets `stepStatus = "spawned"` and stores it in `step_stats` array in the manager's metadata.

2. **StepMonitor** (`step_monitor.go:206`): When all children complete, it correctly updates the step job's status via `UpdateJobStatus(ctx, stepJob.ID, finalStatus)`.

3. **BUG**: StepMonitor does NOT update the `step_stats` in the manager's metadata. The UI reads `step_stats[].status` to determine step status, so it still sees "spawned".

## Skills Required
- [x] go - for StepMonitor and JobManager modifications

## Work Packages

### WP1: Update step_stats in Manager Metadata on Step Completion [PARALLEL-SAFE]
**Skills:** go
**Files:** `internal/queue/state/step_monitor.go`
**Description:**
- After calling `UpdateJobStatus`, update the manager's metadata `step_stats` array to change the step's status from "spawned" to the final status
- Need to:
  1. Get the step's index from metadata (`step_index`)
  2. Get the manager job's metadata
  3. Update the `step_stats[step_index].status` value
  4. Save the updated metadata back

**Acceptance:**
- Steps that complete (all children done) show "completed" in UI
- Steps that fail show "failed" in UI
- UI receives real-time updates via existing event publishing

### WP2: Add UpdateStepStat Helper to JobStatusManager [PARALLEL-SAFE]
**Skills:** go
**Files:**
- `internal/queue/job_manager.go`
- `internal/interfaces/job_status_manager.go` (if interface needs update)
**Description:**
- Add a method `UpdateStepStatInManager(ctx, stepID, managerID, status string)` to update step_stats in manager metadata
- This encapsulates the logic of finding and updating the step's status in the array

**Acceptance:**
- Method correctly updates step_stats array
- Returns error if step not found in array

## Execution Order
1. WP1 and WP2 (parallel since WP1 can use jobMgr directly)
2. Actually WP1 depends on understanding the data structure, so let's do them sequentially:
   - WP2 first (create helper)
   - WP1 second (use helper in StepMonitor)

## Validation Checklist
- [ ] Build passes: `go build -o /tmp/quaero ./cmd/quaero`
- [ ] Tests pass: `go test ./internal/queue/... ./internal/queue/state/...`
- [ ] Manual test: Run codebase_assess job and verify steps transition from "spawned" to "completed"
- [ ] Follows skill patterns (error wrapping, structured logging)
