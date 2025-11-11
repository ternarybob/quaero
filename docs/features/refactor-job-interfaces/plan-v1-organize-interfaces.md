I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase follows a clean architecture pattern with interfaces centralized in `internal/interfaces/` directory. Currently, three job-related interfaces are scattered in subdirectories (`internal/jobs/manager/`, `internal/jobs/orchestrator/`, `internal/jobs/worker/`), which violates this pattern and causes import cycle issues. The `job_definition_orchestrator.go` file contains duplicate interface definitions to work around these cycles. Moving these interfaces to `internal/interfaces/` will align with the project's established pattern, eliminate duplication, and resolve import cycles.

### Approach

Consolidate all job-related interfaces into `internal/interfaces/` by creating a new `job_interfaces.go` file. This centralizes interface definitions, eliminates duplicate interfaces in `job_definition_orchestrator.go`, and updates all import statements across the codebase. The refactoring maintains backward compatibility by preserving interface signatures while improving code organization.

### Reasoning

I examined the three interface files in their current locations, reviewed the existing `internal/interfaces/` directory structure, identified the duplicate interfaces in `job_definition_orchestrator.go`, and searched for all files importing these interfaces to understand the full scope of changes required.

## Proposed File Changes

### internal\interfaces\job_interfaces.go(NEW)

References: 

- internal\jobs\manager\interfaces.go(DELETE)
- internal\jobs\orchestrator\interfaces.go(DELETE)
- internal\jobs\worker\interfaces.go(DELETE)

Create a new file to consolidate all job-related interfaces from the three separate interface files. This file should contain:

1. **JobManager interface** - Moved from `internal/jobs/manager/interfaces.go`
   - Methods: `CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (jobID string, err error)` and `GetManagerType() string`
   - Documentation comments explaining the interface purpose and usage

2. **ParentJobOrchestrator interface** - Moved from `internal/jobs/orchestrator/interfaces.go`
   - Methods: `StartMonitoring(ctx context.Context, job *models.JobModel)` and `SubscribeToChildStatusChanges()`
   - Documentation comments explaining background monitoring and event subscription

3. **JobWorker interface** - Moved from `internal/jobs/worker/interfaces.go`
   - Methods: `Execute(ctx context.Context, job *models.JobModel) error`, `GetWorkerType() string`, and `Validate(job *models.JobModel) error`
   - Documentation comments explaining worker execution pattern

4. **JobSpawner interface** - Moved from `internal/jobs/worker/interfaces.go`
   - Method: `SpawnChildJob(ctx context.Context, parentJob *models.JobModel, childType, childName string, config map[string]interface{}) error`
   - Documentation comments explaining optional child job spawning

Package declaration should be `package interfaces` with appropriate imports for `context`, `github.com/ternarybob/quaero/internal/models`.

### internal\jobs\manager\interfaces.go(DELETE)

Delete this file as the JobManager interface has been moved to `internal/interfaces/job_interfaces.go`. All implementations in the manager package will now reference the centralized interface.

### internal\jobs\orchestrator\interfaces.go(DELETE)

Delete this file as the ParentJobOrchestrator interface has been moved to `internal/interfaces/job_interfaces.go`. The orchestrator implementation will now reference the centralized interface.

### internal\jobs\worker\interfaces.go(DELETE)

Delete this file as the JobWorker and JobSpawner interfaces have been moved to `internal/interfaces/job_interfaces.go`. All worker implementations will now reference the centralized interfaces.

### internal\jobs\job_definition_orchestrator.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Remove the duplicate local interface definitions and update to use centralized interfaces:

1. **Remove duplicate interfaces** (lines 13-27):
   - Delete the local `JobManager` interface definition (lines 13-19)
   - Delete the local `ParentJobOrchestrator` interface definition (lines 21-27)
   - Remove the comments explaining these are local copies to avoid import cycles

2. **Update imports**:
   - Add import: `"github.com/ternarybob/quaero/internal/interfaces"`
   - Keep existing imports for `context`, `fmt`, `time`, `github.com/google/uuid`, `github.com/ternarybob/arbor`, and `github.com/ternarybob/quaero/internal/models`

