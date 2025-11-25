# Phase 2: Revised Stepped Migration Plan

**Goal:** Update all implementation files to use revised naming conventions
**Approach:** Incremental steps with verification after each
**Breaking Changes:** Acceptable - goal is clearer code

## Revised Naming Conventions

**Three Clear Domains:**
1. **Jobs Domain** - Job definitions (Job or JobDefinition)
2. **Queue Domain** - Queued work (Queue prefix)
3. **Queue State Domain** - Runtime information (QueueJobState prefix)

### Complete Mapping

| Old Name (V1) | V2 (Phase 1) | **V3 (Revised)** | Domain |
|---------------|--------------|------------------|--------|
| `JobModel` | `JobQueued` | **`QueueJob`** | Queue |
| `Job` | `JobExecutionState` | **`QueueJobState`** | Queue State |
| `NewJobModel()` | `NewJobQueued()` | **`NewQueueJob()`** | Queue |
| `NewChildJobModel()` | `NewJobQueuedChild()` | **`NewQueueJobChild()`** | Queue |
| `NewJob()` | `NewJobExecutionState()` | **`NewQueueJobState()`** | Queue State |
| `Job.ToJobModel()` | `JobExecutionState.ToJobQueued()` | **`QueueJobState.ToQueueJob()`** | Queue State |
| `JobModel.FromJSON()` | `JobQueued.FromJSON()` | **`QueueJob.FromJSON()` | Queue |

## Step 0: Update Core Model File

**Files:**
- `internal/models/job_model.go`

**Changes:**
- Rename `JobQueued` → `QueueJob`
- Rename `JobExecutionState` → `QueueJobState`
- Rename all methods:
  - `NewJobQueued()` → `NewQueueJob()`
  - `NewJobQueuedChild()` → `NewQueueJobChild()`
  - `NewJobExecutionState()` → `NewQueueJobState()`
  - `JobQueuedFromJSON()` → `QueueJobFromJSON()`
  - `JobExecutionState.ToJobQueued()` → `QueueJobState.ToQueueJob()`

**Verification:**
```bash
go build ./internal/models/... 2>&1 | grep "job_model.go"
```

**Expected:** Models package compiles successfully

## Step 1: Update Interface Definitions

**Files:**
- `internal/interfaces/job_interfaces.go`
- `internal/interfaces/queue_service.go`
- `internal/interfaces/storage.go`

**Changes:**
- Replace `*models.JobQueued` → `*models.QueueJob`
- Replace `*models.JobExecutionState` → `*models.QueueJobState`
- Replace `[]*models.JobQueued` → `[]*models.QueueJob`
- Replace `[]*models.JobExecutionState` → `[]*models.QueueJobState`

**Verification:**
```bash
go build ./internal/interfaces/... 2>&1 | grep "interfaces"
```

**Expected:** Interfaces package compiles successfully

## Step 2: Update Storage Layer

**Files:**
- `internal/storage/badger/job_storage.go`

**Changes:**
- Update `SaveJob()` to accept `*models.QueueJobState` and store `QueueJob`
- Update `GetJob()` to load `QueueJob` and convert to `QueueJobState`
- Update `ListJobs()` to return `[]*models.QueueJobState`
- Update `GetChildJobs()` to return `[]*models.QueueJob`
- Update `GetJobsByStatus()` to return `[]*models.QueueJob`
- Update `GetStaleJobs()` to return `[]*models.QueueJob`
- Update all internal references

**Verification:**
```bash
go build ./internal/storage/badger/... 2>&1 | grep "job_storage.go"
```

**Expected:** Storage package compiles successfully

## Step 3: Update Job Manager Core

**Files:**
- `internal/jobs/manager.go`

**Changes:**
- Replace `models.NewJobQueued()` → `models.NewQueueJob()`
- Replace `models.NewJobExecutionState()` → `models.NewQueueJobState()`
- Replace `*models.JobQueued` → `*models.QueueJob`
- Replace `*models.JobExecutionState` → `*models.QueueJobState`
- Update all type assertions

