# Step 2: Create Batch Fetcher Service

- Task: task-2.md | Group: 2 | Model: sonnet

## Actions
1. Created `BatchFetcher` struct with configurable batch size and max file size
2. Implemented file categorization (batchable vs oversized)
3. Implemented GraphQL batch fetching with automatic batching
4. Implemented REST fallback for oversized files with concurrent execution
5. Added progress callback support
6. Created document creation helper

## Files
- `internal/connectors/github/batch_fetcher.go` - NEW: Complete batch fetching implementation

## Decisions
- **Default batch size 50**: Conservative default to avoid GraphQL complexity limits
- **Default max file size 1MB**: Files over 1MB use REST fallback (GraphQL text field limitation)
- **5 concurrent REST requests**: Rate-limited fallback to avoid overwhelming API
- **Auto-fallback on batch failure**: If GraphQL batch fails, falls back to REST for that batch

## Implementation Details

### BatchFetcher API
```go
bf := NewBatchFetcher(connector).
    WithBatchSize(50).
    WithMaxFileSize(1024 * 1024)

result, err := bf.FetchFilesWithProgress(ctx, owner, repo, branch, files, progressCallback)
```

### BatchResult Structure
```go
type BatchResult struct {
    Documents []*models.Document
    Errors    []FileError
    Stats     BatchStats
}

type BatchStats struct {
    TotalFiles    int
    SuccessCount  int
    ErrorCount    int
    BytesFetched  int64
    Duration      time.Duration
    BatchCount    int
    FallbackCount int
}
```

### Flow
1. Categorize files into batchable (<1MB) and oversized (>=1MB)
2. Split batchable files into batches of 50
3. For each batch: GraphQL bulk fetch (with REST fallback on error)
4. For oversized: Concurrent REST fetch (5 workers)
5. Aggregate results and stats

## Verify
Compile: `go build ./...` | Tests: N/A (new code, tests in Task 5)

## Status: COMPLETE
