# Step 3: Fix step_monitor Event Publishing

## Status: COMPLETE

## Changes Made

### step_monitor.go
- Updated `publishStepProgress()` function signature to include `stepName` parameter
- Added `step_name` to all step_progress event payloads

**Before:**
```go
func (m *StepMonitor) publishStepProgress(
    ctx context.Context,
    stepID string,
    managerID string,
    status string,
    stats *ChildJobStats,
)
```

**After:**
```go
func (m *StepMonitor) publishStepProgress(
    ctx context.Context,
    stepID string,
    managerID string,
    stepName string,  // NEW: Critical for UI filtering
    status string,
    stats *ChildJobStats,
)
```

### Call Sites Updated:
1. Line 99: Initial progress on step start
2. Line 147: Step completed (no child jobs spawned)
3. Line 182: Step completed with final status
4. Line 192: Progress update during monitoring

## Build Verification
- `go build ./internal/queue/state/...` - SUCCESS
