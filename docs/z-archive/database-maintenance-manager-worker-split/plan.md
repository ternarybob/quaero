# Plan: Database Maintenance Manager/Worker Split (ARCH-008)

## Overview

Complete the Manager/Worker architecture by splitting database maintenance into proper orchestration (manager) and execution (worker) layers. The manager currently creates a single job with multiple operations; it needs to create parent + child jobs following the established pattern from ARCH-004 through ARCH-007.

## Steps

1. **Update DatabaseMaintenanceManager to create parent + child jobs**
   - Skill: @code-architect
   - Files: `internal/jobs/manager/database_maintenance_manager.go`
   - User decision: no
   - Changes:
     - Add JobOrchestrator dependency to struct and constructor
     - Rewrite CreateParentJob() to create parent job record
     - Loop through operations and create child job for each operation
     - Each child job config: `{"operation": "vacuum"}` (single operation, not array)
     - Start JobOrchestrator monitoring after enqueueing children
     - Update job types: parent=`"database_maintenance_parent"`, child=`"database_maintenance_operation"`

2. **Create DatabaseMaintenanceWorker for individual operation execution**
   - Skill: @go-coder
   - Files: `internal/jobs/worker/database_maintenance_worker.go` (NEW)
   - User decision: no
   - Implementation:
     - Struct with 3 dependencies: db, jobMgr, logger
     - Implement JobWorker interface: GetWorkerType(), Validate(), Execute()
     - Job type: `"database_maintenance_operation"`
     - Execute() processes single operation from config
     - Copy operation methods from old executor: vacuum(), analyze(), reindex(), optimize()
     - Simpler than old executor (no BaseExecutor, no progress tracking within job)

3. **Delete old DatabaseMaintenanceExecutor**
   - Skill: @none
   - Files: `internal/jobs/executor/database_maintenance_executor.go` (DELETE)
   - User decision: no
   - Rationale: Breaking changes acceptable, clean migration

4. **Update app.go registration**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no
   - Changes:
     - Remove old executor registration (lines 334-344)
     - Add new worker registration with 3 dependencies (db, jobMgr, logger)
     - Update manager constructor to include jobOrchestrator parameter (line 392)
     - Update log messages

5. **Compile and validate**
   - Skill: @go-coder
   - Files: All modified files
   - User decision: no
   - Verification:
     - Application compiles successfully
     - No import errors or type mismatches
     - Startup logs show worker registration

6. **Update documentation**
   - Skill: @none
   - Files: `AGENTS.md`, `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
   - User decision: no
   - Updates:
     - Mark ARCH-008 complete in migration status
     - Add database_maintenance_worker.go to worker directory listing
     - Update remaining file counts
     - Document architectural change (single job → parent + child jobs)

## Success Criteria

- Manager creates parent job + N child jobs (one per operation)
- Worker processes single operation per job
- Old executor deleted from codebase
- App.go registers new worker with correct dependencies
- Application compiles without errors
- Documentation reflects ARCH-008 completion
- Job types updated: parent=`"database_maintenance_parent"`, child=`"database_maintenance_operation"`
- JobOrchestrator monitors parent job progress

## Architectural Pattern

**Before (WRONG):**
```
Manager creates: 1 job with config {"operations": ["vacuum", "analyze", "reindex"]}
Worker processes: ALL operations in single job
```

**After (CORRECT):**
```
Manager creates: 1 parent job + 3 child jobs
  - Child 1: {"operation": "vacuum"}
  - Child 2: {"operation": "analyze"}
  - Child 3: {"operation": "reindex"}
Worker processes: ONE operation per job
JobOrchestrator: Monitors all children, updates parent progress
```

## Dependencies

**Manager:**
- jobManager (*jobs.Manager)
- queueMgr (*queue.Manager)
- jobOrchestrator (orchestrator.JobOrchestrator) - NEW
- logger (arbor.ILogger)

**Worker:**
- db (*sql.DB)
- jobMgr (*jobs.Manager)
- logger (arbor.ILogger)

## Job Types

- **Parent**: `"database_maintenance_parent"` (orchestration tracking)
- **Child**: `"database_maintenance_operation"` (individual operations)
- **Old**: `"database_maintenance"` (no longer supported after migration)
