I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The codebase is ready for remaining worker file migration after completing ARCH-005 (crawler worker):

1. **Target Files (2 workers to migrate):**
   - `agent_executor.go` (297 lines) - AgentExecutor struct with 5 dependencies
   - `processor.go` (244 lines) - JobProcessor struct (core routing system)

2. **AgentExecutor Already Partially Updated:**
   - Struct comment says "AgentWorker" but struct name is still "AgentExecutor" (inconsistency from ARCH-002)
   - Methods already implement JobWorker interface: GetWorkerType(), Validate(), Execute()
   - Helper method: publishAgentJobLog()
   - Dependencies: agentService, jobMgr, documentStorage, logger, eventService

3. **JobProcessor Already Uses JobWorker Interface:**
   - Field: `executors map[string]interfaces.JobWorker` (updated in ARCH-002)
   - Method: `RegisterExecutor(worker interfaces.JobWorker)` (updated in ARCH-002)
   - Routes jobs to registered workers based on job type
   - Core methods: Start(), Stop(), processJobs(), processNextJob()

4. **Import Locations (1 file):**
   - `internal/app/app.go`:
     - Line 22: Import statement for processor package
     - Line 67: Field declaration `JobProcessor *processor.JobProcessor`
     - Line 271: Constructor call `processor.NewJobProcessor()`
     - Line 314: Constructor call `processor.NewParentJobExecutor()` (NOT migrating - ARCH-007)
     - Line 323: Constructor call `processor.NewAgentExecutor()`

5. **Target Directory Ready:**
   - `internal/jobs/worker/` exists with interfaces.go and crawler_worker.go
   - JobWorker interface properly defined
   - Ready to receive 2 new files

6. **Files NOT Migrating in This Phase:**
   - `parent_job_executor.go` - Migrates to orchestrator/ in ARCH-007
   - This clear boundary prevents scope creep

**Key Architectural Insight:**

The struct name "AgentExecutor" is inconsistent with the comment "AgentWorker" (from ARCH-002). This phase corrects this inconsistency by renaming the struct to match the comment and the architectural pattern.

**Dependencies Analysis:**

- **AgentWorker**: 5 dependencies (agentService, jobMgr, documentStorage, logger, eventService)
- **JobProcessor**: 3 dependencies (queueMgr, jobMgr, logger)

All dependencies are injected via constructors (good DI pattern).

**Risk Assessment:**

- **Low Risk**: File copying and renaming (mechanical transformation)
- **Low Risk**: Package declaration changes (compile-time checked)
- **Low Risk**: Import path updates in app.go (single file, 4 locations)
- **Low Risk**: Backward compatibility (old files remain, easy rollback)
- **Very Low Risk**: JobProcessor rename (file name only, struct name unchanged)

**Success Criteria:**

1. 2 new worker files created in internal/jobs/worker/
2. AgentExecutor struct renamed to AgentWorker
3. processor.go renamed to job_processor.go (struct name unchanged)
4. app.go successfully imports and uses new workers
5. Application compiles and runs successfully
6. All tests pass (especially agent tests and job processor tests)
7. Old files remain in processor/ for backward compatibility
8. ParentJobExecutor remains in processor/ (migrates in ARCH-007)

### Approach

**Incremental Worker File Migration Strategy**

This phase migrates the remaining 2 worker files from `internal/jobs/processor/` to `internal/jobs/worker/` while maintaining backward compatibility. The approach follows the established pattern from ARCH-004 (manager migration) and ARCH-005 (crawler worker migration).

**Key Principles:**

1. **Copy-First Strategy**: Create new files in worker/ before modifying imports
2. **Dual Import Period**: Support both old and new import paths temporarily
3. **Minimal Scope**: Only migrate AgentExecutor and JobProcessor (leave ParentJobExecutor for ARCH-007)
4. **Backward Compatibility**: Keep old files intact until ARCH-008
5. **Consistent Naming**: Follow worker naming convention (AgentWorker, not AgentExecutor)

**Why This Approach:**