3. **Update struct field types** (lines 29-35):
   - Change `stepExecutors map[string]JobManager` to `stepExecutors map[string]interfaces.JobManager`
   - Change `parentJobOrchestrator ParentJobOrchestrator` to `parentJobOrchestrator interfaces.ParentJobOrchestrator`

4. **Update function signatures**:
   - `NewJobDefinitionOrchestrator` parameter: Change `parentJobOrchestrator ParentJobOrchestrator` to `parentJobOrchestrator interfaces.ParentJobOrchestrator`
   - `RegisterStepExecutor` parameter: Change `mgr JobManager` to `mgr interfaces.JobManager`

No changes to method implementations are required - only type references need updating.

### internal\jobs\manager\database_maintenance_manager.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Update imports and type references to use centralized interfaces:

1. **Update imports**:
   - Add import: `"github.com/ternarybob/quaero/internal/interfaces"`
   - Remove import: `"github.com/ternarybob/quaero/internal/jobs/orchestrator"` (no longer needed)
   - Keep existing imports for other dependencies

2. **Update struct field type** (line 25):
   - Change `parentJobOrchestrator orchestrator.ParentJobOrchestrator` to `parentJobOrchestrator interfaces.ParentJobOrchestrator`

3. **Update function signature** (line 30):
   - Change parameter type from `orchestrator.ParentJobOrchestrator` to `interfaces.ParentJobOrchestrator`

No changes to method implementations are required.

### internal\jobs\worker\job_processor.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Update imports and type references to use centralized interfaces:

1. **Update imports**:
   - Add import: `"github.com/ternarybob/quaero/internal/interfaces"`
   - Keep existing imports for other dependencies

2. **Update struct field type** (line 24):
   - Change `executors map[string]JobWorker` to `executors map[string]interfaces.JobWorker`

3. **Update function signatures**:
   - `RegisterExecutor` parameter (line 50): Change `worker JobWorker` to `worker interfaces.JobWorker`

4. **Update local variable types**:
   - Line 157: Change `worker, ok := jp.executors[msg.Type]` type inference will automatically use `interfaces.JobWorker`

No changes to method implementations are required - the interface methods remain the same.

### internal\app\app.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Update imports to remove now-deleted interface packages:

1. **Remove unnecessary imports** (lines 20-22):
   - The imports `"github.com/ternarybob/quaero/internal/jobs/manager"`, `"github.com/ternarybob/quaero/internal/jobs/orchestrator"`, and `"github.com/ternarybob/quaero/internal/jobs/worker"` are still needed for concrete implementations
   - However, verify that these imports are only used for concrete types (e.g., `manager.NewCrawlerManager`, `orchestrator.NewParentJobOrchestrator`, `worker.NewJobProcessor`)
   - The interface types are now accessed via `interfaces.JobManager`, `interfaces.ParentJobOrchestrator`, `interfaces.JobWorker`

2. **No code changes required**:
   - All concrete type instantiations remain the same (e.g., `manager.NewCrawlerManager`, `worker.NewCrawlerWorker`)
   - The interfaces are already imported via `"github.com/ternarybob/quaero/internal/interfaces"` (line 18)
   - Type inference will automatically use the centralized interfaces

This file primarily uses concrete implementations, so minimal changes are needed. The import cleanup ensures we're not importing packages solely for interface definitions.

### internal\jobs\manager\crawler_manager.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Verify interface implementation compatibility:

1. **No code changes required**:
   - The `CrawlerManager` struct already implements the `JobManager` interface methods: `CreateParentJob` and `GetManagerType`
   - The interface is now defined in `internal/interfaces/job_interfaces.go` instead of `internal/jobs/manager/interfaces.go`
   - Go's duck typing ensures the implementation automatically satisfies the interface

2. **Verification**:
   - Confirm that `CreateParentJob` method signature matches: `CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error)`
   - Confirm that `GetManagerType` method signature matches: `GetManagerType() string`

No import changes needed - this file doesn't directly reference the interface type, only implements it.

