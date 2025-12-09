# Task 1: Add status field to step_stats in orchestrator.go
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
Stores the actual step status (including "failed") in step_stats metadata so the UI can accurately display it.

## Do
1. Modify orchestrator.go to add "status" field to step_stats
2. Handle failure cases - when a step fails (init or execute), record "failed" status in step_stats before continuing

## Accept
- [ ] step_stats includes "status" field for each step
- [ ] Failed steps have status="failed" in step_stats
- [ ] Successful steps have status="completed" or "spawned" in step_stats
- [ ] Code compiles without errors
