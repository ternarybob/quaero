I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The database maintenance system has a hybrid architecture that needs completion:

1. **Manager (Already Migrated in ARCH-004):**
   - Location: `internal/jobs/manager/database_maintenance_manager.go`
   - Current behavior: Creates 1 job with `{"operations": ["vacuum", "analyze", "reindex"]}`
   - Problem: Doesn't follow Manager/Worker pattern (should create child jobs)
   - Dependencies: jobManager, queueMgr, logger (3 dependencies)

2. **Old Executor (Needs Replacement):**
   - Location: `internal/jobs/executor/database_maintenance_executor.go`
   - Current behavior: Processes ALL operations in single job execution
   - Uses BaseExecutor pattern with 6 dependencies
   - Registered in app.go lines 335-344
   - Job type: `"database_maintenance"`

3. **BaseExecutor Pattern:**
   - Provides helper methods: CreateJobLogger, UpdateJobStatus, UpdateJobProgress, etc.
   - Used by old executor but NOT needed for new worker
   - New worker can be simpler with direct dependencies

**Key Architectural Insight:**

The current manager doesn't follow the established Manager/Worker pattern:
- **CrawlerManager** creates parent job + child jobs for each URL
- **AgentManager** creates parent job + child jobs for each document
- **DatabaseMaintenanceManager** creates 1 job with operations array (WRONG)

This needs to be corrected to:
- **DatabaseMaintenanceManager** creates parent job + child jobs for each operation

**Job Type Strategy:**

To avoid conflicts during migration:
- OLD job type: `"database_maintenance"` (processed by old executor)
- NEW job type: `"database_maintenance_operation"` (processed by new worker)
- Parent job type: `"database_maintenance_parent"` (for orchestration tracking)

This allows clean separation and prevents old jobs from being processed by new worker.

**Worker Design Comparison:**

**Old Executor (221 lines):**
- Extends BaseExecutor (6 dependencies)
- Processes multiple operations in sequence
- Progress tracking within job (i+1 of totalOps)
- Complex error handling with partial completion

**New Worker (Simpler):**
- Direct dependencies: db, jobMgr, logger (3 dependencies)
- Processes single operation per job
- No progress tracking (operation is atomic)
- Simple error handling (operation succeeds or fails)

**Manager Update Required:**

Current code (lines 48-61):
```go
// Get operations from step config
operations := []string{"vacuum", "analyze", "reindex"} // Default
// ... parse operations from config ...

// Create job model
jobModel := models.NewChildJobModel(
    parentJobID,
    "database_maintenance",
    step.Name,
    map[string]interface{}{
        "operations": operations,  // ❌ WRONG - array of operations
    },
    ...
)
```

Needs to become:
```go
// Create parent job for orchestration
parentJob := models.NewParentJobModel(...)

// Create child job for EACH operation
for _, operation := range operations {
    childJob := models.NewChildJobModel(
        parentJob.ID,
        "database_maintenance_operation",
        operation,
        map[string]interface{}{
            "operation": operation,  // ✅ CORRECT - single operation
        },
        ...
    )
    // Enqueue child job
}

// Start ParentJobOrchestrator monitoring
```

**Dependencies Analysis:**

**New Worker Dependencies:**
- **db** (*sql.DB): Required for VACUUM, ANALYZE, REINDEX, OPTIMIZE operations
- **jobMgr** (*jobs.Manager): Required for job status updates
- **logger** (arbor.ILogger): Required for structured logging

