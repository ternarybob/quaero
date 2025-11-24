# Migration Plan: V1 to V2 Architecture (Revised Naming)

**Goal:** Rename job structures and operations for clarity
**Breaking Changes:** Acceptable - goal is clearer code and naming conventions
**Status:** Phase 1 Complete (BadgerHold fix), Phase 2-6 Pending

## Revised Naming Conventions

**Three Clear Domains:**
1. **Jobs Domain** - Job definitions (Job or JobDefinition)
2. **Queue Domain** - Queued work (QueueJob prefix)
3. **Queue State Domain** - Runtime information (QueueJobState prefix)

## Overview

This migration renames core job structures to provide clear separation:

| Old Name (V1) | V2 Name (Phase 1) | **V3 Name (Revised)** | Purpose |
|---------------|-------------------|----------------------|---------|
| `JobModel` | `JobQueued` | **`QueueJob`** | Immutable queued job |
| `Job` | `JobExecutionState` | **`QueueJobState`** | Runtime state |
| `NewJobModel()` | `NewJobQueued()` | **`NewQueueJob()`** | Create queued job |
| `NewChildJobModel()` | `NewJobQueuedChild()` | **`NewQueueJobChild()`** | Create child job |
| `NewJob()` | `NewJobExecutionState()` | **`NewQueueJobState()`** | Create state |
| `Job.ToJobModel()` | `JobExecutionState.ToJobQueued()` | **`QueueJobState.ToQueueJob()`** | Extract queued job |

## Phase 1: Fix BadgerHold Serialization ✅ COMPLETE

**Status:** ✅ Complete
**Files Modified:**
- `internal/models/job_model.go` - Renamed structs and methods
- `internal/storage/badger/job_storage.go` - Store only `JobQueued`

**Changes:**
1. ✅ Renamed `JobModel` → `JobQueued` → **`QueueJob`** (revised)
2. ✅ Renamed `Job` → `JobExecutionState` → **`QueueJobState`** (revised)
3. ✅ Updated storage to save/load `QueueJob` only
4. ✅ Changed `Progress` from pointer to value type
5. ✅ Removed status filtering from database queries (TODO: implement via job logs)

**Test Results:**
- ✅ BadgerHold reflection error eliminated
- ✅ Jobs appear in queue
- ⚠️ Job completion pending (separate issue - likely missing API key)

## Phase 2: Update Interface Definitions

**Goal:** Update all interface definitions to use new type names

**Files to Update:**
1. `internal/interfaces/job_interfaces.go`
   - `JobWorker.Execute(ctx, job *models.QueueJob)` (was `*models.JobModel`)
   - `JobWorker.Validate(job *models.QueueJob)` (was `*models.JobModel`)
   - `JobSpawner.SpawnChildJob(..., parentJob *models.QueueJob, ...)` (was `*models.JobModel`)
   - `JobMonitor.StartMonitoring(ctx, job *models.QueueJob)` (was `*models.JobModel`)

**Verification:**
```bash
# Search for old type references in interfaces
rg "models\.JobModel|models\.JobQueued" internal/interfaces/
rg "models\.Job[^Q]|models\.JobExecutionState" internal/interfaces/
```

**Expected Impact:**
- All workers will need signature updates
- All managers will need signature updates
- Compilation will fail until Phase 3 complete

## Phase 3: Update Worker Implementations

**Goal:** Update all worker implementations to use `*models.JobQueued`

**Files to Update:**
1. `internal/jobs/worker/crawler_worker.go`
   - `Execute(ctx context.Context, job *models.JobQueued) error`
   - `Validate(job *models.JobQueued) error`
   - Update all type assertions from `*models.Job` to `*models.JobExecutionState`
   - Update all type assertions from `*models.JobModel` to `*models.JobQueued`

2. `internal/jobs/worker/agent_worker.go`
   - Same changes as crawler_worker.go

3. `internal/jobs/worker/database_maintenance_worker.go`
   - Same changes as crawler_worker.go

4. `internal/jobs/worker/job_processor.go`
   - Update worker registration to use new types
   - Update job routing logic

**Verification:**
```bash
# Search for old type references in workers
rg "models\.JobModel" internal/jobs/worker/
rg "\*models\.Job[^Q]" internal/jobs/worker/
```

**Expected Impact:**
- Workers will compile after updates
- May reveal additional type assertion issues

## Phase 4: Update Manager Implementations

**Goal:** Update all manager implementations to use `*models.JobQueued`

**Files to Update:**
1. `internal/jobs/manager/crawler_manager.go`
   - Update job creation to use `NewJobQueued()`
   - Update child job creation to use `NewJobQueuedChild()`

2. `internal/jobs/manager/agent_manager.go`
   - Same changes as crawler_manager.go

3. `internal/jobs/manager/database_maintenance_manager.go`
   - Same changes as crawler_manager.go

4. `internal/jobs/manager/transform_manager.go`
   - Same changes as crawler_manager.go

5. `internal/jobs/manager/reindex_manager.go`
   - Same changes as crawler_manager.go

