# Task 5: Update manager.go worker registry

- Group: 5 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2,4
- Sandbox: /tmp/3agents/task-5/ | Source: ./ | Output: ./docs/feature/20251129-remove-step-job-manager/

## Files
- `internal/queue/manager.go` - Update worker registry types

## Requirements
1. Update Manager struct field from `workers map[models.StepType]interfaces.StepWorker` to `workers map[models.WorkerType]interfaces.DefinitionWorker`
2. Update `RegisterWorker` method signature to use DefinitionWorker
3. Update `HasWorker` method signature to use WorkerType
4. Update `GetWorker` method signatures to use WorkerType and DefinitionWorker
5. Update `ExecuteJobDefinition` to call `worker.ValidateConfig` instead of `worker.ValidateStep`
6. Update comments referencing "step" terminology to "worker" where appropriate

## Acceptance
- [ ] Manager.workers uses WorkerType keys and DefinitionWorker values
- [ ] All method signatures updated
- [ ] ValidateStep calls changed to ValidateConfig
- [ ] Code compiles without errors
