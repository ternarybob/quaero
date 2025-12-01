# Step 8: Test with github-repo-collector-by-name.toml

Model: manual | Status: ⏳

## Manual Testing Required

This task requires manual testing with the running server.

## Test Job Definition

Location: `test/config/job-definitions/github-repo-collector-by-name.toml`

Structure:
- Single step: `fetch_repo_content` (type: github_repo)
- Tests connector_name lookup
- Max 10 files per test run

## Testing Steps

### 1. Start the Server
```bash
cd C:\development\quaero
go run cmd/quaero/main.go
```

### 2. Navigate to Queue Management
- Open browser to http://localhost:8080
- Go to Queue Management page

### 3. Execute Job Definition
- Find "GitHub Repository Collector (By Name)" job definition
- Click Execute
- Observe the job hierarchy

### 4. Expected Behavior

With the new architecture:
```
Manager Job (type="manager")
└── Step: fetch_repo_content (type="step", parent=manager)
    └── Worker Jobs (type="github_repo", parent=step)
```

Events should flow:
1. `step_progress` events update step status in UI
2. `manager_progress` events update manager status (when ManagerMonitor added)
3. Job panel shows step hierarchy correctly

### 5. Verify

- [ ] Manager job created with type="manager"
- [ ] Step job created with type="step", parent_id=manager_id
- [ ] Worker jobs created with parent_id=step_id, manager_id set
- [ ] StepMonitor starts for the step
- [ ] step_progress events published
- [ ] UI updates with step progress
- [ ] Step completes when all child jobs finish
- [ ] Manager shows overall progress

## Known Limitations

- ManagerMonitor not yet implemented (Task 3 only created StepMonitor)
- Manager job status not yet automatically tracked
- For full hierarchy tracking, ManagerMonitor would need to be created similar to StepMonitor

## Files Changed in This Feature

1. Models: Added JobTypeManager, JobTypeStep, ManagerID field
2. ExecuteJobDefinition: Creates manager + step jobs
3. StepMonitor: Monitors step's child jobs
4. Workers: Use stepID parameter, jobs reference step as parent
5. Events: Added EventStepProgress, EventManagerProgress
6. Storage: GetStepStats, ListStepJobs methods
7. UI: Handles step_progress and manager_progress events

## Verify

Build: ✅ | Manual Test: ⏳
