# Phase 2: Stepped Migration Plan

**Goal:** Update all implementation files to use new type names
**Approach:** Incremental steps with verification after each
**Breaking Changes:** Acceptable - goal is clearer code

## Overview

This plan breaks Phase 2 into small, verifiable steps. Each step:
1. Updates a specific set of files
2. Attempts to build
3. Verifies progress
4. Documents any issues

## Step 1: Update Job Manager Core

**Files:**
- `internal/jobs/manager.go`

**Changes:**
- Line 78: `*models.JobModel` → `*models.JobQueued`
- Line 92: `models.NewJob()` → `models.NewJobExecutionState()`
- Line 127: `models.NewJobModel()` → `models.NewJobQueued()`
- Line 130: `models.NewJob()` → `models.NewJobExecutionState()`
- Line 168: `models.NewJobModel()` → `models.NewJobQueued()`
- Line 172: `models.NewJob()` → `models.NewJobExecutionState()`
- Line 198: `*models.Job` → `*models.JobExecutionState`
- Line 325: `*models.Job` → `*models.JobExecutionState`
- Line 666: `*models.Job` → `*models.JobExecutionState`

**Verification:**
```bash
go build ./internal/jobs/... 2>&1 | grep "manager.go"
```

**Expected:** Errors in manager.go should be eliminated

## Step 2: Update Job Definition Orchestrator

**Files:**
- `internal/jobs/job_definition_orchestrator.go`

**Changes:**
- Line 369: `*models.JobModel` → `*models.JobQueued`
- Search for all `models.Job` and `models.JobModel` references
- Update type assertions and function calls

**Verification:**
```bash
go build ./internal/jobs/... 2>&1 | grep "orchestrator.go"
```

**Expected:** Errors in orchestrator.go should be eliminated

## Step 3: Update Logs Service

**Files:**
- `internal/logs/service.go`

**Changes:**
- Line 82: `models.Job` → `models.JobExecutionState`
- Line 94: `models.JobModel` → `models.JobQueued`
- Line 204: `models.JobModel` → `models.JobQueued`

**Verification:**
```bash
go build ./internal/logs/... 2>&1 | grep "service.go"
```

**Expected:** Errors in logs/service.go should be eliminated

## Step 4: Update Worker Implementations

**Files:**
- `internal/jobs/worker/crawler_worker.go`
- `internal/jobs/worker/agent_worker.go`
- `internal/jobs/worker/database_maintenance_worker.go`
- `internal/jobs/worker/job_processor.go`

**Changes:**
- Update `Execute(ctx context.Context, job *models.JobQueued)` signatures
- Update `Validate(job *models.JobQueued)` signatures
- Update all type assertions from `*models.Job` to `*models.JobExecutionState`
- Update all type assertions from `*models.JobModel` to `*models.JobQueued`
- Update all `NewJob()` calls to `NewJobExecutionState()`
- Update all `NewJobModel()` calls to `NewJobQueued()`

**Search Commands:**
```bash
rg "models\.Job[^Q]" internal/jobs/worker/
rg "models\.JobModel" internal/jobs/worker/
rg "NewJob\(" internal/jobs/worker/
rg "NewJobModel\(" internal/jobs/worker/
```

**Verification:**
```bash
go build ./internal/jobs/worker/...
```

**Expected:** Worker package compiles successfully

## Step 5: Update Manager Implementations

**Files:**
- `internal/jobs/manager/crawler_manager.go`
- `internal/jobs/manager/agent_manager.go`
- `internal/jobs/manager/database_maintenance_manager.go`
- `internal/jobs/manager/transform_manager.go`
- `internal/jobs/manager/reindex_manager.go`
- `internal/jobs/manager/places_search_manager.go`

**Changes:**
- Update all `NewJobModel()` calls to `NewJobQueued()`
- Update all `NewChildJobModel()` calls to `NewJobQueuedChild()`
- Update all type references from `*models.JobModel` to `*models.JobQueued`

**Search Commands:**
```bash
rg "NewJobModel\(" internal/jobs/manager/
rg "NewChildJobModel\(" internal/jobs/manager/
rg "models\.JobModel" internal/jobs/manager/
```

**Verification:**
```bash
go build ./internal/jobs/manager/...
```

**Expected:** Manager package compiles successfully

## Step 6: Update Monitor Implementation

**Files:**
- `internal/jobs/monitor/job_monitor.go`

**Changes:**
- Update `StartMonitoring(ctx context.Context, job *models.JobQueued)` signature
- Update all type references from `*models.JobModel` to `*models.JobQueued`
- Update all type references from `*models.Job` to `*models.JobExecutionState`

**Verification:**
```bash
go build ./internal/jobs/monitor/...
```

**Expected:** Monitor package compiles successfully

## Step 7: Update Services Layer

