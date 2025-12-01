# Complete: Add connector_name support and tests for GitHub jobs

This task added support for resolving GitHub connectors by name (in addition to ID) in job definitions. Previously, jobs could only reference connectors using `connector_id = "{github_connector_id}"` which required storing the connector ID in the KV store. Now jobs can use `connector_name = "Test GitHub Connector"` to resolve connectors directly by their display name, providing a more user-friendly configuration option.

## Stats
Tasks: 5 | Files: 5 | Duration: ~15 minutes
Models: Planning=opus, Workers=sonnet, Review=N/A (not critical)

## Tasks
- Task 1: Updated `github_repo_manager.go` to support `connector_name` as alternative to `connector_id`
- Task 2: Updated `github_actions_manager.go` with same connector_name support
- Task 3: Created `github-repo-collector-by-name.toml` config using connector_name
- Task 4: Added `TestGitHubRepoCollectorByName` test with helper for name-based connector creation
- Task 5: Validated all tests pass (3/3)

## Changes Made

### Manager Updates
- `internal/queue/managers/github_repo_manager.go`:
  - Added `connector_name` config extraction
  - Changed validation to accept either `connector_id` OR `connector_name`
  - Added `GetConnectorByName()` lookup when using connector_name
  - Resolved connector ID stored for downstream metadata consistency

- `internal/queue/managers/github_actions_manager.go`:
  - Same changes as github_repo_manager for consistency

### Test Config
- `test/config/job-definitions/github-repo-collector-by-name.toml`:
  - New job definition using `connector_name = "Test GitHub Connector"`
  - Unique ID: `github-repo-collector-by-name`
  - Same owner/repo settings for consistency

### Test Code
- `test/ui/github_jobs_test.go`:
  - Added `createGitHubConnectorWithoutKV()` helper
  - Added `TestGitHubRepoCollectorByName` test function

## Verify
go build ✅ | go test ✅ 3 passed

| Test | Status | Documents |
|------|--------|-----------|
| TestGitHubRepoCollector | ✅ PASS | 997 |
| TestGitHubActionsCollector | ✅ PASS | 10 |
| TestGitHubRepoCollectorByName | ✅ PASS | 976 |
