# Task 4: Add step_progress events to orchestrator
Depends: 3 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Orchestrator needs to publish `step_progress` events when steps complete (completed/failed) so the UI receives the refresh trigger and fetches step events.

## Skill Patterns to Apply
- Event publishing pattern from StepMonitor
- Async goroutine for event publish

## Do
1. Add `step_progress` event publishing when `stepStatus == "completed"` (line 496)
2. Add `step_progress` event publishing on init failure (line 262)
3. Add `step_progress` event publishing on execution failure (line 324)

## Accept
- [ ] `EventStepProgress` published on step completion
- [ ] `EventStepProgress` published on step init failure
- [ ] `EventStepProgress` published on step execution failure
- [ ] Build succeeds
