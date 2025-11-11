# Step 6: Update Import Paths in job_definition_handler.go

**Skill:** @go-coder
**Files:** `internal/handlers/job_definition_handler.go` (MODIFIED)

---

## Iteration 1

### Agent 2 - Implementation

Updated `internal/handlers/job_definition_handler.go` to use new orchestrator imports.

**Changes Made:**

**1. Import Changes:**
- Removed: `"github.com/ternarybob/quaero/internal/jobs/executor"`
- Already has: `"github.com/ternarybob/quaero/internal/jobs"` (for JobDefinitionOrchestrator)

**2. Struct Field (Line 31):**
- OLD: `jobExecutor *executor.JobExecutor`
- NEW: `jobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator`

**3. Constructor (Lines 39-76):**
- OLD: `func NewJobDefinitionHandler(..., jobExecutor *executor.JobExecutor, ...) *JobDefinitionHandler`
- NEW: `func NewJobDefinitionHandler(..., jobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator, ...) *JobDefinitionHandler`

**Validation:**
- OLD: `if jobExecutor == nil { panic("jobExecutor cannot be nil") }`
- NEW: `if jobDefinitionOrchestrator == nil { panic("jobDefinitionOrchestrator cannot be nil") }`

**Field Assignment:**
- OLD: `jobExecutor: jobExecutor,`
- NEW: `jobDefinitionOrchestrator: jobDefinitionOrchestrator,`

**Log Message (Line 66):**
- OLD: "Job definition handler initialized"
- NEW: "Job definition handler initialized with job definition orchestrator and auth storage (ARCH-009)"

**4. Execute() Call #1 in ExecuteJobDefinitionHandler (Line 485):**
- OLD: `parentJobID, err := h.jobExecutor.Execute(bgCtx, jobDef)`
- NEW: `parentJobID, err := h.jobDefinitionOrchestrator.Execute(bgCtx, jobDef)`

**5. Execute() Call #2 in CreateAndExecuteQuickCrawlHandler (Line 1035):**
- OLD: `parentJobID, err := h.jobExecutor.Execute(bgCtx, jobDef)`
- NEW: `parentJobID, err := h.jobDefinitionOrchestrator.Execute(bgCtx, jobDef)`

**Compilation:**
```bash
go build -o nul ./cmd/quaero
# Result: SUCCESS - No errors
```

### Agent 3 - Validation

**Skill:** @code-architect

**Code Quality:**
✅ File compiles successfully
✅ Application compiles successfully
✅ Import removed: executor package
✅ Struct field updated correctly
✅ Constructor parameter updated
✅ Constructor validation updated
✅ Field assignment updated
✅ All 2 Execute() calls updated:
  - Line 485: ExecuteJobDefinitionHandler
  - Line 1035: CreateAndExecuteQuickCrawlHandler
✅ Log message updated with ARCH-009 marker

**Handler Integration:**
✅ Receives JobDefinitionOrchestrator from app.go
✅ Two execution paths both use orchestrator:
  1. Manual execution via /api/job-definitions/{id}/execute
  2. Quick crawl via /api/job-definitions/quick-crawl
✅ Both paths launch async goroutines for background execution
✅ Both paths return HTTP 202 Accepted immediately

**Functional Integrity:**
✅ HTTP handler methods unchanged
✅ Request validation unchanged
✅ Response format unchanged
✅ Error handling unchanged
✅ Async execution pattern preserved

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
job_definition_handler.go successfully updated to use JobDefinitionOrchestrator. Both execution paths (manual and quick crawl) updated to use orchestrator. Application compiles successfully. All import path updates complete. Ready for Step 7.

**→ Continuing to Step 7**
