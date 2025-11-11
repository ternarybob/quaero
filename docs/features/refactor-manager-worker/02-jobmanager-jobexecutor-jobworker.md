I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Architecture Analysis:**

The codebase has two distinct interface patterns that are confusingly named:

1. **StepExecutor Interface** (`internal/jobs/executor/interfaces.go`):
   - Purpose: Orchestrates job definition steps, creates parent jobs, enqueues child jobs
   - Method: `ExecuteStep()` - Creates parent job and spawns children
   - Method: `GetStepType()` - Returns action type (e.g., "crawl", "agent")
   - **6 Implementations**: CrawlerStepExecutor, AgentStepExecutor, DatabaseMaintenanceStepExecutor, TransformStepExecutor, ReindexStepExecutor, PlacesSearchStepExecutor
   - **Used by**: JobExecutor orchestrator (routes steps to appropriate executors)
   - **Registered in**: `app.go` via `JobExecutor.RegisterStepExecutor()`

2. **JobExecutor Interface** (`internal/interfaces/job_executor.go`):
   - Purpose: Executes individual jobs from queue (worker pattern)
   - Method: `Execute()` - Processes a single job
   - Method: `GetJobType()` - Returns job type (e.g., "crawler_url", "agent")
   - Method: `Validate()` - Validates job model
   - **3 Implementations**: CrawlerExecutor, AgentExecutor, DatabaseMaintenanceExecutor
   - **Used by**: JobProcessor (routes queue jobs to appropriate executors)
   - **Registered in**: `app.go` via `JobProcessor.RegisterExecutor()`

**Key Confusion Points:**

1. Both interfaces use "executor" terminology but serve different purposes
2. `JobExecutor` struct in `internal/jobs/executor/job_executor.go` is NOT implementing the `JobExecutor` interface - it's an orchestrator that uses `StepExecutor` implementations
3. The naming doesn't reflect the Manager/Worker architectural pattern

**Scope of Changes:**

- **2 interface files** to rename
- **6 StepExecutor implementations** to update (method signatures + struct comments)
- **3 JobExecutor implementations** to update (method signatures + struct comments)
- **2 registration points** in `app.go` (RegisterStepExecutor, RegisterExecutor)
- **2 routing systems** (JobExecutor orchestrator, JobProcessor)
- **1 test file** with comment reference to ParentJobExecutor
- **Log messages** throughout for consistency

**Risk Assessment:**

- **Low Risk**: Interface method renames (compile-time safety)
- **Low Risk**: Implementation updates (straightforward mechanical changes)
- **Medium Risk**: Ensuring all log messages and comments are updated
- **Low Risk**: Test impact (minimal - only comment references found)

**Success Criteria:**

1. All code compiles without errors
2. All tests pass (run full test suite)
3. Interface names reflect Manager/Worker pattern
4. Method names clearly indicate purpose (CreateParentJob vs Execute)
5. Log messages use consistent terminology
6. No functional regressions

### Approach

**Phased Rename Strategy with Compile-Time Validation**

This phase focuses on renaming interfaces and updating all implementations while keeping files in their current locations. The subsequent phase (ARCH-003) will handle directory restructuring.

**Key Principles:**

1. **Interface-First Approach**: Rename interfaces first, then update implementations
2. **Compile-Time Safety**: Go compiler will catch all missed references
3. **Atomic Changes**: Each interface rename is independent and can be validated separately
4. **Preserve Behavior**: No functional changes, only naming improvements
5. **Documentation Updates**: Update comments and log messages for consistency

**Rename Sequence:**

1. **StepExecutor → JobManager** (6 implementations)
   - Rename interface in `internal/jobs/executor/interfaces.go`
   - Update method: `ExecuteStep()` → `CreateParentJob()`
   - Update method: `GetStepType()` → `GetManagerType()`
   - Update all 6 implementations
   - Update JobExecutor orchestrator (uses map of JobManagers)
   - Update registration in `app.go`

