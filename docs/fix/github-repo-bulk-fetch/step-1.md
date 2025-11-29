# Step 1: Add GraphQL Client to GitHub Connector

- Task: task-1.md | Group: 1 | Model: sonnet

## Actions
1. Added `token` field to `Connector` struct to store authentication token
2. Created `graphql.go` with GraphQL client implementation
3. Implemented `BulkGetFileContent` method for bulk file fetching
4. Created dynamic GraphQL query builder for multiple file aliases

## Files
- `internal/connectors/github/connector.go` - Added token field, updated NewConnector
- `internal/connectors/github/graphql.go` - NEW: GraphQL bulk fetch implementation

## Decisions
- **No external dependency**: Used raw HTTP client instead of adding shurcooL/graphql library to minimize dependencies
- **Dynamic query building**: Built GraphQL queries dynamically with aliases (f0, f1, f2...) to support variable file counts
- **Batch limit 100**: Set maximum batch size to 100 files per request (GitHub complexity limits)

## Implementation Details

### GraphQL Query Structure
```graphql
query BulkFileContent {
  repository(owner: "owner", name: "repo") {
    f0: object(expression: "branch:path/file1.go") {
      ... on Blob { text byteSize isBinary }
    }
    f1: object(expression: "branch:path/file2.go") { ... }
  }
}
```

### BulkFileResult Type
```go
type BulkFileResult struct {
    Path     string
    Content  string
    Size     int
    IsBinary bool
    Error    error  // Per-file error handling
}
```

## Verify
Compile: `go build ./...` | Tests: N/A (new code, tests in Task 5)

## Status: COMPLETE