6. `internal/jobs/manager/places_search_manager.go`
   - Same changes as crawler_manager.go

**Verification:**
```bash
# Search for old function calls
rg "NewJobModel\(" internal/jobs/manager/
rg "NewChildJobModel\(" internal/jobs/manager/
```

**Expected Impact:**
- Managers will compile after updates
- Job creation will use new naming

## Phase 5: Update Service Layer

**Goal:** Update services that create or manipulate jobs

**Files to Update:**
1. `internal/services/jobs/service.go`
   - `CreateJobFromDefinition()` returns `*models.JobQueued` (was `*models.JobModel`)

2. `internal/services/crawler/service.go`
   - Update all job creation to use `NewJobQueued()`
   - Update all type assertions to use `*models.JobExecutionState`
   - Update all `job.ToJobModel()` to `job.ToJobQueued()`

3. `internal/services/crawler/types.go`
   - Update conversion functions between CrawlJob and JobExecutionState

**Verification:**
```bash
# Search for old type references in services
rg "models\.JobModel" internal/services/
rg "ToJobModel\(\)" internal/services/
```

**Expected Impact:**
- Services will compile after updates
- Job creation flows will use new types

## Phase 6: Update Handlers and API Layer

**Goal:** Update HTTP handlers that expose job data

**Files to Update:**
1. `internal/handlers/job_handler.go`
   - Update type assertions from `*models.Job` to `*models.JobExecutionState`
   - Update JSON responses (field names unchanged, only internal types)

2. `internal/handlers/job_definition_handler.go`
   - Update job creation calls to use new types

3. `internal/jobs/manager.go`
   - Update `GetJobInternal()` to use `*models.JobExecutionState`
   - Update all type assertions

**Verification:**
```bash
# Search for old type references in handlers
rg "models\.Job[^Q]" internal/handlers/
rg "models\.Job[^Q]" internal/jobs/manager.go
```

**Expected Impact:**
- API endpoints will compile after updates
- JSON responses unchanged (backward compatible)

## Phase 7: Update Tests

**Goal:** Update all tests to use new type names

**Files to Update:**
1. `test/ui/queue_test.go` - Update any direct job type references
2. `test/api/*_test.go` - Update API test assertions
3. `internal/*/`*_test.go` - Update unit tests

**Verification:**
```bash
# Run all tests
go test ./...
```

**Expected Impact:**
- All tests should pass
- May reveal edge cases in type conversions

## Phase 8: Update Documentation

**Goal:** Update all documentation to reflect new naming

**Files to Update:**
1. `README.md` - Update job architecture section
2. `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` - Replace with V2
3. `AGENTS.md` - Update job structure references
4. Code comments throughout codebase

**Verification:**
```bash
# Search for old terminology in docs
rg "JobModel" docs/ README.md AGENTS.md
```

## Execution Strategy

### Recommended Approach: Big Bang Migration

**Rationale:** Type renames affect hundreds of files. Incremental migration would leave codebase in broken state for extended period.

**Steps:**
1. ✅ Phase 1 complete (BadgerHold fix tested)
2. Create feature branch: `refactor/job-naming-v2`
3. Execute Phases 2-6 in sequence (expect ~2-4 hours)
4. Run full test suite after each phase
5. Execute Phase 7 (fix failing tests)
6. Execute Phase 8 (update docs)
7. Final integration test
8. Merge to main

### Alternative Approach: Gradual Migration with Aliases

**If big bang is too risky:**

1. Create type aliases in `job_model.go`:
   ```go
   // Deprecated: Use JobQueued instead
   type JobModel = JobQueued
   
   // Deprecated: Use JobExecutionState instead
   type Job = JobExecutionState
   ```

2. Update code incrementally over multiple PRs
3. Remove aliases once all references updated

**Downside:** Confusing to have both names in codebase during transition

## Testing Checklist

After each phase, verify:

- [ ] Code compiles without errors
- [ ] `go test ./...` passes
- [ ] Queue test passes: `go test -v ./test/ui -run TestQueue`
- [ ] Job creation works via UI
- [ ] Jobs appear in queue
- [ ] Workers execute jobs
- [ ] Job completion tracked correctly
- [ ] WebSocket updates work
- [ ] No BadgerHold reflection errors

## Rollback Plan

If migration fails:

1. Revert to main branch
2. Keep Phase 1 changes (BadgerHold fix is critical)
3. Investigate specific failure
4. Fix and retry

## Success Criteria

Migration is successful when:

1. ✅ All code compiles without errors
2. ✅ All tests pass
3. ✅ Queue test passes (job appears and completes)
4. ✅ No BadgerHold serialization errors
5. ✅ Clear naming conventions throughout codebase
6. ✅ Documentation updated to reflect new architecture

## Current Status

**Phase 1:** ✅ COMPLETE
- BadgerHold serialization fixed
- Jobs appear in queue
- Ready to proceed with Phase 2

**Next Steps:**
1. Test current changes thoroughly
2. Create feature branch for Phases 2-8
3. Execute migration plan

