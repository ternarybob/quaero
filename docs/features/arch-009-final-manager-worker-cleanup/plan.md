# Plan: ARCH-009 Final Manager/Worker/Orchestrator Architecture Cleanup

**Agent:** @task-architect (Agent 1 - Planner)

**Created:** 2025-11-11

---

## Overview

This final phase (ARCH-009) completes the Manager/Worker/Orchestrator architecture migration by:
1. Migrating the 3 remaining managers (transform, reindex, places_search)
2. Relocating the JobExecutor orchestrator to jobs/ root as JobDefinitionOrchestrator
3. Updating all import paths throughout the codebase
4. Deleting old directories and duplicate interface files
5. Updating documentation to reflect completed architecture

**Migration Pattern Established (ARCH-004 through ARCH-008):**
- Copy file to new location with new name
- Update package declaration
- Rename struct: `*StepExecutor` → `*Manager` (or `JobExecutor` → `JobDefinitionOrchestrator`)
- Rename constructor: `New*StepExecutor` → `New*Manager` (or `NewJobExecutor` → `NewJobDefinitionOrchestrator`)
- Update receiver variable: `e` → `m` (managers) or `e` → `o` (orchestrators)
- Update all comments to use new terminology
- Update app.go registration
- Delete old file immediately (no backward compatibility)

---

## Steps

### Step 1: Create TransformManager

**Skill:** @go-coder

**Files:**
- `internal/jobs/manager/transform_manager.go` (NEW)

**User Decision Required:** No

**Description:**
Copy `internal/jobs/executor/transform_step_executor.go` to `internal/jobs/manager/transform_manager.go` and apply transformations:
- Package: `executor` → `manager`
- Struct: `TransformStepExecutor` → `TransformManager`
- Constructor: `NewTransformStepExecutor()` → `NewTransformManager()`
- Receiver: `e *TransformStepExecutor` → `m *TransformManager`
- Update all comments and log messages referencing "executor" → "manager"
- Keep all 3 fields unchanged: transformService, jobManager, logger
- Total lines: ~112 (same as original)

**Validation Criteria:**
- File compiles successfully
- Constructor returns correct type
- Implements JobManager interface
- Comments updated consistently

---

### Step 2: Create ReindexManager

**Skill:** @go-coder

**Files:**
- `internal/jobs/manager/reindex_manager.go` (NEW)

**User Decision Required:** No

**Description:**
Copy `internal/jobs/executor/reindex_step_executor.go` to `internal/jobs/manager/reindex_manager.go` and apply transformations:
- Package: `executor` → `manager`
- Struct: `ReindexStepExecutor` → `ReindexManager`
- Constructor: `NewReindexStepExecutor()` → `NewReindexManager()`
- Receiver: `e *ReindexStepExecutor` → `m *ReindexManager`
- Update all comments and log messages referencing "executor" → "manager"
- Keep all 3 fields unchanged: documentStorage, jobManager, logger
- Total lines: ~121 (same as original)

**Validation Criteria:**
- File compiles successfully
- Constructor returns correct type
- Implements JobManager interface
- Comments updated consistently

---

### Step 3: Create PlacesSearchManager

**Skill:** @go-coder

**Files:**
- `internal/jobs/manager/places_search_manager.go` (NEW)

**User Decision Required:** No

**Description:**
Copy `internal/jobs/executor/places_search_step_executor.go` to `internal/jobs/manager/places_search_manager.go` and apply transformations:
- Package: `executor` → `manager`
- Struct: `PlacesSearchStepExecutor` → `PlacesSearchManager`
- Constructor: `NewPlacesSearchStepExecutor()` → `NewPlacesSearchManager()`
- Receiver: `e *PlacesSearchStepExecutor` → `m *PlacesSearchManager`
- Update all comments and log messages referencing "executor" → "manager"
- Keep all 4 fields unchanged: placesService, documentService, eventService, logger
- Total lines: ~274 (same as original)

**Validation Criteria:**
- File compiles successfully
- Constructor returns correct type
- Implements JobManager interface
- Comments updated consistently

---

### Step 4: Relocate JobDefinitionOrchestrator

**Skill:** @go-coder

**Files:**
- `internal/jobs/job_definition_orchestrator.go` (NEW)

**User Decision Required:** No

**Description:**
Move `internal/jobs/executor/job_executor.go` to `internal/jobs/job_definition_orchestrator.go` and apply transformations:
- Package: `executor` → `jobs`
- Struct: `JobExecutor` → `JobDefinitionOrchestrator`
- Constructor: `NewJobExecutor()` → `NewJobDefinitionOrchestrator()`
- Receiver: `e *JobExecutor` → `o *JobDefinitionOrchestrator`
- Update all comments and log messages referencing "JobExecutor" → "JobDefinitionOrchestrator"
- Keep all 4 fields unchanged: stepExecutors, jobManager, parentJobOrchestrator, logger
- Total lines: ~468 (same as original)

**Key Distinction:**
- This orchestrator routes job definition steps to managers (different from ParentJobOrchestrator)
- Lives at jobs/ root since it's the only job definition orchestrator

