# Step 1: Fix step progress rendering to use step-specific progress data
Model: opus | Status: ✅

## Done
- Added `step_name` extraction from child job metadata in `publishStepProgressOnChildChange()` (monitor.go:928)
- Added `step_name` to `step_progress` event payload (monitor.go:971)
- Updated WebSocket handler in queue.html to pass `step_name` from step_progress events (line 1320)
- Modified `updateStepProgress()` to store step progress on manager job keyed by step_name (lines 3597-3620)
- Modified `renderJobs()` to use step-specific progress from `_stepProgress` map instead of aggregate parent stats (lines 2635-2658)

## Files Changed
- `internal/queue/state/monitor.go` - Added step_name to step_progress event payload
- `pages/queue.html` - Updated WebSocket handler and rendering to use step-specific progress

## Build Check
Build: ✅ | Tests: ⏭️
