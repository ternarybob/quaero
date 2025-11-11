# Step 5: Update Import Paths in app.go

**Skill:** @go-coder
**Files:** `internal/app/app.go` (MODIFIED)

---

## Iteration 1

### Agent 2 - Implementation

Updated `internal/app/app.go` to use new manager and orchestrator imports.

**Changes Made:**

**1. Import Changes:**
- Removed: `"github.com/ternarybob/quaero/internal/jobs/executor"`
- Already has: `"github.com/ternarybob/quaero/internal/jobs"` (for JobDefinitionOrchestrator)
- Already has: `"github.com/ternarybob/quaero/internal/jobs/manager"` (for managers)

**2. Struct Field (Line 67):**
- OLD: `JobExecutor *executor.JobExecutor`
- NEW: `JobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator`

**3. Initialization (Line 373):**
- OLD: `a.JobExecutor = executor.NewJobExecutor(jobMgr, jobOrchestrator, a.Logger)`
- NEW: `a.JobDefinitionOrchestrator = jobs.NewJobDefinitionOrchestrator(jobMgr, jobOrchestrator, a.Logger)`

**4. Manager Registrations (Lines 380-403):**

**TransformManager:**
- OLD: `transformStepExecutor := executor.NewTransformStepExecutor(...)`
- OLD: `a.JobExecutor.RegisterStepExecutor(transformStepExecutor)`
- NEW: `transformManager := manager.NewTransformManager(...)`
- NEW: `a.JobDefinitionOrchestrator.RegisterStepExecutor(transformManager)`

**ReindexManager:**
- OLD: `reindexStepExecutor := executor.NewReindexStepExecutor(...)`
- OLD: `a.JobExecutor.RegisterStepExecutor(reindexStepExecutor)`
- NEW: `reindexManager := manager.NewReindexManager(...)`
- NEW: `a.JobDefinitionOrchestrator.RegisterStepExecutor(reindexManager)`

**PlacesSearchManager:**
- OLD: `placesSearchStepExecutor := executor.NewPlacesSearchStepExecutor(...)`
- OLD: `a.JobExecutor.RegisterStepExecutor(placesSearchStepExecutor)`
- NEW: `placesSearchManager := manager.NewPlacesSearchManager(...)`
- NEW: `a.JobDefinitionOrchestrator.RegisterStepExecutor(placesSearchManager)`

**5. Handler Initialization (Line 529):**
- OLD: `a.JobExecutor,`
- NEW: `a.JobDefinitionOrchestrator,`

**6. Log Message (Line 403):**
- OLD: `"JobExecutor initialized with all managers"`
- NEW: `"JobDefinitionOrchestrator initialized with all managers (ARCH-009)"`

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
✅ Constructor call updated
✅ All 3 manager registrations updated:
  - TransformManager
  - ReindexManager
  - PlacesSearchManager
✅ Variable names updated: *StepExecutor → *Manager
✅ RegisterStepExecutor calls updated to use JobDefinitionOrchestrator
✅ Handler initialization updated
✅ Log message updated with ARCH-009 marker

**Manager Registration Pattern:**
✅ All 6 managers now registered:
  1. CrawlerManager (internal/jobs/manager/crawler_manager.go)
  2. AgentManager (internal/jobs/manager/agent_manager.go)
  3. DatabaseMaintenanceManager (internal/jobs/manager/database_maintenance_manager.go)
  4. TransformManager (internal/jobs/manager/transform_manager.go) - NEW
  5. ReindexManager (internal/jobs/manager/reindex_manager.go) - NEW
  6. PlacesSearchManager (internal/jobs/manager/places_search_manager.go) - NEW

**Functional Integrity:**
✅ Dependency injection flow preserved
✅ Service initialization order unchanged
✅ All managers registered with orchestrator
✅ Handler receives orchestrator correctly
✅ Logging consistent

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
app.go successfully updated to use new manager and orchestrator imports. All 3 new managers (Transform, Reindex, PlacesSearch) registered correctly alongside existing managers (Crawler, Agent, DatabaseMaintenance). Total of 6 managers now orchestrated by JobDefinitionOrchestrator. Application compiles successfully. Ready for Step 6.

**→ Continuing to Step 6**