### internal\jobs\manager\agent_manager.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Verify interface implementation compatibility:

1. **No code changes required**:
   - The `AgentManager` struct already implements the `JobManager` interface methods: `CreateParentJob` and `GetManagerType`
   - The interface is now defined in `internal/interfaces/job_interfaces.go` instead of `internal/jobs/manager/interfaces.go`
   - Go's duck typing ensures the implementation automatically satisfies the interface

2. **Verification**:
   - Confirm that `CreateParentJob` method signature matches: `CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error)`
   - Confirm that `GetManagerType` method signature matches: `GetManagerType() string`

No import changes needed - this file doesn't directly reference the interface type, only implements it.

### internal\jobs\worker\crawler_worker.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Verify interface implementation compatibility:

1. **No code changes required**:
   - The `CrawlerWorker` struct already implements the `JobWorker` interface methods: `Execute`, `GetWorkerType`, and `Validate`
   - The interface is now defined in `internal/interfaces/job_interfaces.go` instead of `internal/jobs/worker/interfaces.go`
   - Go's duck typing ensures the implementation automatically satisfies the interface

2. **Verification**:
   - Confirm that `Execute` method signature matches: `Execute(ctx context.Context, job *models.JobModel) error`
   - Confirm that `GetWorkerType` method signature matches: `GetWorkerType() string`
   - Confirm that `Validate` method signature matches: `Validate(job *models.JobModel) error`

No import changes needed - this file doesn't directly reference the interface type, only implements it.

### internal\jobs\worker\agent_worker.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Verify interface implementation compatibility:

1. **No code changes required**:
   - The `AgentWorker` struct already implements the `JobWorker` interface methods: `Execute`, `GetWorkerType`, and `Validate`
   - The interface is now defined in `internal/interfaces/job_interfaces.go` instead of `internal/jobs/worker/interfaces.go`
   - Go's duck typing ensures the implementation automatically satisfies the interface

2. **Verification**:
   - Confirm that `Execute` method signature matches: `Execute(ctx context.Context, job *models.JobModel) error`
   - Confirm that `GetWorkerType` method signature matches: `GetWorkerType() string`
   - Confirm that `Validate` method signature matches: `Validate(job *models.JobModel) error`

No import changes needed - this file doesn't directly reference the interface type, only implements it.

### internal\jobs\worker\database_maintenance_worker.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Verify interface implementation compatibility:

1. **No code changes required**:
   - The `DatabaseMaintenanceWorker` struct already implements the `JobWorker` interface methods: `Execute`, `GetWorkerType`, and `Validate`
   - The interface is now defined in `internal/interfaces/job_interfaces.go` instead of `internal/jobs/worker/interfaces.go`
   - Go's duck typing ensures the implementation automatically satisfies the interface

2. **Verification**:
   - Confirm that `Execute` method signature matches: `Execute(ctx context.Context, job *models.JobModel) error`
   - Confirm that `GetWorkerType` method signature matches: `GetWorkerType() string`
   - Confirm that `Validate` method signature matches: `Validate(job *models.JobModel) error`

No import changes needed - this file doesn't directly reference the interface type, only implements it.

### internal\jobs\orchestrator\parent_job_orchestrator.go(MODIFY)

References: 

- internal\interfaces\job_interfaces.go(NEW)

Verify interface implementation compatibility:

1. **No code changes required**:
   - The `ParentJobOrchestrator` struct already implements the `ParentJobOrchestrator` interface methods: `StartMonitoring` and `SubscribeToChildStatusChanges`
   - The interface is now defined in `internal/interfaces/job_interfaces.go` instead of `internal/jobs/orchestrator/interfaces.go`
   - Go's duck typing ensures the implementation automatically satisfies the interface

2. **Verification**:
   - Confirm that `StartMonitoring` method signature matches: `StartMonitoring(ctx context.Context, job *models.JobModel)`
   - Confirm that `SubscribeToChildStatusChanges` method signature matches: `SubscribeToChildStatusChanges()`

No import changes needed - this file doesn't directly reference the interface type, only implements it.