- **Proven Pattern**: Successfully used in ARCH-004 and ARCH-005
- **Low Risk**: Mechanical transformation with compile-time safety
- **Clear Boundaries**: ParentJobExecutor explicitly excluded (migrates in next phase)
- **Easy Validation**: Each file can be tested independently
- **Supports Parallel Work**: Other engineers can proceed with ARCH-007 immediately after

**Migration Sequence:**

1. **AgentWorker** - Simpler file (297 lines, 5 dependencies)
2. **JobProcessor** - Core routing system (244 lines, stays in worker/ as it routes to workers)

**Key Transformations Per File:**

**AgentExecutor → AgentWorker:**
- Package: `processor` → `worker`
- Struct: `AgentExecutor` → `AgentWorker`
- Constructor: `NewAgentExecutor()` → `NewAgentWorker()`
- Receiver: `func (e *AgentExecutor)` → `func (w *AgentWorker)`
- Comments: Update all references to "executor" → "worker"

**JobProcessor (rename file only):**
- File: `processor.go` → `job_processor.go`
- Package: `processor` → `worker`
- Keep struct name: `JobProcessor` (already correct)
- Keep constructor name: `NewJobProcessor()` (already correct)
- Update comments: Clarify it routes to JobWorkers

**Import Strategy:**

Files that import these workers will temporarily support both paths:
```go
import (
    "github.com/ternarybob/quaero/internal/jobs/processor"  // OLD - Keep for ParentJobExecutor
    "github.com/ternarybob/quaero/internal/jobs/worker"     // NEW - For AgentWorker and JobProcessor
)
```

**Files Requiring Import Updates:**

Only `internal/app/app.go` needs updates:
- Line 22: Add worker import (already exists from ARCH-005)
- Line 67: Update field type: `*processor.JobProcessor` → `*worker.JobProcessor`
- Line 271: Update constructor: `processor.NewJobProcessor()` → `worker.NewJobProcessor()`
- Line 323: Update constructor: `processor.NewAgentExecutor()` → `worker.NewAgentWorker()`
- Line 314: Keep `processor.NewParentJobExecutor()` unchanged (migrates in ARCH-007)

**Validation Strategy:**

After each file migration:
1. Verify new file compiles in worker/ package
2. Update app.go to use new constructor
3. Build application successfully
4. Run relevant tests (agent tests, job processor tests)
5. Verify job execution works end-to-end

**Note on ParentJobExecutor:**

The processor/ directory contains 3 files:
- `agent_executor.go` - Migrating in this phase ✓
- `processor.go` - Migrating in this phase ✓
- `parent_job_executor.go` - NOT migrating (moves to orchestrator/ in ARCH-007)

This clear separation ensures no confusion about scope.

### Reasoning

I systematically explored the codebase to understand the migration requirements:

1. **Read target files** - Examined agent_executor.go (297 lines) and processor.go (244 lines) to understand structure and dependencies
2. **Analyzed struct definitions** - Found AgentExecutor struct with 5 dependencies, JobProcessor struct with worker routing logic
3. **Searched for imports** - Used grep to find all references to processor.New* and AgentExecutor
4. **Read app.go sections** - Examined initialization code (lines 260-360) to understand registration flow
5. **Verified interface compliance** - Confirmed both files already implement JobWorker interface (updated in ARCH-002)
6. **Checked worker directory** - Confirmed worker/ exists with interfaces.go and crawler_worker.go from previous phases
7. **Identified scope boundary** - Confirmed ParentJobExecutor is NOT in scope for this phase (migrates to orchestrator/ in ARCH-007)

This comprehensive exploration revealed that the migration is straightforward: copy files, rename structs/constructors, update single import location in app.go.

## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant OldAgent as processor/agent_executor.go
    participant OldProc as processor/processor.go
    participant NewAgent as worker/agent_worker.go
    participant NewProc as worker/job_processor.go
    participant App as internal/app/app.go
    participant Build as Go Build System
    
    Note over Dev,Build: Phase 1: Create New Worker Files
    
    Dev->>NewAgent: Create agent_worker.go
    Note right of NewAgent: Copy from agent_executor.go<br/>Package: processor → worker<br/>Struct: AgentExecutor → AgentWorker<br/>Constructor: NewAgentExecutor → NewAgentWorker<br/>Receiver: (e *AgentExecutor) → (w *AgentWorker)
    
    Dev->>NewProc: Create job_processor.go
    Note right of NewProc: Copy from processor.go<br/>Package: processor → worker<br/>File rename only (minimal changes)<br/>Struct: JobProcessor (unchanged)<br/>Constructor: NewJobProcessor() (unchanged)
    
    Dev->>Build: Compile new worker files
    Build-->>Dev: ✓ Both files compile successfully
    
    Note over Dev,Build: Phase 2: Update Import Paths
    
    Dev->>App: Update field declaration (line 67)
    Note right of App: *processor.JobProcessor<br/>→ *worker.JobProcessor
    
    Dev->>App: Update JobProcessor initialization (line 271)
    Note right of App: processor.NewJobProcessor()<br/>→ worker.NewJobProcessor()
    
    Dev->>App: Update AgentWorker registration (line 323)
    Note right of App: processor.NewAgentExecutor()<br/>→ worker.NewAgentWorker()<br/>Variable: agentExecutor → agentWorker
    
    Note right of App: Keep ParentJobExecutor unchanged<br/>(line 314 - migrates in ARCH-007)
    
    Dev->>Build: Build application
    Build-->>Dev: ✓ Application compiles successfully
    
    Note over Dev,Build: Phase 3: Add Deprecation Notices
    
    Dev->>OldAgent: Add deprecation comment to agent_executor.go
    Note right of OldAgent: "DEPRECATED: Migrated to<br/>internal/jobs/worker/agent_worker.go"
    
    Dev->>OldProc: Add deprecation comment to processor.go
    Note right of OldProc: "DEPRECATED: Migrated to<br/>internal/jobs/worker/job_processor.go"
    
    Note over OldAgent,OldProc: Files remain functional<br/>for backward compatibility<br/>Will be deleted in ARCH-008
    
    Note over Dev,Build: Phase 4: Validation
    
    Dev->>Build: Run test suite
    Build-->>Dev: ✓ All tests pass
    
    Dev->>App: Start application
    App->>NewProc: Initialize JobProcessor
    App->>NewAgent: Register AgentWorker with JobProcessor
    App-->>Dev: ✓ "Job processor initialized"<br/>✓ "Agent worker registered for job type: agent"
    
    Dev->>App: Trigger agent job via UI
    App->>NewProc: Route job to AgentWorker
    NewProc->>NewAgent: Execute agent job
    NewAgent->>NewAgent: Load document from storage
    NewAgent->>NewAgent: Execute AI agent
    NewAgent->>NewAgent: Update document metadata
    NewAgent->>NewAgent: Publish DocumentSaved event
    NewAgent-->>NewProc: ✓ Job completed successfully
    NewProc-->>App: ✓ Job execution complete
    
    Note over Dev,Build: Migration Complete<br/>2 worker files migrated<br/>Old files deprecated<br/>Backward compatible<br/>ParentJobExecutor remains in processor/

## Proposed File Changes

### internal\jobs\worker\agent_worker.go(NEW)

References: 

- internal\jobs\processor\agent_executor.go(MODIFY)
- internal\jobs\worker\interfaces.go

Create new AgentWorker file by copying from `internal/jobs/processor/agent_executor.go` with the following transformations:

**Package Declaration:**
- Change: `package processor` → `package worker`

**File Header Comment:**
- Update to: "Agent Worker - Processes individual agent jobs from the queue with document loading, AI agent execution, and metadata updates"

**Imports:**
- Keep all imports unchanged:
  - Standard library: `context`, `fmt`, `time`
  - External: `github.com/ternarybob/arbor`
  - Internal: `github.com/ternarybob/quaero/internal/interfaces`, `github.com/ternarybob/quaero/internal/jobs`, `github.com/ternarybob/quaero/internal/models`
- No import changes needed (all use internal packages)

