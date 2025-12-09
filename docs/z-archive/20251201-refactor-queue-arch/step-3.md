# Step 3: Create StepMonitor

Model: opus | Status: ✅

## Done

- Created `StepMonitor` interface in `interfaces/job_interfaces.go`
- Created `JobStatusManager` interface for decoupling StepMonitor from queue.Manager
- Created `state/step_monitor.go` implementing StepMonitor:
  - `StartMonitoring()` starts goroutine for each step with children
  - `monitorStepChildren()` polls child job status every 5 seconds
  - `checkStepChildProgress()` aggregates child stats using JobStatusManager
  - `publishStepProgress()` sends WebSocket events for UI updates
- Integrated StepMonitor into ExecuteJobDefinition:
  - Steps with children ("spawned" status) start StepMonitor
  - StepMonitor marks step as "completed" when all children finish
- Updated handler signatures to pass StepMonitor:
  - `NewJobDefinitionHandler` now takes `stepMonitor interfaces.StepMonitor`
  - `NewGitHubJobsHandler` now takes `stepMonitor interfaces.StepMonitor`
- Created StepMonitor in app.go alongside JobMonitor

## Key Changes

```
StepMonitor lifecycle:
1. ExecuteJobDefinition creates step job
2. Worker creates child jobs under step (spawns)
3. StepMonitor.StartMonitoring() launches goroutine
4. Goroutine polls child stats via JobStatusManager
5. When all children complete -> step marked "completed"
6. WebSocket events published for UI updates
```

## Files Changed

- `internal/interfaces/job_interfaces.go` - Added StepMonitor, JobStatusManager interfaces
- `internal/queue/state/step_monitor.go` - NEW: Step monitoring implementation
- `internal/queue/manager.go` - Integrated StepMonitor into ExecuteJobDefinition
- `internal/handlers/job_definition_handler.go` - Added stepMonitor parameter
- `internal/handlers/github_jobs_handler.go` - Added stepMonitor parameter
- `internal/app/app.go` - Create and wire StepMonitor

## Verify

Build: ✅ | Tests: ⏭️