2. **JobExecutor → JobWorker** (3 implementations)
   - Rename interface in `internal/interfaces/job_executor.go`
   - Update method: `GetJobType()` → `GetWorkerType()`
   - Keep `Execute()` and `Validate()` method names (already clear)
   - Update all 3 implementations
   - Update JobProcessor (uses map of JobWorkers)
   - Update registration in `app.go`

3. **Validation & Cleanup**
   - Run full test suite
   - Update log messages for consistency
   - Update comments and documentation
   - Verify no references to old terminology remain

**Why This Approach:**

- Minimizes risk by keeping files in current locations
- Leverages Go's compile-time type checking
- Each step is independently testable
- Clear separation between interface rename and directory restructuring
- Easier to review and validate changes

### Reasoning

I explored the codebase systematically to understand the interface architecture:

1. **Read interface definitions** - Examined `StepExecutor` and `JobExecutor` interfaces to understand their contracts
2. **Found all implementations** - Used grep to locate all structs implementing these interfaces (6 StepExecutor + 3 JobExecutor implementations)
3. **Traced registration flow** - Analyzed `app.go` to understand how interfaces are registered and used
4. **Examined routing systems** - Reviewed `JobExecutor` orchestrator and `JobProcessor` to understand how they route to implementations
5. **Checked test impact** - Searched test files for interface references (minimal impact found)
6. **Analyzed dependencies** - Understood the relationship between JobExecutor orchestrator (uses StepExecutor) and JobProcessor (uses JobExecutor interface)

This comprehensive exploration revealed the architectural confusion and provided a clear map of all files requiring updates.

## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant Compiler as Go Compiler
    participant Tests as Test Suite
    
    Note over Dev: Phase 1: Rename StepExecutor → JobManager
    
    Dev->>Dev: 1. Rename interface in interfaces.go
    Dev->>Dev: 2. Rename methods: ExecuteStep → CreateParentJob
    Dev->>Dev: 3. Update 6 implementations
    Dev->>Dev: 4. Update JobExecutor orchestrator
    Dev->>Dev: 5. Update app.go registration
    
    Dev->>Compiler: Build application
    alt Compilation Success
        Compiler-->>Dev: ✓ All references updated
        Dev->>Tests: Run test suite
        Tests-->>Dev: ✓ All tests pass
    else Compilation Error
        Compiler-->>Dev: ✗ Missing references found
        Dev->>Dev: Fix remaining references
        Dev->>Compiler: Rebuild
    end
    
    Note over Dev: Phase 2: Rename JobExecutor → JobWorker
    
    Dev->>Dev: 1. Rename interface in job_executor.go
    Dev->>Dev: 2. Rename method: GetJobType → GetWorkerType
    Dev->>Dev: 3. Update 3 implementations
    Dev->>Dev: 4. Update JobProcessor
    Dev->>Dev: 5. Update app.go registration
    
    Dev->>Compiler: Build application
    alt Compilation Success
        Compiler-->>Dev: ✓ All references updated
        Dev->>Tests: Run full test suite
        Tests-->>Dev: ✓ All tests pass
    else Compilation Error
        Compiler-->>Dev: ✗ Missing references found
        Dev->>Dev: Fix remaining references
        Dev->>Compiler: Rebuild
    end
    
    Note over Dev: Phase 3: Validation & Cleanup
    
    Dev->>Dev: Update log messages
    Dev->>Dev: Update comments
    Dev->>Tests: Run full test suite
    Tests-->>Dev: ✓ All tests pass
    
    Dev->>Dev: Verify no old terminology remains
    Dev-->>Dev: ✓ Interface rename complete

## Proposed File Changes

### internal\jobs\executor\interfaces.go(MODIFY)

Rename `StepExecutor` interface to `JobManager` to reflect its role as an orchestrator that creates parent jobs and manages child job execution.

**Interface Rename:**
- `type StepExecutor interface` → `type JobManager interface`

**Method Renames:**
- `ExecuteStep(ctx, step, jobDef, parentJobID) (jobID, error)` → `CreateParentJob(ctx, step, jobDef, parentJobID) (jobID, error)`
- `GetStepType() string` → `GetManagerType() string`

