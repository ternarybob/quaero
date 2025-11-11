# Step 4: Update app.go registration

**Skill:** @go-coder
**Files:** `internal/app/app.go`

---

## Iteration 1

### Agent 2 - Implementation

Updated app.go to remove old executor registration and add new worker registration with correct dependencies.

**Changes Made:**

**1. Remove old executor registration (Lines 334-344 → 334-341):**

OLD CODE (REMOVED):
```go
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

NEW CODE (ADDED):
```go
// Register database maintenance worker (ARCH-008)
dbMaintenanceWorker := worker.NewDatabaseMaintenanceWorker(
    a.StorageManager.DB().(*sql.DB),
    jobMgr,
    a.Logger,
)
jobProcessor.RegisterExecutor(dbMaintenanceWorker)
a.Logger.Info().Msg("Database maintenance worker registered for job type: database_maintenance_operation")
```

**Key Changes:**
- ❌ Removed 6-parameter constructor (old executor)
- ✅ Added 3-parameter constructor (new worker)
- ❌ Removed dependencies: queueMgr, logService, wsHandler
- ✅ Kept dependencies: db, jobMgr, logger
- ✅ Updated log message with job type clarification

**2. Update manager constructor call (Line 389):**

OLD CODE:
```go
dbMaintenanceManager := manager.NewDatabaseMaintenanceManager(a.JobManager, queueMgr, a.Logger)
```

NEW CODE:
```go
dbMaintenanceManager := manager.NewDatabaseMaintenanceManager(a.JobManager, queueMgr, parentJobOrchestrator, a.Logger)
```

**Key Change:**
- ✅ Added parentJobOrchestrator parameter (4th parameter)
- ✅ Updated log message: "Database maintenance manager registered (ARCH-008)"

**Verification:**
- ✅ parentJobOrchestrator variable already exists (created on line 314)
- ✅ Worker registration uses jobProcessor.RegisterExecutor()
- ✅ Manager registration uses a.JobExecutor.RegisterStepExecutor()
- ✅ Import already includes worker package (line 23)

**Commands run:**
```bash
# Verify compilation
cd internal/app && go build
```

### Agent 3 - Validation

**Skill:** @code-architect

**Compilation:**
✅ App compiles cleanly - no import errors or type mismatches

**Code Quality:**
✅ Old executor registration removed completely
✅ New worker registration with 3 dependencies (db, jobMgr, logger)
✅ Manager constructor updated with parentJobOrchestrator parameter
✅ Correct registration methods used (RegisterExecutor vs RegisterStepExecutor)
✅ Log messages updated with ARCH-008 marker
✅ Worker package already imported (no new imports needed)

**Architecture:**
✅ Worker registered with JobProcessor (for queue processing)
✅ Manager registered with JobExecutor (for job definition steps)
✅ parentJobOrchestrator exists and properly initialized (line 314)
✅ No references to old executor remain in app.go
✅ Follows established pattern from other workers (CrawlerWorker, AgentWorker)

**Startup Sequence:**
✅ Worker registration after parentJobOrchestrator creation
✅ Manager registration in step executors section
✅ Proper dependency injection order maintained

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
App.go successfully updated with new worker and manager registrations. Old executor removed, dependencies simplified, compilation verified. Ready for Step 5 (compile and validate).

**→ Continuing to Step 5**

