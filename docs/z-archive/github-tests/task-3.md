# Task 3: Run API Tests

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1,2
- Sandbox: /tmp/3agents/task-3/ | Source: ./ | Output: docs/fixes/github-tests/

## Files
- `test/api/github_jobs_test.go` - run existing tests

## Requirements
1. Set GITHUB_TOKEN environment variable from test config
2. Run `go test -v ./test/api/... -run TestGitHub -timeout 5m`
3. Capture test output
4. Identify any failures for code fixes

## Acceptance
- [ ] Tests run successfully
- [ ] All tests pass or failures identified for code fixes
- [ ] No compilation errors
