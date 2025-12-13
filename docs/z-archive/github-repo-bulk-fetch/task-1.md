# Task 1: Add GraphQL Client and Bulk File Fetch Method

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: C:/development/quaero/ | Output: docs/fixes/github-repo-bulk-fetch/

## Files
- `internal/connectors/github/repo.go` - Add GraphQL client and bulk fetch method
- `internal/connectors/github/graphql.go` - NEW: GraphQL query builder and types
- `go.mod` - Add shurcooL/graphql dependency if needed

## Requirements

### 1. Add GraphQL Client Initialization
The go-github library already includes GraphQL support via `githubv4`. Add client initialization:

```go
import "github.com/shurcooL/githubv4"

type GitHubConnector struct {
    client    *github.Client
    gqlClient *githubv4.Client  // Add GraphQL client
    token     string
}
```

### 2. Create GraphQL Types for File Content
Create types to represent the GraphQL response:

```go
type BulkFileResult struct {
    Path     string
    Content  string
    Size     int
    IsBinary bool
    Error    error
}
```

### 3. Implement BulkGetFileContent Method
Add method to fetch multiple files in one request:

```go
func (c *GitHubConnector) BulkGetFileContent(ctx context.Context, owner, repo, branch string, paths []string) ([]BulkFileResult, error)
```

- Build dynamic GraphQL query with aliases for each file
- Use expression format: `branch:path/to/file.go`
- Handle response mapping back to file paths
- Return results for all files (including errors per file)

### 4. GraphQL Query Structure
```graphql
query {
  repository(owner: $owner, name: $repo) {
    f0: object(expression: "main:README.md") {
      ... on Blob { text byteSize isBinary }
    }
    f1: object(expression: "main:go.mod") {
      ... on Blob { text byteSize isBinary }
    }
  }
}
```

## Acceptance
- [ ] GraphQL client is initialized alongside REST client
- [ ] BulkGetFileContent method accepts up to 100 file paths
- [ ] Method returns content for all requested files
- [ ] Binary files are flagged appropriately
- [ ] Error handling per-file (one failure doesn't fail all)
- [ ] Compiles: `go build ./...`
- [ ] Tests pass: `go test ./internal/connectors/github/...`
