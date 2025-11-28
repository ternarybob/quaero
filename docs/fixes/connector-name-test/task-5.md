# Task 5: Run tests and validate

- Group: 4 | Mode: sequential | Model: sonnet
- Skill: @test-runner | Critical: no | Depends: 1,2,3,4
- Sandbox: /tmp/3agents/task-5/ | Source: . | Output: docs/fixes/connector-name-test/

## Files
- None (validation only)

## Requirements
1. Build project: `go build ./...`
2. Run existing GitHub tests to ensure no regression
3. Run new TestGitHubRepoCollectorByName test
4. Verify all tests pass

## Acceptance
- [ ] Build succeeds
- [ ] Existing tests still pass
- [ ] New test passes
- [ ] Documents collected > 0