**Struct Rename:**
- Change: `type AgentExecutor struct` → `type AgentWorker struct`
- Keep all 5 fields unchanged:
  - agentService interfaces.AgentService
  - jobMgr *jobs.Manager
  - documentStorage interfaces.DocumentStorage
  - logger arbor.ILogger
  - eventService interfaces.EventService
- Update struct comment: "AgentWorker processes individual agent jobs from the queue, loading documents, executing AI agents, and updating document metadata with results" (already correct in original file)

**Constructor Rename:**
- Change: `func NewAgentExecutor(...)` → `func NewAgentWorker(...)`
- Change return type: `*AgentExecutor` → `*AgentWorker`
- Update struct initialization: `return &AgentExecutor{...}` → `return &AgentWorker{...}`
- Update comment: "NewAgentWorker creates a new agent worker for processing individual agent jobs from the queue"
- Keep all 5 parameters unchanged (same order, same types)

**Method Receivers:**
- Change all method receivers: `func (e *AgentExecutor)` → `func (w *AgentWorker)`
- Rename receiver variable from `e` to `w` for consistency (worker convention)
- Update all references to `e.` → `w.` within all method bodies
- This applies to:
  - GetWorkerType() - interface method (returns "agent")
  - Validate() - interface method (validates job type and config)
  - Execute() - interface method (main workflow)
  - publishAgentJobLog() - private helper (event publishing)

**Interface Methods:**
- GetWorkerType() - Already correct (returns "agent")
- Validate() - Already correct (validates job type and required config fields)
- Execute() - Keep all logic unchanged (5-step workflow: load document, prepare input, execute agent, update metadata, publish event)

**Comments:**
- Update all comments referencing "executor" → "worker"
- Update all comments referencing "AgentExecutor" → "AgentWorker"
- Keep all existing detailed comments (especially in Execute() method)
- Update method comment for Validate: "Validate validates that the job model is compatible with this worker"

**Log Messages:**
- Update log messages: "executor" → "worker" where referring to this component
- Keep all other log messages unchanged (e.g., "Starting agent job execution", "Agent execution completed")
- Keep all structured logging fields unchanged

**Validation:**
- Verify file compiles independently: `go build internal/jobs/worker/agent_worker.go`
- Verify implements worker.JobWorker interface
- Verify all method signatures match interface
- Total lines: ~297 (same as original)
- Verify all 5 dependencies are properly injected via constructor

### internal\jobs\worker\job_processor.go(NEW)

References: 

- internal\jobs\processor\processor.go(MODIFY)
- internal\jobs\worker\interfaces.go

Create new JobProcessor file by copying from `internal/jobs/processor/processor.go` with the following transformations:

**Package Declaration:**
- Change: `package processor` → `package worker`

**File Header Comment:**
- Add: "Job Processor - Routes jobs from the queue to registered workers based on job type"

**Imports:**
- Keep all imports unchanged:
  - Standard library: `context`, `fmt`, `sync`, `time`
  - External: `github.com/ternarybob/arbor`
  - Internal: `github.com/ternarybob/quaero/internal/interfaces`, `github.com/ternarybob/quaero/internal/jobs`, `github.com/ternarybob/quaero/internal/models`, `github.com/ternarybob/quaero/internal/queue`
- No import changes needed

**Struct (Keep Unchanged):**
- Keep: `type JobProcessor struct` (struct name already correct)
- Keep all 7 fields unchanged:
  - queueMgr *queue.Manager
  - jobMgr *jobs.Manager
  - executors map[string]interfaces.JobWorker
  - logger arbor.ILogger
  - ctx context.Context
  - cancel context.CancelFunc
  - wg sync.WaitGroup
  - running bool
  - mu sync.Mutex
- Update struct comment: "JobProcessor is a job-agnostic processor that uses goqite for queue management. It routes jobs to registered workers based on job type." (already correct)

**Constructor (Keep Unchanged):**
- Keep: `func NewJobProcessor(...)` (constructor name already correct)
- Keep return type: `*JobProcessor`
- Keep all initialization logic unchanged
- Update comment: "NewJobProcessor creates a new job processor that routes jobs to registered workers" (clarify purpose)
- Keep all 3 parameters unchanged

