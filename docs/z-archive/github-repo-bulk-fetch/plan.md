# Plan: GitHub Repository Bulk Fetch Optimization

## Problem Statement
The `github-repo-collector.toml` job takes significant time to process because it fetches files one-by-one via individual GitHub API calls. With ~500ms per API call, processing 100 files takes ~50 seconds even with 10 concurrent workers.

## Analysis

### Current Architecture
1. **Parent Job**: Uses `Git.GetTree()` (single API call) to enumerate all files - FAST
2. **Child Jobs**: Each file spawns a child job calling `GetFileContent()` - SLOW (1 API call per file)

### Dependencies
- `internal/connectors/github/repo.go` - GitHub API client
- `internal/queue/workers/github_repo_worker.go` - File fetching worker
- `internal/queue/managers/github_repo_manager.go` - Job orchestration
- `go-github/v68` library for API access

### Approach: GraphQL Bulk Fetching
GitHub's GraphQL API allows fetching multiple files in a single request. This can reduce:
- 100 API calls → 1-2 API calls (batches of 100 files)
- 50 seconds → ~1-2 seconds for 100 files

### Alternative Considered: Git Clone
- **Pros**: Gets entire repo instantly, offline processing
- **Cons**:
  - Requires disk space for full repo
  - Needs git binary installed
  - Downloads ALL files including excluded ones
  - More complex cleanup/temp file management
  - Doesn't integrate well with existing document-per-file pipeline

### Why GraphQL over Clone
- Maintains current architecture (document-per-file processing)
- No disk space for full repo clone
- Selective file fetching (only requested extensions)
- Better rate limit usage (fewer requests)
- No external dependencies (git binary)

### Risks
- GraphQL query complexity limits (5000 points/hour)
- Large files may need fallback to REST API
- Need to handle pagination for 100+ files

## Groups

| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Add GraphQL client and bulk file fetch method to GitHub connector | none | no | medium | sonnet |
| 2 | Create batch fetcher service that groups files into GraphQL requests | 1 | no | medium | sonnet |
| 3 | Update github_repo_manager to use batch fetching instead of child jobs | 1,2 | no | medium | sonnet |
| 4 | Add fallback to single-file fetch for files exceeding GraphQL size limits | 3 | no | low | sonnet |
| 5 | Update tests for new bulk fetch functionality | 3,4 | no | low | sonnet |
| 6 | Performance testing and validation | 5 | no | low | sonnet |

## Order
Sequential: [1] → [2] → [3] → [4] → Sequential: [5] → [6]

## Implementation Strategy

### GraphQL Query Structure
```graphql
query BulkFileContent($owner: String!, $repo: String!, $expression1: String!, ...) {
  repository(owner: $owner, name: $repo) {
    file1: object(expression: $expression1) {
      ... on Blob {
        text
        byteSize
        isBinary
      }
    }
    file2: object(expression: $expression2) { ... }
    # Up to 100 files per query
  }
}
```

### File Batching Strategy
- Batch files into groups of 50-100
- Dynamically build GraphQL query with aliases
- Process responses and create documents
- Fall back to REST for binary/large files

## Success Criteria
- [ ] 10x+ improvement in collection time for 100+ files
- [ ] Maintains document-per-file storage model
- [ ] Graceful fallback for edge cases
- [ ] All existing tests pass
- [ ] New tests cover bulk fetch scenarios
