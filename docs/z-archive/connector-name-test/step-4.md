# Step 4: Add TestGitHubRepoCollectorByName test

- Task: task-4.md | Group: 3 | Model: sonnet

## Actions
1. Added `createGitHubConnectorWithoutKV()` helper - creates connector but doesn't store ID in KV
2. Added `TestGitHubRepoCollectorByName` test function
3. Test creates connector with name "Test GitHub Connector"
4. Test triggers job "GitHub Repository Collector (By Name)"
5. Test monitors for completion and verifies document_count > 0

## Files
- `test/ui/github_jobs_test.go` - added new test and helper

## Decisions
- Separate helper function to avoid storing connector ID in KV
- Same monitoring logic as existing tests for consistency
- Clear logging to show connector_name resolution is being tested

## Verify
Compile: ⏳ | Tests: ⏳

## Status: ✅ COMPLETE
