# Task 1: Fix step progress rendering to use step-specific progress data
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
Fixes the step progress display in the step row to show correct per-step child job counts instead of aggregate parent job counts.

## Root Cause
In `renderJobs()` at lines 2635-2642, step progress is populated from `parentJob.pending_children` etc., which represents ALL children across ALL steps. But `updateStepProgress()` at lines 3613-3614 updates the individual step job object's properties (`step.pending_children`, etc.).

## Do
1. Modify `renderJobs()` to look up step-specific progress from step job objects in allJobs
2. Store step progress on the parent job keyed by step name for cases where step jobs aren't loaded
3. Update `updateStepProgress()` to also store progress on parent job's step_progress map

## Accept
- [ ] Step progress in step row shows correct per-step counts
- [ ] Step progress updates in real-time when `step_progress` WebSocket events arrive
