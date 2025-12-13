# Step 2-4: Additional implementation steps
Model: opus | Status: ✅

## Done
Tasks 2-4 were combined and completed as part of the core fix:
- `updateStepProgress()` now stores progress on manager job keyed by step_name
- `renderJobs()` looks up step-specific progress from `_stepProgress` map
- Backend sends `step_name` in `step_progress` events for UI aggregation
- Added `StepProgressAlignment` sub-test to TestNearbyRestaurantsKeywordsMultiStep

## Files Changed
- `internal/queue/state/monitor.go` - Added step_name to step_progress event
- `pages/queue.html` - Store and use step-specific progress
- `test/ui/queue_test.go` - Added StepProgressAlignment sub-test and toInt helper

## Build Check
Build: ✅ | Tests: ⏭️
