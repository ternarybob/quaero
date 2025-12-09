# Task 3: Create StepMonitor

Depends: 1 | Critical: yes:architectural-change | Model: opus

## Do

1. Create new file `internal/queue/state/step_monitor.go`

2. Implement StepMonitor struct:
   - Similar to JobMonitor but monitors a single step's children
   - Tracks: pending, running, completed, failed children
   - Publishes events to UI job panel (not top panel)

3. StepMonitor methods:
   - `StartMonitoring(ctx, stepJob)` - begins monitoring step's children
   - `checkChildProgress(ctx, stepID)` - aggregates child stats
   - `publishStepProgress(ctx, stepID, stats)` - sends to UI

4. Event flow:
   - Children publish status changes
   - StepMonitor receives and aggregates
   - StepMonitor publishes step_progress event
   - UI job panel updates

5. Completion:
   - When all children complete, mark step as completed
   - Notify manager that step is done

## Accept

- [ ] step_monitor.go exists with StepMonitor implementation
- [ ] StepMonitor tracks per-step child statistics
- [ ] StepMonitor publishes step_progress events
- [ ] StepMonitor marks step complete when children finish
- [ ] Code compiles without errors
