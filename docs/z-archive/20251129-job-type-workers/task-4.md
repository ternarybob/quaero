# Task 4: Update TOML Parsing

- Group: 4 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-4/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `internal/jobs/service.go` - Update ParseTOML to handle new step format
- `internal/models/job_definition.go` - Update JobDefinitionFile struct

## Requirements

1. Update `JobDefinitionFile` to support new step format:
   ```go
   type JobStepFile struct {
       Type        string `toml:"type"`        // NEW: Required step type
       Description string `toml:"description"` // NEW: Step description
       Action      string `toml:"action"`      // DEPRECATED: Keep for migration
       OnError     string `toml:"on_error"`
       Depends     string `toml:"depends"`
       // ... type-specific fields
   }
   ```

2. Update `ParseTOML()`:
   - Parse `type` field from step config
   - If `type` not present but `action` is, log deprecation warning and use action as type
   - Validate type against known StepTypes

3. Update `ToJobDefinition()`:
   - Convert string type to `models.StepType`
   - Populate `Description` field
   - Remove redundant field mapping

4. Add migration helper that converts old format to new format

## Acceptance
- [ ] TOML parsing supports new `type` field
- [ ] Backward compatibility with `action` field (with warning)
- [ ] Description field populated
- [ ] Type validation during parsing
- [ ] Compiles: `go build ./...`
- [ ] Tests pass: `go test ./internal/jobs/...`
