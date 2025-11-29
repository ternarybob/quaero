# Task 10: Create Tests for GitHub Collectors

- Group: 10 | Mode: concurrent | Model: sonnet
- Skill: @test-automator | Critical: no | Depends: 7
- Sandbox: /tmp/3agents/task-10/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `test/api/github_jobs_test.go` - NEW: API tests for GitHub job endpoints

## Requirements

### Test Cases:

#### 1. Test_GitHubRepoPreview
```go
func Test_GitHubRepoPreview(t *testing.T) {
    // Setup test environment with GitHub connector
    // POST /api/github/repo/preview
    // Verify response contains file list
    // Verify folder paths extracted correctly
}
```

#### 2. Test_GitHubActionsPreview
```go
func Test_GitHubActionsPreview(t *testing.T) {
    // Setup test environment with GitHub connector
    // POST /api/github/actions/preview
    // Verify response contains workflow runs
    // Verify metadata (workflow_name, started_at) present
}
```

#### 3. Test_GitHubRepoCollector_Start
```go
func Test_GitHubRepoCollector_Start(t *testing.T) {
    // Setup test environment
    // POST /api/github/repo/start
    // Verify job created
    // Verify job has correct type and config
    // Wait for child jobs to be created
}
```

#### 4. Test_GitHubActionsCollector_Start
```go
func Test_GitHubActionsCollector_Start(t *testing.T) {
    // Setup test environment
    // POST /api/github/actions/start
    // Verify job created
    // Verify job has correct type and config
    // Wait for child jobs to be created
}
```

#### 5. Test_GitHubRepoCollector_DocumentCreation
```go
func Test_GitHubRepoCollector_DocumentCreation(t *testing.T) {
    // Setup and run repo collector
    // Wait for documents to be created
    // Verify document has:
    //   - Correct source_type (github_repo)
    //   - Tags from job definition
    //   - Folder in metadata
    //   - Branch in metadata
}
```

#### 6. Test_GitHubActionsCollector_DocumentMetadata
```go
func Test_GitHubActionsCollector_DocumentMetadata(t *testing.T) {
    // Setup and run actions collector
    // Wait for documents to be created
    // Verify document has:
    //   - Correct source_type (github_action_log)
    //   - workflow_name in metadata
    //   - run_started_at in metadata
    //   - run_date in metadata (YYYY-MM-DD)
    //   - conclusion in metadata
    //   - Tags include workflow conclusion
}
```

#### 7. Test_GitHubCollector_MissingConnector
```go
func Test_GitHubCollector_MissingConnector(t *testing.T) {
    // POST with invalid connector_id
    // Verify 404 or 400 error
    // Verify helpful error message
}
```

#### 8. Test_GitHubCollector_ValidationErrors
```go
func Test_GitHubCollector_ValidationErrors(t *testing.T) {
    // POST with missing owner
    // POST with missing repo
    // Verify 400 errors with descriptive messages
}
```

### Test Setup Helper:
```go
// createTestGitHubConnector creates a connector for testing
// Uses a test token or skip_validation_token for unit tests
func createTestGitHubConnector(t *testing.T, helper *common.HTTPTestHelper) string {
    body := map[string]interface{}{
        "name": "Test GitHub",
        "type": "github",
        "config": map[string]interface{}{
            "token": os.Getenv("GITHUB_TOKEN"), // Or skip_validation_token
        },
    }
    resp, err := helper.POST("/api/connectors", body)
    // ... return connector ID
}
```

### Notes:
- Tests may be skipped if no valid GITHUB_TOKEN is available
- Use a public test repository for integration tests
- Mock GitHub API responses for unit tests where possible

## Acceptance
- [ ] Test file created in test/api/
- [ ] Preview endpoint tests
- [ ] Start job endpoint tests
- [ ] Document creation tests
- [ ] Document metadata tests
- [ ] Error handling tests
- [ ] Tests handle missing GitHub token gracefully
- [ ] All tests pass
