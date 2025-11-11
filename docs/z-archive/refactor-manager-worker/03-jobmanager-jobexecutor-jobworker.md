I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The codebase is ready for manager file migration after completing ARCH-002 (interface renames) and ARCH-003 (directory creation):

1. **Target Files (3 managers to migrate):**
   - `crawler_step_executor.go` (255 lines) - CrawlerStepExecutor struct, NewCrawlerStepExecutor constructor
   - `agent_step_executor.go` (286 lines) - AgentStepExecutor struct, NewAgentStepExecutor constructor
   - `database_maintenance_step_executor.go` (137 lines) - DatabaseMaintenanceStepExecutor struct, NewDatabaseMaintenanceStepExecutor constructor

2. **All 3 Files Already Updated:**
   - Implement `CreateParentJob()` method (renamed from ExecuteStep in ARCH-002)
   - Implement `GetManagerType()` method (renamed from GetStepType in ARCH-002)
   - Have updated comments mentioning "Manager" terminology
   - Use correct interface methods

3. **Import Locations (2 files):**
   - `internal/app/app.go` (lines 20, 378, 390, 400) - Imports executor package and creates all 3 managers
   - `internal/handlers/job_definition_handler.go` (line 20) - Imports executor package (uses executor.JobExecutor orchestrator, not these specific managers)

4. **Target Directory Ready:**
   - `internal/jobs/manager/` exists with interfaces.go
   - JobManager interface properly defined
   - No implementation files yet (this phase adds them)

5. **Other Manager Files (NOT migrating in this phase):**
   - `transform_step_executor.go` (112 lines) - TransformStepExecutor
   - `reindex_step_executor.go` (121 lines) - ReindexStepExecutor
   - `places_search_step_executor.go` (274 lines) - PlacesSearchStepExecutor
   - These will be migrated separately (likely by other team members or follow-up phase)

**Key Architectural Insight:**

The struct names still use "StepExecutor" suffix even though comments mention "Manager". This phase corrects this inconsistency:
- `CrawlerStepExecutor` → `CrawlerManager`
- `AgentStepExecutor` → `AgentManager`
- `DatabaseMaintenanceStepExecutor` → `DatabaseMaintenanceManager`

**Dependencies Analysis:**

- **CrawlerManager**: CrawlerService, Logger (2 dependencies)
- **DatabaseMaintenanceManager**: JobManager, QueueManager, Logger (3 dependencies)
- **AgentManager**: JobManager, QueueManager, SearchService, Logger (4 dependencies)

All dependencies are injected via constructors (good DI pattern).

**Risk Assessment:**

- **Low Risk**: File copying and renaming (mechanical transformation)
- **Low Risk**: Package declaration changes (compile-time checked)
- **Medium Risk**: Import path updates in app.go (must update 3 registration calls)
- **Low Risk**: Backward compatibility (old files remain, easy rollback)

**Success Criteria:**

1. 3 new manager files created in internal/jobs/manager/
2. All struct names use "Manager" suffix (not "StepExecutor")
3. All constructor names use "Manager" suffix
4. app.go successfully imports and uses new managers
5. Application compiles and runs successfully
6. All tests pass (especially crawler, agent, and database maintenance tests)
7. Old files remain in executor/ for backward compatibility

### Approach

**Incremental File Migration with Dual Import Strategy**

This phase migrates 3 manager files from `internal/jobs/executor/` to `internal/jobs/manager/` while maintaining backward compatibility. The approach follows these principles:

1. **Copy-First Strategy**: Create new files in manager/ before modifying imports
2. **Dual Import Period**: Support both old and new import paths temporarily
3. **Gradual Transition**: Update registration calls one at a time
4. **Backward Compatibility**: Keep old files intact until ARCH-008
5. **Minimal Risk**: Each file migration is independent and testable

**Why This Approach:**

- **Zero Downtime**: Application continues working throughout migration
- **Incremental Validation**: Each file can be tested independently
- **Easy Rollback**: Simply revert to old imports if issues arise
- **Clear Audit Trail**: Git history shows exact transformation
- **Supports Parallel Work**: Other engineers can work on subsequent phases

