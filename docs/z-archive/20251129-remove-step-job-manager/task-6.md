# Task 6: Update app.go worker registration

- Group: 6 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 4,5
- Sandbox: /tmp/3agents/task-6/ | Source: ./ | Output: ./docs/feature/20251129-remove-step-job-manager/

## Files
- `internal/app/app.go` - Update worker registration calls

## Requirements
1. Update any references to StepWorker type assertions
2. Ensure worker registration still works with new interface names
3. Update comments if any reference old terminology

## Acceptance
- [ ] Worker registration updated for new interface names
- [ ] Code compiles without errors