**Method Names (Keep Unchanged):**
- Keep: `RegisterExecutor(worker interfaces.JobWorker)` (method name already correct)
- Keep: `Start()`, `Stop()`, `processJobs()`, `processNextJob()`
- All method signatures are already correct

**Method Receivers (Keep Unchanged):**
- Keep all method receivers: `func (jp *JobProcessor)` (receiver variable already correct)
- No changes to receiver variable or references

**Comments:**
- Update RegisterExecutor comment: "RegisterExecutor registers a job worker for a job type. The worker must implement the JobWorker interface." (already correct)
- Update field comment: "executors holds registered job workers keyed by job type" (already correct)
- Update processNextJob comment: Clarify it routes to appropriate worker based on job type
- Keep all other comments unchanged

**Log Messages:**
- Update log messages for consistency:
  - "Job worker registered" (already correct at line 52)
  - "No worker registered for job type" (already correct at line 156)
  - Keep all other log messages unchanged

**Key Insight:**
This file requires minimal changes - only package declaration and file name change. The struct name, constructor name, and method names are already correct. The file already uses JobWorker interface (updated in ARCH-002).

**Validation:**
- Verify file compiles independently: `go build internal/jobs/worker/job_processor.go`
- Verify uses worker.JobWorker interface correctly
- Verify all method signatures are correct
- Total lines: ~244 (same as original)
- Verify routing logic works correctly with registered workers

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\worker\agent_worker.go(NEW)
- internal\jobs\worker\job_processor.go(NEW)
- internal\jobs\processor\parent_job_executor.go

Update app.go to import and use the new worker package for AgentWorker and JobProcessor.

**Import Section Updates (line 22):**

The worker import already exists from ARCH-005 (crawler worker migration). Verify it's present:
```go
import (
    // ... existing imports ...
    "github.com/ternarybob/quaero/internal/jobs/processor"  // OLD - Keep for ParentJobExecutor (migrates in ARCH-007)
    "github.com/ternarybob/quaero/internal/jobs/worker"     // NEW - Already added in ARCH-005
    // ... rest of imports ...
)
```

No import changes needed - worker import already exists.

**Field Declaration Update (line 67):**

Replace:
```go
JobProcessor *processor.JobProcessor
```

With:
```go
JobProcessor *worker.JobProcessor
```

**JobProcessor Initialization Update (line 271):**

Replace:
```go
jobProcessor := processor.NewJobProcessor(queueMgr, jobMgr, a.Logger)
```

With:
```go
jobProcessor := worker.NewJobProcessor(queueMgr, jobMgr, a.Logger)
```

Keep all 3 parameters unchanged (queueMgr, jobMgr, a.Logger).

**AgentWorker Registration Update (line 323):**

Replace:
```go
agentExecutor := processor.NewAgentExecutor(
    a.AgentService,
    jobMgr,
    a.StorageManager.DocumentStorage(),
    a.Logger,
    a.EventService,
)
jobProcessor.RegisterExecutor(agentExecutor)
a.Logger.Info().Msg("Agent worker registered for job type: agent")
```

With:
```go
agentWorker := worker.NewAgentWorker(
    a.AgentService,
    jobMgr,
    a.StorageManager.DocumentStorage(),
    a.Logger,
    a.EventService,
)
jobProcessor.RegisterExecutor(agentWorker)
a.Logger.Info().Msg("Agent worker registered for job type: agent")
```

**Variable Naming:**
- Changed from `agentExecutor` to `agentWorker` for clarity and consistency
- Constructor call: `processor.NewAgentExecutor()` → `worker.NewAgentWorker()`
- All 5 parameters remain in same order (no changes to parameter list)

**Keep Unchanged:**
- ParentJobExecutor initialization (line 314) still uses `processor.NewParentJobExecutor()` - NOT migrating in this phase (migrates to orchestrator/ in ARCH-007)
- CrawlerWorker registration (line 298) already uses `worker.NewCrawlerWorker()` from ARCH-005
- DatabaseMaintenanceExecutor registration (line 335) still uses `executor.NewDatabaseMaintenanceExecutor()` - migrates in ARCH-007
- All other initialization code remains unchanged

