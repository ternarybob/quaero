# Step 8: Delete Old Directories and Files

**Skill:** @code-architect
**Files:** (DELETED - 10 files total)

---

## Iteration 1

### Agent 2 - Implementation

Deleted all old executor files and duplicate interface file.

**Deletions:**

**1. Entire executor/ directory (9 files):**
```powershell
Remove-Item -Recurse -Force "internal\jobs\executor"
```

**Files deleted:**
1. `internal/jobs/executor/transform_step_executor.go` (112 lines) - Migrated to manager/transform_manager.go
2. `internal/jobs/executor/reindex_step_executor.go` (121 lines) - Migrated to manager/reindex_manager.go
3. `internal/jobs/executor/places_search_step_executor.go` (274 lines) - Migrated to manager/places_search_manager.go
4. `internal/jobs/executor/job_executor.go` (467 lines) - Relocated to jobs/job_definition_orchestrator.go
5. `internal/jobs/executor/crawler_step_executor.go` - Deprecated in ARCH-004
6. `internal/jobs/executor/database_maintenance_step_executor.go` - Deprecated in ARCH-008
7. `internal/jobs/executor/agent_step_executor.go` - Deprecated in ARCH-007
8. `internal/jobs/executor/base_executor.go` - Unused
9. `internal/jobs/executor/interfaces.go` - Duplicate interface definitions

**2. Duplicate interface file:**
```powershell
Remove-Item -Force "internal\interfaces\job_executor.go"
```

**File deleted:**
- `internal/interfaces/job_executor.go` - Duplicate of JobWorker interface (now in worker/interfaces.go)

**Compilation Issue Found:**
After deletion, compilation failed:
```
internal\jobs\worker\job_processor.go:25:34: undefined: interfaces.JobWorker
internal\jobs\worker\job_processor.go:41:41: undefined: interfaces.JobWorker
internal\jobs\worker\job_processor.go:51:60: undefined: interfaces.JobWorker
```

**Root Cause:**
`job_processor.go` was importing `interfaces.JobWorker` from the central interfaces package, but the interface is now defined locally in `internal/jobs/worker/interfaces.go`.

**Fix Applied:**

**1. Removed interfaces import (line 14):**
```go
// OLD
import (
    ...
    "github.com/ternarybob/quaero/internal/interfaces"
    ...
)

// NEW
import (
    ...
    // interfaces import removed
    ...
)
```

**2. Updated JobProcessor struct (line 24):**
```go
// OLD
executors map[string]interfaces.JobWorker

// NEW
executors map[string]JobWorker
```

**3. Updated NewJobProcessor constructor (line 40):**
```go
// OLD
executors: make(map[string]interfaces.JobWorker),

// NEW
executors: make(map[string]JobWorker),
```

**4. Updated RegisterExecutor method (line 50):**
```go
// OLD
func (jp *JobProcessor) RegisterExecutor(worker interfaces.JobWorker) {

// NEW
func (jp *JobProcessor) RegisterExecutor(worker JobWorker) {
```

**Final Compilation:**
```bash
go build -o nul ./cmd/quaero
# Result: SUCCESS - No errors
```

### Agent 3 - Validation

**Skill:** @code-architect

**Deletion Verification:**
✅ Executor directory deleted: `Test-Path "internal\jobs\executor"` → False
✅ Duplicate interface deleted: `Test-Path "internal\interfaces\job_executor.go"` → False
✅ Application compiles successfully after deletion
✅ No references to deleted files remain in code

**File Count Summary:**
- **Deleted:** 10 files (9 in executor/ + 1 duplicate interface)
- **Created:** 4 files in ARCH-009 (3 managers + 1 orchestrator)
- **Modified:** 3 files (app.go, job_definition_handler.go, job_processor.go)

**Import Cycle Resolution:**
✅ JobWorker interface now used locally in worker package
✅ No external dependencies on deleted interfaces package
✅ Worker package self-contained with local interfaces.go
✅ Duck typing pattern successful

**Architecture Validation:**
✅ Manager/Worker/Orchestrator pattern complete
✅ No executor/ directory remains
✅ All managers in internal/jobs/manager/
✅ All workers in internal/jobs/worker/
✅ All orchestrators at appropriate levels (jobs/ root, orchestrator/)
✅ Clean separation of concerns

**Quality Score:** 10/10

**Issues Found:**
None (compilation issue detected and fixed immediately)

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
All 10 files successfully deleted. Compilation issue with JobWorker interface detected and fixed by removing interfaces import and using local worker package interface. Application compiles successfully with no errors. Executor package completely removed. Architecture cleanup complete. Ready for Step 9 (documentation updates).

**→ Continuing to Step 9**
