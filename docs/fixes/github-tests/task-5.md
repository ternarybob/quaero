# Task 5: Run UI Tests and Iterate

- Group: 4 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 3,4
- Sandbox: /tmp/3agents/task-5/ | Source: ./ | Output: docs/fixes/github-tests/

## Files
- `test/ui/github_jobs_test.go` - run tests
- Various source files - fix code issues (not tests)

## Requirements
1. Run `go test -v ./test/ui/... -run TestGitHub -timeout 10m`
2. Capture test output
3. If tests fail, identify code issues (not test issues)
4. Fix application code to make tests pass
5. Iterate until tests pass

## Acceptance
- [ ] UI tests run successfully
- [ ] All tests pass
- [ ] Code fixes documented
- [ ] No compilation errors
