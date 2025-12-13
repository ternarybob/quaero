# Task 4: Create GitHubRepoWorker

- Group: 4 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-4/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `internal/queue/workers/github_repo_worker.go` - NEW: File processing worker

## Requirements

### Create worker following CrawlerWorker pattern:

```go
package workers

type GitHubRepoWorker struct {
    connectorService interfaces.ConnectorService
    jobMgr           *queue.Manager
    documentStorage  interfaces.DocumentStorage
    logger           arbor.ILogger
    eventService     interfaces.EventService
}

func NewGitHubRepoWorker(
    connectorService interfaces.ConnectorService,
    jobMgr *queue.Manager,
    documentStorage interfaces.DocumentStorage,
    logger arbor.ILogger,
    eventService interfaces.EventService,
) *GitHubRepoWorker

// GetWorkerType returns "github_repo_file"
func (w *GitHubRepoWorker) GetWorkerType() string

// Validate checks required config fields
func (w *GitHubRepoWorker) Validate(job *models.QueueJob) error

// Execute fetches file content and saves as document
func (w *GitHubRepoWorker) Execute(ctx context.Context, job *models.QueueJob) error
```

### Execute Logic:
1. Extract config from job:
   - `owner`, `repo`, `branch`, `path`, `folder`, `sha`
   - `connector_id` from metadata
   - `tags` from metadata

2. Get GitHub connector from connectorService

3. Fetch file content using connector.GetFileContent()

4. Create document:
```go
doc := &models.Document{
    ID:              fmt.Sprintf("doc_%s", uuid.New().String()),
    SourceType:      models.SourceTypeGitHubRepo,
    SourceID:        fmt.Sprintf("%s/%s/%s/%s", owner, repo, branch, path),
    Title:           filepath.Base(path),
    ContentMarkdown: content,  // Already text or convert to markdown
    URL:             fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, path),
    Tags:            mergeTags(baseTags, []string{"github", repo, branch}),
    Metadata: map[string]interface{}{
        "owner":      owner,
        "repo":       repo,
        "branch":     branch,
        "folder":     folder,
        "path":       path,
        "sha":        sha,
        "file_type":  filepath.Ext(path),
    },
    CreatedAt: time.Now(),
    UpdatedAt: time.Now(),
}
```

5. Save document using documentStorage.SaveDocument()

6. Publish event for real-time UI updates

### Validate Logic:
```go
func (w *GitHubRepoWorker) Validate(job *models.QueueJob) error {
    requiredFields := []string{"owner", "repo", "branch", "path"}
    for _, field := range requiredFields {
        if _, ok := job.GetConfigString(field); !ok {
            return fmt.Errorf("missing required config field: %s", field)
        }
    }
    return nil
}
```

## Acceptance
- [ ] Worker struct defined
- [ ] GetWorkerType returns "github_repo_file"
- [ ] Validate checks required fields
- [ ] Execute fetches file content
- [ ] Document created with correct metadata
- [ ] Tags include job definition tags + auto tags
- [ ] Folder path stored in metadata
- [ ] Events published
- [ ] Compiles
- [ ] Tests pass
