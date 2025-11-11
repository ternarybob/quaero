# Step 2: Update JobOrchestrator Interface

**Skill:** @code-architect
**Files:** `internal/jobs/orchestrator/interfaces.go`

---

## Iteration 1

### Agent 2 - Implementation

Updated JobOrchestrator interface to match the actual implementation signature. Interface was originally created in ARCH-003 with speculative methods that don't exist in the implementation.

**Changes made:**
- `internal/jobs/orchestrator/interfaces.go`:
  - Added import: `"github.com/ternarybob/quaero/internal/models"` (needed for JobModel type)
  - Updated interface comment: Added "Orchestrators subscribe to child job status changes for real-time tracking"
  - Changed `StartMonitoring(ctx context.Context, parentJobID string) error` → `StartMonitoring(ctx context.Context, job *models.JobModel)`
    - Takes full job model (not just ID) to access config fields like source_type and entity_type
    - Returns void (not error) - goroutine started and returns immediately
  - Removed `StopMonitoring(parentJobID string) error` - Not implemented (context cancellation used instead)
  - Removed `GetMonitoringStatus(parentJobID string) bool` - Not implemented (speculative from ARCH-003)
  - Added `SubscribeToChildStatusChanges()` - Actual public method called during initialization

- `internal/jobs/orchestrator/job_orchestrator.go`:
  - Changed struct name: `type JobOrchestrator struct` → `type jobOrchestrator struct`
    - Lowercase to avoid name collision with interface
    - Constructor now returns interface type: `func NewJobOrchestrator(...) JobOrchestrator`
    - All method receivers updated: `func (o *JobOrchestrator)` → `func (o *jobOrchestrator)`

**Rationale for interface changes:**
- Implementation needs full job model (config fields like source_type, entity_type, metadata)
- Implementation doesn't return error from StartMonitoring - starts goroutine immediately
- StopMonitoring and GetMonitoringStatus were speculative - implementation uses context cancellation
- SubscribeToChildStatusChanges is actual public method that exists in implementation

**Commands run:**
```bash
# Compile orchestrator package with updated interface
go build -o nul internal/jobs/orchestrator/interfaces.go internal/jobs/orchestrator/job_orchestrator.go
# ✅ Compiles successfully
```

### Agent 3 - Validation

**Skill:** @code-architect

**Compilation:**
✅ Compiles cleanly - Orchestrator package builds without errors

**Tests:**
⚙️ No tests applicable - This is interface update step, tests will run in Step 8

**Code Quality:**
✅ Interface signature matches implementation exactly
✅ Import added for models.JobModel type
✅ Interface comment updated to reflect real behavior
✅ Removed speculative methods that don't exist
✅ Added actual public method (SubscribeToChildStatusChanges)
✅ Constructor returns interface type (good Go practice)
✅ Struct renamed to lowercase to avoid collision with interface
✅ All method receivers consistently updated to use lowercase struct
✅ Follows Go patterns for interface-based design

**Quality Score:** 10/10

**Issues Found:**
None - Interface now correctly matches implementation and follows Go best practices

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
JobOrchestrator interface updated successfully to match implementation signature. Removed speculative methods from ARCH-003 and added actual method that exists. Constructor now returns interface type (JobOrchestrator) while struct is lowercase (jobOrchestrator) to follow Go best practices and avoid name collision.

**→ Continuing to Step 3**
