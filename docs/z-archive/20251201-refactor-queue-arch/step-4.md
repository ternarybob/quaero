# Step 4: Update worker.CreateJobs

Model: opus | Status: ✅

## Done

- Updated `DefinitionWorker.CreateJobs` interface docs to use `stepID` naming
- Updated all worker implementations to use `stepID` parameter:
  - `github_repo_worker.go` - renamed parameter to stepID
  - `github_log_worker.go` - renamed parameter to stepID
  - `crawler_worker.go` - renamed parameter to stepID
  - `places_worker.go` - renamed parameter to stepID
  - `agent_worker.go` - renamed parameter to stepID
  - `web_search_worker.go` - renamed parameter to stepID
- Updated all references within CreateJobs functions:
  - `parentJob.ParentID = &stepID` (not parentJobID)
  - Log messages use "step_id" instead of "parent_job_id"
  - Event payloads use "step_id" field
  - Return statements return `stepID`

## Key Changes

```
Before:                            After:
func CreateJobs(..., parentJobID)  func CreateJobs(..., stepID)
job.ParentID = &parentJobID        job.ParentID = &stepID
"parent_job_id" in logs            "step_id" in logs
return parentJobID                 return stepID
```

## Hierarchy Result

Jobs created by workers now have:
- `parent_id = stepID` - direct parent is the step
- Manager ID can be retrieved from step metadata

## Files Changed

- `internal/interfaces/job_interfaces.go` - Updated docs for CreateJobs
- `internal/queue/workers/github_repo_worker.go`
- `internal/queue/workers/github_log_worker.go`
- `internal/queue/workers/crawler_worker.go`
- `internal/queue/workers/places_worker.go`
- `internal/queue/workers/agent_worker.go`
- `internal/queue/workers/web_search_worker.go`

## Verify

Build: ✅ | Tests: ⏭️
