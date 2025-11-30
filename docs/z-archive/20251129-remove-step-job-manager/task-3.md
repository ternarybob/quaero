# Task 3: Remove StepManager interface

- Group: 3 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-3/ | Source: ./ | Output: ./docs/feature/20251129-remove-step-job-manager/

## Files
- `internal/interfaces/job_interfaces.go` - Remove StepManager interface

## Requirements
1. Remove the `StepManager` interface entirely (unused, superseded by StepWorker/DefinitionWorker)
2. Update comments to clarify the relationship between JobWorker and DefinitionWorker

## Acceptance
- [ ] StepManager interface removed
- [ ] Comments clarify interface relationships
- [ ] Code compiles without errors
