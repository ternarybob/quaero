# Task 5: Update Tests for Bulk Fetch

- Group: 5 | Mode: sequential | Model: sonnet
- Skill: @test-automator | Critical: no | Depends: 3, 4
- Sandbox: /tmp/3agents/task-5/ | Source: C:/development/quaero/ | Output: docs/fixes/github-repo-bulk-fetch/

## Files
- `test/ui/github_jobs_test.go` - Update existing tests
- `internal/connectors/github/batch_fetcher_test.go` - NEW: Unit tests for batch fetcher
- `internal/connectors/github/graphql_test.go` - NEW: Unit tests for GraphQL client

## Requirements

### 1. Unit Tests for GraphQL Client
```go
func TestBulkGetFileContent(t *testing.T) {
    // Test single file fetch
    // Test multiple files fetch
    // Test handling of missing files
    // Test binary file detection
}
```

### 2. Unit Tests for BatchFetcher
```go
func TestBatchFetcher_Categorization(t *testing.T) {
    // Test files are correctly split into batchable vs oversized
}

func TestBatchFetcher_BatchSizing(t *testing.T) {
    // Test 50 files = 1 batch
    // Test 100 files = 2 batches
    // Test mixed sizes
}

func TestBatchFetcher_Fallback(t *testing.T) {
    // Test REST fallback for large files
    // Test full fallback on GraphQL error
}
```

### 3. Integration Tests
Update existing github_jobs_test.go:
```go
func TestGitHubRepoCollector_BatchMode(t *testing.T) {
    // Test full workflow with batch_mode=true
    // Verify documents created correctly
    // Verify performance improvement logged
}

func TestGitHubRepoCollector_BackwardCompatibility(t *testing.T) {
    // Test existing child job flow still works
    // Test batch_mode=false uses old path
}
```

### 4. Mock GitHub API Responses
Create test fixtures for:
- GraphQL bulk response with multiple files
- GraphQL partial failure response
- REST single file response
- Rate limit response

### 5. Performance Comparison Test
```go
func TestBatchMode_Performance(t *testing.T) {
    // Compare time for 50 files: batch vs sequential
    // Log timing results (not strict assertion)
}
```

## Acceptance
- [ ] Unit tests for GraphQL client methods
- [ ] Unit tests for BatchFetcher logic
- [ ] Integration tests for full workflow
- [ ] Backward compatibility tests pass
- [ ] Mocks cover all API scenarios
- [ ] All tests pass: `go test ./...`
