# Task 3: Register Job Type in app.go

- Group: 3 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-3/ | Source: C:\development\quaero | Output: C:\development\quaero\docs\plans\web-search

## Files
- `internal/app/app.go` - Add registration for WebSearchManager and WebSearchWorker

## Requirements

Register the new web_search job type with the application:

1. Add constants for job type:
   - `JobTypeWebSearch = "web_search"`
   - `JobDefinitionTypeWebSearch = "web_search"`

2. Create WebSearchManager instance and register with orchestrator:
```go
webSearchManager := managers.NewWebSearchManager(
    a.JobManager,
    a.QueueManager,
    a.StorageManager.DocumentStorage(),
    a.StorageManager.KeyValueStorage(),
    a.EventsService,
    a.Logger,
)
a.Orchestrator.RegisterStepExecutor(webSearchManager)
```

3. Create WebSearchWorker instance and register with job processor:
```go
webSearchWorker := workers.NewWebSearchWorker(
    a.StorageManager.DocumentStorage(),
    a.StorageManager.KeyValueStorage(),
    a.EventsService,
    a.Logger,
)
a.JobProcessor.RegisterWorker(webSearchWorker)
```

## Acceptance
- [ ] Job type constants defined
- [ ] Manager created and registered
- [ ] Worker created and registered
- [ ] Compiles
