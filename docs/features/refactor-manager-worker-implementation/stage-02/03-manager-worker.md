I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The codebase has completed ARCH-002 (interface renames) and is ready for directory structure creation:

1. **Existing Structure:**
   - `internal/jobs/executor/` - Contains JobManager interface + 9 implementation files
   - `internal/jobs/processor/` - Contains 5 files including JobProcessor and workers
   - `internal/interfaces/job_executor.go` - Contains JobWorker interface (shared location)

2. **Interface Status:**
   - `JobManager` interface: Fully renamed with `CreateParentJob()` and `GetManagerType()` methods
   - `JobWorker` interface: Fully renamed with `GetWorkerType()` method
   - Both interfaces have updated comments reflecting manager/worker pattern

3. **Implementation Files (NOT moving in this phase):**
   - **Managers (6)**: crawler_step_executor.go, agent_step_executor.go, database_maintenance_step_executor.go, transform_step_executor.go, reindex_step_executor.go, places_search_step_executor.go
   - **Workers (3)**: crawler_executor.go, agent_executor.go, database_maintenance_executor.go
   - **Orchestrator (1)**: parent_job_executor.go
   - **Routing (1)**: processor.go (JobProcessor)
   - **Other (3)**: job_executor.go (orchestrator), base_executor.go, crawler_executor_auth.go

4. **Key Insight - Orchestrator Needs Interface:**
   - `ParentJobExecutor` in `processor/parent_job_executor.go` currently has no interface
   - This phase should create `ParentJobOrchestrator` interface for consistency
   - Interface will define monitoring and progress aggregation methods

**Architectural Clarity:**

The new structure will clearly separate three concerns:
- **Managers** (`internal/jobs/manager/`) - Create parent jobs, enqueue children, orchestrate workflows
- **Workers** (`internal/jobs/worker/`) - Execute individual jobs from queue, perform actual work
- **Orchestrator** (`internal/jobs/orchestrator/`) - Monitor parent jobs, aggregate child progress, publish events

**Risk Assessment:**

- **Zero Risk**: Creating empty directories has no impact on existing code
- **Low Risk**: Copying interface files creates temporary duplication (intentional for migration)
- **Low Risk**: New packages won't be imported until ARCH-004, so no compilation impact
- **Medium Risk**: Orchestrator interface is new (not just a copy) - needs careful design

**Success Criteria:**

1. Three new directories created: `manager/`, `worker/`, `orchestrator/`
2. Interface files copied and updated with correct package names
3. Orchestrator interface created with appropriate methods
4. All new files compile independently
5. Existing code remains unchanged and functional
6. No broken imports or compilation errors

### Approach

**Parallel Directory Creation with Interface Duplication Strategy**

This phase establishes the new directory structure alongside the existing one, enabling a smooth transition without breaking existing code. The approach follows these principles:

1. **Non-Breaking Changes**: Create new directories without touching existing files
2. **Interface Duplication**: Copy interface files to new locations (temporary duplication for transition)
3. **Package Isolation**: Each new directory gets its own package with proper interfaces
4. **Backward Compatibility**: Keep original files intact for subsequent phases to migrate implementations
5. **Clear Separation**: Establish three distinct layers (manager, worker, orchestrator)

**Why This Approach:**

- **Risk Mitigation**: No existing code is modified, eliminating risk of breaking changes
- **Incremental Migration**: Subsequent phases can migrate implementations one at a time
- **Compile-Time Safety**: New packages can be tested independently before migration
- **Clear Intent**: Directory structure immediately communicates the new architecture
- **Rollback Safety**: If issues arise, simply delete new directories (no code changes to revert)

**Directory Structure After This Phase:**

```
internal/jobs/
├── executor/              # OLD - Will be deleted in ARCH-008
│   ├── interfaces.go      # JobManager interface (original)
│   └── [9 implementation files]
├── processor/             # OLD - Will be deleted in ARCH-008
│   ├── processor.go       # JobProcessor (routes to workers)
│   └── [4 implementation files]
├── manager/               # NEW - Managers orchestrate workflows
│   └── interfaces.go      # JobManager interface (copy)
├── worker/                # NEW - Workers execute jobs
│   └── interfaces.go      # JobWorker interface (copy)
└── orchestrator/          # NEW - Orchestrator monitors parent jobs
    └── interfaces.go      # ParentJobOrchestrator interface (new)
```

**Key Decisions:**

1. **Interface Duplication**: Temporary duplication allows gradual migration without breaking imports
2. **Orchestrator Interface**: Create new interface for ParentJobOrchestrator (currently has no interface)
3. **Package Names**: Use directory names as package names (manager, worker, orchestrator)
4. **No Implementation Moves**: Defer all implementation file moves to subsequent phases (ARCH-004 through ARCH-006)