**Migration Sequence:**

1. **CrawlerManager** - Simplest (fewest dependencies)
2. **DatabaseMaintenanceManager** - Medium complexity (JobManager + QueueManager)
3. **AgentManager** - Most complex (JobManager + QueueManager + SearchService + polling logic)

**Key Transformations Per File:**

- Package: `executor` → `manager`
- Struct: `*StepExecutor` → `*Manager`
- Constructor: `New*StepExecutor` → `New*Manager`
- Comments: Update to reflect manager terminology
- Imports: No changes needed (all use internal/interfaces, internal/models, etc.)

**Import Strategy:**

Files that import these managers will temporarily support both paths:
```go
import (
    "github.com/ternarybob/quaero/internal/jobs/executor"  // OLD - Remove in ARCH-008
    "github.com/ternarybob/quaero/internal/jobs/manager"   // NEW
)
```

Then update registration calls to use new constructors:
```go
// OLD: crawlerStepExecutor := executor.NewCrawlerStepExecutor(...)
// NEW: crawlerManager := manager.NewCrawlerManager(...)
```

**Files Requiring Import Updates:**

1. `internal/app/app.go` - Registers all 3 managers
2. `internal/handlers/job_definition_handler.go` - Imports executor package (may not use these specific managers)

**Validation Strategy:**

After each file migration:
1. Verify new file compiles in manager/ package
2. Update app.go to use new constructor
3. Build application successfully
4. Run relevant tests (crawler tests, agent tests, database maintenance tests)
5. Verify job execution works end-to-end

**Note on Other Manager Files:**

The executor/ directory contains 3 additional manager files that are NOT migrated in this phase:
- `transform_step_executor.go` - Will be migrated separately
- `reindex_step_executor.go` - Will be migrated separately  
- `places_search_step_executor.go` - Will be migrated separately

These will likely be migrated in a follow-up phase or handled by other team members.

### Reasoning

I systematically explored the codebase to understand the migration scope:

1. **Read the 3 target files** - Examined crawler_step_executor.go, agent_step_executor.go, database_maintenance_step_executor.go to understand structure and dependencies
2. **Listed executor directory** - Found 10 files total (6 managers + 2 database maintenance + 1 orchestrator + 1 interfaces)
3. **Searched for imports** - Found app.go and job_definition_handler.go import from internal/jobs/executor
4. **Examined registration code** - Analyzed how managers are registered in app.go (lines 377-403)
5. **Verified target directory** - Confirmed internal/jobs/manager/ exists with interfaces.go from ARCH-003
6. **Read other manager files** - Examined transform, reindex, and places_search to understand full scope (not migrating these in this phase)
7. **Analyzed interface** - Confirmed JobManager interface in manager/interfaces.go matches what implementations expect

This exploration revealed that the migration is straightforward: copy files, rename structs/constructors, update imports in 2 files.

## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant Old as internal/jobs/executor/
    participant New as internal/jobs/manager/
    participant App as internal/app/app.go
    participant Build as Go Build System
    
    Note over Dev,Build: Phase 1: Create New Manager Files
    
    Dev->>New: Create crawler_manager.go
    Note right of New: Copy from crawler_step_executor.go<br/>Package: executor → manager<br/>Struct: CrawlerStepExecutor → CrawlerManager<br/>Constructor: New*StepExecutor → New*Manager
    
    Dev->>New: Create database_maintenance_manager.go
    Note right of New: Copy from database_maintenance_step_executor.go<br/>Apply same transformations
    
    Dev->>New: Create agent_manager.go
    Note right of New: Copy from agent_step_executor.go<br/>Apply same transformations
    
    Dev->>Build: Compile new manager files
    Build-->>Dev: ✓ All 3 files compile successfully
    
    Note over Dev,Build: Phase 2: Update Import Paths
    
    Dev->>App: Add manager import
    Note right of App: import "internal/jobs/manager"<br/>Keep executor import for other managers
    
    Dev->>App: Update CrawlerManager registration
    Note right of App: executor.NewCrawlerStepExecutor()<br/>→ manager.NewCrawlerManager()
    
    Dev->>App: Update DatabaseMaintenanceManager registration
    Note right of App: executor.NewDatabaseMaintenanceStepExecutor()<br/>→ manager.NewDatabaseMaintenanceManager()
    
    Dev->>App: Update AgentManager registration
    Note right of App: executor.NewAgentStepExecutor()<br/>→ manager.NewAgentManager()
    
    Dev->>Build: Build application
    Build-->>Dev: ✓ Application compiles successfully
    
    Note over Dev,Build: Phase 3: Add Deprecation Notices
    
    Dev->>Old: Add deprecation comment to crawler_step_executor.go
    Dev->>Old: Add deprecation comment to database_maintenance_step_executor.go
    Dev->>Old: Add deprecation comment to agent_step_executor.go
    
    Note over Old: Files remain functional<br/>for backward compatibility<br/>Will be deleted in ARCH-008
    
    Note over Dev,Build: Phase 4: Validation
    
    Dev->>Build: Run test suite
    Build-->>Dev: ✓ All tests pass
    
    Dev->>App: Start application
    App->>New: Register CrawlerManager
    App->>New: Register DatabaseMaintenanceManager
    App->>New: Register AgentManager
    App-->>Dev: ✓ All managers registered successfully
    
    Note over Dev,Build: Migration Complete<br/>3 managers migrated<br/>Old files deprecated<br/>Backward compatible

## Proposed File Changes

### internal\jobs\manager\crawler_manager.go(NEW)

References: 

- internal\jobs\executor\crawler_step_executor.go(MODIFY)
- internal\jobs\manager\interfaces.go

Create new CrawlerManager file by copying from `internal/jobs/executor/crawler_step_executor.go` with the following transformations:

**Package Declaration:**
- Change: `package executor` → `package manager`

**Struct Rename:**
- Change: `type CrawlerStepExecutor struct` → `type CrawlerManager struct`
- Keep all fields unchanged: `crawlerService`, `logger`

**Constructor Rename:**
- Change: `func NewCrawlerStepExecutor(...)` → `func NewCrawlerManager(...)`
- Change return type: `*CrawlerStepExecutor` → `*CrawlerManager`
- Update struct initialization: `return &CrawlerStepExecutor{...}` → `return &CrawlerManager{...}`
- Update comment: "creates a new crawler step executor" → "creates a new crawler manager"

**Method Receivers:**
- Change all method receivers: `func (e *CrawlerStepExecutor)` → `func (m *CrawlerManager)`
- Rename receiver variable from `e` to `m` for consistency (manager convention)
- Update all references to `e.` → `m.` within method bodies

**Method Implementations:**
- Keep `CreateParentJob()` method signature unchanged (already updated in ARCH-002)
- Keep `GetManagerType()` method signature unchanged (already updated in ARCH-002)
- Keep `buildCrawlConfig()` helper method (update receiver to `m`)
- Keep `buildSeedURLs()` helper method (update receiver to `m`)

**Comments:**
- Struct comment already says "CrawlerManager creates parent crawler jobs..." - keep as is
- Method comments already updated in ARCH-002 - keep as is
- Update any remaining references to "executor" → "manager" in comments

**Imports:**
- Keep all imports unchanged (no changes needed):
  - `context`, `fmt`, `time`
  - `github.com/ternarybob/arbor`
  - `github.com/ternarybob/quaero/internal/interfaces`
  - `github.com/ternarybob/quaero/internal/models`
  - `github.com/ternarybob/quaero/internal/services/crawler`

**Log Messages:**
- Update log messages for consistency:
  - "Creating parent crawler job" (already correct)
  - "Parent crawler job created successfully" (already correct)
  - No "executor" references found in log messages

**Validation:**
- Verify file compiles independently: `go build internal/jobs/manager/crawler_manager.go`
- Verify implements JobManager interface from `internal/jobs/manager/interfaces.go`
- Verify all method signatures match interface
- Total lines: ~255 (same as original)

### internal\jobs\manager\database_maintenance_manager.go(NEW)

References: 