**Comment Updates:**
- Update interface comment to explain manager responsibilities: "JobManager creates parent jobs, enqueues child jobs to the queue, and manages job orchestration for a specific action type"
- Update method comments to clarify: "CreateParentJob creates a parent job record, enqueues child jobs, and returns the parent job ID for tracking"
- Update GetManagerType comment: "GetManagerType returns the action type this manager handles (e.g., 'crawl', 'agent', 'database_maintenance')"

**Rationale:**
- "JobManager" clearly indicates orchestration responsibility
- "CreateParentJob" is more descriptive than "ExecuteStep" (which sounds like execution, not orchestration)
- "GetManagerType" aligns with new interface name and clarifies purpose

### internal\interfaces\job_executor.go(MODIFY)

Rename `JobExecutor` interface to `JobWorker` to reflect its role as a worker that executes individual jobs from the queue.

**Interface Rename:**
- `type JobExecutor interface` → `type JobWorker interface`

**Method Renames:**
- `GetJobType() string` → `GetWorkerType() string`
- Keep `Execute(ctx, job) error` unchanged (already clear)
- Keep `Validate(job) error` unchanged (already clear)

**Comment Updates:**
- Update interface comment: "JobWorker defines the interface that all job workers must implement. The queue engine uses this interface to execute jobs in a type-agnostic manner. Workers process individual jobs from the queue and perform the actual work."
- Update GetWorkerType comment: "GetWorkerType returns the job type this worker handles. Examples: 'database_maintenance', 'crawler_url', 'agent'"
- Update Execute comment: "Execute processes a single job from the queue. Returns error if execution fails. Worker is responsible for updating job status and logging progress."
- Update Validate comment: "Validate validates that the job model is compatible with this worker. Returns error if the job model is invalid for this worker."

**JobSpawner Interface:**
- Keep `JobSpawner` interface unchanged (optional interface for workers that spawn child jobs)
- Update comment to clarify: "JobSpawner defines the interface for workers that can spawn child jobs. This is optional - not all job workers need to implement this."

**Rationale:**
- "JobWorker" clearly indicates execution responsibility (not orchestration)
- "GetWorkerType" aligns with new interface name
- Execute and Validate method names are already clear and don't need changes

### internal\jobs\executor\crawler_step_executor.go(MODIFY)

References: 

- internal\jobs\executor\interfaces.go(MODIFY)

Update `CrawlerStepExecutor` to implement the renamed `JobManager` interface.

**Struct Comment Update:**
- Change: "CrawlerStepExecutor executes 'crawl' action steps" → "CrawlerManager creates parent crawler jobs and orchestrates URL crawling workflows"

**Method Signature Updates:**
- `func (e *CrawlerStepExecutor) ExecuteStep(...)` → `func (e *CrawlerStepExecutor) CreateParentJob(...)`
- `func (e *CrawlerStepExecutor) GetStepType()` → `func (e *CrawlerStepExecutor) GetManagerType()`

**Method Comment Updates:**
- CreateParentJob: "CreateParentJob creates a parent crawler job and triggers the crawler service to start crawling. The crawler service will create child jobs for each URL discovered."
- GetManagerType: "GetManagerType returns 'crawl' - the action type this manager handles"

**Log Message Updates:**
- Update any log messages that reference "step executor" to "manager"
- Update any log messages that reference "executing step" to "creating parent job"

**Implementation Notes:**
- No functional changes to the logic
- Only rename method signatures and update comments
- Ensure interface compliance with new `JobManager` interface

**Note:** Struct name remains `CrawlerStepExecutor` in this phase - will be renamed to `CrawlerManager` in ARCH-004 when files are moved to `internal/jobs/manager/`

### internal\jobs\executor\agent_step_executor.go(MODIFY)

References: 

- internal\jobs\executor\interfaces.go(MODIFY)

Update `AgentStepExecutor` to implement the renamed `JobManager` interface.

**Struct Comment Update:**
- Change: "AgentStepExecutor executes 'agent' action steps" → "AgentManager creates parent agent jobs and orchestrates AI-powered document processing workflows"

