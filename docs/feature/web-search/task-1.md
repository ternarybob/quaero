# Task 1: Create WebSearchManager

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: C:\development\quaero | Output: C:\development\quaero\docs\plans\web-search

## Files
- `internal/queue/managers/web_search_manager.go` - Create new file

## Requirements

Create a new StepManager implementation for web search jobs. Unlike agent/crawler managers that spawn child jobs, this manager will:

1. Implement `interfaces.StepManager` interface
2. Parse step config for:
   - `query` (string) - Natural language search query
   - `depth` (int) - Number of follow-up exploration queries (default 1, max 10)
   - `breadth` (int) - Results per query (default 3, max 5)
   - `api_key` (string) - Reference to Google API key in KV store
3. Create a single worker job (not child jobs) that will execute the search
4. Return immediately after enqueueing (no polling needed)

```go
type WebSearchManager struct {
    jobMgr       *queue.Manager
    queueMgr     interfaces.QueueManager
    docStorage   interfaces.DocumentStorage
    kvStorage    interfaces.KeyValueStorage
    eventService interfaces.EventService
    logger       arbor.ILogger
}
```

## Acceptance
- [ ] Implements StepManager interface
- [ ] Parses depth/breadth parameters with defaults and limits
- [ ] Resolves API key from KV store
- [ ] Creates and enqueues worker job
- [ ] Compiles