- internal\jobs\executor\database_maintenance_step_executor.go(MODIFY)
- internal\jobs\manager\interfaces.go

Create new DatabaseMaintenanceManager file by copying from `internal/jobs/executor/database_maintenance_step_executor.go` with the following transformations:

**Package Declaration:**
- Change: `package executor` → `package manager`

**Struct Rename:**
- Change: `type DatabaseMaintenanceStepExecutor struct` → `type DatabaseMaintenanceManager struct`
- Keep all fields unchanged: `jobManager`, `queueMgr`, `logger`

**Constructor Rename:**
- Change: `func NewDatabaseMaintenanceStepExecutor(...)` → `func NewDatabaseMaintenanceManager(...)`
- Change return type: `*DatabaseMaintenanceStepExecutor` → `*DatabaseMaintenanceManager`
- Update struct initialization: `return &DatabaseMaintenanceStepExecutor{...}` → `return &DatabaseMaintenanceManager{...}`
- Update comment: "creates a new database maintenance step executor" → "creates a new database maintenance manager"

**Method Receivers:**
- Change all method receivers: `func (e *DatabaseMaintenanceStepExecutor)` → `func (m *DatabaseMaintenanceManager)`
- Rename receiver variable from `e` to `m` for consistency (manager convention)
- Update all references to `e.` → `m.` within method bodies

**Method Implementations:**
- Keep `CreateParentJob()` method signature unchanged (already updated in ARCH-002)
- Keep `GetManagerType()` method signature unchanged (already updated in ARCH-002)

**Comments:**
- Struct comment already says "DatabaseMaintenanceManager creates parent database maintenance jobs..." - keep as is
- Method comments already updated in ARCH-002 - keep as is
- Update file header comment: "Database Maintenance Step Executor" → "Database Maintenance Manager"
- Update any remaining references to "executor" → "manager" in comments

**Imports:**
- Keep all imports unchanged:
  - `context`, `encoding/json`, `fmt`
  - `github.com/google/uuid`
  - `github.com/ternarybob/arbor`
  - `github.com/ternarybob/quaero/internal/jobs`
  - `github.com/ternarybob/quaero/internal/models`
  - `github.com/ternarybob/quaero/internal/queue`

**Log Messages:**
- Update log messages for consistency:
  - "Creating parent database maintenance job" (already correct)
  - "Database maintenance job created and enqueued successfully" (already correct)
  - No "executor" references found in log messages

**Validation:**
- Verify file compiles independently
- Verify implements JobManager interface from `internal/jobs/manager/interfaces.go`
- Verify all method signatures match interface
- Total lines: ~137 (same as original)

### internal\jobs\manager\agent_manager.go(NEW)

References: 

- internal\jobs\executor\agent_step_executor.go(MODIFY)
- internal\jobs\manager\interfaces.go

Create new AgentManager file by copying from `internal/jobs/executor/agent_step_executor.go` with the following transformations:

**Package Declaration:**
- Change: `package executor` → `package manager`

**Struct Rename:**
- Change: `type AgentStepExecutor struct` → `type AgentManager struct`
- Keep all fields unchanged: `jobMgr`, `queueMgr`, `searchService`, `logger`

**Constructor Rename:**
- Change: `func NewAgentStepExecutor(...)` → `func NewAgentManager(...)`
- Change return type: `*AgentStepExecutor` → `*AgentManager`
- Update struct initialization: `return &AgentStepExecutor{...}` → `return &AgentManager{...}`
- Update comment: "creates a new agent step executor" → "creates a new agent manager"

**Method Receivers:**
- Change all method receivers: `func (e *AgentStepExecutor)` → `func (m *AgentManager)`
- Rename receiver variable from `e` to `m` for consistency (manager convention)
- Update all references to `e.` → `m.` within method bodies (including helper methods)

**Method Implementations:**
- Keep `CreateParentJob()` method signature unchanged (already updated in ARCH-002)
- Keep `GetManagerType()` method signature unchanged (already updated in ARCH-002)
- Keep `queryDocuments()` helper method (update receiver to `m`)
- Keep `createAgentJob()` helper method (update receiver to `m`)
- Keep `pollJobCompletion()` helper method (update receiver to `m`)

