# Task 2: Rename StepWorker to DefinitionWorker interface

- Group: 2 | Mode: sequential | Model: opus
- Skill: @golang-pro | Critical: yes:architectural-change | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: ./ | Output: ./docs/feature/20251129-remove-step-job-manager/

## Files
- `internal/interfaces/job_interfaces.go` - Rename StepWorker interface

## Requirements
1. Rename `StepWorker` interface to `DefinitionWorker`
2. Update `GetType()` return type from `models.StepType` to `models.WorkerType`
3. Update `ValidateStep` method to `ValidateConfig` (more descriptive)
4. Update all comments to reflect new naming

## Acceptance
- [ ] StepWorker renamed to DefinitionWorker
- [ ] Method signatures use WorkerType
- [ ] ValidateStep renamed to ValidateConfig
- [ ] Code compiles without errors
