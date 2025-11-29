# Plan: Worker Naming Consistency

## Classification
- Type: fix
- Workdir: ./docs/fix/20251129-worker-naming/

## Analysis
- Two files use `_step_worker` suffix instead of `_worker`: `places_step_worker.go`, `web_search_step_worker.go`
- Two struct types use `StepWorker` naming: `PlacesStepWorker`, `WebSearchStepWorker`
- These need to be renamed to match other workers: `PlacesWorker`, `WebSearchWorker`
- Documentation needs update to reflect consistent naming
- References in app.go need updating

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Rename places_step_worker.go to places_worker.go and update struct/function names | none | no | low | sonnet |
| 2 | Rename web_search_step_worker.go to web_search_worker.go and update struct/function names | none | no | low | sonnet |
| 3 | Update app.go references | 1,2 | no | low | sonnet |
| 4 | Update documentation | 1,2,3 | no | low | sonnet |
| 5 | Run tests and verify | 4 | no | low | sonnet |

## Order
Concurrent: [1,2] -> Sequential: [3] -> Sequential: [4] -> Sequential: [5]
