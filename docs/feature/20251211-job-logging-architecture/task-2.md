# Task 2: Enhance Tree API to Include Backend-Driven Expansion State
Depends: 1 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
"The json should be structured in such a way as to be able to re-render a running job keep the front code and output simple"

## Skill Patterns to Apply
From go/SKILL.md:
- Wrap errors with context using fmt.Errorf + %w
- Use structured logging with arbor
- Keep handlers thin (business logic in services)

## Do
- Modify GetJobTreeHandler in internal/handlers/job_handler.go
- Enhance JobTreeStep struct to include `Expanded` field based on:
  - Step has logs AND (status == "running" OR status == "failed") → expanded = true
  - Step is current_step from metadata → expanded = true
  - Otherwise → expanded = false
- Ensure tree response includes all necessary data for UI rendering

## Accept
- [ ] JobTreeStep includes `Expanded` field computed by backend
- [ ] Expansion logic considers: hasLogs, status, isCurrentStep
- [ ] Code compiles without errors
- [ ] Existing tests pass
