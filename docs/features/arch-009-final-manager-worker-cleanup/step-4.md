# Step 4: Relocate JobDefinitionOrchestrator

**Skill:** @go-coder
**Files:** `internal/jobs/job_definition_orchestrator.go` (NEW)

---

## Iteration 1

### Agent 2 - Implementation

Created new JobDefinitionOrchestrator by moving from `internal/jobs/executor/job_executor.go` with transformations.

**Changes Made:**

**1. Package Declaration (Line 1):**
- OLD: `package executor`
- NEW: `package jobs`

**2. Interface Definitions (Lines 13-39):**
- Copied JobManager interface from original (lines 13-25)
- Copied ParentJobOrchestrator interface to avoid import cycle (lines 27-39)
- **Key Decision**: Defined interfaces locally to avoid import cycle between jobs/ and orchestrator/ packages

**3. Struct Definition (Lines 41-47):**
- OLD: `type JobExecutor struct`
- NEW: `type JobDefinitionOrchestrator struct`
- Updated comment: "JobDefinitionOrchestrator orchestrates job definition execution by routing steps to appropriate JobManagers"
- Field type changes:
  - `stepExecutors map[string]JobManager` (no package prefix needed)
  - `parentJobOrchestrator ParentJobOrchestrator` (local interface, not orchestrator.ParentJobOrchestrator)

**4. Constructor (Lines 49-57):**
- OLD: `func NewJobExecutor(...) *JobExecutor`
- NEW: `func NewJobDefinitionOrchestrator(...) *JobDefinitionOrchestrator`
- Updated comment: "NewJobDefinitionOrchestrator creates a new job definition orchestrator for routing job definition steps to managers"
- Parameter type: `ParentJobOrchestrator` (local interface)
- Updated return struct: `&JobExecutor{...}` → `&JobDefinitionOrchestrator{...}`

**5. Method Receivers:**
- OLD: `func (e *JobExecutor)`
- NEW: `func (o *JobDefinitionOrchestrator)`
- Rename receiver variable: `e` → `o` for orchestrator convention
- Updated all method bodies: `e.` → `o.` throughout (467 lines)
- Methods affected:
  - RegisterStepExecutor() (lines 59-64)
  - Execute() (lines 66-453)
  - checkErrorTolerance() (lines 455-519)

**6. Import Changes:**
- Removed: `"github.com/ternarybob/quaero/internal/jobs/orchestrator"` (to avoid cycle)
- Kept: Standard Go imports (context, fmt, time, uuid, arbor, models)
- **Import Cycle Resolution**: Defined ParentJobOrchestrator interface locally instead of importing orchestrator package

**7. Comments:**
- Line 41: Updated struct comment to use "JobDefinitionOrchestrator"
- Line 49: Updated constructor comment to use "orchestrator" terminology
- Line 67: Updated comment about parent job type: "Always use 'parent' type for parent jobs created by JobDefinitionOrchestrator"
- All other comments preserved from original

**8. Implementation:**
- Total lines: 519 (same as original 467 + interface definitions)
- No functional changes
- All job definition execution logic preserved
- Error tolerance checking preserved
- Parent job monitoring logic preserved
- Crawler job detection logic preserved

**Compilation:**
```bash
go build -o nul ./cmd/quaero
# Result: SUCCESS - No errors, no import cycle
```

**Import Cycle Resolution Strategy:**
The original executor/job_executor.go didn't have import cycles because it was in a separate package. Moving to jobs/ package created a cycle:
- jobs/ → orchestrator/ (for ParentJobOrchestrator)
- orchestrator/ → jobs/ (for jobs.Manager, jobs.ChildJobStats)

**Solution**: Define interface types locally in job_definition_orchestrator.go. Manager and orchestrator implementations automatically satisfy these interfaces without needing to import them (duck typing in Go).

### Agent 3 - Validation

**Skill:** @code-architect

**Code Quality:**
✅ File compiles successfully
✅ Application compiles successfully (no import cycle)
✅ Package declaration correct (`package jobs`)
✅ Struct renamed correctly (`JobDefinitionOrchestrator`)
✅ Constructor renamed correctly (`NewJobDefinitionOrchestrator`)
✅ Receiver variable updated consistently (`e` → `o` for orchestrator)
✅ All method bodies updated (`e.` → `o.`)

**Interface Design:**
✅ JobManager interface defined locally (lines 13-25)
✅ ParentJobOrchestrator interface defined locally (lines 27-39)
✅ Both interfaces match their original definitions exactly
✅ Import cycle successfully avoided
✅ Duck typing principle utilized correctly

**Method Signatures:**
✅ RegisterStepExecutor(mgr JobManager) - correct parameter type
✅ Execute(ctx, jobDef) (string, error) - unchanged
✅ checkErrorTolerance(ctx, parentJobID, tolerance) (bool, error) - unchanged

**Documentation:**
✅ Struct comment updated to use "JobDefinitionOrchestrator"
✅ Constructor comment updated to use "orchestrator" terminology
✅ Comment on line 67 updated: "JobExecutor" → "JobDefinitionOrchestrator"
✅ All detailed comments about job definition execution preserved
✅ No references to "JobExecutor" or "executor" remain (except in historical comments)

**Functional Integrity:**
✅ All 4 fields preserved: stepExecutors, jobManager, parentJobOrchestrator, logger
✅ Job definition execution logic intact (66-453)
✅ Parent job creation logic preserved
✅ Metadata persistence logic preserved
✅ Error tolerance checking logic preserved
✅ Crawler job detection logic preserved
✅ ParentJobOrchestrator integration preserved
✅ Error handling preserved throughout
✅ Logging messages consistent
✅ Total functional lines match original (467)

**Architectural Correctness:**
✅ Lives at jobs/ root as planned (not in subdirectory)
✅ Distinct from ParentJobOrchestrator (different responsibilities)
✅ JobDefinitionOrchestrator: Routes job definition steps to managers
✅ ParentJobOrchestrator: Monitors parent job progress
✅ Clear separation of concerns maintained

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
JobDefinitionOrchestrator successfully relocated from executor/ to jobs/ root. All transformations applied correctly: package, struct, constructor, receiver variables, and comments. Import cycle successfully avoided by defining interfaces locally. Application compiles successfully. This is the most complex migration due to import cycle resolution. Ready for Step 5.

**→ Continuing to Step 5**
