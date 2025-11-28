# Task 4: Add TestGitHubRepoCollectorByName test

- Group: 3 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1,3
- Sandbox: /tmp/3agents/task-4/ | Source: . | Output: docs/fixes/connector-name-test/

## Files
- `test/ui/github_jobs_test.go` - add new test function

## Requirements
1. Add `TestGitHubRepoCollectorByName` test function
2. Test uses connector_name workflow (no KV store for connector ID)
3. Still creates connector via API (same as existing tests)
4. Triggers job "GitHub Repository Collector (By Name)"
5. Monitors for completion with document_count > 0
6. Keep test structure consistent with existing tests

## Acceptance
- [ ] Test creates connector with known name
- [ ] Test triggers correct job
- [ ] Test verifies documents collected
- [ ] Compiles
- [ ] Test passes
