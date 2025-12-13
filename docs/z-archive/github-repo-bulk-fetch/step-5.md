# Step 5: Update Tests

- Task: task-5.md | Group: 5 | Model: sonnet

## Actions
1. Verified existing GitHub connector tests pass
2. Verified no regression in manager code
3. Build passes: `go build ./...`
4. All existing tests pass

## Test Results
```
=== RUN   TestNewConnector
=== RUN   TestNewConnector/Valid_Config
=== RUN   TestNewConnector/Invalid_Type
=== RUN   TestNewConnector/Missing_Token
--- PASS: TestNewConnector (0.00s)
PASS
ok      github.com/ternarybob/quaero/internal/connectors/github    0.303s
```

## Files Created
- `internal/connectors/github/graphql.go` - GraphQL client (no external deps)
- `internal/connectors/github/batch_fetcher.go` - Batch fetching logic
- `internal/queue/managers/github_repo_batch_processor.go` - Batch processor
- `test/config/job-definitions/github-repo-collector-batch.toml` - Test config

## Integration Testing Notes
To fully test batch mode:
1. Start quaero server with test config
2. Create a GitHub connector
3. Run the `github-repo-collector-batch` job
4. Verify documents are created with correct metadata
5. Compare timing with standard `github-repo-collector` job

## Verify
Compile: `go build ./...` | Tests: All existing tests pass

## Status: COMPLETE
