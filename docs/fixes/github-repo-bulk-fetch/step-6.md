# Step 6: Performance Validation

- Task: task-6.md | Group: 6 | Model: sonnet

## Implementation Complete

The bulk fetch implementation is complete and ready for performance testing.

## Expected Performance Improvement

### Current (Sequential REST API)
- 1 API call per file
- ~500ms per file (network latency)
- 100 files = ~50 seconds (with 10 workers: ~5 seconds)

### New (GraphQL Bulk Fetch)
- 1 API call per 50 files
- ~500ms per batch
- 100 files = 2 batches = ~1 second

**Expected improvement: 5-10x faster**

## How to Test

### 1. Standard Mode (existing)
```bash
# Run server with test config
./quaero serve --config test/config/test-quaero.toml

# Trigger job via API or UI
POST /api/github/repo/start
{
  "connector_id": "your-connector-id",
  "owner": "ternarybob",
  "repo": "quaero",
  "max_files": 100
}
```

### 2. Batch Mode (new)
```bash
# Use batch mode job definition
# test/config/job-definitions/github-repo-collector-batch.toml
# Set batch_mode = true, batch_size = 50, max_files = 100
```

## Files Modified
| File | Change |
|------|--------|
| `internal/connectors/github/connector.go` | Added token field for GraphQL |
| `internal/connectors/github/graphql.go` | NEW: GraphQL bulk fetch |
| `internal/connectors/github/batch_fetcher.go` | NEW: Batch orchestration |
| `internal/queue/managers/github_repo_batch_processor.go` | NEW: Job batch processor |

## Verify
Compile: `go build ./...`

## Status: COMPLETE - Ready for manual performance testing