**Method Signature Updates:**
- `func (e *AgentStepExecutor) ExecuteStep(...)` → `func (e *AgentStepExecutor) CreateParentJob(...)`
- `func (e *AgentStepExecutor) GetStepType()` → `func (e *AgentStepExecutor) GetManagerType()`

**Method Comment Updates:**
- CreateParentJob: "CreateParentJob creates a parent agent job, queries documents matching the filter, and enqueues individual agent jobs for each document. Returns the parent job ID for tracking."
- GetManagerType: "GetManagerType returns 'agent' - the action type this manager handles"

**Log Message Updates:**
- Update log messages: "step executor" → "manager"
- Update log messages: "executing step" → "creating parent job"
- Update log messages: "step execution" → "job orchestration"

**Implementation Notes:**
- No functional changes to the logic
- Only rename method signatures and update comments
- Ensure interface compliance with new `JobManager` interface

**Note:** Struct name remains `AgentStepExecutor` in this phase - will be renamed to `AgentManager` in ARCH-004

### internal\jobs\executor\database_maintenance_step_executor.go(MODIFY)

References: 

- internal\jobs\executor\interfaces.go(MODIFY)

Update `DatabaseMaintenanceStepExecutor` to implement the renamed `JobManager` interface.

**Struct Comment Update:**
- Change: "DatabaseMaintenanceStepExecutor handles 'database_maintenance' action steps. It creates a database_maintenance job and enqueues it to the queue" → "DatabaseMaintenanceManager creates parent database maintenance jobs and orchestrates database optimization workflows (VACUUM, ANALYZE, REINDEX, OPTIMIZE)"

**Method Signature Updates:**
- `func (e *DatabaseMaintenanceStepExecutor) ExecuteStep(...)` → `func (e *DatabaseMaintenanceStepExecutor) CreateParentJob(...)`
- `func (e *DatabaseMaintenanceStepExecutor) GetStepType()` → `func (e *DatabaseMaintenanceStepExecutor) GetManagerType()`

**Method Comment Updates:**
- CreateParentJob: "CreateParentJob creates a parent database maintenance job and enqueues it to the queue for processing. The job will execute database optimization operations based on the configuration."
- GetManagerType: "GetManagerType returns 'database_maintenance' - the action type this manager handles"

**Log Message Updates:**
- Update log messages: "step executor" → "manager"
- Update log messages: "executing step" → "creating parent job"
- Update log messages: "step execution" → "job orchestration"

**Implementation Notes:**
- No functional changes to the logic
- Only rename method signatures and update comments
- Ensure interface compliance with new `JobManager` interface

**Note:** Struct name remains `DatabaseMaintenanceStepExecutor` in this phase - will be renamed to `DatabaseMaintenanceManager` in ARCH-004

### internal\jobs\executor\transform_step_executor.go(MODIFY)

References: 

- internal\jobs\executor\interfaces.go(MODIFY)

Update `TransformStepExecutor` to implement the renamed `JobManager` interface.

**Struct Comment Update:**
- Change: "TransformStepExecutor executes transform steps in job definitions. Transforms HTML content to markdown using the transform service" → "TransformManager orchestrates document transformation workflows, converting HTML content to markdown format"

**Method Signature Updates:**
- `func (e *TransformStepExecutor) ExecuteStep(...)` → `func (e *TransformStepExecutor) CreateParentJob(...)`
- `func (e *TransformStepExecutor) GetStepType()` → `func (e *TransformStepExecutor) GetManagerType()`

**Method Comment Updates:**
- CreateParentJob: "CreateParentJob executes a transform operation for the given job definition. This is a synchronous operation that directly transforms HTML to markdown. Returns a placeholder job ID since transforms don't create async jobs."
- GetManagerType: "GetManagerType returns 'transform' - the action type this manager handles"

**Log Message Updates:**
- Update log messages: "step executor" → "manager"
- Update log messages: "executing step" → "orchestrating transformation"