**Log Message:**
- Already uses "worker" terminology (line 331) - no changes needed
- Message: "Agent worker registered for job type: agent"

**Validation:**
- Verify application compiles successfully
- Verify JobProcessor is initialized correctly with worker package
- Verify AgentWorker is registered correctly with JobProcessor
- Run application and check startup logs for:
  - "Job processor initialized"
  - "Agent worker registered for job type: agent"
- Verify agent jobs execute correctly via UI or API

### AGENTS.md(MODIFY)

References: 

- docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update AGENTS.md to document the progress of the remaining worker file migration (ARCH-006 completion).

**Section to Update: "Directory Structure (In Transition - ARCH-005)"**

Update the migration status to reflect ARCH-006 completion:

Change the section title from "Directory Structure (In Transition - ARCH-005)" to "Directory Structure (In Transition - ARCH-006)".

Update the worker directory listing:

```markdown
- `internal/jobs/worker/` - Job workers (execution layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `crawler_worker.go` (ARCH-005) - Merged from crawler_executor.go + crawler_executor_auth.go
  - ✅ `agent_worker.go` (ARCH-006)
  - ✅ `job_processor.go` (ARCH-006) - Routes jobs to workers
  - ⏳ `database_maintenance_worker.go` (pending - ARCH-007)
```

Update the old directories listing:

```markdown
**Old Directories (Still Active - Will be removed in ARCH-008):**
- `internal/jobs/executor/` - Old manager implementations (6 remaining files)
- `internal/jobs/processor/` - Old worker implementations (1 remaining file: parent_job_executor.go, migrating to orchestrator/ in ARCH-007)
```

Update the migration progress:

```markdown
**Migration Progress:**
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated (merged crawler_executor.go + crawler_executor_auth.go)
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker.go, job_processor.go) (YOU ARE HERE)
- Phase ARCH-007: ⏳ Parent job orchestrator migration (pending)
```

**Section to Update: "Interfaces"**

Update to reflect that AgentWorker and JobProcessor are now in worker/ package:

```markdown
### Interfaces

**New Architecture (ARCH-003+):**
- `JobManager` interface - `internal/jobs/manager/interfaces.go`
  - Implementations: `CrawlerManager`, `DatabaseMaintenanceManager`, `AgentManager` (ARCH-004)
- `JobWorker` interface - `internal/jobs/worker/interfaces.go`
  - Implementations: `CrawlerWorker` (ARCH-005), `AgentWorker` (ARCH-006)
- `ParentJobOrchestrator` interface - `internal/jobs/orchestrator/interfaces.go`

**Core Components:**
- `JobProcessor` - `internal/jobs/worker/job_processor.go` (ARCH-006)
  - Routes jobs from queue to registered workers
  - Manages worker pool lifecycle (Start/Stop)

**Old Architecture (deprecated, will be removed in ARCH-008):**
- `JobManager` interface - `internal/jobs/executor/interfaces.go` (duplicate)
  - Remaining implementations: `TransformStepExecutor`, `ReindexStepExecutor`, `PlacesSearchStepExecutor`
- `JobWorker` interface - `internal/interfaces/job_executor.go` (duplicate)
  - Remaining implementations: `DatabaseMaintenanceExecutor` (in executor/ directory)
```

**Implementation Notes:**
- Update migration status to show ARCH-006 complete
- Add checkmarks (✅) for agent_worker.go and job_processor.go
- Update remaining file count in processor/ directory (1 file remaining: parent_job_executor.go)
- Clarify which workers are migrated vs remaining in old location
- Add note about JobProcessor being the core routing system

### docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update MANAGER_WORKER_ARCHITECTURE.md to document the completion of ARCH-006 (remaining worker file migration).

**Section to Update: "Current Status (After ARCH-005)"**

Change the section title and content to reflect ARCH-006 completion:

```markdown
### Current Status (After ARCH-006)

**New Directories Created:**
- ✅ `internal/jobs/manager/` - Created with interfaces.go (ARCH-003)
  - ✅ `crawler_manager.go` - Migrated from executor/ (ARCH-004)
  - ✅ `database_maintenance_manager.go` - Migrated from executor/ (ARCH-004)
  - ✅ `agent_manager.go` - Migrated from executor/ (ARCH-004)
- ✅ `internal/jobs/worker/` - Created with interfaces.go (ARCH-003)
  - ✅ `crawler_worker.go` - Merged and migrated from processor/ (ARCH-005)
  - ✅ `agent_worker.go` - Migrated from processor/ (ARCH-006)
  - ✅ `job_processor.go` - Migrated from processor/ (ARCH-006)
- ✅ `internal/jobs/orchestrator/` - Created with interfaces.go (ARCH-003)

**Old Directories (Still Active):**
- `internal/jobs/executor/` - Contains 6 remaining implementation files:
  - `transform_step_executor.go` (pending migration)
  - `reindex_step_executor.go` (pending migration)
  - `places_search_step_executor.go` (pending migration)
  - `job_executor.go` (orchestrator - will be refactored separately)
  - `base_executor.go` (shared utilities - will be refactored separately)
  - `database_maintenance_executor.go` (old worker - will be deleted in ARCH-007)
- `internal/jobs/processor/` - Contains 1 remaining implementation file:
  - `parent_job_executor.go` (migrating to orchestrator/ in ARCH-007)

**Migration Status:**
- Phase ARCH-001: ✅ Documentation created
- Phase ARCH-002: ✅ Interfaces renamed
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ Manager files migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated and merged
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker, job_processor) (YOU ARE HERE)
- Phase ARCH-007: ⏳ Parent job orchestrator migration (pending)
- Phase ARCH-008: ⏳ Database maintenance worker split (pending)
- Phase ARCH-009: ⏳ Import path updates and cleanup (pending)
- Phase ARCH-010: ⏳ End-to-end validation (pending)
```

**Section to Add: "Remaining Worker Files Migrated (ARCH-006)"**

Add a new subsection after "Crawler Worker File Merge (ARCH-005)":

```markdown
### Remaining Worker Files Migrated (ARCH-006)

**Files Moved from processor/ to worker/:**

1. **AgentWorker** (`agent_worker.go`)
   - Old: `internal/jobs/processor/agent_executor.go`
   - New: `internal/jobs/worker/agent_worker.go`
   - Struct: `AgentExecutor` → `AgentWorker`
   - Constructor: `NewAgentExecutor()` → `NewAgentWorker()`
   - Receiver: `func (e *AgentExecutor)` → `func (w *AgentWorker)`
   - Dependencies: AgentService, JobManager, DocumentStorage, Logger, EventService (5 dependencies)
   - Purpose: Processes individual agent jobs from queue, executes AI agents, updates document metadata

2. **JobProcessor** (`job_processor.go`)
   - Old: `internal/jobs/processor/processor.go`
   - New: `internal/jobs/worker/job_processor.go`
   - Struct: `JobProcessor` (unchanged - already correct)
   - Constructor: `NewJobProcessor()` (unchanged - already correct)
   - File rename only: `processor.go` → `job_processor.go`
   - Dependencies: QueueManager, JobManager, Logger (3 dependencies)
   - Purpose: Routes jobs from queue to registered workers based on job type

**Transformations Applied:**

**AgentWorker:**
- **Package**: `processor` → `worker`
- **Struct**: `AgentExecutor` → `AgentWorker`
- **Constructor**: `NewAgentExecutor()` → `NewAgentWorker()`
- **Receiver**: `func (e *AgentExecutor)` → `func (w *AgentWorker)`
- **Interface**: Implements `worker.JobWorker` (Execute, GetWorkerType, Validate)

**JobProcessor:**
- **Package**: `processor` → `worker`
- **File Name**: `processor.go` → `job_processor.go`
- **Struct**: `JobProcessor` (unchanged - already correct)
- **Constructor**: `NewJobProcessor()` (unchanged - already correct)
- **Minimal Changes**: Only package declaration and file name changed

**Key Features Preserved:**

**AgentWorker:**
- **5-Step Workflow**: Load document → Prepare input → Execute agent → Update metadata → Publish event
- **Real-Time Logging**: publishAgentJobLog() for WebSocket streaming
- **Error Handling**: Comprehensive error handling with job status updates
- **Event Publishing**: DocumentSaved event for workflow coordination
- **Structured Logging**: Correlation IDs for parent job aggregation

**JobProcessor:**
- **Worker Routing**: Routes jobs to registered workers based on job type
- **Queue Management**: Polls goqite queue with timeout context
- **Worker Registration**: RegisterExecutor() for dynamic worker registration
- **Lifecycle Management**: Start/Stop methods for graceful shutdown
- **Parent Job Handling**: Special handling for parent jobs (remain in running state)
- **Error Handling**: Job validation, worker lookup, execution error handling

**Import Path Updates:**

- `internal/app/app.go` (lines 67, 271, 323) - Updated to import and use `worker.JobProcessor` and `worker.NewAgentWorker()`
- Variable renamed: `agentExecutor` → `agentWorker`
- Field type updated: `*processor.JobProcessor` → `*worker.JobProcessor`

**Backward Compatibility:**

- Old files remain in `internal/jobs/processor/` with deprecation notices until ARCH-008
- ParentJobExecutor remains in processor/ (migrates to orchestrator/ in ARCH-007)
- Dual import strategy allows gradual transition
- No breaking changes to external APIs or job execution behavior
```