**Comments:**
- Struct comment already says "AgentManager creates parent agent jobs..." - keep as is
- Method comments already updated in ARCH-002 - keep as is
- Update any remaining references to "executor" → "manager" in comments

**Imports:**
- Keep all imports unchanged:
  - `context`, `fmt`, `time`
  - `github.com/ternarybob/arbor`
  - `github.com/ternarybob/quaero/internal/interfaces`
  - `github.com/ternarybob/quaero/internal/jobs`
  - `github.com/ternarybob/quaero/internal/models`
  - `github.com/ternarybob/quaero/internal/queue`

**Log Messages:**
- Update log messages for consistency:
  - "Creating parent agent job" (already correct)
  - "Agent jobs created and enqueued" (already correct)
  - "Agent job orchestration completed successfully" (already correct)
  - No "executor" references found in log messages

**Validation:**
- Verify file compiles independently
- Verify implements JobManager interface from `internal/jobs/manager/interfaces.go`
- Verify all method signatures match interface
- Total lines: ~286 (same as original)

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\manager\crawler_manager.go(NEW)
- internal\jobs\manager\database_maintenance_manager.go(NEW)
- internal\jobs\manager\agent_manager.go(NEW)
- internal\jobs\executor\job_executor.go

Update app.go to import and use the new manager package alongside the old executor package (dual import strategy for backward compatibility).

**Import Section Updates (around line 20):**

Add new import for manager package:
```go
import (
    // ... existing imports ...
    "github.com/ternarybob/quaero/internal/jobs/executor"  // OLD - Keep for other managers (transform, reindex, places_search)
    "github.com/ternarybob/quaero/internal/jobs/manager"   // NEW - For migrated managers
    // ... rest of imports ...
)
```

**CrawlerManager Registration (around line 378):**

Replace:
```go
crawlerStepExecutor := executor.NewCrawlerStepExecutor(a.CrawlerService, a.Logger)
a.JobExecutor.RegisterStepExecutor(crawlerStepExecutor)
a.Logger.Info().Msg("Crawler manager registered")
```

With:
```go
crawlerManager := manager.NewCrawlerManager(a.CrawlerService, a.Logger)
a.JobExecutor.RegisterStepExecutor(crawlerManager)
a.Logger.Info().Msg("Crawler manager registered")
```

**DatabaseMaintenanceManager Registration (around line 390):**

Replace:
```go
dbMaintenanceStepExecutor := executor.NewDatabaseMaintenanceStepExecutor(a.JobManager, queueMgr, a.Logger)
a.JobExecutor.RegisterStepExecutor(dbMaintenanceStepExecutor)
a.Logger.Info().Msg("Database maintenance manager registered")
```

With:
```go
dbMaintenanceManager := manager.NewDatabaseMaintenanceManager(a.JobManager, queueMgr, a.Logger)
a.JobExecutor.RegisterStepExecutor(dbMaintenanceManager)
a.Logger.Info().Msg("Database maintenance manager registered")
```

**AgentManager Registration (around line 400):**

Replace:
```go
agentStepExecutor := executor.NewAgentStepExecutor(jobMgr, queueMgr, a.SearchService, a.Logger)
a.JobExecutor.RegisterStepExecutor(agentStepExecutor)
a.Logger.Info().Msg("Agent manager registered")
```

With:
```go
agentManager := manager.NewAgentManager(jobMgr, queueMgr, a.SearchService, a.Logger)
a.JobExecutor.RegisterStepExecutor(agentManager)
a.Logger.Info().Msg("Agent manager registered")
```

**Keep Unchanged:**
- Transform, Reindex, and PlacesSearch manager registrations still use `executor` package (not migrated in this phase)
- JobExecutor orchestrator initialization (line 375) still uses `executor.NewJobExecutor`
- All other initialization code remains unchanged

**Variable Naming:**
- Changed from `*StepExecutor` suffix to `*Manager` suffix for clarity
- Examples: `crawlerStepExecutor` → `crawlerManager`, `agentStepExecutor` → `agentManager`

