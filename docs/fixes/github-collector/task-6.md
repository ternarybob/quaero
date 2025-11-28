# Task 6: Update GitHubLogWorker with Enhanced Metadata

- Group: 6 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-6/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `internal/queue/workers/github_log_worker.go` - Enhance with metadata

## Requirements

### Update existing GitHubLogWorker to capture metadata:

The worker should now extract metadata from job config (populated by manager) and include it in the document.

### Updated Execute Logic:
```go
func (w *GitHubLogWorker) Execute(ctx context.Context, job *models.QueueJob) error {
    // Extract config from job (now includes metadata from manager)
    owner, _ := job.GetConfigString("owner")
    repo, _ := job.GetConfigString("repo")
    runID, _ := job.GetConfigInt("run_id")
    workflowName, _ := job.GetConfigString("workflow_name")
    runStartedAt, _ := job.GetConfigString("run_started_at")
    branch, _ := job.GetConfigString("branch")
    commitSHA, _ := job.GetConfigString("commit_sha")
    conclusion, _ := job.GetConfigString("conclusion")

    // Get connector
    connectorID, _ := job.Metadata["connector_id"].(string)
    connector, err := w.connectorService.GetConnector(ctx, connectorID)
    ghConnector, err := github.NewConnector(connector)

    // Fetch log content
    logContent, err := ghConnector.GetWorkflowRunLogs(ctx, owner, repo, runID)

    // Parse run_started_at for proper timestamp
    var runTime time.Time
    if runStartedAt != "" {
        runTime, _ = time.Parse(time.RFC3339, runStartedAt)
    }

    // Get tags from metadata
    baseTags, _ := job.Metadata["tags"].([]string)

    // Create document with full metadata
    doc := &models.Document{
        ID:              fmt.Sprintf("doc_%s", uuid.New().String()),
        SourceType:      models.SourceTypeGitHubActionLog,
        SourceID:        fmt.Sprintf("%s/%s/actions/runs/%d", owner, repo, runID),
        Title:           fmt.Sprintf("GitHub Actions: %s - %s/%s #%d", workflowName, owner, repo, runID),
        ContentMarkdown: logContent,
        URL:             fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", owner, repo, runID),
        Tags:            mergeTags(baseTags, []string{"github", "actions", repo, conclusion}),
        Metadata: map[string]interface{}{
            "owner":          owner,
            "repo":           repo,
            "run_id":         runID,
            "workflow_name":  workflowName,
            "run_started_at": runStartedAt,  // ISO8601 string
            "run_date":       runTime.Format("2006-01-02"),  // Date only for filtering
            "branch":         branch,
            "commit_sha":     commitSHA,
            "conclusion":     conclusion,  // success, failure, cancelled
        },
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    // Save document
    err = w.documentStorage.SaveDocument(doc)

    // Publish event
    w.eventService.Publish(...)

    return nil
}
```

### Key Metadata Fields for Action Logs:
- `workflow_name`: Name of the workflow (e.g., "CI Build")
- `run_started_at`: ISO8601 timestamp of when run started
- `run_date`: Date only (YYYY-MM-DD) for easy filtering
- `branch`: Branch the workflow ran on
- `commit_sha`: Commit that triggered the workflow
- `conclusion`: success/failure/cancelled/skipped

### Tag Generation:
Combine:
1. Job definition tags (from TOML)
2. Auto-generated: `["github", "actions", repo-name, conclusion]`

Example final tags: `["ci-cd", "logs", "github", "actions", "quaero", "success"]`

## Acceptance
- [ ] Worker extracts all metadata from job config
- [ ] Document includes workflow_name in title
- [ ] Document includes run_started_at and run_date
- [ ] Document includes branch and commit_sha
- [ ] Document includes conclusion (success/failure)
- [ ] Tags include both job def tags and auto-generated tags
- [ ] Compiles
- [ ] Tests pass