**Validation Criteria:**
- File compiles successfully
- Constructor returns correct type
- All methods work correctly (RegisterStepExecutor, Execute, checkErrorTolerance)
- Comments updated consistently

---

### Step 5: Update Import Paths in app.go

**Skill:** @go-coder

**Files:**
- `internal/app/app.go` (MODIFY)

**User Decision Required:** No

**Description:**
Update app.go to remove executor import and use new manager/orchestrator locations:

**Import Section (line 20):**
- Remove: `"github.com/ternarybob/quaero/internal/jobs/executor"`
- Keep existing: manager, orchestrator, worker imports

**Field Declaration (line 68):**
- Change: `JobExecutor *executor.JobExecutor` → `JobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator`

**Orchestrator Initialization (line 374):**
- Change: `a.JobExecutor = executor.NewJobExecutor(...)` → `a.JobDefinitionOrchestrator = jobs.NewJobDefinitionOrchestrator(...)`

**Manager Registrations (lines 377-401):**
- Transform (line 381): `executor.NewTransformStepExecutor()` → `manager.NewTransformManager()`
- Reindex (line 385): `executor.NewReindexStepExecutor()` → `manager.NewReindexManager()`
- Places Search (line 393): `executor.NewPlacesSearchStepExecutor()` → `manager.NewPlacesSearchManager()`
- Update variable names: `*StepExecutor` → `*Manager`
- Update all `a.JobExecutor.RegisterStepExecutor()` → `a.JobDefinitionOrchestrator.RegisterStepExecutor()`

**Log Messages:**
- Update: "JobExecutor initialized" → "JobDefinitionOrchestrator initialized"

**Validation Criteria:**
- File compiles successfully
- All managers registered correctly
- Orchestrator initialized correctly
- No references to executor package remain

---

### Step 6: Update Import Paths in job_definition_handler.go

**Skill:** @go-coder

**Files:**
- `internal/handlers/job_definition_handler.go` (MODIFY)

**User Decision Required:** No

**Description:**
Update job_definition_handler.go to use new JobDefinitionOrchestrator location:

**Import Section (line 20):**
- Remove: `"github.com/ternarybob/quaero/internal/jobs/executor"`
- No new import needed (JobDefinitionOrchestrator is in jobs package, already imported at line 19)

**Field Declaration:**
- Change: `jobExecutor *executor.JobExecutor` → `jobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator`

**Constructor:**
- Update parameter: `jobExecutor *executor.JobExecutor` → `jobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator`
- Update field assignment: `jobExecutor: jobExecutor` → `jobDefinitionOrchestrator: jobDefinitionOrchestrator`

**Method Calls (throughout file):**
- Change all: `h.jobExecutor.Execute(...)` → `h.jobDefinitionOrchestrator.Execute(...)`
- Update nil checks: `h.jobExecutor == nil` → `h.jobDefinitionOrchestrator == nil`

**Comments:**
- Update references to "JobExecutor" → "JobDefinitionOrchestrator"

**Validation Criteria:**
- File compiles successfully
- All method calls work correctly
- Nil checks work correctly
- Handler tests pass (if available)

---

### Step 7: Compile and Validate All Changes

**Skill:** @go-coder

**Files:** All modified files

**User Decision Required:** No

**Description:**
Compile the full application and verify all changes work together:

**Compilation Steps:**
1. Build full application: `go build -o nul ./cmd/quaero`
2. Verify no compile errors
3. Check for any remaining executor package references: `grep -r "internal/jobs/executor" internal/ cmd/`
4. Verify all imports resolved correctly

**Validation Steps:**
1. Verify all 3 new managers exist in manager/ directory
2. Verify JobDefinitionOrchestrator exists at jobs/ root
3. Verify app.go has no executor imports
4. Verify job_definition_handler.go has no executor imports
5. Run application startup test (if available)

**Success Criteria:**
- Application compiles without errors
- No references to executor package in code files
- All managers registered correctly
- Orchestrator initialized correctly

---

### Step 8: Delete Old Directories and Files

**Skill:** @none

**Files:**
- `internal/jobs/executor/` (DELETE - entire directory, 9 files)
- `internal/interfaces/job_executor.go` (DELETE)

**User Decision Required:** No

**Description:**
Delete old executor/ directory and duplicate interface file:

**Executor Directory (9 files to delete):**
1. `transform_step_executor.go` - Migrated to manager/transform_manager.go
2. `reindex_step_executor.go` - Migrated to manager/reindex_manager.go
3. `places_search_step_executor.go` - Migrated to manager/places_search_manager.go
4. `job_executor.go` - Moved to jobs/job_definition_orchestrator.go
5. `crawler_step_executor.go` - Deprecated (migrated in ARCH-004)
6. `database_maintenance_step_executor.go` - Deprecated (migrated in ARCH-008)
7. `agent_step_executor.go` - Deprecated (migrated in ARCH-006)
8. `base_executor.go` - Unused utility (no longer referenced)
9. `interfaces.go` - Duplicate of manager/interfaces.go

**Duplicate Interface File:**
- `internal/interfaces/job_executor.go` - Old JobWorker interface (duplicated to worker/interfaces.go in ARCH-003)