**Implementation Notes:**
- No functional changes to the logic
- Only rename method signatures and update comments
- Note: This manager performs synchronous operations (no child jobs)
- Ensure interface compliance with new `JobManager` interface

**Note:** Struct name remains `TransformStepExecutor` in this phase - will be renamed to `TransformManager` in ARCH-004

### internal\jobs\executor\reindex_step_executor.go(MODIFY)

References: 

- internal\jobs\executor\interfaces.go(MODIFY)

Update `ReindexStepExecutor` to implement the renamed `JobManager` interface.

**Struct Comment Update:**
- Change: "ReindexStepExecutor handles 'reindex' action steps. It rebuilds the FTS5 full-text search index for optimal search performance" → "ReindexManager orchestrates FTS5 full-text search index rebuilding workflows for optimal search performance"

**Method Signature Updates:**
- `func (e *ReindexStepExecutor) ExecuteStep(...)` → `func (e *ReindexStepExecutor) CreateParentJob(...)`
- `func (e *ReindexStepExecutor) GetStepType()` → `func (e *ReindexStepExecutor) GetManagerType()`

**Method Comment Updates:**
- CreateParentJob: "CreateParentJob executes a reindex operation to rebuild the FTS5 full-text search index. This is a synchronous operation. Returns a placeholder job ID since reindex doesn't create async jobs."
- GetManagerType: "GetManagerType returns 'reindex' - the action type this manager handles"

**Log Message Updates:**
- Update log messages: "step executor" → "manager"
- Update log messages: "executing step" → "orchestrating reindex"

**Implementation Notes:**
- No functional changes to the logic
- Only rename method signatures and update comments
- Note: This manager performs synchronous operations (no child jobs)
- Ensure interface compliance with new `JobManager` interface

**Note:** Struct name remains `ReindexStepExecutor` in this phase - will be renamed to `ReindexManager` in ARCH-004

### internal\jobs\executor\places_search_step_executor.go(MODIFY)

References: 

- internal\jobs\executor\interfaces.go(MODIFY)

Update `PlacesSearchStepExecutor` to implement the renamed `JobManager` interface.

**Struct Comment Update:**
- Change: "PlacesSearchStepExecutor executes 'places_search' action steps" → "PlacesSearchManager orchestrates Google Places API search workflows and document creation"

**Method Signature Updates:**
- `func (e *PlacesSearchStepExecutor) ExecuteStep(...)` → `func (e *PlacesSearchStepExecutor) CreateParentJob(...)`
- `func (e *PlacesSearchStepExecutor) GetStepType()` → `func (e *PlacesSearchStepExecutor) GetManagerType()`

**Method Comment Updates:**
- CreateParentJob: "CreateParentJob executes a places search operation using the Google Places API. Searches for places matching the query and creates documents for each result. Returns a placeholder job ID since places search doesn't create async jobs."
- GetManagerType: "GetManagerType returns 'places_search' - the action type this manager handles"

**Log Message Updates:**
- Update log messages: "step executor" → "manager"
- Update log messages: "executing step" → "orchestrating places search"

**Implementation Notes:**
- No functional changes to the logic
- Only rename method signatures and update comments
- Note: This manager performs synchronous operations (no child jobs)
- Ensure interface compliance with new `JobManager` interface

**Note:** Struct name remains `PlacesSearchStepExecutor` in this phase - will be renamed to `PlacesSearchManager` in ARCH-004

### internal\jobs\executor\job_executor.go(MODIFY)

References: 

- internal\jobs\executor\interfaces.go(MODIFY)

Update `JobExecutor` orchestrator to use the renamed `JobManager` interface.

**Important Note:** This struct is named `JobExecutor` but it's NOT implementing the `JobExecutor` interface (which is being renamed to `JobWorker`). This is an orchestrator that uses `JobManager` implementations. The struct will be renamed in a later phase to avoid confusion.

**Field Renames:**
- `stepExecutors map[string]StepExecutor` → `stepExecutors map[string]JobManager`

**Method Signature Updates:**
- `func (e *JobExecutor) RegisterStepExecutor(executor StepExecutor)` → `func (e *JobExecutor) RegisterStepExecutor(executor JobManager)`