**Log Messages:**
- Already use "manager" terminology (updated in ARCH-002) - no changes needed

**Validation:**
- Verify application compiles successfully
- Verify all 3 managers are registered correctly
- Verify JobExecutor orchestrator can route to new managers
- Run application and check startup logs for "Crawler manager registered", "Database maintenance manager registered", "Agent manager registered"

### internal\handlers\job_definition_handler.go(MODIFY)

References: 

- internal\jobs\executor\job_executor.go
- internal\jobs\manager\interfaces.go

Update job_definition_handler.go to add dual import support for both executor and manager packages.

**Import Section Updates (around line 20):**

Add new import for manager package:
```go
import (
    // ... existing imports ...
    "github.com/ternarybob/quaero/internal/jobs/executor"  // OLD - Keep for JobExecutor orchestrator
    "github.com/ternarybob/quaero/internal/jobs/manager"   // NEW - For future use
    // ... rest of imports ...
)
```

**Note on Usage:**
This file imports the executor package but primarily uses `executor.JobExecutor` (the orchestrator struct, not the individual managers). The individual managers (CrawlerManager, AgentManager, etc.) are registered in app.go and accessed via the JobExecutor orchestrator's internal map.

**No Code Changes Required:**
- The handler doesn't directly instantiate managers
- It only uses the JobExecutor orchestrator to execute job definitions
- The orchestrator internally routes to registered managers (whether from executor or manager package)

**Why Add Manager Import:**
- Prepares for future refactoring when executor package is removed (ARCH-008)
- Maintains consistency with app.go import strategy
- Documents the transition state for developers

**Validation:**
- Verify file compiles successfully
- Verify no functional changes (handler behavior unchanged)
- Verify job definition execution still works correctly

### AGENTS.md(MODIFY)

References: 

- docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update AGENTS.md to document the progress of the manager file migration (ARCH-004 completion).

**Section to Update: "Directory Structure (In Transition)"**

Update the migration status:

```markdown
### Directory Structure (In Transition - ARCH-003)

Quaero is migrating to a Manager/Worker/Orchestrator architecture. The new structure is:

**New Directories (Created in ARCH-003):**
- `internal/jobs/manager/` - Job managers (orchestration layer) with `interfaces.go`
- `internal/jobs/worker/` - Job workers (execution layer) with `interfaces.go`
- `internal/jobs/orchestrator/` - Parent job orchestrator (monitoring layer) with `interfaces.go`

**Old Directories (Still Active - Will be removed in ARCH-008):**
- `internal/jobs/executor/` - Old manager implementations (9 files, migrating in ARCH-004)
- `internal/jobs/processor/` - Old worker implementations (5 files, migrating in ARCH-005/ARCH-006)

**Current Status:** Directory structure created, implementation files will be migrated in phases ARCH-004 through ARCH-006.

See [Manager/Worker Architecture](docs/architecture/MANAGER_WORKER_ARCHITECTURE.md) for complete migration details.
```

Change to:

```markdown
### Directory Structure (In Transition - ARCH-004)

Quaero is migrating to a Manager/Worker/Orchestrator architecture. The new structure is:

**New Directories:**
- `internal/jobs/manager/` - Job managers (orchestration layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `crawler_manager.go` (ARCH-004)
  - ✅ `database_maintenance_manager.go` (ARCH-004)
  - ✅ `agent_manager.go` (ARCH-004)
  - ⏳ `transform_manager.go` (pending)
  - ⏳ `reindex_manager.go` (pending)
  - ⏳ `places_search_manager.go` (pending)
- `internal/jobs/worker/` - Job workers (execution layer) with `interfaces.go`
- `internal/jobs/orchestrator/` - Parent job orchestrator (monitoring layer) with `interfaces.go`

**Old Directories (Still Active - Will be removed in ARCH-008):**
- `internal/jobs/executor/` - Old manager implementations (6 remaining files)
- `internal/jobs/processor/` - Old worker implementations (5 files, migrating in ARCH-005/ARCH-006)

**Migration Progress:**
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ⏳ Crawler worker migration (pending)
- Phase ARCH-006: ⏳ Remaining worker files migration (pending)

See [Manager/Worker Architecture](docs/architecture/MANAGER_WORKER_ARCHITECTURE.md) for complete details.
```