**Implementation Notes:**
- Update migration status to show ARCH-006 complete
- Add detailed documentation of both file migrations
- Document transformations and key features preserved
- Clarify JobProcessor required minimal changes (file rename only)
- Provide context for developers working during transition
- Emphasize that ParentJobExecutor is NOT in scope for this phase

### internal\jobs\processor\agent_executor.go(MODIFY)

References: 

- internal\jobs\worker\agent_worker.go(NEW)

Add deprecation notice to the old agent_executor.go file to indicate it has been migrated.

**Add Deprecation Comment at Top of File (after existing header comment):**

```go
// -----------------------------------------------------------------------
// Agent Executor - Individual agent job execution with document processing
// -----------------------------------------------------------------------

// DEPRECATED: This file has been migrated to internal/jobs/worker/agent_worker.go (ARCH-006).
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/worker and use AgentWorker instead.
//
// Migration Details:
// - Struct renamed: AgentExecutor → AgentWorker
// - Constructor renamed: NewAgentExecutor() → NewAgentWorker()
// - Package changed: processor → worker
// - Receiver variable changed: e → w

package processor
```

**No Other Changes:**
- Keep all existing code unchanged
- File remains functional for backward compatibility
- Will be deleted in ARCH-008 when all imports are updated

**Purpose:**
- Clearly communicate to developers that this file is deprecated
- Provide guidance on where to find the new implementation
- Document the migration timeline (removal in ARCH-008)
- Explain the struct and constructor renames
- Prevent new code from using the old location

### internal\jobs\processor\processor.go(MODIFY)

References: 

- internal\jobs\worker\job_processor.go(NEW)

Add deprecation notice to the old processor.go file to indicate it has been migrated.

**Add Deprecation Comment at Top of File (before package declaration):**

```go
// -----------------------------------------------------------------------
// Job Processor - Routes jobs from queue to registered workers
// -----------------------------------------------------------------------

// DEPRECATED: This file has been migrated to internal/jobs/worker/job_processor.go (ARCH-006).
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/worker and use worker.JobProcessor instead.
//
// Migration Details:
// - File renamed: processor.go → job_processor.go
// - Package changed: processor → worker
// - Struct name unchanged: JobProcessor (already correct)
// - Constructor unchanged: NewJobProcessor() (already correct)

package processor
```

**No Other Changes:**
- Keep all existing code unchanged
- File remains functional for backward compatibility
- Will be deleted in ARCH-008 when all imports are updated

**Purpose:**
- Clearly communicate to developers that this file is deprecated
- Provide guidance on where to find the new implementation
- Document the migration timeline (removal in ARCH-008)
- Explain that only file name and package changed (struct/constructor unchanged)
- Prevent new code from using the old location