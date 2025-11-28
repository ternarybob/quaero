# Task 3: Update GitHub Repo Manager for Batch Fetching

- Group: 3 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1, 2
- Sandbox: /tmp/3agents/task-3/ | Source: C:/development/quaero/ | Output: docs/fixes/github-repo-bulk-fetch/

## Files
- `internal/queue/managers/github_repo_manager.go` - Modify to use batch fetching
- `internal/queue/workers/github_repo_worker.go` - May need updates for compatibility

## Requirements

### 1. Add Batch Fetch Mode to Manager
Update CreateParentJob to support batch fetching:

```go
type GitHubRepoManager struct {
    // existing fields...
    batchFetcher *github.BatchFetcher
    useBatchMode bool  // Toggle between old and new behavior
}
```

### 2. Implement Batch Processing Path
When batch mode is enabled:
```go
func (m *GitHubRepoManager) processBatch(ctx context.Context, job *models.QueueJob, files []github.FileMetadata) error {
    // 1. Use BatchFetcher to get all file contents
    // 2. Create documents directly (no child jobs)
    // 3. Save documents to storage
    // 4. Update job progress
}
```

### 3. Maintain Backward Compatibility
Keep existing child job flow for:
- Repos exceeding batch limits
- Configuration option to disable batch mode
- Fallback when GraphQL fails

```go
func (m *GitHubRepoManager) shouldUseBatchMode(fileCount int) bool {
    return m.useBatchMode && fileCount <= MaxBatchFiles
}
```

### 4. Update Job Progress Tracking
Modify progress updates for batch mode:
```go
// Old: Progress per child job completion
// New: Progress per batch completion or per document saved
```

### 5. Configuration Support
Add config option in job definition:
```toml
[steps.config]
batch_mode = true  # Enable GraphQL batch fetching
batch_size = 50    # Files per GraphQL request
```

## Acceptance
- [ ] Batch mode can be enabled via job config
- [ ] Manager uses BatchFetcher when batch_mode=true
- [ ] Falls back to child jobs when batch mode fails
- [ ] Progress tracking works in both modes
- [ ] Existing child job flow unchanged when batch_mode=false
- [ ] Compiles: `go build ./...`
- [ ] Tests pass
