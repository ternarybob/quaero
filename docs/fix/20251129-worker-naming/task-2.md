# Task 2: Rename WebSearchStepWorker to WebSearchWorker

- Group: 1 | Mode: concurrent | Model: sonnet
- Critical: no | Depends: none

## Files
- `internal/queue/workers/web_search_step_worker.go` -> `internal/queue/workers/web_search_worker.go`

## Requirements
1. Rename file from `web_search_step_worker.go` to `web_search_worker.go`
2. Rename struct `WebSearchStepWorker` to `WebSearchWorker`
3. Rename constructor `NewWebSearchStepWorker` to `NewWebSearchWorker`
4. Update all method receivers from `WebSearchStepWorker` to `WebSearchWorker`
5. Update comments referencing the old names

## Acceptance
- [ ] File renamed to web_search_worker.go
- [ ] All struct/function names updated
- [ ] Compiles
