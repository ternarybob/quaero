# Task 1: Rename StepType to WorkerType in models

- Group: 1 | Mode: sequential | Model: opus
- Skill: @golang-pro | Critical: yes:architectural-change | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: ./docs/feature/20251129-remove-step-job-manager/

## Files
- `internal/models/step_type.go` - Rename to worker_type.go
- `internal/models/job_definition.go` - Update JobStep.Type field

## Requirements
1. Rename `internal/models/step_type.go` to `internal/models/worker_type.go`
2. Rename `StepType` type to `WorkerType`
3. Rename all `StepType*` constants to `WorkerType*`:
   - StepTypeAgent → WorkerTypeAgent
   - StepTypeCrawler → WorkerTypeCrawler
   - StepTypePlacesSearch → WorkerTypePlacesSearch
   - StepTypeWebSearch → WorkerTypeWebSearch
   - StepTypeGitHubRepo → WorkerTypeGitHubRepo
   - StepTypeGitHubActions → WorkerTypeGitHubActions
   - StepTypeTransform → WorkerTypeTransform
   - StepTypeReindex → WorkerTypeReindex
   - StepTypeDatabaseMaintenance → WorkerTypeDatabaseMaintenance
4. Rename `AllStepTypes()` to `AllWorkerTypes()`
5. Update JobStep.Type field type from StepType to WorkerType

## Acceptance
- [ ] step_type.go renamed to worker_type.go
- [ ] All StepType references renamed to WorkerType
- [ ] JobStep.Type field uses WorkerType
- [ ] Code compiles without errors
