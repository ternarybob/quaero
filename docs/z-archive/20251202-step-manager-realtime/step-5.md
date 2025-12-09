# Step 5: Add immediate step_progress events on child completion
Model: sonnet | Status: ✅

## Done
- Added `publishStepProgressOnChildChange()` method in monitor.go
- Called from EventJobStatusChange subscription handler
- Publishes step_progress event immediately when child job status changes
- Includes full progress stats (total, pending, running, completed, failed, cancelled)
- Complements existing 5-second polling mechanism

## Files Changed
- `internal/queue/state/monitor.go` - New publishStepProgressOnChildChange method, call from status change handler

## Build Check
Build: ✅ | Tests: ⏭️
