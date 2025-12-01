# Task 4: Update worker.CreateJobs

Depends: 2 | Critical: no | Model: sonnet

## Do

1. Update DefinitionWorker interface in `internal/interfaces/job_interfaces.go`:
   - Change CreateJobs signature to accept stepID parameter
   - `CreateJobs(ctx, step, jobDef, managerID, stepID) (string, error)`

2. Update all worker implementations to use stepID as parent:
   - `internal/queue/workers/github_repo_worker.go`
   - `internal/queue/workers/places_worker.go`
   - `internal/queue/workers/web_crawler_worker.go`
   - Any other workers

3. When workers call CreateChildJob:
   - Set parent_id = stepID (not managerID)
   - Set manager_id = managerID (for reference)

4. If a job spawns more jobs (grandchildren):
   - Those jobs also get parent_id = stepID (stay flat under step)
   - This keeps all jobs at one level under their step

## Accept

- [ ] DefinitionWorker.CreateJobs accepts stepID parameter
- [ ] All workers updated to use stepID as parent
- [ ] Child jobs have parent_id = step, manager_id = manager
- [ ] Code compiles without errors
