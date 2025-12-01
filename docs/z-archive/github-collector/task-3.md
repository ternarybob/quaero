# Task 3: Create GitHubRepoManager

- Group: 3 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-3/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `internal/queue/managers/github_repo_manager.go` - NEW: Repository job orchestration

## Requirements

### Create manager following CrawlerManager pattern:

```go
package managers

type GitHubRepoManager struct {
    connectorService interfaces.ConnectorService
    jobMgr           *queue.Manager
    queueMgr         interfaces.QueueManager
    logger           arbor.ILogger
}

func NewGitHubRepoManager(
    connectorService interfaces.ConnectorService,
    jobMgr *queue.Manager,
    queueMgr interfaces.QueueManager,
    logger arbor.ILogger,
) *GitHubRepoManager

// GetManagerType returns "github_repo_fetch"
func (m *GitHubRepoManager) GetManagerType() string

// ReturnsChildJobs returns true (spawns file jobs)
func (m *GitHubRepoManager) ReturnsChildJobs() bool

// CreateParentJob creates parent job and spawns children for each file
func (m *GitHubRepoManager) CreateParentJob(
    ctx context.Context,
    step models.JobStep,
    jobDef *models.JobDefinition,
    parentJobID string,
) (string, error)
```

### CreateParentJob Logic:
1. Extract config from step:
   - `connector_id`: GitHub connector to use
   - `owner`: Repository owner
   - `repo`: Repository name
   - `branches`: List of branches to process (default: ["main"])
   - `extensions`: File extensions to include (default: [".go", ".ts", ".js", ".md"])
   - `exclude_paths`: Paths to exclude (default: ["vendor/", "node_modules/"])

2. Get GitHub connector from connectorService

3. For each branch:
   - List files using connector.ListFiles()
   - For each file, create and enqueue a `github_repo_file` job:
     ```go
     queueJob := models.NewQueueJob(
         models.JobTypeGitHubRepoFile,
         fmt.Sprintf("Fetch: %s/%s@%s:%s", owner, repo, branch, file.Path),
         map[string]interface{}{
             "owner":    owner,
             "repo":     repo,
             "branch":   branch,
             "path":     file.Path,
             "folder":   file.Folder,
             "sha":      file.SHA,
         },
         map[string]interface{}{
             "connector_id": connectorID,
             "tags":         jobDef.Tags,
         },
     )
     queueJob.ParentID = &parentJobID
     ```

4. Update parent job with total file count

### Config Example:
```toml
[steps.config]
connector_id = "github"
owner = "ternarybob"
repo = "quaero"
branches = ["main", "develop"]
extensions = [".go", ".ts", ".md"]
exclude_paths = ["vendor/", "node_modules/", "test/"]
```

## Acceptance
- [ ] Manager struct defined
- [ ] GetManagerType returns "github_repo_fetch"
- [ ] CreateParentJob extracts config correctly
- [ ] Files are listed per branch
- [ ] Child jobs enqueued for each file
- [ ] Tags propagated from job definition
- [ ] Folder path extracted correctly
- [ ] Compiles
- [ ] Tests pass
