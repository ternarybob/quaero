# Step 4: Create UI Test for GitHub Jobs

- Task: task-4.md | Group: 3 | Model: sonnet

## Actions
1. Created new test file `test/ui/github_jobs_test.go`
2. Implemented `githubTestContext` struct for shared test state
3. Implemented `createGitHubConnector()` to create connector using token from `.env.test`
4. Implemented `triggerJob()` to trigger jobs via UI (matches queue_test.go pattern)
5. Implemented `monitorJob()` to monitor job completion (matches queue_test.go pattern)
6. Created `TestGitHubRepoCollector` test function
7. Created `TestGitHubActionsCollector` test function

## Files
- `test/ui/github_jobs_test.go` - created new file (350+ lines)

## Decisions
- Used same patterns as queue_test.go for consistency
- Created connector via API and stored ID in KV store for job definitions to use
- Set 3-minute timeout for job monitoring (reasonable for small file/run counts)

## Verify
Compile: ✅ | Tests: pending

## Status: ✅ COMPLETE
