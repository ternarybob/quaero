# Task 9: Add API Endpoints for GitHub Jobs

- Group: 9 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 7
- Sandbox: /tmp/3agents/task-9/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `internal/api/handlers/github_jobs_handler.go` - NEW: GitHub job endpoints
- `internal/api/router.go` - Add routes

## Requirements

### Create github_jobs_handler.go:

```go
package handlers

type GitHubJobsHandler struct {
    connectorService interfaces.ConnectorService
    jobMgr           *queue.Manager
    queueMgr         interfaces.QueueManager
    orchestrator     *queue.Orchestrator
    logger           arbor.ILogger
}

func NewGitHubJobsHandler(...) *GitHubJobsHandler

// POST /api/github/repo/preview
// Preview files that would be fetched from a repo
func (h *GitHubJobsHandler) PreviewRepoFiles(c *fiber.Ctx) error

// POST /api/github/actions/preview
// Preview workflow runs that would be fetched
func (h *GitHubJobsHandler) PreviewActionRuns(c *fiber.Ctx) error

// POST /api/github/repo/start
// Start a repo collector job
func (h *GitHubJobsHandler) StartRepoCollector(c *fiber.Ctx) error

// POST /api/github/actions/start
// Start an actions log collector job
func (h *GitHubJobsHandler) StartActionsCollector(c *fiber.Ctx) error
```

### Request/Response Types:

```go
// PreviewRepoRequest - request to preview repo files
type PreviewRepoRequest struct {
    ConnectorID  string   `json:"connector_id"`
    Owner        string   `json:"owner"`
    Repo         string   `json:"repo"`
    Branches     []string `json:"branches"`
    Extensions   []string `json:"extensions"`
    ExcludePaths []string `json:"exclude_paths"`
}

// PreviewRepoResponse - response with file list
type PreviewRepoResponse struct {
    Files      []RepoFilePreview `json:"files"`
    TotalCount int               `json:"total_count"`
    Branches   []string          `json:"branches"`
}

type RepoFilePreview struct {
    Path   string `json:"path"`
    Folder string `json:"folder"`
    Size   int    `json:"size"`
    Branch string `json:"branch"`
}

// PreviewActionsRequest - request to preview action runs
type PreviewActionsRequest struct {
    ConnectorID  string `json:"connector_id"`
    Owner        string `json:"owner"`
    Repo         string `json:"repo"`
    Limit        int    `json:"limit"`
    StatusFilter string `json:"status_filter"`
    BranchFilter string `json:"branch_filter"`
}

// PreviewActionsResponse - response with run list
type PreviewActionsResponse struct {
    Runs       []ActionRunPreview `json:"runs"`
    TotalCount int                `json:"total_count"`
}

type ActionRunPreview struct {
    ID           int64  `json:"id"`
    WorkflowName string `json:"workflow_name"`
    Status       string `json:"status"`
    Conclusion   string `json:"conclusion"`
    Branch       string `json:"branch"`
    StartedAt    string `json:"started_at"`
}

// StartJobRequest - request to start a collector job
type StartJobRequest struct {
    ConnectorID  string   `json:"connector_id"`
    Owner        string   `json:"owner"`
    Repo         string   `json:"repo"`
    Tags         []string `json:"tags"`
    // Repo-specific
    Branches     []string `json:"branches,omitempty"`
    Extensions   []string `json:"extensions,omitempty"`
    ExcludePaths []string `json:"exclude_paths,omitempty"`
    // Actions-specific
    Limit        int    `json:"limit,omitempty"`
    StatusFilter string `json:"status_filter,omitempty"`
    BranchFilter string `json:"branch_filter,omitempty"`
}

// StartJobResponse - response with job ID
type StartJobResponse struct {
    JobID   string `json:"job_id"`
    Message string `json:"message"`
}
```

### Router Registration:
```go
// GitHub job endpoints
github := api.Group("/github")
github.Post("/repo/preview", githubJobsHandler.PreviewRepoFiles)
github.Post("/repo/start", githubJobsHandler.StartRepoCollector)
github.Post("/actions/preview", githubJobsHandler.PreviewActionRuns)
github.Post("/actions/start", githubJobsHandler.StartActionsCollector)
```

## Acceptance
- [ ] Handler struct created
- [ ] PreviewRepoFiles returns file list
- [ ] PreviewActionRuns returns run list
- [ ] StartRepoCollector creates and starts job
- [ ] StartActionsCollector creates and starts job
- [ ] Routes registered
- [ ] Error handling for missing connector
- [ ] Compiles
- [ ] Tests pass
