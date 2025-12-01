# Task 5: Update Event Publishing

Depends: 3 | Critical: no | Model: sonnet

## Do

1. Define event routing:
   - Jobs → publish to Step (parent_id)
   - Steps → publish to UI job panel + Manager
   - Manager → publish to UI top panel

2. Update event types in `internal/interfaces/event_service.go`:
   - `EventStepProgress` - step-level progress (children stats)
   - `EventManagerProgress` - manager-level progress (steps stats)

3. Update JobMonitor → becomes ManagerMonitor:
   - Monitors step completion (not job completion)
   - Publishes manager_progress events

4. StepMonitor publishes:
   - `step_progress` event with:
     - step_id, step_name, step_type
     - pending_jobs, running_jobs, completed_jobs, failed_jobs
     - document_count (aggregated from jobs)

5. ManagerMonitor publishes:
   - `manager_progress` event with:
     - manager_id
     - total_steps, completed_steps, running_steps
     - overall job counts (aggregated from steps)

## Accept

- [ ] EventStepProgress and EventManagerProgress defined
- [ ] StepMonitor publishes step_progress events
- [ ] ManagerMonitor publishes manager_progress events
- [ ] Event routing follows hierarchy
- [ ] Code compiles without errors
