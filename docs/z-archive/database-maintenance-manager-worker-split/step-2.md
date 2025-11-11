# Step 2: Create DatabaseMaintenanceWorker for individual operation execution

**Skill:** @go-coder
**Files:** `internal/jobs/worker/database_maintenance_worker.go` (NEW)

---

## Iteration 1

### Agent 2 - Implementation

Created new DatabaseMaintenanceWorker that implements the JobWorker interface for processing individual database maintenance operations.

**Architectural Pattern:**
- NEW Worker pattern (not BaseExecutor)
- Simple struct with 3 dependencies: db, jobMgr, logger
- No progress tracking within job (JobOrchestrator handles that)
- Process single operation per job (not operation array)

**Implementation Details:**

**File:** `internal/jobs/worker/database_maintenance_worker.go` (NEW, 239 lines)

**1. Struct Definition (Lines 17-22):**
```go
type DatabaseMaintenanceWorker struct {
    db     *sql.DB
    jobMgr *jobs.Manager
    logger arbor.ILogger
}
```

**2. Constructor (Lines 24-35):**
- 3 parameters: db, jobMgr, logger
- Simple initialization, no BaseExecutor

**3. Interface Implementation:**

- **GetWorkerType()** (Lines 42-44):
  ```go
  return "database_maintenance_operation"
  ```

- **Validate()** (Lines 47-58):
  - Validates job type matches "database_maintenance_operation"
  - Validates presence of "operation" in config
  - Returns error if invalid

- **Execute()** (Lines 61-120):
  - Creates job-specific logger with correlation ID
  - Extracts operation from config (single operation, not array)
  - Updates job status to "running"
  - Calls executeOperation() dispatcher
  - Handles errors with proper logging and job status updates
  - Updates job status to "completed" on success

**4. Operation Methods (Lines 122-239):**

Copied 4 operation methods from old executor with identical logic:

- **executeOperation()** (Lines 128-141):
  - Switch statement dispatcher for 4 operations
  - Returns error for unknown operations

- **vacuum()** (Lines 144-154):
  - Executes "VACUUM" SQL command
  - Logs debug and info messages

- **analyze()** (Lines 157-167):
  - Executes "ANALYZE" SQL command
  - Logs debug and info messages

- **reindex()** (Lines 170-228):
  - Queries sqlite_master for all indexes (excluding sqlite_% system indexes)
  - Loops through indexes and reindexes each
  - Logs warnings for failed indexes but continues
  - Returns success if any indexes reindexed

- **optimize()** (Lines 231-239):
  - Executes "PRAGMA optimize" command
  - Logs debug and info messages

**Key Differences from Old Executor:**
1. ❌ NO BaseExecutor inheritance
2. ❌ NO progress tracking (UpdateJobProgress not called)
3. ❌ NO WebSocketHandler or LogService dependencies
4. ✅ Simpler constructor (3 params instead of 6)
5. ✅ Job type changed: "database_maintenance" → "database_maintenance_operation"
6. ✅ Config changed: `{"operations": [...]}` → `{"operation": "vacuum"}`
7. ✅ Correlation ID logging for parent job tracking

**Commands run:**
```bash
# Verify syntax
go build -o nul internal/jobs/worker/database_maintenance_worker.go
```

### Agent 3 - Validation

**Skill:** @code-architect

**Compilation:**
✅ Compiles cleanly - no syntax errors

**Code Quality:**
✅ Implements JobWorker interface correctly (3 methods: GetWorkerType, Validate, Execute)
✅ Job type: "database_maintenance_operation" (matches child job type from Step 1)
✅ Simple struct with 3 dependencies (db, jobMgr, logger) as required
✅ No BaseExecutor inheritance (correct for worker pattern)
✅ Config validation checks for single "operation" field (not array)
✅ Proper error handling with context wrapping
✅ Structured logging with correlation IDs (parent job tracking)
✅ Job status updates (pending → running → completed/failed)

**Operation Methods:**
✅ All 4 operation methods copied correctly (vacuum, analyze, reindex, optimize)
✅ VACUUM: Simple SQL execution
✅ ANALYZE: Simple SQL execution
✅ REINDEX: Queries indexes, loops and reindexes each, continues on error
✅ OPTIMIZE: PRAGMA optimize execution
✅ All methods use proper error wrapping and logging

**Architecture:**
✅ Worker processes single operation per job (not array)
✅ Matches established Worker pattern (see CrawlerWorker, AgentWorker)
✅ No progress tracking within worker (JobOrchestrator handles parent progress)
✅ Correlation ID logging for parent job log aggregation
✅ Follows plan requirements exactly

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Worker successfully created following the established Worker pattern. Implements JobWorker interface with 3 methods, processes single operation per job, and copies all 4 operation methods from old executor with proper logging and error handling.

**→ Continuing to Step 3**

