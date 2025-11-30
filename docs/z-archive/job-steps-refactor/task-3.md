# Task 3: Update Step Validation

- Group: 3 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-3/ | Source: ./ | Output: ./docs/feature/job-steps-refactor/

## Files
- `internal/models/job_definition.go` - Update ValidateStep function

## Requirements

1. Update `ValidateStep` to handle flat config:
   - Look for `agent_type` directly in Config
   - Look for `filter_limit`, `filter_tags`, etc. in Config
   - Remove nested `document_filter` validation (now flat)

2. Add `Depends` validation:
   - If `depends` is set, validate referenced step names exist in job
   - Detect circular dependencies
   - Allow empty `depends` (no dependencies)

3. Update agent-specific validation:
   - `agent_type` in Config root (not nested)
   - `filter_*` fields instead of `document_filter.*`

## Acceptance
- [ ] ValidateStep handles flat config structure
- [ ] Depends field is validated (step exists, no cycles)
- [ ] Agent job validation works with new format
- [ ] Compiles: `go build ./...`
