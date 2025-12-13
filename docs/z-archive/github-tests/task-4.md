# Task 4: Create UI Test for GitHub Jobs

- Group: 3 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1,2
- Sandbox: /tmp/3agents/task-4/ | Source: ./ | Output: docs/fixes/github-tests/

## Files
- `test/ui/github_jobs_test.go` - create new file based on queue_test.go template

## Requirements
1. Create new UI test file `test/ui/github_jobs_test.go`
2. Use `test/ui/queue_test.go` as template
3. Create test functions:
   - `TestGitHubRepoCollector` - triggers and monitors GitHub Repository Collector job
   - `TestGitHubActionsCollector` - triggers and monitors GitHub Actions Log Collector job
4. Use chromedp for browser automation
5. Monitor job completion using existing `monitorJob` pattern

## Acceptance
- [ ] New test file created
- [ ] TestGitHubRepoCollector function implemented
- [ ] TestGitHubActionsCollector function implemented
- [ ] Compiles with `go build ./test/ui/...`
- [ ] Tests follow existing patterns from queue_test.go
