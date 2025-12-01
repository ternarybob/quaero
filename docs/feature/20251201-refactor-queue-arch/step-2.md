# Step 2: Update Manager.ExecuteJobDefinition

Model: opus | Status: ✅

## Done

- Changed "parent job" to "manager job" (type="manager")
- Added step job creation for each step:
  - Creates step job (type="step") before calling worker
  - Step job has parent_id = manager_id
  - Step stores metadata: manager_id, step_index, step_name, step_type
- Updated worker.CreateJobs to receive stepID (instead of parentJobID):
  - Workers now create jobs under the step, not manager
  - Jobs will have parent_id = step_id
- Updated job logging to log to both manager and step
- Updated step completion logic:
  - Step without children → marked "completed"
  - Step with children → remains "running" until children finish
- Updated manager completion handling:
  - Stores step_job_ids in metadata for monitoring
  - Passes manager job to JobMonitor

## Key Changes

```
Before:                          After:
parentJobID = uuid()             managerID = uuid()
type = "parent"                  type = "manager"
children under parent            step jobs under manager
                                 children under step
```

## Files Changed

- `internal/queue/manager.go` - Refactored ExecuteJobDefinition to create manager + step jobs

## Verify

Build: ✅ | Tests: ⏭️
