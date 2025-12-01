# Task 1: Update Job Models

Depends: - | Critical: yes:architectural-change | Model: opus

## Do

1. Add new JobType constants in `internal/models/crawler_job.go`:
   - `JobTypeManager` = "manager" (top-level orchestrator)
   - `JobTypeStep` = "step" (step container that monitors its jobs)

2. Add `ManagerID` field to `QueueJob` in `internal/models/job_model.go`:
   - `ManagerID *string` - references the top-level manager job
   - This allows jobs to know their manager even when their parent is a step

3. Update `NewQueueJob` constructor to accept optional manager_id

4. Update `QueueJobState` to include the new field

## Accept

- [ ] JobTypeManager and JobTypeStep constants exist
- [ ] QueueJob has ManagerID field
- [ ] QueueJobState has ManagerID field
- [ ] Code compiles without errors
