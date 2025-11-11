# Step 6: Delete old interface files

**Skill:** @go-coder
**Files:** `internal/jobs/manager/interfaces.go`, `internal/jobs/orchestrator/interfaces.go`, `internal/jobs/worker/interfaces.go` (DELETE)

---

## Iteration 1

### Agent 2 - Implementation

Deleting obsolete interface files after all references have been updated to use centralized interfaces.

**Changes made:**
- `internal/jobs/manager/interfaces.go`: DELETED
- `internal/jobs/orchestrator/interfaces.go`: DELETED
- `internal/jobs/worker/interfaces.go`: DELETED
- `internal/jobs/orchestrator/parent_job_orchestrator.go`: Fixed return type from `ParentJobOrchestrator` to `interfaces.ParentJobOrchestrator` in NewParentJobOrchestrator function

**Commands run:**
```bash
rm internal/jobs/manager/interfaces.go internal/jobs/orchestrator/interfaces.go internal/jobs/worker/interfaces.go
go build ./internal/...   # Compiled successfully after fixing orchestrator return type
```

### Agent 3 - Validation
**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly (after fixing orchestrator return type)

**Tests:**
⚙️ No tests applicable (deletion + fix)

**Code Quality:**
✅ Successfully deleted all obsolete interface files
✅ Fixed missing type reference in orchestrator
✅ All packages compile after cleanup
✅ No remaining references to deleted files

**Quality Score:** 9/10

**Issues Found:**
1. orchestrator return type needed updating (fixed in this step)

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 9/10

**Notes:**
Successfully deleted all three obsolete interface files. During compilation, discovered that parent_job_orchestrator.go had a bare `ParentJobOrchestrator` return type that needed to be updated to `interfaces.ParentJobOrchestrator`. Fixed and all packages now compile successfully.

**→ Continuing to Step 7**
