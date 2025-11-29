# Task 4: Add Fallback for Large Files

- Group: 4 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 3
- Sandbox: /tmp/3agents/task-4/ | Source: C:/development/quaero/ | Output: docs/fixes/github-repo-bulk-fetch/

## Files
- `internal/connectors/github/batch_fetcher.go` - Add fallback logic
- `internal/connectors/github/repo.go` - Ensure GetFileContent works for fallback

## Requirements

### 1. Implement Fallback Detection
Identify when to use REST fallback:

```go
const (
    MaxGraphQLFileSize = 1024 * 1024  // 1MB limit for GraphQL text retrieval
    MaxBatchableFiles  = 100          // GitHub GraphQL complexity limit
)

func (bf *BatchFetcher) needsFallback(file FileMetadata) bool {
    return file.Size > MaxGraphQLFileSize
}
```

### 2. Hybrid Fetching Strategy
```go
func (bf *BatchFetcher) FetchFilesHybrid(ctx context.Context, owner, repo, branch string, files []FileMetadata) (*BatchResult, error) {
    // 1. Categorize files
    batchable, oversized := bf.categorizeFiles(files)

    // 2. Bulk fetch batchable files via GraphQL
    batchResults := bf.fetchBatch(ctx, owner, repo, branch, batchable)

    // 3. Fetch oversized files via REST (concurrent)
    restResults := bf.fetchOversizedFiles(ctx, owner, repo, branch, oversized)

    // 4. Merge results
    return bf.mergeResults(batchResults, restResults)
}
```

### 3. Concurrent REST Fallback
Fetch oversized files concurrently with controlled parallelism:
```go
func (bf *BatchFetcher) fetchOversizedFiles(ctx context.Context, owner, repo, branch string, files []FileMetadata) []BulkFileResult {
    // Use worker pool with semaphore for rate limiting
    // Default: 5 concurrent REST requests
}
```

### 4. Error Recovery
If GraphQL batch fails completely, fall back to REST for all files:
```go
func (bf *BatchFetcher) fetchWithFallback(ctx context.Context, ...) (*BatchResult, error) {
    result, err := bf.fetchBatch(ctx, ...)
    if err != nil {
        log.Warn("GraphQL batch failed, falling back to REST", "error", err)
        return bf.fetchAllViaREST(ctx, ...)
    }
    return result, nil
}
```

### 5. Binary File Handling
GraphQL returns isBinary flag - skip content for binary files:
```go
if result.IsBinary {
    // Store metadata only, content = "[Binary file - content not stored]"
}
```

## Acceptance
- [ ] Files >1MB fetched via REST API
- [ ] REST fallback runs concurrently (5 workers)
- [ ] GraphQL failures trigger full REST fallback
- [ ] Binary files handled gracefully
- [ ] Results merged correctly from both sources
- [ ] Compiles: `go build ./...`
- [ ] Tests pass