**Method Call Updates:**
- `executor.GetStepType()` → `executor.GetManagerType()`
- `executor.ExecuteStep(ctx, step, jobDef, parentJobID)` → `executor.CreateParentJob(ctx, step, jobDef, parentJobID)`

**Comment Updates:**
- Update struct comment: "JobExecutor orchestrates job definition execution. It routes steps to appropriate JobManagers and manages parent-child hierarchy."
- Update RegisterStepExecutor comment: "RegisterStepExecutor registers a job manager for an action type"
- Update field comment: "stepExecutors holds registered job managers keyed by action type"

**Log Message Updates:**
- "Step executor registered" → "Job manager registered"
- "action_type" field remains unchanged (still accurate)
- Update any references to "executor" in context of managers to "manager"

**Variable Name Updates:**
- Consider renaming local variable `executor` to `manager` where it refers to JobManager instances for clarity

**Implementation Notes:**
- Update all references to StepExecutor interface to JobManager
- Update all method calls to use new method names
- Ensure map operations use correct interface type
- No functional changes to the orchestration logic

**Note:** This struct will be renamed in a future phase to avoid confusion with the JobWorker interface (currently named JobExecutor)

### internal\jobs\processor\crawler_executor.go(MODIFY)

References: 

- internal\interfaces\job_executor.go(MODIFY)

Update `CrawlerExecutor` to implement the renamed `JobWorker` interface.

**Struct Comment Update:**
- Change: "CrawlerExecutor executes crawler jobs with ChromeDP rendering, content processing, and child job spawning for discovered links" → "CrawlerWorker processes individual crawler jobs from the queue, rendering pages with ChromeDP, extracting content, and spawning child jobs for discovered links"

**Method Signature Updates:**
- `func (e *CrawlerExecutor) GetJobType()` → `func (e *CrawlerExecutor) GetWorkerType()`
- Keep `func (e *CrawlerExecutor) Execute(...)` unchanged (already clear)
- Keep `func (e *CrawlerExecutor) Validate(...)` unchanged (already clear)

**Method Comment Updates:**
- GetWorkerType: "GetWorkerType returns 'crawler_url' - the job type this worker handles"
- Execute: Update comment to emphasize worker role: "Execute processes a single crawler job from the queue. Workflow: 1. ChromeDP rendering, 2. Content extraction, 3. Document storage, 4. Link discovery, 5. Child job spawning (respecting depth limits)"
- Validate: "Validate validates that the job model is compatible with this worker. Checks job type is 'crawler_url'."

**Log Message Updates:**
- Update log messages: "executor" → "worker" where referring to this component
- Update log messages: "executing job" → "processing job" for consistency
- Keep job-specific log messages unchanged (e.g., "Crawling URL", "Document saved")

**Implementation Notes:**
- No functional changes to the crawling logic
- Only rename method signatures and update comments
- Ensure interface compliance with new `JobWorker` interface
- Update import statement if interface moved to different package

**Note:** Struct name remains `CrawlerExecutor` in this phase - will be renamed to `CrawlerWorker` in ARCH-005 when files are moved to `internal/jobs/worker/`

### internal\jobs\processor\agent_executor.go(MODIFY)

References: 

- internal\interfaces\job_executor.go(MODIFY)

Update `AgentExecutor` to implement the renamed `JobWorker` interface.

**Struct Comment Update:**
- Change: "AgentExecutor executes agent jobs with document loading, agent processing, and metadata updates" → "AgentWorker processes individual agent jobs from the queue, loading documents, executing AI agents, and updating document metadata with results"

**Method Signature Updates:**
- `func (e *AgentExecutor) GetJobType()` → `func (e *AgentExecutor) GetWorkerType()`
- Keep `func (e *AgentExecutor) Execute(...)` unchanged (already clear)
- Keep `func (e *AgentExecutor) Validate(...)` unchanged (already clear)

