# Task 1: Update JobStep Model

- Group: 1 | Mode: sequential | Model: opus
- Skill: @golang-pro | Critical: yes:architectural-change | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `internal/models/job_definition.go` - Add Type field to JobStep, update validation
- `internal/models/step_type.go` - NEW: Define StepType enum constants

## Requirements

1. Add `Type` field to `JobStep` struct as `StepType`
2. Create new `StepType` type with constants:
   - `StepTypeAgent`
   - `StepTypeCrawler`
   - `StepTypePlacesSearch`
   - `StepTypeWebSearch`
   - `StepTypeGitHubRepo`
   - `StepTypeGitHubActions`
   - `StepTypeTransform`
   - `StepTypeReindex`
   - `StepTypeDatabaseMaintenance`
3. Add `Description` field to JobStep (replacing action's descriptive role)
4. Keep `Action` field temporarily for backward compatibility during migration
5. Update `JobStep.Validate()` to require Type field
6. Add `StepType.IsValid()` validation method

## Acceptance
- [ ] JobStep has Type field of type StepType
- [ ] StepType enum defined with all worker types
- [ ] Description field added to JobStep
- [ ] Validation updated to require Type
- [ ] Compiles: `go build ./...`
- [ ] Tests pass: `go test ./internal/models/...`
