# Task 1: Rename PlacesStepWorker to PlacesWorker

- Group: 1 | Mode: concurrent | Model: sonnet
- Critical: no | Depends: none

## Files
- `internal/queue/workers/places_step_worker.go` -> `internal/queue/workers/places_worker.go`

## Requirements
1. Rename file from `places_step_worker.go` to `places_worker.go`
2. Rename struct `PlacesStepWorker` to `PlacesWorker`
3. Rename constructor `NewPlacesStepWorker` to `NewPlacesWorker`
4. Update all method receivers from `PlacesStepWorker` to `PlacesWorker`
5. Update comments referencing the old names

## Acceptance
- [ ] File renamed to places_worker.go
- [ ] All struct/function names updated
- [ ] Compiles
