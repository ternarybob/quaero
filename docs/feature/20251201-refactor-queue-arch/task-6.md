# Task 6: Update GetJobChildStats

Depends: 1 | Critical: no | Model: sonnet

## Do

1. Update `GetJobChildStats` in `internal/queue/manager.go`:
   - Support querying by step_id (for StepMonitor)
   - Support querying by manager_id (for ManagerMonitor to get steps)

2. Add new method `GetStepStats(ctx, managerID)`:
   - Returns stats for all step jobs under a manager
   - Counts: total_steps, completed_steps, running_steps, failed_steps

3. Update ListChildJobs:
   - `ListChildJobs(ctx, parentID)` - returns jobs under a step
   - `ListStepJobs(ctx, managerID)` - returns step jobs under manager

4. Update BadgerDB queries in `internal/storage/badger/manager.go`:
   - Query by parent_id (step's jobs)
   - Query by type="step" AND parent_id=manager (manager's steps)

## Accept

- [ ] GetJobChildStats works with step_id
- [ ] GetStepStats method exists for manager-level aggregation
- [ ] ListStepJobs method returns step jobs for a manager
- [ ] Code compiles without errors