**Search Commands:**
```bash
rg "models\.JobQueued|models\.JobExecutionState" internal/jobs/manager.go
rg "NewJobQueued|NewJobExecutionState" internal/jobs/manager.go
```

**Verification:**
```bash
go build ./internal/jobs/... 2>&1 | grep "manager.go"
```

**Expected:** Manager.go compiles successfully

## Step 4: Update Job Definition Orchestrator

**Files:**
- `internal/jobs/job_definition_orchestrator.go`

**Changes:**
- Replace `*models.JobQueued` → `*models.QueueJob`
- Replace `*models.JobExecutionState` → `*models.QueueJobState`
- Update all type assertions and function calls

**Verification:**
```bash
go build ./internal/jobs/... 2>&1 | grep "orchestrator.go"
```

**Expected:** Orchestrator compiles successfully

## Step 5: Update Worker Implementations

**Files:**
- `internal/jobs/worker/crawler_worker.go`
- `internal/jobs/worker/agent_worker.go`
- `internal/jobs/worker/database_maintenance_worker.go`
- `internal/jobs/worker/job_processor.go`

**Changes:**
- Update `Execute(ctx context.Context, job *models.QueueJob)` signatures
- Update `Validate(job *models.QueueJob)` signatures
- Replace `models.NewJobExecutionState()` → `models.NewQueueJobState()`
- Replace `models.NewJobQueued()` → `models.NewQueueJob()`
- Replace `models.NewJobQueuedChild()` → `models.NewQueueJobChild()`
- Update all type assertions

**Search Commands:**
```bash
rg "models\.JobQueued|models\.JobExecutionState" internal/jobs/worker/
rg "NewJobQueued|NewJobExecutionState|NewJobQueuedChild" internal/jobs/worker/
```

**Verification:**
```bash
go build ./internal/jobs/worker/...
```

**Expected:** Worker package compiles successfully

## Step 6: Update Manager Implementations

**Files:**
- `internal/jobs/manager/crawler_manager.go`
- `internal/jobs/manager/agent_manager.go`
- `internal/jobs/manager/database_maintenance_manager.go`
- `internal/jobs/manager/transform_manager.go`
- `internal/jobs/manager/reindex_manager.go`
- `internal/jobs/manager/places_search_manager.go`

**Changes:**
- Replace `models.NewJobQueued()` → `models.NewQueueJob()`
- Replace `models.NewJobQueuedChild()` → `models.NewQueueJobChild()`
- Replace `*models.JobQueued` → `*models.QueueJob`

**Search Commands:**
```bash
rg "NewJobQueued|NewJobQueuedChild" internal/jobs/manager/
rg "models\.JobQueued" internal/jobs/manager/
```

**Verification:**
```bash
go build ./internal/jobs/manager/...
```

**Expected:** Manager package compiles successfully

## Step 7: Update Monitor Implementation

**Files:**
- `internal/jobs/monitor/job_monitor.go`

**Changes:**
- Update `StartMonitoring(ctx context.Context, job *models.QueueJob)` signature
- Replace `*models.JobQueued` → `*models.QueueJob`
- Replace `*models.JobExecutionState` → `*models.QueueJobState`

**Verification:**
```bash
go build ./internal/jobs/monitor/...
```

**Expected:** Monitor package compiles successfully

## Step 8: Update Services Layer

**Files:**
- `internal/services/jobs/service.go`
- `internal/services/crawler/service.go`
- `internal/services/crawler/types.go`

**Changes:**
- Update `CreateJobFromDefinition()` to return `*models.QueueJob`
- Replace `models.NewJobQueued()` → `models.NewQueueJob()`
- Replace `job.ToJobQueued()` → `job.ToQueueJob()`
- Replace `*models.JobQueued` → `*models.QueueJob`
- Replace `*models.JobExecutionState` → `*models.QueueJobState`

