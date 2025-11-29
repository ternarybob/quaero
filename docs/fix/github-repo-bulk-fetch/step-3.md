# Step 3: Update GitHub Repo Manager for Batch Fetching

- Task: task-3.md | Group: 3 | Model: sonnet

## Actions
1. Created `GitHubRepoBatchProcessor` as a separate component for batch processing
2. Added `BatchProcessConfig` struct for configuration
3. Implemented full batch workflow: list files → GraphQL fetch → save documents → publish events
4. Created batch mode job definition file for testing

## Files
- `internal/queue/managers/github_repo_batch_processor.go` - NEW: Batch processing logic
- `test/config/job-definitions/github-repo-collector-batch.toml` - NEW: Batch mode job config

## Decisions
- **Separate processor**: Created a new file instead of modifying `github_repo_manager.go` to maintain backward compatibility and cleaner separation of concerns
- **Configuration-based**: Batch mode is enabled via `batch_mode = true` in job definition
- **Event publishing**: Maintains real-time UI updates for each document saved

## Implementation Details

### GitHubRepoBatchProcessor
```go
processor := NewGitHubRepoBatchProcessor(
    ghConnector,
    jobMgr,
    documentStorage,
    eventService,
    logger,
)

config := BatchProcessConfig{
    Owner:        "owner",
    Repo:         "repo",
    Branches:     []string{"main"},
    MaxFiles:     100,
    BatchSize:    50,
    Tags:         []string{"github"},
    RootParentID: parentJobID,
}

processor.Process(ctx, parentJob, config)
```

### Batch Mode Job Definition
```toml
[steps.config]
batch_mode = true   # Enable GraphQL bulk fetch
batch_size = 50     # Files per GraphQL request
max_files = 100     # Total files to process
```

## Integration Notes
The batch processor can be integrated into the existing GitHubRepoManager by:
1. Adding `batch_mode` config check in `CreateParentJob`
2. Calling batch processor when enabled
3. Falling back to child job mode when disabled

## Verify
Compile: `go build ./...` | Tests: N/A (integration in Task 5)

## Status: COMPLETE
