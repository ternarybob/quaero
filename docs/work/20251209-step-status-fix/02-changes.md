# Changes Log

## Fix 1: Update step_stats in Manager Metadata on Step Completion

### Files Modified

#### `internal/interfaces/job_interfaces.go`
- Added `UpdateStepStatInManager(ctx, stepID, managerID, status string) error` method to `JobStatusManager` interface
- Added `UpdateJobMetadata(ctx, jobID string, metadata map[string]interface{}) error` method to `JobStatusManager` interface

#### `internal/queue/job_manager.go`
- Added `UpdateStepStatInManager()` implementation that:
  1. Gets the manager job from storage
  2. Finds the step in the `step_stats` array by `step_id`
  3. Updates the `status` field in the step's stat entry
  4. Updates `current_step_status` in manager metadata if this is the current step
  5. Saves the updated manager job back to storage

#### `internal/queue/state/step_monitor.go`
- Updated `monitorStepChildren()` to call `UpdateStepStatInManager()` in all terminal paths:
  1. Context cancellation (`ctx.Done()`) - status "cancelled"
  2. Timeout - status "failed"
  3. API cancellation detection - status "cancelled"
  4. Grace period completion (no children) - status "completed"
  5. All children completed - final status determined by child outcomes

---

## Fix 2: Step Status Reverts to "Spawned" on API Refresh

### Root Cause
The orchestrator waits synchronously for child jobs to complete (polling loop), but after the wait finishes, it still sets `stepStatus = "spawned"` because the condition only checked `returnsChildJobs && stepChildCount > 0`.

This caused the step_stats to be written with status="spawned" even though all children had completed, which then overwrote the UI state on page refresh.

### Files Modified

#### `internal/queue/orchestrator.go`
- Added `childrenWaitedSynchronously` flag to track whether the orchestrator waited inline for children
- Set this flag to `true` when the synchronous wait loop completes successfully
- Updated step status determination logic to only set "spawned" when NOT waiting synchronously:
  ```go
  if returnsChildJobs && stepChildCount > 0 && !childrenWaitedSynchronously {
      stepStatus = "spawned"
  }
  ```
- Updated logging to include `children_waited_synchronously` flag for debugging

### Skill Compliance
- [x] Error wrapping with context (`fmt.Errorf("...: %w", err)`)
- [x] Structured logging with arbor
- [x] Context passed to all storage calls
- [x] Interface-based dependency

### Ready for Validation