**Search Commands:**
```bash
rg "models\.JobQueued|models\.JobExecutionState" internal/services/
rg "ToJobQueued|NewJobQueued" internal/services/
```

**Verification:**
```bash
go build ./internal/services/...
```

**Expected:** Services package compiles successfully

## Step 9: Update Handlers Layer

**Files:**
- `internal/handlers/job_handler.go`
- `internal/handlers/job_definition_handler.go`

**Changes:**
- Replace `*models.JobQueued` → `*models.QueueJob`
- Replace `*models.JobExecutionState` → `*models.QueueJobState`
- Update all type assertions

**Search Commands:**
```bash
rg "models\.JobQueued|models\.JobExecutionState" internal/handlers/
```

**Verification:**
```bash
go build ./internal/handlers/...
```

**Expected:** Handlers package compiles successfully

## Step 10: Update Logs Service

**Files:**
- `internal/logs/service.go`

**Changes:**
- Replace `models.JobQueued` → `models.QueueJob`
- Replace `models.JobExecutionState` → `models.QueueJobState`

**Verification:**
```bash
go build ./internal/logs/...
```

**Expected:** Logs package compiles successfully

## Step 11: Full Build Verification

**Command:**
```bash
.\scripts\build.ps1
```

**Expected:** Clean build with no errors

**If Errors Remain:**
1. Identify remaining files with errors
2. Apply same pattern: find old type names, replace with new
3. Rebuild
4. Repeat until clean

## Step 12: Update Tests

**Files:**
- `test/ui/queue_test.go`
- `test/api/*_test.go`
- `internal/*/`*_test.go`

**Changes:**
- Replace `models.JobQueued` → `models.QueueJob`
- Replace `models.JobExecutionState` → `models.QueueJobState`
- Replace `NewJobQueued` → `NewQueueJob`
- Replace `NewJobExecutionState` → `NewQueueJobState`

**Verification:**
```bash
go test ./...
```

**Expected:** All tests pass (or fail for reasons unrelated to renaming)

## Step 13: Run Queue Test

**Command:**
```bash
go test -v -timeout 180s ./test/ui -run "^TestQueue$"
```

**Expected:**
- ✅ No BadgerHold reflection errors
- ✅ Job appears in queue
- ✅ Job executes successfully

## Automation Script

PowerShell script to find all occurrences:

```powershell
# Find all files with old type names (Phase 1 names)
$patterns = @(
    "models\.JobQueued",
    "models\.JobExecutionState",
    "NewJobQueued\(",
    "NewJobQueuedChild\(",
    "NewJobExecutionState\(",
    "ToJobQueued\(",
    "JobQueuedFromJSON"
)

foreach ($pattern in $patterns) {
    Write-Host "`n=== Searching for: $pattern ===`n"
    rg $pattern internal/ test/
}
```

## Success Criteria

Phase 2 is complete when:
- ✅ All code compiles without errors
- ✅ All tests pass
- ✅ Queue test passes (job appears and executes)
- ✅ No BadgerHold reflection errors
- ✅ Clear naming conventions: Jobs vs Queue vs QueueState
- ✅ Documentation updated

## Estimated Time

- Step 0: 15 minutes (core models)
- Steps 1-2: 15 minutes (interfaces + storage)
- Steps 3-4: 30 minutes (manager + orchestrator)
- Steps 5-7: 1 hour (workers + managers + monitor)
- Steps 8-10: 45 minutes (services + handlers + logs)
- Steps 11-13: 30 minutes (build + tests)

**Total:** 3-4 hours

## Current Status

- ✅ Phase 1 complete (BadgerHold fix with JobQueued/JobExecutionState)
- ⏳ Ready to begin Step 0 (rename to QueueJob/QueueJobState)

**Next Action:** Await user approval to proceed with Step 0