**Section to Update: "Interfaces"**

Update to reflect that managers are now in both locations:

```markdown
### Interfaces

**New Architecture (ARCH-003+):**
- `JobManager` interface - `internal/jobs/manager/interfaces.go`
  - Implementations: `CrawlerManager`, `DatabaseMaintenanceManager`, `AgentManager` (ARCH-004)
- `JobWorker` interface - `internal/jobs/worker/interfaces.go`
- `JobOrchestrator` interface - `internal/jobs/orchestrator/interfaces.go`

**Old Architecture (deprecated, will be removed in ARCH-008):**
- `JobManager` interface - `internal/jobs/executor/interfaces.go` (duplicate)
  - Remaining implementations: `TransformStepExecutor`, `ReindexStepExecutor`, `PlacesSearchStepExecutor`
- `JobWorker` interface - `internal/interfaces/job_executor.go` (duplicate)
```

**Implementation Notes:**
- Update migration status to show ARCH-004 complete
- Add checkmarks (✅) for completed files
- Add pending indicators (⏳) for remaining files
- Clarify which managers are migrated vs remaining in old location

### docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update MANAGER_WORKER_ARCHITECTURE.md to document the completion of ARCH-004 (manager file migration).

**Section to Update: "Current Status (After ARCH-003)"**

Change the migration status section to reflect ARCH-004 completion:

```markdown
### Current Status (After ARCH-004)

**New Directories Created:**
- ✅ `internal/jobs/manager/` - Created with interfaces.go (ARCH-003)
  - ✅ `crawler_manager.go` - Migrated from executor/ (ARCH-004)
  - ✅ `database_maintenance_manager.go` - Migrated from executor/ (ARCH-004)
  - ✅ `agent_manager.go` - Migrated from executor/ (ARCH-004)
- ✅ `internal/jobs/worker/` - Created with interfaces.go (ARCH-003)
- ✅ `internal/jobs/orchestrator/` - Created with interfaces.go (ARCH-003)

**Old Directories (Still Active):**
- `internal/jobs/executor/` - Contains 6 remaining implementation files:
  - `transform_step_executor.go` (pending migration)
  - `reindex_step_executor.go` (pending migration)
  - `places_search_step_executor.go` (pending migration)
  - `job_executor.go` (orchestrator - will be refactored separately)
  - `base_executor.go` (shared utilities - will be refactored separately)
  - `database_maintenance_executor.go` (old worker - will be deleted in ARCH-007)
- `internal/jobs/processor/` - Contains 5 implementation files (will be migrated in ARCH-005/ARCH-006)

**Migration Status:**
- Phase ARCH-001: ✅ Documentation created
- Phase ARCH-002: ✅ Interfaces renamed
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ Manager files migrated (crawler, database_maintenance, agent) (YOU ARE HERE)
- Phase ARCH-005: ⏳ Crawler worker migration (pending)
- Phase ARCH-006: ⏳ Remaining worker files migration (pending)
- Phase ARCH-007: ⏳ Parent job orchestrator migration (pending)
- Phase ARCH-008: ⏳ Database maintenance worker split (pending)
- Phase ARCH-009: ⏳ Import path updates and cleanup (pending)
- Phase ARCH-010: ⏳ End-to-end validation (pending)
```

**Section to Update: "File Structure Changes"**

Add a new subsection showing the actual migration:

