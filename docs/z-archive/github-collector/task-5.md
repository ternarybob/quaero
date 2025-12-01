# Task 5: Create GitHubActionsManager

- Group: 5 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-5/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `internal/queue/managers/github_actions_manager.go` - NEW: Actions log orchestration

## Requirements

### Create manager following existing pattern:

```go
package managers

type GitHubActionsManager struct {
    connectorService interfaces.ConnectorService
    jobMgr           *queue.Manager
    queueMgr         interfaces.QueueManager
    logger           arbor.ILogger
}

func NewGitHubActionsManager(
    connectorService interfaces.ConnectorService,
    jobMgr *queue.Manager,
    queueMgr interfaces.QueueManager,
    logger arbor.ILogger,
) *GitHubActionsManager

// GetManagerType returns "github_actions_fetch"
func (m *GitHubActionsManager) GetManagerType() string

// ReturnsChildJobs returns true (spawns log jobs)
func (m *GitHubActionsManager) ReturnsChildJobs() bool

// CreateParentJob creates parent job and spawns children for each workflow run
func (m *GitHubActionsManager) CreateParentJob(
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
   - `limit`: Max runs to fetch (default: 10)
   - `status_filter`: Filter by status (optional: "completed", "failure")
   - `branch_filter`: Filter by branch (optional)

2. Get GitHub connector from connectorService

3. List workflow runs using connector.ListWorkflowRuns()

4. For each run, create and enqueue a `github_action_log` job:
```go
queueJob := models.NewQueueJob(
    models.JobTypeGitHubActionLog,
    fmt.Sprintf("Fetch log: %s/%s run %d", owner, repo, run.ID),
    map[string]interface{}{
        "owner":         owner,
        "repo":          repo,
        "run_id":        run.ID,
        "workflow_name": run.WorkflowName,
        "run_started_at": run.RunStartedAt.Format(time.RFC3339),
        "branch":        run.Branch,
        "commit_sha":    run.CommitSHA,
        "conclusion":    run.Conclusion,
    },
    map[string]interface{}{
        "connector_id": connectorID,
        "tags":         jobDef.Tags,
    },
)
queueJob.ParentID = &parentJobID
```

5. Update parent job with total run count

### Config Example:
```toml
[steps.config]
connector_id = "github"
owner = "ternarybob"
repo = "quaero"
limit = 20
status_filter = "completed"
branch_filter = "main"
```

## Acceptance
- [ ] Manager struct defined
- [ ] GetManagerType returns "github_actions_fetch"
- [ ] CreateParentJob extracts config correctly
- [ ] Workflow runs listed from GitHub
- [ ] Child jobs enqueued for each run
- [ ] Metadata captured (workflow_name, run_started_at, etc.)
- [ ] Tags propagated from job definition
- [ ] Compiles
- [ ] Tests pass
