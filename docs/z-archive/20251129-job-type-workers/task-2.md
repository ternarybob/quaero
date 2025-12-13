# Task 2: Create Generic StepManager

- Group: 2 | Mode: sequential | Model: opus
- Skill: @golang-pro | Critical: yes:architectural-change | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `internal/queue/managers/generic_manager.go` - NEW: Generic step manager
- `internal/interfaces/job_interfaces.go` - Update StepWorker interface
- `internal/queue/orchestrator.go` - Update to use generic manager

## Requirements

1. Create `GenericStepManager` that:
   - Holds registry of `StepWorker` implementations by `StepType`
   - Routes step execution based on `step.Type`
   - Handles placeholder resolution
   - Provides consistent error handling

2. Update `StepWorker` interface:
   ```go
   type StepWorker interface {
       GetType() models.StepType
       CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, parentJobID string) error
       ReturnsChildJobs() bool
       Validate(step models.JobStep) error
   }
   ```

3. Update `Orchestrator`:
   - Replace `stepExecutors map[string]StepManager` with single `GenericStepManager`
   - Remove action-based routing logic
   - Delegate all step execution to GenericStepManager

4. Keep existing managers but refactor them to implement new `StepWorker` interface

## Acceptance
- [ ] GenericStepManager created with worker registry
- [ ] StepWorker interface updated
- [ ] Orchestrator uses GenericStepManager
- [ ] Existing managers converted to StepWorker interface
- [ ] Compiles: `go build ./...`
- [ ] Tests pass: `go test ./internal/queue/...`