```markdown
### Manager Files Migrated (ARCH-004)

**Files Moved from executor/ to manager/:**

1. **CrawlerManager** (`crawler_manager.go`)
   - Old: `internal/jobs/executor/crawler_step_executor.go`
   - New: `internal/jobs/manager/crawler_manager.go`
   - Struct: `CrawlerStepExecutor` → `CrawlerManager`
   - Constructor: `NewCrawlerStepExecutor()` → `NewCrawlerManager()`
   - Dependencies: CrawlerService, Logger

2. **DatabaseMaintenanceManager** (`database_maintenance_manager.go`)
   - Old: `internal/jobs/executor/database_maintenance_step_executor.go`
   - New: `internal/jobs/manager/database_maintenance_manager.go`
   - Struct: `DatabaseMaintenanceStepExecutor` → `DatabaseMaintenanceManager`
   - Constructor: `NewDatabaseMaintenanceStepExecutor()` → `NewDatabaseMaintenanceManager()`
   - Dependencies: JobManager, QueueManager, Logger

3. **AgentManager** (`agent_manager.go`)
   - Old: `internal/jobs/executor/agent_step_executor.go`
   - New: `internal/jobs/manager/agent_manager.go`
   - Struct: `AgentStepExecutor` → `AgentManager`
   - Constructor: `NewAgentStepExecutor()` → `NewAgentManager()`
   - Dependencies: JobManager, QueueManager, SearchService, Logger

**Import Path Updates:**
- `internal/app/app.go` - Updated to import and use new manager package
- `internal/handlers/job_definition_handler.go` - Added manager import for future use

**Backward Compatibility:**
- Old files remain in `internal/jobs/executor/` until ARCH-008
- Dual import strategy allows gradual transition
- No breaking changes to external APIs
```

**Implementation Notes:**
- Update migration status to show ARCH-004 complete
- Add detailed file mapping for migrated managers
- Document struct and constructor renames
- Clarify backward compatibility strategy

### internal\jobs\executor\crawler_step_executor.go(MODIFY)

References: 

- internal\jobs\manager\crawler_manager.go(NEW)

Add deprecation notice to the old CrawlerStepExecutor file to indicate it has been migrated.

**Add Deprecation Comment at Top of File (after package declaration):**

```go
package executor

// DEPRECATED: This file has been migrated to internal/jobs/manager/crawler_manager.go (ARCH-004).
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/manager and use CrawlerManager instead.
```

**No Other Changes:**
- Keep all existing code unchanged
- File remains functional for backward compatibility
- Will be deleted in ARCH-008 when all imports are updated

**Purpose:**
- Clearly communicate to developers that this file is deprecated
- Provide guidance on where to find the new implementation
- Document the migration timeline (removal in ARCH-008)
- Prevent new code from using the old location

### internal\jobs\executor\database_maintenance_step_executor.go(MODIFY)

References: 

- internal\jobs\manager\database_maintenance_manager.go(NEW)

Add deprecation notice to the old DatabaseMaintenanceStepExecutor file to indicate it has been migrated.

**Add Deprecation Comment at Top of File (after existing header comment):**

```go
// -----------------------------------------------------------------------
// Database Maintenance Step Executor - Handles "database_maintenance" action in job definitions
// -----------------------------------------------------------------------

// DEPRECATED: This file has been migrated to internal/jobs/manager/database_maintenance_manager.go (ARCH-004).
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/manager and use DatabaseMaintenanceManager instead.

package executor
```

**No Other Changes:**
- Keep all existing code unchanged
- File remains functional for backward compatibility
- Will be deleted in ARCH-008 when all imports are updated

**Purpose:**
- Clearly communicate to developers that this file is deprecated
- Provide guidance on where to find the new implementation
- Document the migration timeline (removal in ARCH-008)
- Prevent new code from using the old location

### internal\jobs\executor\agent_step_executor.go(MODIFY)

References: 

- internal\jobs\manager\agent_manager.go(NEW)

Add deprecation notice to the old AgentStepExecutor file to indicate it has been migrated.

**Add Deprecation Comment at Top of File (after package declaration):**

```go
package executor

// DEPRECATED: This file has been migrated to internal/jobs/manager/agent_manager.go (ARCH-004).
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/manager and use AgentManager instead.
```

**No Other Changes:**
- Keep all existing code unchanged
- File remains functional for backward compatibility
- Will be deleted in ARCH-008 when all imports are updated

**Purpose:**
- Clearly communicate to developers that this file is deprecated
- Provide guidance on where to find the new implementation
- Document the migration timeline (removal in ARCH-008)
- Prevent new code from using the old location