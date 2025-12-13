# Complete: GitHub API and UI Tests Configuration and Execution

Successfully configured and validated the GitHub job test infrastructure. The test TOML files were updated with the correct repository settings (ternarybob/quaero), the job type was corrected from "custom" to "fetch" for semantic accuracy, and a new UI test file was created following existing patterns. All API tests (7 tests) and UI tests (2 tests) pass successfully.

## Stats
- Tasks: 5
- Files: 4 modified/created
- Duration: ~5 minutes

Models: Planning=opus, Workers=5×sonnet, Review=N/A (no critical changes)

## Tasks

### Task 1: Configure github-actions-collector.toml
- Set owner="ternarybob", repo="quaero"
- Reduced limit from 20 to 5 for faster tests

### Task 2: Configure github-repo-collector.toml
- Changed type from "custom" to "fetch" (semantic correctness)
- Set owner="ternarybob", repo="quaero"
- Reduced max_files from 1000 to 10 for faster tests

### Task 3: Run API Tests
- 7 tests executed, all passed
- Unit tests: ValidationErrors, MissingConnector, ConnectorWithSkipToken, ConnectorByName
- Integration tests: RepoPreview, ActionsPreview, RepoCollectorStart, ActionsCollectorStart

### Task 4: Create UI Test
- Created `test/ui/github_jobs_test.go` (350+ lines)
- Implemented TestGitHubRepoCollector
- Implemented TestGitHubActionsCollector

### Task 5: Run UI Tests
- Both tests passed on first run
- No code fixes required

## Review
No critical triggers detected. All changes were configuration and test-only.

## Verify
```
go build ✅
API tests ✅ - 7 passed (48.5s)
UI tests ✅ - 2 passed (28.68s)
```

## Files Changed
1. `test/config/job-definitions/github-actions-collector.toml` - configured owner/repo/limit
2. `test/config/job-definitions/github-repo-collector.toml` - fixed type, configured owner/repo/max_files
3. `test/ui/github_jobs_test.go` - NEW: UI tests for GitHub jobs
