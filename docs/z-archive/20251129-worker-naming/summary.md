# Worker Naming Fix Summary

## Completed: 2025-11-29

## Problem
1. Files `internal/queue/workers/places_step_worker.go` and `web_search_step_worker.go` had incorrect naming convention
2. Struct types `PlacesStepWorker` and `WebSearchStepWorker` were inconsistent with other workers (should be `XxxWorker`, not `XxxStepWorker`)

## Changes Made

### Task 1: Renamed places_step_worker.go to places_worker.go
- Renamed struct `PlacesStepWorker` to `PlacesWorker`
- Renamed constructor `NewPlacesStepWorker` to `NewPlacesWorker`
- File: `internal/queue/workers/places_worker.go`

### Task 2: Renamed web_search_step_worker.go to web_search_worker.go
- Renamed struct `WebSearchStepWorker` to `WebSearchWorker`
- Renamed constructor `NewWebSearchStepWorker` to `NewWebSearchWorker`
- Added missing type definitions `WebSearchResults` and `WebSearchSource` (were previously in deleted manager file)
- File: `internal/queue/workers/web_search_worker.go`

### Task 3: Updated app.go references
- Updated `internal/app/app.go` to use:
  - `workers.NewPlacesWorker()` instead of `workers.NewPlacesStepWorker()`
  - `workers.NewWebSearchWorker()` instead of `workers.NewWebSearchStepWorker()`
  - Variable names `placesWorker` and `webSearchWorker` instead of `placesStepWorker` and `webSearchStepWorker`

### Task 4: Updated documentation
- Updated `docs/architecture/manager_worker_architecture.md` to reflect correct file names:
  - `places_worker.go` (was listed as `places_search_worker.go`)
  - `github_log_worker.go` (was listed as `github_actions_worker.go`)

### Task 5: Verified changes
- Build passes: `go build ./...`
- Queue tests pass: Both `TestQueue` and `TestQueueWithKeywordExtraction` pass

## Files Modified
- `internal/queue/workers/places_worker.go` - NEW (content from previous session, updated naming)
- `internal/queue/workers/web_search_worker.go` - NEW (content from previous session, updated naming, added type definitions)
- `internal/app/app.go` - Updated worker instantiation
- `docs/architecture/manager_worker_architecture.md` - Updated file listing

## Test Results
```
=== RUN   TestQueue
--- PASS: TestQueue (19.20s)
=== RUN   TestQueueWithKeywordExtraction
--- PASS: TestQueueWithKeywordExtraction (28.91s)
PASS
ok  	command-line-arguments	48.573s
```

## Notes
- The `WebSearchResults` and `WebSearchSource` types were originally defined in the deleted `internal/queue/managers/web_search_manager.go` file
- These types were added to `web_search_worker.go` to fix build errors
