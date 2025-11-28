# Task 7: Register Managers and Workers in app.go

- Group: 7 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 3,4,5,6
- Sandbox: /tmp/3agents/task-7/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `internal/app/app.go` - Register new managers and workers

## Requirements

### Add imports:
```go
import (
    // ... existing imports
    "github.com/ternarybob/quaero/internal/queue/managers"
)
```

### Register GitHubRepoWorker (after existing workers ~line 490):
```go
// Register GitHub repo file worker
githubRepoWorker := workers.NewGitHubRepoWorker(
    a.ConnectorService,
    jobMgr,
    a.StorageManager.DocumentStorage(),
    a.Logger,
    eventService,
)
jobProcessor.RegisterExecutor(githubRepoWorker)
a.Logger.Debug().Msg("GitHub repo worker registered")
```

### Register GitHubRepoManager (after existing managers ~line 560):
```go
// Register GitHub repo manager
githubRepoManager := managers.NewGitHubRepoManager(
    a.ConnectorService,
    jobMgr,
    queueMgr,
    a.Logger,
)
orchestrator.RegisterStepExecutor(githubRepoManager)
a.Logger.Debug().Msg("GitHub repo manager registered")
```

### Register GitHubActionsManager:
```go
// Register GitHub actions manager
githubActionsManager := managers.NewGitHubActionsManager(
    a.ConnectorService,
    jobMgr,
    queueMgr,
    a.Logger,
)
orchestrator.RegisterStepExecutor(githubActionsManager)
a.Logger.Debug().Msg("GitHub actions manager registered")
```

### Verify existing GitHubLogWorker is registered:
The existing github_log_worker.go should already be registered. Verify and update if needed.

## Acceptance
- [ ] GitHubRepoWorker registered with JobProcessor
- [ ] GitHubRepoManager registered with Orchestrator
- [ ] GitHubActionsManager registered with Orchestrator
- [ ] GitHubLogWorker verified registered
- [ ] Debug log messages added
- [ ] Compiles
- [ ] Tests pass
