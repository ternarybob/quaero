# Task 1: Update JobStep Model

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: ./docs/feature/job-steps-refactor/

## Files
- `internal/models/job_definition.go` - Add Depends field to JobStep struct

## Requirements

1. Add `Depends` field to `JobStep` struct:
   - Type: `string` (comma-separated list of dependent step names)
   - JSON tag: `json:"depends,omitempty"`
   - Used for step execution ordering

2. Keep existing fields:
   - `Name` - step identifier
   - `Action` - action type
   - `Config` - step-specific config (will be flat after parsing)
   - `OnError` - error handling strategy
   - `Condition` - optional conditional execution

## Acceptance
- [ ] JobStep struct has Depends field
- [ ] Compiles: `go build ./...`
- [ ] Tests pass