**Method Comment Updates:**
- GetWorkerType: "GetWorkerType returns 'agent' - the job type this worker handles"
- Execute: Update comment to emphasize worker role: "Execute processes a single agent job from the queue. Workflow: 1. Load document, 2. Execute agent with document content, 3. Update document metadata with agent results, 4. Publish DocumentUpdated event"
- Validate: "Validate validates that the job model is compatible with this worker. Checks job type is 'agent'."

**Log Message Updates:**
- Update log messages: "executor" → "worker" where referring to this component
- Update log messages: "executing job" → "processing job" for consistency
- Keep agent-specific log messages unchanged (e.g., "Agent processing started", "Document metadata updated")

**Implementation Notes:**
- No functional changes to the agent processing logic
- Only rename method signatures and update comments
- Ensure interface compliance with new `JobWorker` interface
- Update import statement if interface moved to different package

**Note:** Struct name remains `AgentExecutor` in this phase - will be renamed to `AgentWorker` in ARCH-005

### internal\jobs\executor\database_maintenance_executor.go(MODIFY)

References: 

- internal\interfaces\job_executor.go(MODIFY)

Update `DatabaseMaintenanceExecutor` to implement the renamed `JobWorker` interface.

**Important Note:** This file will be deleted in ARCH-007 (migration to manager/worker split). For now, update it to maintain consistency with the new interface naming.

**Struct Comment Update:**
- Change: "DatabaseMaintenanceExecutor handles database maintenance jobs" → "DatabaseMaintenanceWorker processes individual database maintenance jobs from the queue (DEPRECATED - will be replaced by manager/worker split)"

**Method Signature Updates:**
- `func (e *DatabaseMaintenanceExecutor) GetJobType()` → `func (e *DatabaseMaintenanceExecutor) GetWorkerType()`
- Keep `func (e *DatabaseMaintenanceExecutor) Execute(...)` unchanged (already clear)
- Keep `func (e *DatabaseMaintenanceExecutor) Validate(...)` unchanged (already clear)

**Method Comment Updates:**
- GetWorkerType: "GetWorkerType returns 'database_maintenance' - the job type this worker handles"
- Execute: "Execute processes a database maintenance job from the queue. Performs VACUUM, ANALYZE, REINDEX, and OPTIMIZE operations."
- Validate: "Validate validates that the job model is compatible with this worker. Checks job type matches."

**Log Message Updates:**
- Update log messages: "executor" → "worker" where referring to this component
- Update log messages: "executing job" → "processing job" for consistency

**Implementation Notes:**
- No functional changes to the maintenance logic
- Only rename method signatures and update comments
- Add deprecation notice in comments
- Ensure interface compliance with new `JobWorker` interface

**Note:** This file will be deleted in ARCH-007 when database maintenance is migrated to the manager/worker split pattern

### internal\jobs\processor\processor.go(MODIFY)

References: 

- internal\interfaces\job_executor.go(MODIFY)

Update `JobProcessor` to use the renamed `JobWorker` interface.

**Struct Comment Update:**
- Change: "JobProcessor is a job-agnostic processor that uses goqite for queue management. It routes jobs to registered executors based on job type." → "JobProcessor is a job-agnostic processor that uses goqite for queue management. It routes jobs to registered workers based on job type."

**Field Renames:**
- `executors map[string]interfaces.JobExecutor` → `executors map[string]interfaces.JobWorker`

**Method Signature Updates:**
- `func (jp *JobProcessor) RegisterExecutor(executor interfaces.JobExecutor)` → `func (jp *JobProcessor) RegisterExecutor(executor interfaces.JobWorker)`

**Method Call Updates:**
- `executor.GetJobType()` → `executor.GetWorkerType()`
- Keep `executor.Execute(ctx, jobModel)` unchanged
- Keep `executor.Validate(jobModel)` unchanged

**Comment Updates:**
- Update RegisterExecutor comment: "RegisterExecutor registers a job worker for a job type. The worker must implement the JobWorker interface."
- Update field comment: "executors holds registered job workers keyed by job type"

**Log Message Updates:**
- "Job executor registered" → "Job worker registered"
- "No executor registered for job type" → "No worker registered for job type"
- Update any references to "executor" in context of workers to "worker"