**Validation Before Deletion:**
1. Verify no remaining imports of internal/jobs/executor in codebase
2. Verify no remaining imports of internal/interfaces/job_executor.go
3. Verify application still compiles successfully
4. Run git status to confirm files are deleted

**Note:** This is an aggressive cleanup with no backward compatibility, as explicitly requested by the user.

---

### Step 9: Update AGENTS.md Documentation

**Skill:** @none

**Files:**
- `AGENTS.md` (MODIFY)

**User Decision Required:** No

**Description:**
Update AGENTS.md to reflect completed Manager/Worker/Orchestrator architecture migration:

**Section Updates:**

1. **Directory Structure Section:**
   - Change title: "In Transition - ARCH-006" → "Migration Complete - ARCH-009"
   - Update manager list to show all 6 managers with checkmarks
   - Update worker list to show all 3 workers with checkmarks
   - Add job_definition_orchestrator.go to jobs/ root listing
   - Remove all "⏳ pending" indicators
   - Remove "Old Directories" section

2. **Migration Progress Section:**
   - Add Phase ARCH-009: ✅ Final cleanup and migration complete
   - Mark all phases complete with checkmarks
   - Remove "YOU ARE HERE" marker

3. **Interfaces Section:**
   - Remove "Old Architecture" section entirely
   - Update JobManager implementations list: Add TransformManager, ReindexManager, PlacesSearchManager
   - Update core components: Add JobDefinitionOrchestrator description

**Key Changes:**
- Remove all "In Transition" status indicators
- Remove all references to executor/ or processor/ directories
- Mark ARCH-009 as complete
- Emphasize that migration is complete, not in transition

**Validation Criteria:**
- All status indicators updated
- No references to old directories
- All file paths accurate
- Migration timeline complete

---

### Step 10: Update MANAGER_WORKER_ARCHITECTURE.md Documentation

**Skill:** @none

**Files:**
- `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` (MODIFY)

**User Decision Required:** No

**Description:**
Update architecture documentation to reflect ARCH-009 completion:

**Section Updates:**

1. **Current Status Section:**
   - Change title: "After ARCH-008" → "Migration Complete (ARCH-009)"
   - Update to show all components migrated
   - Mark old directories as deleted
   - Show complete migration timeline

2. **Add New Section: "Final Cleanup (ARCH-009)":**
   - Document 3 remaining managers migrated (transform, reindex, places_search)
   - Document JobDefinitionOrchestrator relocation
   - Document directories deleted (executor/, interfaces/job_executor.go)
   - Document import path updates
   - Document breaking changes
   - Emphasize architectural completion

**Key Content:**
- All managers in internal/jobs/manager/ directory (6 total)
- All workers in internal/jobs/worker/ directory (3 workers + job processor)
- ParentJobOrchestrator in internal/jobs/orchestrator/
- JobDefinitionOrchestrator in internal/jobs/ root
- Old directories deleted: executor/, interfaces/job_executor.go

**Validation Criteria:**
- All file paths accurate
- All migration phases documented
- Breaking changes documented
- Architectural completion emphasized

---

## Success Criteria

**Code Changes:**
1. ✅ All 3 managers created: TransformManager, ReindexManager, PlacesSearchManager
2. ✅ JobDefinitionOrchestrator relocated to jobs/ root
3. ✅ app.go updated: No executor imports, all managers use manager/ package
4. ✅ job_definition_handler.go updated: Uses JobDefinitionOrchestrator
5. ✅ Application compiles successfully with no errors
6. ✅ No remaining references to executor package in code files

**File Operations:**
7. ✅ executor/ directory deleted (9 files)
8. ✅ internal/interfaces/job_executor.go deleted

**Documentation:**
9. ✅ AGENTS.md updated to reflect completed migration
10. ✅ MANAGER_WORKER_ARCHITECTURE.md updated with ARCH-009 section
11. ✅ All status indicators show completion (no "In Transition" status)

**Architectural Goals:**
12. ✅ Manager/Worker/Orchestrator architecture complete
13. ✅ Clear separation of concerns: Managers (orchestration), Workers (execution), Orchestrators (monitoring + routing)
14. ✅ No backward compatibility maintained (breaking changes acceptable)
15. ✅ Clean architecture with no legacy code remaining

---

## Risk Assessment

- **Low Risk**: Manager migrations follow established pattern (ARCH-004 through ARCH-008)
- **Medium Risk**: JobExecutor rename affects 2 files (app.go, handler) - validated by compilation
- **Low Risk**: Import updates are compile-time checked
- **Very Low Risk**: Directory deletion (breaking changes acceptable per user)
- **Low Risk**: Documentation updates (no code impact)

---

## Notes

- **Breaking Changes Acceptable:** User explicitly approved aggressive cleanup with no backward compatibility
- **Follow Established Pattern:** ARCH-004 through ARCH-008 established successful migration pattern
- **Test After Each Step:** Compile after each step to catch issues early
- **Complete Migration:** This is the final phase - no more executor/ references will remain

---

**Agent 1 (Planner) Complete - Ready for Agent 2 (Implementation)**