**Validation Strategy:**

- Verify directories are created successfully
- Verify interface files compile independently
- Verify no existing code is broken
- Verify new packages can be imported (even if unused yet)

### Reasoning

I systematically explored the codebase to understand the current structure:

1. **Listed root directory** - Identified `internal/` as the main source directory
2. **Explored internal/jobs/** - Found existing `executor/` and `processor/` directories with 14 implementation files
3. **Read interface files** - Examined `internal/jobs/executor/interfaces.go` (JobManager) and `internal/interfaces/job_executor.go` (JobWorker)
4. **Analyzed structure** - Understood that:
   - `executor/` contains 9 files (6 managers + 2 database maintenance + 1 orchestrator)
   - `processor/` contains 5 files (3 workers + 1 processor + 1 parent job executor)
   - Interfaces have been successfully renamed in ARCH-002
5. **Reviewed subsequent phases** - Confirmed that implementation file moves happen in ARCH-004 through ARCH-006

This exploration confirmed that the task is straightforward: create new directories and copy interface files without touching implementations.

## Proposed File Changes

### internal\jobs\manager(NEW)

Create new directory for job managers (orchestration layer).

This directory will contain:
- `interfaces.go` - JobManager interface (copied from executor/interfaces.go)
- Manager implementations (migrated in ARCH-004)

**Purpose:**
Managers create parent jobs, enqueue child jobs to the queue, and orchestrate workflows. They are responsible for:
- Creating parent job records in the database
- Enqueuing child jobs to the goqite queue
- Configuring job parameters and metadata
- Returning parent job ID for tracking

**Examples of managers:**
- CrawlerManager - Orchestrates URL crawling workflows
- AgentManager - Orchestrates AI document processing workflows
- DatabaseMaintenanceManager - Orchestrates database optimization workflows

**Note:** Implementation files will be moved here in ARCH-004. This phase only creates the directory structure.

### internal\jobs\worker(NEW)

Create new directory for job workers (execution layer).

This directory will contain:
- `interfaces.go` - JobWorker interface (copied from internal/interfaces/job_executor.go)
- Worker implementations (migrated in ARCH-005 and ARCH-006)
- `job_processor.go` - JobProcessor that routes jobs to workers (migrated in ARCH-006)

**Purpose:**
Workers execute individual jobs from the queue and perform the actual work. They are responsible for:
- Processing single jobs received from the queue
- Executing the actual work (crawl URL, process document, run maintenance)
- Updating job status and progress
- Spawning child jobs if needed (e.g., discovered links)
- Logging execution details

**Examples of workers:**
- CrawlerWorker - Processes individual URL crawl jobs
- AgentWorker - Processes individual AI document processing jobs
- DatabaseMaintenanceWorker - Processes individual database operations

**Note:** Implementation files will be moved here in ARCH-005 and ARCH-006. This phase only creates the directory structure.

### internal\jobs\orchestrator(NEW)

Create new directory for parent job orchestrator (monitoring layer).

This directory will contain:
- `interfaces.go` - ParentJobOrchestrator interface (new, created in this phase)
- `parent_job_orchestrator.go` - Implementation (migrated from processor/parent_job_executor.go in ARCH-006)

**Purpose:**
The orchestrator monitors parent job progress and aggregates child job statistics. It is responsible for:
- Monitoring parent job lifecycle in background goroutines
- Polling child job statistics from the database
- Aggregating progress (total, completed, failed counts)
- Publishing progress events for real-time UI updates
- Detecting parent job completion (all children finished)
- Updating parent job status (running → completed/failed)

**Key Distinction:**
- **Managers** create parent jobs and enqueue children (orchestration)
- **Workers** execute individual jobs from queue (execution)
- **Orchestrator** monitors parent jobs and aggregates progress (monitoring)

**Note:** The ParentJobExecutor implementation will be moved here in ARCH-006 and renamed to ParentJobOrchestrator. This phase creates the directory and interface.

### internal\jobs\manager\interfaces.go(NEW)

References: 

- internal\jobs\executor\interfaces.go

Copy JobManager interface from `internal/jobs/executor/interfaces.go` to new manager package.

**Content to Copy:**
- Package declaration: `package manager` (change from `package executor`)
- Import statements: Keep `context` and `models` imports
- JobManager interface: Copy entire interface definition unchanged
- Comments: Copy all interface and method comments

**Interface Definition:**
```go
type JobManager interface {
    CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (jobID string, err error)
    GetManagerType() string
}
```

**Purpose:**
This creates a clean interface definition in the new manager package. The original file in `executor/interfaces.go` will remain until ARCH-008 when old directories are deleted.

**Temporary Duplication:**
Yes, this creates temporary duplication of the JobManager interface. This is intentional:
- Existing implementations in `executor/` still import from `executor/interfaces.go`
- New implementations (after ARCH-004) will import from `manager/interfaces.go`
- Duplication is resolved in ARCH-008 when old directories are deleted

**Validation:**
- Verify package name is `manager`
- Verify interface compiles independently
- Verify import paths are correct
- Verify comments are preserved

### internal\jobs\worker\interfaces.go(NEW)

References: 

- internal\interfaces\job_executor.go

Copy JobWorker interface from `internal/interfaces/job_executor.go` to new worker package.

**Content to Copy:**
- Package declaration: `package worker` (change from `package interfaces`)
- Import statements: Keep `context` and `models` imports
- JobWorker interface: Copy entire interface definition unchanged
- JobSpawner interface: Copy entire interface definition unchanged
- Comments: Copy all interface and method comments

**Interface Definitions:**
```go
type JobWorker interface {
    Execute(ctx context.Context, job *models.JobModel) error
    GetWorkerType() string
    Validate(job *models.JobModel) error
}

type JobSpawner interface {
    SpawnChildJob(ctx context.Context, parentJob *models.JobModel, childType, childName string, config map[string]interface{}) error
}
```

**Purpose:**
This creates a clean interface definition in the new worker package. The original file in `internal/interfaces/job_executor.go` will remain until ARCH-008 when it's deleted.

**Temporary Duplication:**
Yes, this creates temporary duplication of the JobWorker interface. This is intentional:
- Existing implementations in `processor/` still import from `internal/interfaces`
- New implementations (after ARCH-005) will import from `worker/interfaces.go`
- Duplication is resolved in ARCH-008 when old interface file is deleted

**JobSpawner Interface:**
Include the JobSpawner interface as well - it's an optional interface for workers that spawn child jobs (e.g., CrawlerWorker spawns jobs for discovered links).

**Validation:**
- Verify package name is `worker`
- Verify both interfaces compile independently
- Verify import paths are correct
- Verify comments are preserved

### internal\jobs\orchestrator\interfaces.go(NEW)

References: 

- internal\jobs\processor\parent_job_executor.go

Create new ParentJobOrchestrator interface for the orchestrator package.

**Purpose:**
Define the interface for parent job monitoring and progress aggregation. Currently, `ParentJobExecutor` in `processor/parent_job_executor.go` has no interface - this creates one for architectural consistency.

**Interface Definition:**

```go
package orchestrator

import (
    "context"
    "github.com/ternarybob/quaero/internal/models"
)

// ParentJobOrchestrator monitors parent job progress and aggregates child job statistics.
// It runs in background goroutines (not via queue) and publishes real-time progress events.
type ParentJobOrchestrator interface {
    // StartMonitoring begins monitoring a parent job in a background goroutine.
    // Polls child job statistics periodically and publishes progress events.
    // Automatically stops when all child jobs complete or parent job is cancelled.
    StartMonitoring(ctx context.Context, parentJobID string) error
    
    // StopMonitoring stops monitoring a specific parent job.
    // Used for cleanup or when parent job is cancelled.
    StopMonitoring(parentJobID string) error
    
    // GetMonitoringStatus returns whether a parent job is currently being monitored.
    GetMonitoringStatus(parentJobID string) bool
}
```

**Design Rationale:**

1. **StartMonitoring**: Initiates background monitoring for a parent job
   - Called by managers after creating parent job
   - Runs in separate goroutine (non-blocking)
   - Polls database for child job statistics
   - Publishes progress events via EventService

2. **StopMonitoring**: Gracefully stops monitoring
   - Called when parent job completes or is cancelled
   - Cleans up goroutines and resources
   - Prevents memory leaks from long-running monitors

3. **GetMonitoringStatus**: Query monitoring state
   - Useful for debugging and health checks
   - Allows checking if monitoring is active

**Key Distinction from Manager/Worker:**
- **Managers**: Create jobs and enqueue (orchestration)
- **Workers**: Execute jobs from queue (execution)
- **Orchestrator**: Monitor parent jobs and aggregate progress (monitoring)

**Implementation Note:**
The actual implementation in `processor/parent_job_executor.go` will be migrated to `orchestrator/parent_job_orchestrator.go` in ARCH-006 and updated to implement this interface.

**Validation:**
- Verify package name is `orchestrator`
- Verify interface compiles independently
- Verify method signatures are clear and complete
- Verify comments explain purpose and usage

### AGENTS.md(MODIFY)

References: 

- docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update AGENTS.md to document the new directory structure created in this phase.

**Section to Update: "Job System Architecture"**

Add a note explaining the transition:

```markdown
### Directory Structure (In Transition)

Quaero is migrating to a Manager/Worker/Orchestrator architecture. The new structure is:

- `internal/jobs/manager/` - Job managers (orchestration layer) - **NEW in ARCH-003**
- `internal/jobs/worker/` - Job workers (execution layer) - **NEW in ARCH-003**
- `internal/jobs/orchestrator/` - Parent job orchestrator (monitoring layer) - **NEW in ARCH-003**
- `internal/jobs/executor/` - Old manager implementations (will be deleted in ARCH-008)
- `internal/jobs/processor/` - Old worker implementations (will be deleted in ARCH-008)

**Current Status:** Directory structure created, implementation files will be migrated in phases ARCH-004 through ARCH-006.

See `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` for complete documentation.
```

**Section to Update: "Job Execution Flow"**

Add a note about the new interfaces:

```markdown
### Interfaces

**New Architecture (ARCH-003+):**
- `JobManager` interface - `internal/jobs/manager/interfaces.go`
- `JobWorker` interface - `internal/jobs/worker/interfaces.go`
- `ParentJobOrchestrator` interface - `internal/jobs/orchestrator/interfaces.go`

**Old Architecture (deprecated, will be removed in ARCH-008):**
- `JobManager` interface - `internal/jobs/executor/interfaces.go` (duplicate)
- `JobWorker` interface - `internal/interfaces/job_executor.go` (duplicate)
```

**Implementation Notes:**
- Add notes in relevant sections explaining the transition
- Keep existing documentation intact (implementations haven't moved yet)
- Add forward references to upcoming phases
- Link to MANAGER_WORKER_ARCHITECTURE.md for details

**Placement:**
Add these notes near the top of the "Job System Architecture" section so developers understand the transition state.

### docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update MANAGER_WORKER_ARCHITECTURE.md to document the directory structure created in this phase.

**Section to Update: "File Structure Changes"**

Update the "Target Structure" section to reflect that directories are now created:

```markdown
### Current Status (After ARCH-003)

**New Directories Created:**
- ✅ `internal/jobs/manager/` - Created with interfaces.go
- ✅ `internal/jobs/worker/` - Created with interfaces.go
- ✅ `internal/jobs/orchestrator/` - Created with interfaces.go

**Old Directories (Still Active):**
- `internal/jobs/executor/` - Contains 9 implementation files (will be migrated in ARCH-004)
- `internal/jobs/processor/` - Contains 5 implementation files (will be migrated in ARCH-005/ARCH-006)

**Migration Status:**
- Phase ARCH-001: ✅ Documentation created
- Phase ARCH-002: ✅ Interfaces renamed
- Phase ARCH-003: ✅ Directory structure created (YOU ARE HERE)
- Phase ARCH-004: ⏳ Manager files migration (pending)
- Phase ARCH-005: ⏳ Crawler worker migration (pending)
- Phase ARCH-006: ⏳ Remaining worker files migration (pending)
- Phase ARCH-007: ⏳ Parent job orchestrator migration (pending)
- Phase ARCH-008: ⏳ Database maintenance migration (pending)
- Phase ARCH-009: ⏳ Import path updates and cleanup (pending)
- Phase ARCH-010: ⏳ End-to-end validation (pending)
```

**Section to Add: "Interface Duplication (Temporary)"**

Add a new section explaining the temporary duplication:

```markdown
### Interface Duplication (Temporary)

During the migration (ARCH-003 through ARCH-008), interfaces are temporarily duplicated:

**JobManager Interface:**
- Original: `internal/jobs/executor/interfaces.go` (used by old implementations)
- New: `internal/jobs/manager/interfaces.go` (used by new implementations)
- Resolution: Original deleted in ARCH-008

**JobWorker Interface:**
- Original: `internal/interfaces/job_executor.go` (used by old implementations)
- New: `internal/jobs/worker/interfaces.go` (used by new implementations)
- Resolution: Original deleted in ARCH-008

**ParentJobOrchestrator Interface:**
- New: `internal/jobs/orchestrator/interfaces.go` (created in ARCH-003)
- No duplication - this is a new interface (ParentJobExecutor had no interface before)

This duplication is intentional and allows gradual migration without breaking existing code.
```

**Implementation Notes:**
- Update migration status to show ARCH-003 complete
- Add clear indicators of what's done vs pending
- Explain temporary duplication strategy
- Provide context for developers working during transition