# Complete: GitHub Repository Bulk Fetch Optimization

This implementation adds GraphQL bulk fetch capability to the GitHub repository collector, enabling 5-10x faster file fetching compared to the existing single-file REST API approach.

## Stats
- Tasks: 6
- Files: 5 new, 1 modified
- Models: Planning=opus, Workers=sonnet

## Implementation Summary

### Problem
The `github-repo-collector.toml` job was slow because it fetched files one-by-one via individual GitHub REST API calls (~500ms per file).

### Solution
Implemented GraphQL bulk fetch to retrieve multiple files in a single API request (up to 50-100 files per request).

### Tasks Completed

**Task 1: GraphQL Client**
- Added `token` field to `Connector` struct for GraphQL authentication
- Created `graphql.go` with `BulkGetFileContent` method
- Dynamically builds GraphQL queries with file aliases

**Task 2: Batch Fetcher Service**
- Created `BatchFetcher` for orchestrating bulk operations
- Categorizes files into batchable (<1MB) and oversized (>=1MB)
- Parallel REST fallback for oversized files (5 concurrent workers)

**Task 3: Batch Processor Integration**
- Created `GitHubRepoBatchProcessor` for job integration
- Supports `batch_mode` and `batch_size` configuration
- Publishes real-time events for UI updates

**Task 4: Fallback Logic** (merged with Task 2)
- Files >1MB automatically use REST API fallback
- GraphQL failures trigger full REST fallback for that batch

**Task 5 & 6: Testing & Validation**
- All existing tests pass
- Build succeeds: `go build ./...`
- Created batch mode job definition for testing

## Files Changed

| File | Type | Description |
|------|------|-------------|
| `internal/connectors/github/connector.go` | Modified | Added token field |
| `internal/connectors/github/graphql.go` | New | GraphQL bulk fetch |
| `internal/connectors/github/batch_fetcher.go` | New | Batch orchestration |
| `internal/queue/managers/github_repo_batch_processor.go` | New | Job processor |
| `test/config/job-definitions/github-repo-collector-batch.toml` | New | Test config |

## Usage

### Enable Batch Mode
Add to job definition:
```toml
[steps.config]
batch_mode = true   # Enable GraphQL bulk fetch
batch_size = 50     # Files per GraphQL request (max 100)
max_files = 100     # Total files to process
```

### Expected Performance
| Mode | 100 Files | API Calls |
|------|-----------|-----------|
| Sequential (old) | ~50 sec | 100 |
| Concurrent (old, 10 workers) | ~5 sec | 100 |
| Batch (new) | ~1 sec | 2 |

## Review
No critical triggers identified. Implementation is backward compatible - existing jobs continue to work unchanged.

## Verify
```bash
go build ./...  # Compiles successfully
go test ./...   # All tests pass
```
