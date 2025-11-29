# Task 2: Extend GitHub Connector with Repo API Methods

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `internal/connectors/github/repo.go` - NEW: Repository content fetching
- `internal/connectors/github/connector.go` - Add interface methods

## Requirements

### Create repo.go with methods:

```go
// RepoFile represents a file from a GitHub repository
type RepoFile struct {
    Path        string    // Full path: src/components/Button.tsx
    Folder      string    // Parent folder: src/components/
    Name        string    // File name: Button.tsx
    SHA         string    // File SHA
    Size        int       // File size in bytes
    Content     string    // Decoded content (for text files)
    URL         string    // GitHub URL
    DownloadURL string    // Raw download URL
}

// RepoBranch represents branch info
type RepoBranch struct {
    Name      string
    CommitSHA string
    Protected bool
}

// ListBranches returns all branches for a repo
func (c *Connector) ListBranches(ctx context.Context, owner, repo string) ([]RepoBranch, error)

// ListFiles returns all files in a repo for a given branch
// Filters by extension (e.g., ".go", ".ts", ".md")
// Excludes binary files and vendor/node_modules
func (c *Connector) ListFiles(ctx context.Context, owner, repo, branch string, extensions []string) ([]RepoFile, error)

// GetFileContent fetches the content of a single file
func (c *Connector) GetFileContent(ctx context.Context, owner, repo, branch, path string) (*RepoFile, error)
```

### Implementation Notes:
1. Use GitHub Trees API for efficient listing: `GET /repos/{owner}/{repo}/git/trees/{branch}?recursive=1`
2. Filter files by extension (default: code files)
3. Exclude common directories: `vendor/`, `node_modules/`, `.git/`
4. Decode base64 content from GitHub API
5. Handle pagination for large repos

### Also add Actions API methods:
```go
// WorkflowRun represents a GitHub Actions workflow run
type WorkflowRun struct {
    ID           int64
    Name         string
    WorkflowName string
    Status       string    // queued, in_progress, completed
    Conclusion   string    // success, failure, cancelled, skipped
    Branch       string
    CommitSHA    string
    RunStartedAt time.Time
    RunAttempt   int
    URL          string
}

// ListWorkflowRuns returns recent workflow runs for a repo
func (c *Connector) ListWorkflowRuns(ctx context.Context, owner, repo string, limit int) ([]WorkflowRun, error)

// GetWorkflowRunLogs fetches logs for a specific run
func (c *Connector) GetWorkflowRunLogs(ctx context.Context, owner, repo string, runID int64) (string, error)
```

## Acceptance
- [ ] RepoFile struct defined
- [ ] ListBranches method works
- [ ] ListFiles method filters by extension
- [ ] GetFileContent decodes base64
- [ ] WorkflowRun struct defined
- [ ] ListWorkflowRuns method works
- [ ] GetWorkflowRunLogs method works
- [ ] Compiles
- [ ] Tests pass
