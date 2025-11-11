# Step 2: Create ReindexManager

**Skill:** @go-coder
**Files:** `internal/jobs/manager/reindex_manager.go` (NEW)

---

## Iteration 1

### Agent 2 - Implementation

Created new ReindexManager by copying from `internal/jobs/executor/reindex_step_executor.go` with transformations.

**Changes Made:**

**1. Package Declaration (Line 1):**
- OLD: `package executor`
- NEW: `package manager`

**2. Struct Definition (Lines 14-18):**
- OLD: `type ReindexStepExecutor struct`
- NEW: `type ReindexManager struct`
- Fields unchanged: documentStorage, jobManager, logger
- Updated comment: "ReindexManager orchestrates FTS5 full-text search index rebuilding workflows"

**3. Constructor (Lines 21-27):**
- OLD: `func NewReindexStepExecutor(...) *ReindexStepExecutor`
- NEW: `func NewReindexManager(...) *ReindexManager`
- Updated comment: "NewReindexManager creates a new reindex manager for orchestrating FTS5 index rebuilding"
- Updated return struct: `&ReindexStepExecutor{...}` → `&ReindexManager{...}`

**4. Method Receivers:**
- OLD: `func (e *ReindexStepExecutor)`
- NEW: `func (m *ReindexManager)`
- Updated all method bodies: `e.` → `m.` throughout (lines 32-114)
- Methods affected: CreateParentJob() (lines 30-114), GetManagerType() (lines 117-120)

**5. Implementation:**
- Total lines: 121 (same as original)
- No functional changes
- Dry run logic preserved
- Job tracking logic preserved
- FTS5 index rebuild logic preserved
- Error handling intact

**Compilation:**
```bash
go build -o nul ./internal/jobs/manager/reindex_manager.go
# Result: SUCCESS - No errors
```

### Agent 3 - Validation

**Skill:** @code-architect

**Code Quality:**
✅ File compiles successfully
✅ Package declaration correct (`package manager`)
✅ Struct renamed correctly (`ReindexManager`)
✅ Constructor renamed correctly (`NewReindexManager`)
✅ Receiver variable updated consistently (`e` → `m`)
✅ All method bodies updated (`e.` → `m.`)

**Interface Compliance:**
✅ Implements JobManager interface methods:
  - `CreateParentJob(ctx, step, jobDef, parentJobID) (string, error)`
  - `GetManagerType() string`
✅ Method signatures match interface requirements
✅ Return types correct

**Documentation:**
✅ Struct comment updated to use "ReindexManager"
✅ Constructor comment updated to use "manager" terminology
✅ All detailed comments about FTS5 rebuild operation preserved
✅ No references to "executor" or "ReindexStepExecutor" remain

**Functional Integrity:**
✅ All 3 fields preserved: documentStorage, jobManager, logger
✅ Dry run configuration logic intact
✅ Job record creation preserved
✅ FTS5 index rebuild logic preserved
✅ Error handling preserved (job status updates on success/failure)
✅ Logging messages consistent
✅ Total lines match original (121)

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
ReindexManager successfully created following established migration pattern. All transformations applied correctly: package, struct, constructor, receiver variables, and comments. File compiles successfully and implements JobManager interface. Ready for Step 3.

**→ Continuing to Step 3**
