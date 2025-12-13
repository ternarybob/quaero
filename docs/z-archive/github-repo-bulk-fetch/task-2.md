# Task 2: Create Batch Fetcher Service

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: C:/development/quaero/ | Output: docs/fixes/github-repo-bulk-fetch/

## Files
- `internal/connectors/github/batch_fetcher.go` - NEW: Batch fetching logic
- `internal/connectors/github/repo.go` - Integration with existing connector

## Requirements

### 1. Create BatchFetcher Service
Create a service that groups files into optimal batches:

```go
type BatchFetcher struct {
    connector     *GitHubConnector
    batchSize     int  // Default: 50 files per GraphQL request
    maxFileSize   int  // Files larger than this use REST fallback
}

func NewBatchFetcher(connector *GitHubConnector) *BatchFetcher
```

### 2. Implement File Batching Logic
```go
func (bf *BatchFetcher) FetchFiles(ctx context.Context, owner, repo, branch string, files []FileMetadata) ([]Document, error)
```

- Accept list of file metadata from ListFiles()
- Group files into batches of 50
- Execute GraphQL bulk fetch for each batch
- Collect results and create Document models
- Track progress for logging

### 3. Handle File Size Filtering
Files over 1MB should be flagged for REST API fallback:
```go
func (bf *BatchFetcher) categorizeFiles(files []FileMetadata) (batchable []FileMetadata, oversized []FileMetadata)
```

### 4. Create Result Aggregation
```go
type BatchResult struct {
    Documents []Document
    Errors    []FileError
    Stats     BatchStats
}

type BatchStats struct {
    TotalFiles     int
    SuccessCount   int
    ErrorCount     int
    BytesFetched   int64
    Duration       time.Duration
}
```

### 5. Progress Callback Support
Allow caller to track progress:
```go
type ProgressCallback func(processed, total int, currentBatch int)

func (bf *BatchFetcher) FetchFilesWithProgress(ctx context.Context, owner, repo, branch string, files []FileMetadata, progress ProgressCallback) (*BatchResult, error)
```

## Acceptance
- [ ] BatchFetcher groups files into configurable batch sizes
- [ ] Large files (>1MB) are separated for REST fallback
- [ ] Progress callback reports batch completion
- [ ] Returns aggregated results with statistics
- [ ] Handles partial failures gracefully
- [ ] Compiles: `go build ./...`
- [ ] Tests pass