**Removed Dependencies (from old executor):**
- **queueMgr**: Not needed (worker doesn't spawn child jobs)
- **logService**: Not needed (use logger with correlation ID)
- **wsHandler**: Not needed (events published via jobMgr)

**Risk Assessment:**

- **Low Risk**: Manager update is straightforward (add loop for child jobs)
- **Low Risk**: Worker creation follows established pattern
- **Low Risk**: Old executor deletion (breaking changes acceptable)
- **Low Risk**: Single registration location in app.go
- **Very Low Risk**: No backward compatibility needed (user confirmed)

**Success Criteria:**

1. Manager creates parent job + N child jobs (one per operation)
2. Worker processes single operation per job
3. Old executor deleted from codebase
4. App.go registers new worker with correct dependencies
5. Application compiles without errors
6. Database maintenance job executes all operations
7. ParentJobOrchestrator tracks progress correctly
8. Job logs show individual operation execution
9. Documentation updated (AGENTS.md, MANAGER_WORKER_ARCHITECTURE.md)

### Approach

**Manager/Worker Split Strategy for Database Maintenance**

This phase completes the Manager/Worker architecture by splitting database maintenance into proper orchestration (manager) and execution (worker) layers. The approach follows the established pattern from ARCH-004 through ARCH-007.

**Key Architectural Change:**

**OLD Pattern (Single Job):**
- Manager creates 1 job with config: `{"operations": ["vacuum", "analyze", "reindex"]}`
- Worker executes ALL operations in that single job
- No parallelization, no granular progress tracking

**NEW Pattern (Parent + Child Jobs):**
- Manager creates 1 parent job + N child jobs (one per operation)
- Each child job has config: `{"operation": "vacuum"}` (single operation)
- Worker executes ONE operation per job
- Enables parallelization, granular progress tracking, individual operation retry

**Why This Approach:**

1. **Consistency**: Matches CrawlerManager pattern (parent job + child jobs for URLs)
2. **Granularity**: Each operation is a separate job with its own status/logs
3. **Parallelization**: Multiple operations can run concurrently (queue concurrency)
4. **Retry Logic**: Failed operations can be retried individually
5. **Progress Tracking**: ParentJobOrchestrator aggregates child progress
6. **Clean Architecture**: Clear separation between orchestration and execution

**Implementation Strategy:**

1. **Update Manager** - Modify existing `database_maintenance_manager.go` to create child jobs
2. **Create Worker** - New `database_maintenance_worker.go` for single operation execution
3. **Delete Old Executor** - Remove `database_maintenance_executor.go` immediately (breaking change OK)
4. **Update Registration** - Change app.go to register new worker instead of old executor
5. **Update Documentation** - Reflect ARCH-008 completion in AGENTS.md and architecture docs

**Key Design Decisions:**

**Manager Changes:**
- Create parent job record (for orchestration tracking)
- Loop through operations and create child job for each
- Each child job type: `"database_maintenance_operation"` (new type to distinguish from old)
- Child job config: `{"operation": "vacuum"}` (single operation, not array)
- Start ParentJobOrchestrator monitoring after enqueueing children

**Worker Design:**
- Simpler than old executor (no BaseExecutor dependency)
- Direct dependencies: db, jobMgr, logger (3 dependencies vs old 6)
- Single Execute() method that processes one operation
- Reuse operation methods: vacuum(), analyze(), reindex(), optimize()
- No progress tracking within job (each operation is atomic)

**Job Type Strategy:**
- OLD: `"database_maintenance"` (processed by old executor)
- NEW: `"database_maintenance_operation"` (processed by new worker)
- This allows clean migration without conflicts

**Breaking Changes (Acceptable per User):**
- Old executor deleted immediately (no deprecation period)
- Old job type `"database_maintenance"` no longer supported
- Any in-flight jobs with old type will fail (acceptable for single build)

**Validation Strategy:**

After implementation:
1. Verify manager creates parent + child jobs correctly
2. Verify worker processes single operation correctly
3. Verify ParentJobOrchestrator aggregates progress
4. Build application successfully
5. Run database maintenance job end-to-end
6. Verify all operations execute (vacuum, analyze, reindex, optimize)
7. Verify job logs and status updates work correctly

**Risk Assessment:**

- **Low Risk**: Manager update (add child job creation loop)
- **Low Risk**: Worker creation (simplified version of old executor)
- **Low Risk**: Old executor deletion (breaking changes acceptable)
- **Low Risk**: App.go registration update (single location)
- **Very Low Risk**: No backward compatibility needed

**Success Criteria:**

1. Manager creates parent job + child jobs (one per operation)
2. Worker processes single operation per job
3. Old executor deleted from codebase
4. App.go registers new worker (not old executor)
5. Application compiles and runs successfully
6. Database maintenance job executes all operations correctly
7. ParentJobOrchestrator tracks progress correctly
8. Documentation updated to reflect ARCH-008 completion

### Reasoning

I systematically explored the codebase to understand the database maintenance architecture:

1. **Read target files** - Examined both database_maintenance_executor.go (old worker) and database_maintenance_step_executor.go (already migrated manager)
2. **Read migrated manager** - Analyzed database_maintenance_manager.go to understand current implementation
3. **Read app.go** - Identified registration location (lines 335-344) and dependencies
4. **Read BaseExecutor** - Understood helper methods provided by base class
5. **Analyzed architecture** - Compared with CrawlerManager/CrawlerWorker pattern from ARCH-005
6. **Identified key change** - Manager needs to create child jobs (not single job with operations array)

This comprehensive exploration revealed:
- Current manager creates 1 job with operations array (wrong pattern)
- Old executor processes all operations in single job (monolithic)
- Need to split into parent + child jobs (one per operation)
- Worker should be simpler (no BaseExecutor, fewer dependencies)
- Job type should change to avoid conflicts during migration

## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant Manager as database_maintenance_manager.go
    participant Worker as database_maintenance_worker.go (NEW)
    participant OldExec as database_maintenance_executor.go (OLD)
    participant App as internal/app/app.go
    participant Build as Go Build System
    
    Note over Dev,Build: Phase 1: Update Manager (Parent + Child Jobs)
    
    Dev->>Manager: Update CreateParentJob() method
    Note right of Manager: OLD: Create 1 job with operations array<br/>NEW: Create parent + N child jobs<br/>Each child: single operation
    
    Dev->>Manager: Add ParentJobOrchestrator dependency
    Note right of Manager: Start monitoring after enqueueing children
    
    Dev->>Manager: Update job types
    Note right of Manager: Parent: "database_maintenance_parent"<br/>Child: "database_maintenance_operation"
    
    Dev->>Build: Compile updated manager
    Build-->>Dev: ✓ Manager compiles successfully
    
    Note over Dev,Build: Phase 2: Create New Worker
    
    Dev->>Worker: Create database_maintenance_worker.go
    Note right of Worker: Package: worker<br/>Struct: DatabaseMaintenanceWorker<br/>Dependencies: db, jobMgr, logger (3)<br/>Job type: "database_maintenance_operation"
    
    Dev->>Worker: Implement interface methods
    Note right of Worker: GetWorkerType() → "database_maintenance_operation"<br/>Validate() → check operation field<br/>Execute() → process single operation
    
    Dev->>Worker: Copy operation methods
    Note right of Worker: vacuum(), analyze(), reindex(), optimize()<br/>From old executor (lines 135-219)
    
    Dev->>Build: Compile new worker
    Build-->>Dev: ✓ Worker compiles successfully
    
    Note over Dev,Build: Phase 3: Delete Old Executor
    
    Dev->>OldExec: Delete database_maintenance_executor.go
    Note right of OldExec: Breaking change acceptable<br/>No backward compatibility<br/>Old job type no longer supported
    
    Note over Dev,Build: Phase 4: Update App Registration
    
    Dev->>App: Remove old executor registration (lines 334-344)
    Note right of App: Delete: executor.NewDatabaseMaintenanceExecutor()<br/>6 dependencies removed
    
    Dev->>App: Add new worker registration
    Note right of App: Add: worker.NewDatabaseMaintenanceWorker()<br/>3 dependencies: db, jobMgr, logger
    
    Dev->>App: Update manager constructor (line 392)
    Note right of App: Add parentJobOrchestrator parameter<br/>Matches CrawlerManager pattern
    
    Dev->>Build: Build application
    Build-->>Dev: ✓ Application compiles successfully
    
    Note over Dev,Build: Phase 5: Validation
    
    Dev->>Build: Run test suite
    Build-->>Dev: ✓ All tests pass
    
    Dev->>App: Start application
    App->>Manager: Initialize DatabaseMaintenanceManager
    App->>Worker: Register DatabaseMaintenanceWorker
    App-->>Dev: ✓ "Database maintenance worker registered for job type: database_maintenance_operation"
    
    Dev->>App: Trigger database maintenance job via UI
    App->>Manager: CreateParentJob(ctx, step, jobDef, parentJobID)
    Manager->>Manager: Create parent job record
    Manager->>Manager: Loop through operations
    loop For each operation
        Manager->>Manager: Create child job (single operation)
        Manager->>Manager: Enqueue child job to queue
    end
    Manager->>Manager: Start ParentJobOrchestrator monitoring
    Manager-->>App: Return parent job ID
    
    App->>Worker: Queue routes child job to worker
    Worker->>Worker: Execute(ctx, job)
    Worker->>Worker: Get operation from config
    Worker->>Worker: executeOperation(ctx, logger, "vacuum")
    Worker->>Worker: vacuum() - Execute VACUUM
    Worker->>Worker: Update job status to completed
    Worker-->>App: ✓ Operation completed
    
    Note over Dev,Build: Migration Complete<br/>Manager creates parent + child jobs<br/>Worker processes single operation<br/>Old executor deleted<br/>Architectural pattern complete

## Proposed File Changes

### internal\jobs\manager\database_maintenance_manager.go(MODIFY)

References: 

- internal\jobs\orchestrator\parent_job_orchestrator.go
- internal\models\job_model.go

Update DatabaseMaintenanceManager to create parent job + child jobs (one per operation) instead of single job with operations array.

**Current Behavior (WRONG):**
- Creates 1 job with config: `{"operations": ["vacuum", "analyze", "reindex"]}`
- Job type: `"database_maintenance"`
- Worker processes all operations in single job

**New Behavior (CORRECT):**
- Creates 1 parent job for orchestration tracking
- Creates N child jobs (one per operation)
- Each child job config: `{"operation": "vacuum"}` (single operation)
- Child job type: `"database_maintenance_operation"`
- Worker processes one operation per job

**Implementation Changes:**

1. **Import ParentJobOrchestrator** (line 7):
   - Add: `"github.com/ternarybob/quaero/internal/jobs/orchestrator"`

2. **Add Field to Struct** (line 24):
   - Add: `parentJobOrchestrator *orchestrator.ParentJobOrchestrator`

3. **Update Constructor** (line 28):
   - Add parameter: `parentJobOrchestrator *orchestrator.ParentJobOrchestrator`
   - Initialize field: `parentJobOrchestrator: parentJobOrchestrator`

4. **Rewrite CreateParentJob Method** (lines 38-130):

**Step 1: Create Parent Job Record**
- Generate parent job ID: `parentJobID := uuid.New().String()`
- Create parent job record in database:
  ```
  parentJob := &jobs.Job{
      ID:       parentJobID,
      ParentID: &parentJobID, // Self-reference for top-level parent
      Type:     "database_maintenance_parent",
      Name:     "Database Maintenance",
      Phase:    "orchestration",
      Status:   "running",
  }
  ```
- Call: `m.jobManager.CreateJobRecord(ctx, parentJob)`

**Step 2: Parse Operations from Config**
- Keep existing logic (lines 48-61)
- Default operations: `["vacuum", "analyze", "reindex"]`

**Step 3: Create Child Job for Each Operation**
- Loop through operations:
  ```
  for _, operation := range operations {
      childJobID := uuid.New().String()
      
      // Create child job model
      childJob := models.NewChildJobModel(
          parentJobID,
          "database_maintenance_operation",
          operation, // Use operation as name
          map[string]interface{}{
              "operation": operation, // Single operation
          },
          map[string]interface{}{
              "step_name": step.Name,
          },
          1, // depth
      )
      childJob.ID = childJobID
      
      // Create job record
      dbJob := &jobs.Job{
          ID:       childJobID,
          ParentID: &parentJobID,
          Type:     "database_maintenance_operation",
          Name:     fmt.Sprintf("Database Maintenance: %s", operation),
          Phase:    "execution",
          Status:   "pending",
      }
      m.jobManager.CreateJobRecord(ctx, dbJob)
      
      // Serialize and enqueue
      payloadBytes, _ := childJob.ToJSON()
      queueMsg := queue.Message{
          JobID:   childJobID,
          Type:    "database_maintenance_operation",
          Payload: json.RawMessage(payloadBytes),
      }
      m.queueMgr.Enqueue(ctx, queueMsg)
  }
  ```

**Step 4: Start ParentJobOrchestrator Monitoring**
- Convert parent job record to JobModel:
  ```
  parentJobModel := &models.JobModel{
      ID:       parentJobID,
      ParentID: &parentJobID,
      Type:     "database_maintenance_parent",
      Name:     "Database Maintenance",
      // ... other fields ...
  }
  ```
- Start monitoring: `m.parentJobOrchestrator.StartMonitoring(ctx, parentJobModel)`

**Step 5: Update Log Messages**
- Line 43: "Creating parent database maintenance job and child jobs for each operation"
- Line 127: "Database maintenance parent job created with N child jobs enqueued"
- Add log field: `.Int("child_job_count", len(operations))`

**Step 6: Return Parent Job ID**
- Return: `return parentJobID, nil`

**Validation:**
- Verify parent job created with correct type
- Verify N child jobs created (one per operation)
- Verify each child job has single operation in config
- Verify ParentJobOrchestrator monitoring started
- Verify all jobs enqueued to queue

### internal\jobs\worker\database_maintenance_worker.go(NEW)

References: 

- internal\jobs\executor\database_maintenance_executor.go(DELETE)
- internal\jobs\worker\interfaces.go
- internal\models\job_model.go

Create new DatabaseMaintenanceWorker that processes individual database maintenance operations (one operation per job).

**File Structure:**

**Package Declaration:**
- `package worker`

**Imports:**
- Standard library: `context`, `database/sql`, `fmt`
- External: `github.com/ternarybob/arbor`
- Internal: `github.com/ternarybob/quaero/internal/jobs`, `github.com/ternarybob/quaero/internal/models`

**Struct Definition:**
```
type DatabaseMaintenanceWorker struct {
    db         *sql.DB
    jobMgr     *jobs.Manager
    logger     arbor.ILogger
}
```

**Constructor:**
```
func NewDatabaseMaintenanceWorker(
    db *sql.DB,
    jobMgr *jobs.Manager,
    logger arbor.ILogger,
) *DatabaseMaintenanceWorker {
    return &DatabaseMaintenanceWorker{
        db:     db,
        jobMgr: jobMgr,
        logger: logger,
    }
}
```

**Interface Methods:**

1. **GetWorkerType()** - Returns job type:
   ```
   func (w *DatabaseMaintenanceWorker) GetWorkerType() string {
       return "database_maintenance_operation"
   }
   ```

2. **Validate()** - Validates job model:
   ```
   func (w *DatabaseMaintenanceWorker) Validate(job *models.JobModel) error {
       if job.Type != w.GetWorkerType() {
           return fmt.Errorf("invalid job type: expected %s, got %s", w.GetWorkerType(), job.Type)
       }
       
       // Validate operation field exists
       operation, ok := job.GetConfigString("operation")
       if !ok || operation == "" {
           return fmt.Errorf("missing required config field: operation")
       }
       
       // Validate operation is supported
       validOps := map[string]bool{"vacuum": true, "analyze": true, "reindex": true, "optimize": true}
       if !validOps[operation] {
           return fmt.Errorf("unsupported operation: %s", operation)
       }
       
       return nil
   }
   ```

3. **Execute()** - Main execution method:
   ```
   func (w *DatabaseMaintenanceWorker) Execute(ctx context.Context, job *models.JobModel) error {
       // Create job-specific logger with correlation ID
       jobLogger := w.logger.WithCorrelationId(job.ID)
       
       // Log job start
       jobLogger.Info().Msg("Database maintenance operation started")
       
       // Update job status to running
       if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
           jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
       }
       
       // Get operation from config
       operation, _ := job.GetConfigString("operation")
       
       jobLogger.Info().
           Str("operation", operation).
           Msg("Executing database operation")
       
       // Execute operation
       if err := w.executeOperation(ctx, jobLogger, operation); err != nil {
           jobLogger.Error().
               Err(err).
               Str("operation", operation).
               Msg("Database operation failed")
           
           // Set job error
           w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Operation %s failed: %v", operation, err))
           
           // Update status to failed
           w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
           
           return fmt.Errorf("database operation %s failed: %w", operation, err)
       }
       
       // Mark job as completed
       if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
           jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
       }
       
       jobLogger.Info().Msg("Database maintenance operation completed successfully")
       return nil
   }
   ```

**Private Helper Methods:**

1. **executeOperation()** - Routes to specific operation:
   ```
   func (w *DatabaseMaintenanceWorker) executeOperation(ctx context.Context, logger arbor.ILogger, operation string) error {
       switch operation {
       case "vacuum":
           return w.vacuum(ctx, logger)
       case "analyze":
           return w.analyze(ctx, logger)
       case "reindex":
           return w.reindex(ctx, logger)
       case "optimize":
           return w.optimize(ctx, logger)
       default:
           return fmt.Errorf("unknown operation: %s", operation)
       }
   }
   ```

2. **vacuum()** - Copy from old executor (lines 135-145):
   - Execute: `w.db.ExecContext(ctx, "VACUUM")`
   - Log: "Executing VACUUM" (debug), "VACUUM completed successfully" (info)
   - Return error if failed

3. **analyze()** - Copy from old executor (lines 148-158):
   - Execute: `w.db.ExecContext(ctx, "ANALYZE")`
   - Log: "Executing ANALYZE" (debug), "ANALYZE completed successfully" (info)
   - Return error if failed

4. **reindex()** - Copy from old executor (lines 161-206):
   - Query all indexes: `SELECT name FROM sqlite_master WHERE type = 'index' AND name NOT LIKE 'sqlite_%'`
   - Loop through indexes and execute: `REINDEX {indexName}`
   - Log: "Executing REINDEX" (debug), "Reindexing database indexes" (info), "REINDEX completed successfully" (info)
   - Continue on individual index failure (log warning)
   - Return nil (best effort)

5. **optimize()** - Copy from old executor (lines 209-219):
   - Execute: `w.db.ExecContext(ctx, "PRAGMA optimize")`
   - Log: "Executing OPTIMIZE" (debug), "OPTIMIZE completed successfully" (info)
   - Return error if failed

**Key Differences from Old Executor:**

- **No BaseExecutor**: Direct dependencies, simpler structure
- **No Progress Tracking**: Each operation is atomic (no current/total)
- **No Multiple Operations**: Processes single operation per job
- **Simpler Error Handling**: Operation succeeds or fails (no partial completion)
- **Fewer Dependencies**: 3 dependencies (db, jobMgr, logger) vs old 6
- **New Job Type**: `"database_maintenance_operation"` vs old `"database_maintenance"`

**Validation:**
- Verify implements JobWorker interface correctly
- Verify GetWorkerType() returns correct type
- Verify Validate() checks operation field
- Verify Execute() processes single operation
- Verify all operation methods work correctly
- Total lines: ~250 (simpler than old 221-line executor)

### internal\jobs\executor\database_maintenance_executor.go(DELETE)

Delete the old DatabaseMaintenanceExecutor file immediately after creating the new worker.

**Rationale:**
- User explicitly stated: "delete old executor file right after creating the worker. No backward compatibility needed."
- Breaking changes are acceptable for this project
- No deprecation period required
- Clean break ensures developers use new worker
- Prevents confusion about which file to use

**Impact:**
- Old job type `"database_maintenance"` will no longer be processed
- Any in-flight jobs with old type will fail (acceptable for single build)
- App.go registration will be updated to use new worker

**Validation Before Deletion:**
- Verify new worker exists in worker/ directory
- Verify app.go compiles with new worker registration
- Verify application builds successfully

**Note:**
After this deletion, the executor/ directory will contain 5 remaining files (transform, reindex, places_search, job_executor, base_executor). These will be handled in subsequent phases.

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\worker\database_maintenance_worker.go(NEW)
- internal\jobs\manager\database_maintenance_manager.go(MODIFY)

Update app.go to register new DatabaseMaintenanceWorker instead of old DatabaseMaintenanceExecutor.

**Registration Section Updates (lines 334-344):**

**Remove Old Executor Registration:**
Delete lines 334-344:
```
// Register database maintenance executor (new interface)
dbMaintenanceExecutor := executor.NewDatabaseMaintenanceExecutor(
    a.StorageManager.DB().(*sql.DB),
    jobMgr,
    queueMgr,
    a.Logger,
    a.LogService,
    a.WSHandler,
)
jobProcessor.RegisterExecutor(dbMaintenanceExecutor)
a.Logger.Info().Msg("Database maintenance worker registered")
```

**Add New Worker Registration:**
Insert after agent worker registration (after line 332):
```
// Register database maintenance worker (processes individual operations)
dbMaintenanceWorker := worker.NewDatabaseMaintenanceWorker(
    a.StorageManager.DB().(*sql.DB),
    jobMgr,
    a.Logger,
)
jobProcessor.RegisterExecutor(dbMaintenanceWorker)
a.Logger.Info().Msg("Database maintenance worker registered for job type: database_maintenance_operation")
```

**Key Changes:**

1. **Import Change:**
   - Remove: `"github.com/ternarybob/quaero/internal/jobs/executor"` (if no longer used)
   - Keep: `"github.com/ternarybob/quaero/internal/jobs/worker"` (already exists)

2. **Constructor Change:**
   - OLD: `executor.NewDatabaseMaintenanceExecutor(db, jobMgr, queueMgr, logger, logService, wsHandler)` (6 params)
   - NEW: `worker.NewDatabaseMaintenanceWorker(db, jobMgr, logger)` (3 params)

3. **Variable Name:**
   - OLD: `dbMaintenanceExecutor`
   - NEW: `dbMaintenanceWorker`

4. **Dependencies Removed:**
   - `queueMgr` - Not needed (worker doesn't spawn child jobs)
   - `a.LogService` - Not needed (use logger with correlation ID)
   - `a.WSHandler` - Not needed (events published via jobMgr)

5. **Log Message:**
   - Update to include job type: "Database maintenance worker registered for job type: database_maintenance_operation"

**Manager Registration Update (lines 392-394):**

**Update Manager Constructor:**
Replace line 392:
```
dbMaintenanceManager := manager.NewDatabaseMaintenanceManager(a.JobManager, queueMgr, a.Logger)
```

With:
```
dbMaintenanceManager := manager.NewDatabaseMaintenanceManager(a.JobManager, queueMgr, parentJobOrchestrator, a.Logger)
```

**Rationale:**
- Manager needs ParentJobOrchestrator to start monitoring after creating child jobs
- Matches CrawlerManager pattern (uses orchestrator for parent job monitoring)

**Validation:**
- Verify application compiles successfully
- Verify DatabaseMaintenanceWorker is registered correctly
- Verify manager has access to ParentJobOrchestrator
- Run application and check startup logs for:
  - "Database maintenance worker registered for job type: database_maintenance_operation"
  - "Database maintenance manager registered"
- Verify database maintenance job executes correctly via UI or API

### AGENTS.md(MODIFY)

References: 

- docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update AGENTS.md to document the completion of ARCH-008 (database maintenance manager/worker split).

**Section to Update: "Directory Structure (In Transition - ARCH-007)"**

Update the section title and content to reflect ARCH-008 completion:

Change from:
```markdown
### Directory Structure (In Transition - ARCH-007)
```

To:
```markdown
### Directory Structure (In Transition - ARCH-008)
```

Update the worker directory listing:

```markdown
- `internal/jobs/worker/` - Job workers (execution layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `crawler_worker.go` (ARCH-005) - Merged from crawler_executor.go + crawler_executor_auth.go
  - ✅ `agent_worker.go` (ARCH-006)
  - ✅ `job_processor.go` (ARCH-006) - Routes jobs to workers
  - ✅ `database_maintenance_worker.go` (ARCH-008) - Processes individual operations
```

Update the old directories listing:

```markdown
**Old Directories (Still Active - Will be removed in ARCH-009):**
- `internal/jobs/executor/` - Old manager implementations (5 remaining files: transform, reindex, places_search, job_executor, base_executor)
- `internal/jobs/processor/` - EMPTY (all files migrated, directory will be deleted in ARCH-009)
```

Update the migration progress:

```markdown
**Migration Progress:**
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated (merged crawler_executor.go + crawler_executor_auth.go)
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker.go, job_processor.go)
- Phase ARCH-007: ✅ Parent job orchestrator migrated
- Phase ARCH-008: ✅ Database maintenance manager/worker split (YOU ARE HERE)
- Phase ARCH-009: ⏳ Import path updates and cleanup (pending)
```

**Section to Update: "Core Components"**

Update to reflect database maintenance worker:

```markdown
**Core Components:**
- `JobProcessor` - `internal/jobs/worker/job_processor.go` (ARCH-006)
  - Routes jobs from queue to registered workers
  - Manages worker pool lifecycle (Start/Stop)
- `ParentJobOrchestrator` - `internal/jobs/orchestrator/parent_job_orchestrator.go` (ARCH-007)
  - Monitors parent job progress in background goroutines
  - Aggregates child job statistics
  - Publishes real-time progress events
- `DatabaseMaintenanceWorker` - `internal/jobs/worker/database_maintenance_worker.go` (ARCH-008)
  - Processes individual database operations (VACUUM, ANALYZE, REINDEX, OPTIMIZE)
  - Each operation is a separate job for granular tracking
```

**Implementation Notes:**
- Update migration status to show ARCH-008 complete
- Add checkmark (✅) for database_maintenance_worker.go
- Update remaining file count in executor/ directory (5 files remaining)
- Add DatabaseMaintenanceWorker to Core Components section
- Clarify that each operation is a separate job

### docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update MANAGER_WORKER_ARCHITECTURE.md to document the completion of ARCH-008 (database maintenance manager/worker split).

**Section to Update: "Current Status (After ARCH-007)"**

Change the section title and content to reflect ARCH-008 completion:

```markdown
### Current Status (After ARCH-008)

**New Directories Created:**
- ✅ `internal/jobs/manager/` - Created with interfaces.go (ARCH-003)
  - ✅ `crawler_manager.go` - Migrated from executor/ (ARCH-004)
  - ✅ `database_maintenance_manager.go` - Updated for manager/worker split (ARCH-004, ARCH-008)
  - ✅ `agent_manager.go` - Migrated from executor/ (ARCH-004)
- ✅ `internal/jobs/worker/` - Created with interfaces.go (ARCH-003)
  - ✅ `crawler_worker.go` - Merged and migrated from processor/ (ARCH-005)
  - ✅ `agent_worker.go` - Migrated from processor/ (ARCH-006)
  - ✅ `job_processor.go` - Migrated from processor/ (ARCH-006)
  - ✅ `database_maintenance_worker.go` - Created for manager/worker split (ARCH-008)
- ✅ `internal/jobs/orchestrator/` - Created with interfaces.go (ARCH-003)
  - ✅ `parent_job_orchestrator.go` - Migrated from processor/ (ARCH-007)

**Old Directories (Still Active):**
- `internal/jobs/executor/` - Contains 5 remaining implementation files:
  - `transform_step_executor.go` (pending migration)
  - `reindex_step_executor.go` (pending migration)
  - `places_search_step_executor.go` (pending migration)
  - `job_executor.go` (orchestrator - will be refactored separately)
  - `base_executor.go` (shared utilities - will be refactored separately)
- `internal/jobs/processor/` - EMPTY (all files migrated, directory will be deleted in ARCH-009)

**Migration Status:**
- Phase ARCH-001: ✅ Documentation created
- Phase ARCH-002: ✅ Interfaces renamed
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ Manager files migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated and merged
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker, job_processor)
- Phase ARCH-007: ✅ Parent job orchestrator migrated
- Phase ARCH-008: ✅ Database maintenance manager/worker split (YOU ARE HERE)
- Phase ARCH-009: ⏳ Import path updates and cleanup (pending)
- Phase ARCH-010: ⏳ End-to-end validation (pending)
```

**Section to Add: "Database Maintenance Manager/Worker Split (ARCH-008)"**

Add a new subsection after "Parent Job Orchestrator Migrated (ARCH-007)":

```markdown
### Database Maintenance Manager/Worker Split (ARCH-008)

**Architectural Change:**

Completed the Manager/Worker split for database maintenance by updating the manager to create child jobs and creating a new worker for individual operation execution.

**OLD Pattern (Single Job):**
- Manager created 1 job with config: `{"operations": ["vacuum", "analyze", "reindex"]}`
- Worker processed ALL operations in single job execution
- No parallelization, no granular progress tracking

**NEW Pattern (Parent + Child Jobs):**
- Manager creates 1 parent job + N child jobs (one per operation)
- Each child job has config: `{"operation": "vacuum"}` (single operation)
- Worker processes ONE operation per job
- Enables parallelization, granular progress tracking, individual operation retry

**Files Modified/Created:**

1. **DatabaseMaintenanceManager** (UPDATED)
   - File: `internal/jobs/manager/database_maintenance_manager.go`
   - Changes: Updated CreateParentJob() to create parent + child jobs
   - Parent job type: `"database_maintenance_parent"`
   - Child job type: `"database_maintenance_operation"`
   - Starts ParentJobOrchestrator monitoring after enqueueing children

2. **DatabaseMaintenanceWorker** (NEW)
   - File: `internal/jobs/worker/database_maintenance_worker.go`
   - Purpose: Processes individual database operations
   - Job type: `"database_maintenance_operation"`
   - Dependencies: db, jobMgr, logger (3 dependencies vs old 6)
   - Operations: vacuum(), analyze(), reindex(), optimize()

3. **DatabaseMaintenanceExecutor** (DELETED)
   - Old file: `internal/jobs/executor/database_maintenance_executor.go`
   - Deleted immediately (breaking change acceptable)
   - Old job type `"database_maintenance"` no longer supported

**Transformations Applied:**

**Manager Updates:**
- Added ParentJobOrchestrator dependency
- Create parent job record for orchestration tracking
- Loop through operations and create child job for each
- Each child job has single operation in config (not array)
- Start ParentJobOrchestrator monitoring after enqueueing

**Worker Design:**
- Simpler than old executor (no BaseExecutor dependency)
- Direct dependencies: db, jobMgr, logger (3 vs old 6)
- Single Execute() method processes one operation
- Reused operation methods: vacuum(), analyze(), reindex(), optimize()
- No progress tracking within job (each operation is atomic)

**Key Features:**

- **Granular Tracking**: Each operation is a separate job with its own status/logs
- **Parallelization**: Multiple operations can run concurrently (queue concurrency)
- **Retry Logic**: Failed operations can be retried individually
- **Progress Aggregation**: ParentJobOrchestrator aggregates child progress
- **Simpler Worker**: Fewer dependencies, clearer responsibilities

**Job Type Strategy:**

- **Parent Job**: `"database_maintenance_parent"` (orchestration tracking)
- **Child Jobs**: `"database_maintenance_operation"` (individual operations)
- **Old Type**: `"database_maintenance"` (no longer supported)

**Import Path Updates:**

- `internal/app/app.go` (lines 334-344, 392) - Updated to register new worker and pass orchestrator to manager
- Variable renamed: `dbMaintenanceExecutor` → `dbMaintenanceWorker`
- Dependencies reduced: 6 → 3 (removed queueMgr, logService, wsHandler)

**Breaking Changes:**

- Old executor deleted immediately (no backward compatibility)
- Old job type `"database_maintenance"` no longer processed
- Any in-flight jobs with old type will fail (acceptable for single build)

**Architectural Completion:**

This completes the Manager/Worker split for all core job types:
- ✅ **Crawler**: Manager creates parent + URL jobs, Worker processes URLs
- ✅ **Agent**: Manager creates parent + document jobs, Worker processes documents
- ✅ **Database Maintenance**: Manager creates parent + operation jobs, Worker processes operations

All three follow the same pattern: Manager orchestrates, Worker executes, Orchestrator monitors.
```

**Implementation Notes:**
- Update migration status to show ARCH-008 complete
- Add detailed documentation of manager/worker split
- Document architectural change (single job → parent + child jobs)
- Clarify job type strategy and breaking changes
- Emphasize architectural completion for core job types
- Provide context for developers working during transition