**Variable Name Updates:**
- Consider renaming local variable `executor` to `worker` where it refers to JobWorker instances for clarity
- Update error messages: "executor" → "worker"

**Implementation Notes:**
- Update all references to JobExecutor interface to JobWorker
- Update all method calls to use new method names
- Ensure map operations use correct interface type
- No functional changes to the routing logic
- Update import statement to reference renamed interface

**Note:** This is the core worker routing system - ensure all references are updated consistently

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\executor\interfaces.go(MODIFY)
- internal\interfaces\job_executor.go(MODIFY)

Update `app.go` to use renamed interfaces and update registration calls.

**Import Updates:**
- Ensure imports reference the correct interface packages (no changes needed, but verify)

**JobManager Registration Updates (lines 377-403):**

**Variable Name Updates:**
- Consider renaming variables for clarity:
  - `crawlerStepExecutor` → `crawlerManager` (or keep for now, rename in ARCH-004)
  - `agentStepExecutor` → `agentManager`
  - `dbMaintenanceStepExecutor` → `dbMaintenanceManager`
  - `transformStepExecutor` → `transformManager`
  - `reindexStepExecutor` → `reindexManager`
  - `placesSearchStepExecutor` → `placesSearchManager`

**Log Message Updates:**
- "Crawler step executor registered" → "Crawler manager registered"
- "Agent step executor registered" → "Agent manager registered"
- "Database maintenance step executor registered" → "Database maintenance manager registered"
- "Transform step executor registered" → "Transform manager registered"
- "Reindex step executor registered" → "Reindex manager registered"
- "Places search step executor registered" → "Places search manager registered"

**JobWorker Registration Updates (lines 296-342):**

**Variable Name Updates:**
- Consider renaming variables for clarity:
  - `crawlerExecutor` → `crawlerWorker` (or keep for now, rename in ARCH-005)
  - `agentExecutor` → `agentWorker`
  - `dbMaintenanceExecutor` → `dbMaintenanceWorker`

**Log Message Updates:**
- "Crawler URL executor registered for job type: crawler_url" → "Crawler worker registered for job type: crawler_url"
- "Agent executor registered for job type: agent" → "Agent worker registered for job type: agent"
- "Database maintenance executor registered" → "Database maintenance worker registered"

**ParentJobExecutor Updates (lines 309-317):**
- Update comment: "Create parent job executor for managing parent job lifecycle" → "Create parent job orchestrator for monitoring parent job lifecycle"
- Update log message: "Parent job executor created (runs in background goroutines, not via queue)" → "Parent job orchestrator created (runs in background goroutines, not via queue)"
- Note: Variable name `parentJobExecutor` will be updated in ARCH-006

**JobExecutor Orchestrator Updates (line 375):**
- Update comment: "Initialize JobExecutor for job definition execution" → "Initialize JobExecutor orchestrator for job definition execution (will be renamed to JobOrchestrator in future phase)"
- Update log message (line 405): "JobExecutor initialized with all step executors" → "JobExecutor orchestrator initialized with all job managers"

**Implementation Notes:**
- Update all log messages to use consistent terminology
- Consider variable renames for clarity (optional in this phase)
- No functional changes to initialization logic
- Ensure all registration calls use correct interface types
- Verify imports are correct after interface renames

**Testing:**
- After changes, verify application starts successfully
- Check logs for updated messages
- Ensure all managers and workers are registered correctly

### test\api\places_job_document_test.go(MODIFY)

Update test file comment to use new terminology.

**Comment Update (line 379):**
- Change: "This is set by the event-driven ParentJobExecutor when EventDocumentSaved is published" → "This is set by the event-driven ParentJobOrchestrator when EventDocumentSaved is published"

**Implementation Notes:**
- This is the only test file with a reference to the old terminology
- Only a comment update, no functional changes
- Ensures documentation consistency with new architecture

**Note:** The actual `ParentJobExecutor` struct will be renamed to `ParentJobOrchestrator` in ARCH-006