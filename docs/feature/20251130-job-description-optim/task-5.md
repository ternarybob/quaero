# Task 5: Run queue tests and validate
- Group: 5 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2,3,4
- Sandbox: /tmp/3agents/task-5/ | Source: ./ | Output: ./docs/feature/20251130-job-description-optim/

## Files
- Queue test files (read-only validation)

## Requirements
1. Run the queue tests: `go test ./test/api/... -v -run Queue`
2. If tests fail, analyze failures and fix job configurations
3. Iterate until tests pass

## Acceptance
- [ ] Queue tests pass
- [ ] All job definitions load correctly
- [ ] No parsing errors for TOML files