**Files:**
- `internal/services/jobs/service.go`
- `internal/services/crawler/service.go` (already partially updated)
- `internal/services/crawler/types.go` (already partially updated)

**Changes:**
- `CreateJobFromDefinition()` returns `*models.JobQueued` (was `*models.JobModel`)
- Update all `NewJobModel()` calls to `NewJobQueued()`
- Update all `job.ToJobModel()` calls to `job.ToJobQueued()`
- Update all type assertions

**Search Commands:**
```bash
rg "models\.JobModel" internal/services/
rg "ToJobModel\(\)" internal/services/
rg "NewJobModel\(" internal/services/
```

**Verification:**
```bash
go build ./internal/services/...
```

**Expected:** Services package compiles successfully

## Step 8: Update Handlers Layer

**Files:**
- `internal/handlers/job_handler.go`
- `internal/handlers/job_definition_handler.go`

**Changes:**
- Update all type assertions from `*models.Job` to `*models.JobExecutionState`
- Update all type assertions from `*models.JobModel` to `*models.JobQueued`
- Update function calls to use new types

**Search Commands:**
```bash
rg "models\.Job[^Q]" internal/handlers/
rg "models\.JobModel" internal/handlers/
```

**Verification:**
```bash
go build ./internal/handlers/...
```

**Expected:** Handlers package compiles successfully

## Step 9: Update Storage Layer

**Files:**
- `internal/storage/badger/job_storage.go` (already updated in Phase 1)
- Verify all methods match interface signatures

**Changes:**
- Verify `GetChildJobs()` returns `[]*models.JobQueued`
- Verify `GetJobsByStatus()` returns `[]*models.JobQueued`
- Verify `GetStaleJobs()` returns `[]*models.JobQueued`
- Verify `ListJobs()` returns `[]*models.JobExecutionState`

**Verification:**
```bash
go build ./internal/storage/badger/...
```

**Expected:** Storage package compiles successfully

## Step 10: Full Build Verification

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

## Step 11: Update Tests

**Files:**
- `test/ui/queue_test.go`
- `test/api/*_test.go`
- `internal/*/`*_test.go`

**Changes:**
- Update all type references to use new names
- Update all function calls to use new names
- Verify test assertions still valid

**Verification:**
```bash
go test ./...
```

**Expected:** All tests pass (or fail for reasons unrelated to renaming)

## Step 12: Run Queue Test

**Command:**
```bash
go test -v -timeout 180s ./test/ui -run "^TestQueue$"
```

**Expected:**
- ✅ No BadgerHold reflection errors
- ✅ Job appears in queue
- ✅ Job executes (may fail due to missing API key, but should start)

## Step 13: Update Documentation

**Files:**
- `README.md` - Update job architecture section
- `AGENTS.md` - Update job structure references
- Code comments throughout codebase

**Search Commands:**
```bash
rg "JobModel" README.md AGENTS.md docs/
rg "Job struct" README.md AGENTS.md docs/
```

**Changes:**
- Replace references to old names with new names
- Update architecture diagrams if needed
- Update code examples

## Automation Script

To speed up the process, here's a PowerShell script to find all occurrences:

```powershell
# Find all files with old type names
$files = @(
    "internal/jobs/manager.go",
    "internal/jobs/job_definition_orchestrator.go",
    "internal/logs/service.go",
    "internal/jobs/worker/*.go",
    "internal/jobs/manager/*.go",
    "internal/jobs/monitor/*.go",
    "internal/services/**/*.go",
    "internal/handlers/*.go"
)

foreach ($pattern in @("models\.Job[^Q]", "models\.JobModel", "NewJob\(", "NewJobModel\(", "ToJobModel\(")) {
    Write-Host "`n=== Searching for: $pattern ===`n"
    rg $pattern $files
}
```

## Rollback Plan

If migration fails at any step:

1. Identify the failing step
2. Review the error messages
3. Fix the specific issue
4. Continue from that step

If complete rollback needed:
1. Revert all changes: `git reset --hard HEAD`
2. Keep only Phase 1 changes (BadgerHold fix)
3. Consider type alias approach instead

## Success Criteria

Phase 2 is complete when:
- ✅ All code compiles without errors
- ✅ All tests pass
- ✅ Queue test passes (job appears and executes)
- ✅ No BadgerHold reflection errors
- ✅ Clear naming conventions throughout codebase
- ✅ Documentation updated

## Estimated Time

- Steps 1-3: 30 minutes
- Steps 4-6: 1 hour
- Steps 7-9: 1 hour
- Steps 10-12: 30 minutes
- Step 13: 30 minutes

**Total:** 3-4 hours

## Current Status

- ✅ Phase 1 complete (BadgerHold fix)
- ✅ Interfaces updated
- ⏳ Ready to begin Step 1

**Next Action:** Await user approval to proceed with Step 1

