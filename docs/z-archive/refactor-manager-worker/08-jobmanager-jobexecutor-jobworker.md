I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The codebase is at ARCH-008 completion with the following remaining work:

**Files in executor/ Directory (9 total):**
1. **Deprecated with notices (3)**: crawler_step_executor.go, database_maintenance_step_executor.go, agent_step_executor.go
2. **Unmigrated managers (3)**: transform_step_executor.go, reindex_step_executor.go, places_search_step_executor.go
3. **Orchestrator (1)**: job_executor.go - Routes job definition steps to managers
4. **Unused utility (1)**: base_executor.go - No longer referenced
5. **Duplicate interface (1)**: interfaces.go - Duplicate of manager/interfaces.go

**Import Dependencies:**
- **app.go** (line 20): Imports executor package, uses executor.JobExecutor, calls executor.New*StepExecutor for transform/reindex/places_search
- **job_definition_handler.go** (line 20): Imports executor package, uses executor.JobExecutor field

**Duplicate Interface File:**
- **internal/interfaces/job_executor.go**: Old JobWorker interface, duplicated to worker/interfaces.go in ARCH-003

**Key Architectural Insight:**

The `job_executor.go` file is fundamentally different from managers:
- **Managers**: Create parent jobs, enqueue child jobs (CrawlerManager, AgentManager, etc.)
- **JobExecutor**: Orchestrates job definitions by routing steps to appropriate managers
- **JobOrchestrator**: Monitors parent job progress (different responsibility)

The JobExecutor is a job definition orchestrator, not a job manager. It should be renamed and relocated to clarify its purpose.

**Migration Pattern Established:**

From ARCH-004 through ARCH-008, the pattern is:
1. Copy file to new location with new name
2. Update package declaration
3. Rename struct: *StepExecutor → *Manager
4. Rename constructor: New*StepExecutor → New*Manager
5. Update receiver variable: e → m
6. Update all comments to use "manager" terminology
7. Update app.go registration
8. Delete old file immediately (no backward compatibility)

**Remaining Work:**

1. **Migrate 3 managers**: transform, reindex, places_search (follow established pattern)
2. **Relocate orchestrator**: job_executor.go → job_definition_orchestrator.go (special handling)
3. **Update 2 files**: app.go and job_definition_handler.go (import paths and references)
4. **Delete 2 locations**: executor/ directory and interfaces/job_executor.go
5. **Update documentation**: AGENTS.md to reflect completed migration

**Success Criteria:**

1. All managers in internal/jobs/manager/ directory
2. JobDefinitionOrchestrator in internal/jobs/ root
3. No references to internal/jobs/executor package
4. No references to internal/interfaces/job_executor.go
5. executor/ directory deleted
6. Application compiles successfully
7. All tests pass
8. Documentation reflects completed architecture
9. No "In Transition" status in AGENTS.md

### Approach

**Aggressive Cleanup Strategy with Complete Migration**

This final phase (ARCH-009) completes the Manager/Worker/Orchestrator architecture migration by:
1. Migrating the 3 remaining managers (transform, reindex, places_search)
2. Handling the JobExecutor orchestrator (special case - not a manager)
3. Updating all import paths throughout the codebase
4. Deleting old directories and duplicate interface files
5. Updating documentation to reflect completed architecture

**Key Architectural Decision: JobExecutor Placement**

The `job_executor.go` file is NOT a manager - it's an orchestrator that routes job definition steps to registered managers. It should remain in a dedicated location. Options:

1. **Keep in executor/ package** (rename to orchestration/) - Maintains separation
2. **Move to jobs/ root** - Simplifies structure (it's the only orchestrator of its kind)
3. **Create orchestration/ directory** - Most explicit but adds directory for single file

**Recommended: Option 2 (Move to jobs/ root)** - The file orchestrates job definitions by routing to managers. It's fundamentally different from JobOrchestrator (which monitors parent jobs). Keeping it at `internal/jobs/job_definition_orchestrator.go` makes its purpose clear and avoids creating a directory for a single file.

**Migration Strategy:**

**Phase 1: Migrate Remaining Managers**
- Move transform_step_executor.go → manager/transform_manager.go
- Move reindex_step_executor.go → manager/reindex_manager.go
- Move places_search_step_executor.go → manager/places_search_manager.go
- Update struct names: *StepExecutor → *Manager
- Update constructor names: New*StepExecutor → New*Manager
- Update receiver variables: e → m
- Update all comments to use "manager" terminology

**Phase 2: Handle JobExecutor Orchestrator**
- Move job_executor.go → internal/jobs/job_definition_orchestrator.go
- Rename struct: JobExecutor → JobDefinitionOrchestrator
- Update constructor: NewJobExecutor → NewJobDefinitionOrchestrator
- Update field in app.go: JobExecutor → JobDefinitionOrchestrator
- Update all references in app.go and job_definition_handler.go

**Phase 3: Update Import Paths**
- app.go: Remove executor import, update all executor.New* calls to manager.New*
- job_definition_handler.go: Update executor import to jobs package for orchestrator
- Search entire codebase for any remaining executor/processor imports

**Phase 4: Delete Old Files and Directories**
- Delete internal/jobs/executor/ directory (all files)
- Delete internal/interfaces/job_executor.go (duplicate of worker/interfaces.go)
- Verify processor/ directory already deleted (ARCH-007)

**Phase 5: Update Documentation**
- Update AGENTS.md to reflect completed migration
- Mark all phases as complete
- Remove "In Transition" status
- Update directory structure documentation
- Remove references to old architecture

**Breaking Changes (Acceptable per User):**
- All imports from executor/ package will break (intentional)
- JobExecutor struct renamed to JobDefinitionOrchestrator
- No backward compatibility maintained
- Aggressive cleanup without deprecation notices

**Validation Strategy:**
- Compile application after each phase
- Run full test suite after all changes
- Verify no remaining references to old packages
- Verify all managers registered correctly
- Test end-to-end job execution

**Risk Assessment:**
- **Low Risk**: Manager migrations follow established pattern (ARCH-004)
- **Medium Risk**: JobExecutor rename affects 2 files (app.go, handler)
- **Low Risk**: Import updates are compile-time checked
- **Very Low Risk**: Directory deletion (breaking changes acceptable)
- **Low Risk**: Documentation updates (no code impact)

### Reasoning

I systematically explored the codebase to understand the complete scope:

1. **Read app.go** - Identified executor import (line 20) and usage of executor.JobExecutor (line 68, 374) and executor.New*StepExecutor calls (lines 381-395)

2. **Read job_definition_handler.go** - Found executor import (line 20) and usage of executor.JobExecutor field

3. **Searched for executor imports** - Found only 2 files import from internal/jobs/executor

4. **Searched for processor imports** - Found none (already migrated in ARCH-007)

5. **Listed executor directory** - Found 9 files:
   - 3 deprecated managers with notices (crawler, database_maintenance, agent)
   - 3 unmigrated managers (transform, reindex, places_search)
   - 1 orchestrator (job_executor.go)
   - 1 utility (base_executor.go - unused)
   - 1 interface (interfaces.go - duplicate)

6. **Listed processor directory** - Already deleted (ARCH-007)

7. **Found duplicate interface** - internal/interfaces/job_executor.go exists (duplicate of worker/interfaces.go)

8. **Read AGENTS.md** - Confirmed migration status shows ARCH-008 complete, ARCH-009 pending

9. **Analyzed job_executor.go** - Confirmed it's an orchestrator that routes to managers, not a manager itself

This exploration revealed the complete scope: 3 managers to migrate, 1 orchestrator to relocate, 2 files to update imports, 2 directories/files to delete, and documentation to update.

## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant OldExec as executor/ directory
    participant NewMgr as manager/ directory
    participant NewOrch as jobs/ root
    participant App as app.go
    participant Handler as job_definition_handler.go
    participant Build as Go Build System
    
    Note over Dev,Build: Phase 1: Migrate Remaining Managers
    
    Dev->>NewMgr: Create transform_manager.go
    Note right of NewMgr: Copy from transform_step_executor.go<br/>Package: executor → manager<br/>Struct: TransformStepExecutor → TransformManager<br/>Constructor: New*StepExecutor → New*Manager
    
    Dev->>NewMgr: Create reindex_manager.go
    Note right of NewMgr: Same transformations
    
    Dev->>NewMgr: Create places_search_manager.go
    Note right of NewMgr: Same transformations
    
    Dev->>Build: Compile new managers
    Build-->>Dev: ✓ All 3 managers compile
    
    Note over Dev,Build: Phase 2: Relocate Job Definition Orchestrator
    
    Dev->>NewOrch: Create job_definition_orchestrator.go
    Note right of NewOrch: Move from executor/job_executor.go<br/>Package: executor → jobs<br/>Struct: JobExecutor → JobDefinitionOrchestrator<br/>Constructor: NewJobExecutor → NewJobDefinitionOrchestrator<br/>Receiver: e → o
    
    Dev->>Build: Compile orchestrator
    Build-->>Dev: ✓ Orchestrator compiles
    
    Note over Dev,Build: Phase 3: Update Import Paths
    
    Dev->>App: Remove executor import (line 20)
    Dev->>App: Update field: JobExecutor → JobDefinitionOrchestrator
    Dev->>App: Update initialization (line 374)
    Dev->>App: Update 3 manager registrations (lines 381-395)
    Note right of App: transform, reindex, places_search<br/>executor.New* → manager.New*
    
    Dev->>Handler: Remove executor import (line 20)
    Dev->>Handler: Update field: jobExecutor → jobDefinitionOrchestrator
    Dev->>Handler: Update all method calls
    
    Dev->>Build: Build application
    Build-->>Dev: ✓ Application compiles successfully
    
    Note over Dev,Build: Phase 4: Delete Old Files
    
    Dev->>OldExec: Delete entire executor/ directory
    Note right of OldExec: 9 files deleted:<br/>- 3 migrated managers<br/>- 1 relocated orchestrator<br/>- 3 deprecated managers<br/>- 1 unused utility<br/>- 1 duplicate interface
    
    Dev->>Dev: Delete internal/interfaces/job_executor.go
    Note right of Dev: Duplicate interface removed
    
    Note over Dev,Build: Phase 5: Update Documentation
    
    Dev->>Dev: Update AGENTS.md
    Note right of Dev: Remove "In Transition" status<br/>Mark all phases complete<br/>Update directory structure<br/>Remove old architecture references
    
    Dev->>Dev: Update MANAGER_WORKER_ARCHITECTURE.md
    Note right of Dev: Add ARCH-009 section<br/>Document final cleanup<br/>Mark migration complete
    
    Note over Dev,Build: Phase 6: Validation
    
    Dev->>Build: Run full test suite
    Build-->>Dev: ✓ All tests pass
    
    Dev->>App: Start application
    App->>NewMgr: Register all 6 managers
    App->>NewOrch: Initialize JobDefinitionOrchestrator
    App-->>Dev: ✓ All components initialized
    
    Dev->>App: Execute job definition
    App->>NewOrch: JobDefinitionOrchestrator.Execute()
    NewOrch->>NewMgr: Route steps to managers
    NewMgr->>NewMgr: Create parent jobs, enqueue children
    NewMgr-->>NewOrch: Return job IDs
    NewOrch-->>App: ✓ Job execution complete
    
    Note over Dev,Build: Migration Complete<br/>Manager/Worker/Orchestrator architecture<br/>All old directories deleted<br/>No backward compatibility<br/>Clean architecture achieved

## Proposed File Changes

### internal\jobs\manager\transform_manager.go(NEW)

References: 

- internal\jobs\executor\transform_step_executor.go
- internal\jobs\manager\interfaces.go

Create new TransformManager by copying from `internal/jobs/executor/transform_step_executor.go` with transformations:

**Package Declaration:**
- Change: `package executor` → `package manager`

**Struct Rename:**
- Change: `type TransformStepExecutor struct` → `type TransformManager struct`
- Keep all 3 fields unchanged: transformService, jobManager, logger
- Update struct comment: "TransformManager orchestrates document transformation workflows, converting HTML content to markdown format"

**Constructor Rename:**
- Change: `func NewTransformStepExecutor(...)` → `func NewTransformManager(...)`
- Change return type: `*TransformStepExecutor` → `*TransformManager`
- Update struct initialization: `return &TransformStepExecutor{...}` → `return &TransformManager{...}`
- Update comment: "NewTransformManager creates a new transform manager for orchestrating document transformation workflows"

**Method Receivers:**
- Change all method receivers: `func (e *TransformStepExecutor)` → `func (m *TransformManager)`
- Rename receiver variable from `e` to `m` for consistency (manager convention)
- Update all references to `e.` → `m.` within method bodies
- Methods: CreateParentJob(), GetManagerType()

**Comments:**
- Update all comments referencing "executor" → "manager"
- Update all comments referencing "TransformStepExecutor" → "TransformManager"
- Keep all existing detailed comments about synchronous operation

**Log Messages:**
- Update log messages: "executor" → "manager" where referring to this component
- Keep all other log messages unchanged

**Implementation Notes:**
- This is a synchronous manager (doesn't create async jobs)
- Returns placeholder job ID for tracking
- Validates input/output formats (only html→markdown supported)
- Total lines: ~112 (same as original)

### internal\jobs\manager\reindex_manager.go(NEW)

References: 

- internal\jobs\executor\reindex_step_executor.go
- internal\jobs\manager\interfaces.go

Create new ReindexManager by copying from `internal/jobs/executor/reindex_step_executor.go` with transformations:

**Package Declaration:**
- Change: `package executor` → `package manager`

**Struct Rename:**
- Change: `type ReindexStepExecutor struct` → `type ReindexManager struct`
- Keep all 3 fields unchanged: documentStorage, jobManager, logger
- Update struct comment: "ReindexManager orchestrates FTS5 full-text search index rebuilding workflows for optimal search performance"

**Constructor Rename:**
- Change: `func NewReindexStepExecutor(...)` → `func NewReindexManager(...)`
- Change return type: `*ReindexStepExecutor` → `*ReindexManager`
- Update struct initialization: `return &ReindexStepExecutor{...}` → `return &ReindexManager{...}`
- Update comment: "NewReindexManager creates a new reindex manager for orchestrating FTS5 index rebuilding"

**Method Receivers:**
- Change all method receivers: `func (e *ReindexStepExecutor)` → `func (m *ReindexManager)`
- Rename receiver variable from `e` to `m` for consistency (manager convention)
- Update all references to `e.` → `m.` within method bodies
- Methods: CreateParentJob(), GetManagerType()

**Comments:**
- Update all comments referencing "executor" → "manager"
- Update all comments referencing "ReindexStepExecutor" → "ReindexManager"
- Keep all existing detailed comments about FTS5 rebuild operation

**Log Messages:**
- Update log messages: "executor" → "manager" where referring to this component
- Keep all other log messages unchanged (e.g., "Rebuilding FTS5 index")

**Implementation Notes:**
- This is a synchronous manager (doesn't create async jobs)
- Returns placeholder job ID for tracking
- Directly calls documentStorage.RebuildFTS5Index()
- Total lines: ~121 (same as original)

### internal\jobs\manager\places_search_manager.go(NEW)

References: 

- internal\jobs\executor\places_search_step_executor.go
- internal\jobs\manager\interfaces.go

Create new PlacesSearchManager by copying from `internal/jobs/executor/places_search_step_executor.go` with transformations:

**Package Declaration:**
- Change: `package executor` → `package manager`

**Struct Rename:**
- Change: `type PlacesSearchStepExecutor struct` → `type PlacesSearchManager struct`
- Keep all 4 fields unchanged: placesService, documentService, eventService, logger
- Update struct comment: "PlacesSearchManager orchestrates Google Places API search workflows and document creation"

**Constructor Rename:**
- Change: `func NewPlacesSearchStepExecutor(...)` → `func NewPlacesSearchManager(...)`
- Change return type: `*PlacesSearchStepExecutor` → `*PlacesSearchManager`
- Update struct initialization: `return &PlacesSearchStepExecutor{...}` → `return &PlacesSearchManager{...}`
- Update comment: "NewPlacesSearchManager creates a new places search manager for orchestrating Google Places API searches"

**Method Receivers:**
- Change all method receivers: `func (e *PlacesSearchStepExecutor)` → `func (m *PlacesSearchManager)`
- Rename receiver variable from `e` to `m` for consistency (manager convention)
- Update all references to `e.` → `m.` within method bodies
- Methods: CreateParentJob(), GetManagerType(), parsePlacesSearchConfig()

**Comments:**
- Update all comments referencing "executor" → "manager"
- Update all comments referencing "PlacesSearchStepExecutor" → "PlacesSearchManager"
- Keep all existing detailed comments about Places API integration

**Log Messages:**
- Update log messages: "executor" → "manager" where referring to this component
- Keep all other log messages unchanged (e.g., "Searching for places", "Places found")

**Implementation Notes:**
- This is a synchronous manager (doesn't create async jobs)
- Returns placeholder job ID for tracking
- Calls placesService.SearchPlaces() and creates documents
- Publishes DocumentSaved events for each place
- Total lines: ~274 (same as original)

### internal\jobs\job_definition_orchestrator.go(NEW)

References: 

- internal\jobs\executor\job_executor.go
- internal\jobs\orchestrator\job_orchestrator.go

Create new JobDefinitionOrchestrator by moving from `internal/jobs/executor/job_executor.go` with transformations:

**Package Declaration:**
- Change: `package executor` → `package jobs`

**Struct Rename:**
- Change: `type JobExecutor struct` → `type JobDefinitionOrchestrator struct`
- Keep all 4 fields unchanged: stepExecutors (map of JobManagers), jobManager, jobOrchestrator, logger
- Update struct comment: "JobDefinitionOrchestrator orchestrates job definition execution by routing steps to appropriate JobManagers and managing parent-child hierarchy"

**Constructor Rename:**
- Change: `func NewJobExecutor(...)` → `func NewJobDefinitionOrchestrator(...)`
- Change return type: `*JobExecutor` → `*JobDefinitionOrchestrator`
- Update struct initialization: `return &JobExecutor{...}` → `return &JobDefinitionOrchestrator{...}`
- Update comment: "NewJobDefinitionOrchestrator creates a new job definition orchestrator for routing job definition steps to managers"

**Method Receivers:**
- Change all method receivers: `func (e *JobExecutor)` → `func (o *JobDefinitionOrchestrator)`
- Rename receiver variable from `e` to `o` for consistency (orchestrator convention)
- Update all references to `e.` → `o.` within method bodies
- Methods: RegisterStepExecutor(), Execute(), checkErrorTolerance()

**Import Updates:**
- Keep: `"github.com/ternarybob/quaero/internal/jobs/orchestrator"` (for JobOrchestrator)
- Update: Import manager package for JobManager interface (if not already imported via jobs package)

**Comments:**
- Update all comments referencing "JobExecutor" → "JobDefinitionOrchestrator"
- Update RegisterStepExecutor comment: "RegisterStepExecutor registers a job manager for an action type" (keep method name for now)
- Keep all existing detailed comments about job definition execution flow

**Log Messages:**
- Update log messages: "JobExecutor" → "JobDefinitionOrchestrator" where referring to this component
- Keep all other log messages unchanged

**Key Distinction:**
- This orchestrator routes job definition steps to managers (CrawlerManager, AgentManager, etc.)
- Different from JobOrchestrator which monitors parent job progress
- Lives at jobs/ root level since it's the only job definition orchestrator

**Implementation Notes:**
- Total lines: ~468 (same as original)
- No functional changes, only naming and location
- Maintains all error handling, tolerance checking, and parent job creation logic

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\job_definition_orchestrator.go(NEW)
- internal\jobs\manager\transform_manager.go(NEW)
- internal\jobs\manager\reindex_manager.go(NEW)
- internal\jobs\manager\places_search_manager.go(NEW)

Update app.go to remove executor import and use new manager/orchestrator locations.

**Import Section Updates (line 20):**

Remove executor import:
```
"github.com/ternarybob/quaero/internal/jobs/executor"  // DELETE - migrated to manager/orchestrator
```

Keep existing imports (already correct):
- Line 21: `"github.com/ternarybob/quaero/internal/jobs/manager"` (already exists)
- Line 22: `"github.com/ternarybob/quaero/internal/jobs/orchestrator"` (already exists)
- Line 23: `"github.com/ternarybob/quaero/internal/jobs/worker"` (already exists)

**Field Declaration Update (line 68):**

Replace:
```
JobExecutor *executor.JobExecutor
```

With:
```
JobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator
```

**Orchestrator Initialization Update (line 374):**

Replace:
```
a.JobExecutor = executor.NewJobExecutor(jobMgr, jobOrchestrator, a.Logger)
```

With:
```
a.JobDefinitionOrchestrator = jobs.NewJobDefinitionOrchestrator(jobMgr, jobOrchestrator, a.Logger)
```

**Manager Registration Updates (lines 377-401):**

**Transform Manager (line 381):**
Replace:
```
transformStepExecutor := executor.NewTransformStepExecutor(a.TransformService, a.JobManager, a.Logger)
a.JobExecutor.RegisterStepExecutor(transformStepExecutor)
```

With:
```
transformManager := manager.NewTransformManager(a.TransformService, a.JobManager, a.Logger)
a.JobDefinitionOrchestrator.RegisterStepExecutor(transformManager)
```

**Reindex Manager (line 385):**
Replace:
```
reindexStepExecutor := executor.NewReindexStepExecutor(a.StorageManager.DocumentStorage(), a.JobManager, a.Logger)
a.JobExecutor.RegisterStepExecutor(reindexStepExecutor)
```

With:
```
reindexManager := manager.NewReindexManager(a.StorageManager.DocumentStorage(), a.JobManager, a.Logger)
a.JobDefinitionOrchestrator.RegisterStepExecutor(reindexManager)
```

**Places Search Manager (line 393):**
Replace:
```
placesSearchStepExecutor := executor.NewPlacesSearchStepExecutor(a.PlacesService, a.DocumentService, a.EventService, a.Logger)
a.JobExecutor.RegisterStepExecutor(placesSearchStepExecutor)
```

With:
```
placesSearchManager := manager.NewPlacesSearchManager(a.PlacesService, a.DocumentService, a.EventService, a.Logger)
a.JobDefinitionOrchestrator.RegisterStepExecutor(placesSearchManager)
```

**Log Message Updates:**
- Line 404: "JobExecutor initialized with all managers" → "JobDefinitionOrchestrator initialized with all managers"

**Variable Naming:**
- Changed from `*StepExecutor` suffix to `*Manager` suffix for clarity
- Changed from `JobExecutor` to `JobDefinitionOrchestrator` for clarity

**Keep Unchanged:**
- Crawler manager registration (line 377) - already uses manager.NewCrawlerManager
- Database maintenance manager registration (line 389) - already uses manager.NewDatabaseMaintenanceManager
- Agent manager registration (line 399) - already uses manager.NewAgentManager

**Validation:**
- Verify application compiles successfully
- Verify all managers are registered correctly
- Verify orchestrator is initialized correctly
- Run application and check startup logs

### internal\handlers\job_definition_handler.go(MODIFY)

References: 

- internal\jobs\job_definition_orchestrator.go(NEW)

Update job_definition_handler.go to use new JobDefinitionOrchestrator location.

**Import Section Updates (line 20):**

Remove executor import:
```
"github.com/ternarybob/quaero/internal/jobs/executor"  // DELETE
```

No new import needed - JobDefinitionOrchestrator is now in jobs package which is already imported at line 19:
```
"github.com/ternarybob/quaero/internal/jobs"  // Already exists
```

**Field Declaration Update (in JobDefinitionHandler struct):**

Replace:
```
jobExecutor *executor.JobExecutor
```

With:
```
jobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator
```

**Constructor Parameter Update (in NewJobDefinitionHandler):**

Replace parameter:
```
jobExecutor *executor.JobExecutor
```

With:
```
jobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator
```

Update field assignment:
```
jobExecutor: jobExecutor
```

With:
```
jobDefinitionOrchestrator: jobDefinitionOrchestrator
```

**Method Call Updates (throughout file):**

Replace all references:
```
h.jobExecutor.Execute(...)
```

With:
```
h.jobDefinitionOrchestrator.Execute(...)
```

**Nil Check Updates:**

Replace:
```
if h.jobExecutor == nil
```

With:
```
if h.jobDefinitionOrchestrator == nil
```

**Comment Updates:**

Update comments referencing "JobExecutor" to "JobDefinitionOrchestrator" where appropriate.

**Validation:**
- Verify file compiles successfully
- Verify all method calls work correctly
- Verify nil checks work correctly
- Run handler tests if available

### internal\jobs\executor(DELETE)

Delete the entire executor/ directory and all its contents.

**Files to be deleted (9 total):**
1. `transform_step_executor.go` - Migrated to manager/transform_manager.go
2. `reindex_step_executor.go` - Migrated to manager/reindex_manager.go
3. `places_search_step_executor.go` - Migrated to manager/places_search_manager.go
4. `job_executor.go` - Moved to jobs/job_definition_orchestrator.go
5. `crawler_step_executor.go` - Deprecated (migrated in ARCH-004)
6. `database_maintenance_step_executor.go` - Deprecated (migrated in ARCH-004)
7. `agent_step_executor.go` - Deprecated (migrated in ARCH-004)
8. `base_executor.go` - Unused utility (no longer referenced)
9. `interfaces.go` - Duplicate of manager/interfaces.go

**Rationale:**
- All managers have been migrated to manager/ directory
- Orchestrator moved to jobs/ root
- Deprecated files no longer needed (breaking changes acceptable)
- Base utility no longer used
- Interface duplicated in manager/interfaces.go

**Validation Before Deletion:**
- Verify no remaining imports of internal/jobs/executor in codebase
- Verify all managers registered in app.go from manager/ package
- Verify orchestrator imported from jobs/ package
- Verify application compiles successfully

**Note:** This is an aggressive cleanup with no backward compatibility, as explicitly requested by the user.

### internal\interfaces\job_executor.go(DELETE)

Delete the duplicate JobWorker interface file.

**Rationale:**
- This file contains the old JobWorker interface
- Interface was duplicated to `internal/jobs/worker/interfaces.go` in ARCH-003
- All workers now import from worker/interfaces.go
- No remaining references to this file

**Validation Before Deletion:**
- Verify no imports of internal/interfaces/job_executor.go in codebase
- Verify all workers import from internal/jobs/worker/interfaces.go
- Verify application compiles successfully

**Note:** Breaking change acceptable per user request.

### AGENTS.md(MODIFY)

References: 

- docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update AGENTS.md to reflect completed Manager/Worker/Orchestrator architecture migration.

**Section to Update: "Directory Structure (In Transition - ARCH-006)"**

Change section title from:
```markdown
#### Directory Structure (In Transition - ARCH-006)
```

To:
```markdown
#### Directory Structure (Migration Complete - ARCH-009)
```

Update content to reflect completed state:

```markdown
Quaero uses a Manager/Worker/Orchestrator architecture for job orchestration and execution:

**Directory Structure:**
- `internal/jobs/manager/` - Job managers (orchestration layer)
  - ✅ `interfaces.go` - JobManager interface
  - ✅ `crawler_manager.go` - Orchestrates URL crawling workflows
  - ✅ `database_maintenance_manager.go` - Orchestrates database optimization
  - ✅ `agent_manager.go` - Orchestrates AI document processing
  - ✅ `transform_manager.go` - Orchestrates HTML→markdown transformation
  - ✅ `reindex_manager.go` - Orchestrates FTS5 index rebuilding
  - ✅ `places_search_manager.go` - Orchestrates Google Places API searches

- `internal/jobs/worker/` - Job workers (execution layer)
  - ✅ `interfaces.go` - JobWorker interface
  - ✅ `crawler_worker.go` - Processes individual URL crawl jobs
  - ✅ `agent_worker.go` - Processes individual AI agent jobs
  - ✅ `database_maintenance_worker.go` - Processes individual database operations
  - ✅ `job_processor.go` - Routes jobs from queue to workers

- `internal/jobs/orchestrator/` - Parent job orchestrator (monitoring layer)
  - ✅ `interfaces.go` - JobOrchestrator interface
  - ✅ `job_orchestrator.go` - Monitors parent job progress

- `internal/jobs/` - Job definition orchestration
  - ✅ `job_definition_orchestrator.go` - Routes job definition steps to managers
  - ✅ `manager.go` - Job CRUD operations

**Migration Complete:**
- Phase ARCH-001: ✅ Documentation created
- Phase ARCH-002: ✅ Interfaces renamed
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated (merged crawler_executor.go + crawler_executor_auth.go)
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker.go, job_processor.go)
- Phase ARCH-007: ✅ Parent job orchestrator migrated
- Phase ARCH-008: ✅ Database maintenance manager/worker split
- Phase ARCH-009: ✅ Final cleanup and migration complete

See [Manager/Worker Architecture](docs/architecture/MANAGER_WORKER_ARCHITECTURE.md) for complete details.
```

**Section to Update: "Interfaces"**

Remove "Old Architecture" section entirely:

```markdown
#### Interfaces

**Architecture:**
- `JobManager` interface - `internal/jobs/manager/interfaces.go`
  - Implementations: CrawlerManager, DatabaseMaintenanceManager, AgentManager, TransformManager, ReindexManager, PlacesSearchManager
  - Methods: CreateParentJob(), GetManagerType()

- `JobWorker` interface - `internal/jobs/worker/interfaces.go`
  - Implementations: CrawlerWorker, AgentWorker, DatabaseMaintenanceWorker
  - Methods: Execute(), GetWorkerType(), Validate()

- `JobOrchestrator` interface - `internal/jobs/orchestrator/interfaces.go`
  - Implementation: JobOrchestrator
  - Methods: StartMonitoring(), SubscribeToChildStatusChanges()

**Core Components:**
- `JobProcessor` - `internal/jobs/worker/job_processor.go`
  - Routes jobs from queue to registered workers
  - Manages worker pool lifecycle (Start/Stop)

- `JobDefinitionOrchestrator` - `internal/jobs/job_definition_orchestrator.go`
  - Routes job definition steps to registered managers
  - Manages parent-child job hierarchy
  - Handles error tolerance and retry logic

- `JobOrchestrator` - `internal/jobs/orchestrator/job_orchestrator.go`
  - Monitors parent job progress in background goroutines
  - Aggregates child job statistics
  - Publishes real-time progress events
```

**Remove All References to:**
- "In Transition" status
- "Old Directories" section
- "Old Architecture" section
- "pending" migration indicators
- References to executor/ or processor/ directories

**Implementation Notes:**
- Mark all phases as complete with checkmarks (✅)
- Remove all "⏳ pending" indicators
- Update all file paths to reflect new locations
- Emphasize that migration is complete, not in transition

### docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update MANAGER_WORKER_ARCHITECTURE.md to reflect completed migration (ARCH-009).

**Section to Update: "Current Status (After ARCH-008)"**

Change section title and content:

```markdown
### Migration Complete (ARCH-009)

**Final Architecture:**

All components have been migrated to the Manager/Worker/Orchestrator architecture:

- ✅ `internal/jobs/manager/` - 6 managers (crawler, database_maintenance, agent, transform, reindex, places_search)
- ✅ `internal/jobs/worker/` - 3 workers + job processor (crawler, agent, database_maintenance, job_processor)
- ✅ `internal/jobs/orchestrator/` - 1 orchestrator (parent_job_orchestrator)
- ✅ `internal/jobs/` - 1 job definition orchestrator (job_definition_orchestrator)

**Old Directories Removed:**
- ❌ `internal/jobs/executor/` - DELETED (all files migrated)
- ❌ `internal/jobs/processor/` - DELETED (all files migrated in ARCH-007)
- ❌ `internal/interfaces/job_executor.go` - DELETED (duplicate interface)

**Migration Timeline:**
- Phase ARCH-001: ✅ Documentation created (2024-11-11)
- Phase ARCH-002: ✅ Interfaces renamed (2024-11-11)
- Phase ARCH-003: ✅ Directory structure created (2024-11-11)
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated and merged
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker, job_processor)
- Phase ARCH-007: ✅ Parent job orchestrator migrated
- Phase ARCH-008: ✅ Database maintenance manager/worker split
- Phase ARCH-009: ✅ Final cleanup - remaining managers migrated, directories deleted (COMPLETE)
```

**Section to Add: "Final Cleanup (ARCH-009)"**

Add new section after ARCH-008:

```markdown
### Final Cleanup (ARCH-009)

**Remaining Managers Migrated:**

1. **TransformManager** (`transform_manager.go`)
   - Old: `internal/jobs/executor/transform_step_executor.go`
   - New: `internal/jobs/manager/transform_manager.go`
   - Purpose: Orchestrates HTML→markdown transformation workflows
   - Synchronous operation (no child jobs)

2. **ReindexManager** (`reindex_manager.go`)
   - Old: `internal/jobs/executor/reindex_step_executor.go`
   - New: `internal/jobs/manager/reindex_manager.go`
   - Purpose: Orchestrates FTS5 full-text search index rebuilding
   - Synchronous operation (no child jobs)

3. **PlacesSearchManager** (`places_search_manager.go`)
   - Old: `internal/jobs/executor/places_search_step_executor.go`
   - New: `internal/jobs/manager/places_search_manager.go`
   - Purpose: Orchestrates Google Places API searches and document creation
   - Synchronous operation (no child jobs)

**Job Definition Orchestrator Relocated:**

- **JobDefinitionOrchestrator** (`job_definition_orchestrator.go`)
  - Old: `internal/jobs/executor/job_executor.go`
  - New: `internal/jobs/job_definition_orchestrator.go`
  - Struct: `JobExecutor` → `JobDefinitionOrchestrator`
  - Constructor: `NewJobExecutor()` → `NewJobDefinitionOrchestrator()`
  - Purpose: Routes job definition steps to appropriate managers
  - Key distinction: Orchestrates job definitions (not parent jobs)
  - Lives at jobs/ root since it's the only job definition orchestrator

**Directories Deleted:**

1. **internal/jobs/executor/** - All 9 files deleted:
   - 3 migrated managers (transform, reindex, places_search)
   - 1 relocated orchestrator (job_executor → job_definition_orchestrator)
   - 3 deprecated managers (crawler, database_maintenance, agent)
   - 1 unused utility (base_executor)
   - 1 duplicate interface (interfaces.go)

2. **internal/interfaces/job_executor.go** - Duplicate interface deleted:
   - Old JobWorker interface
   - Duplicated to worker/interfaces.go in ARCH-003
   - No longer needed

**Import Path Updates:**

- `internal/app/app.go` - Updated to use manager/ package for all managers, jobs/ package for orchestrator
- `internal/handlers/job_definition_handler.go` - Updated to use jobs/ package for orchestrator
- All executor/ imports removed
- All processor/ imports already removed (ARCH-007)

**Breaking Changes:**

- All imports from executor/ package no longer work (intentional)
- JobExecutor renamed to JobDefinitionOrchestrator
- No backward compatibility maintained
- Aggressive cleanup without deprecation notices

**Architectural Completion:**

The Manager/Worker/Orchestrator architecture is now complete:

- **Managers** (`internal/jobs/manager/`) - Create parent jobs, enqueue children, orchestrate workflows
- **Workers** (`internal/jobs/worker/`) - Execute individual jobs from queue, perform actual work
- **Orchestrators** - Two types:
  - `JobOrchestrator` (`internal/jobs/orchestrator/`) - Monitors parent job progress
  - `JobDefinitionOrchestrator` (`internal/jobs/`) - Routes job definition steps to managers

All three layers have clear separation of concerns and distinct responsibilities.
```

**Update All Status Indicators:**
- Change all "⏳ pending" to "✅ complete"
- Remove all "In Transition" references
- Update all file paths to reflect new locations
- Mark ARCH-009 as complete