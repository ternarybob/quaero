I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The codebase is ready for parent job orchestrator migration after completing ARCH-006 (remaining worker files):

1. **Target File (1 orchestrator to migrate):**
   - `parent_job_executor.go` (510 lines) - ParentJobExecutor struct with 3 dependencies
   - Package: `processor`
   - Struct: `ParentJobExecutor`
   - Constructor: `NewParentJobExecutor(jobMgr, eventService, logger)`
   - Methods: StartMonitoring, validate, monitorChildJobs, checkChildJobProgress, publishParentJobProgress, publishChildJobStats, SubscribeToChildStatusChanges, formatProgressText, publishParentJobProgressUpdate, calculateOverallStatus

2. **Key Responsibilities:**
   - Monitors parent job lifecycle in background goroutines (NOT via queue)
   - Polls child job statistics from database every 5 seconds
   - Aggregates progress (total, completed, failed, cancelled, running, pending counts)
   - Publishes progress events for real-time UI updates
   - Detects parent job completion when all children finish
   - Updates parent job status (running → completed/failed/cancelled)
   - Subscribes to child job status change events for real-time tracking
   - Subscribes to document_saved events for document count tracking

3. **Import Locations (2 files):**
   - `internal/app/app.go` (lines 22, 314-319, 377):
     - Import: `"github.com/ternarybob/quaero/internal/jobs/processor"`
     - Constructor: `processor.NewParentJobExecutor(jobMgr, a.EventService, a.Logger)`
     - Variable: `parentJobExecutor`
     - Passed to: `executor.NewJobExecutor(jobMgr, parentJobExecutor, a.Logger)`
     - Log message: "Parent job executor created (runs in background goroutines, not via queue)"
   
   - `internal/jobs/executor/job_executor.go` (lines 11, 20, 25, 29, 370):
     - Import: `"github.com/ternarybob/quaero/internal/jobs/processor"`
     - Field: `parentJobExecutor *processor.ParentJobExecutor`
     - Constructor parameter: `parentJobExecutor *processor.ParentJobExecutor`
     - Method call: `e.parentJobExecutor.StartMonitoring(ctx, parentJobModel)`

4. **Comment-Only References (4 files):**
   - `internal/jobs/worker/job_processor.go` (lines 221, 227) - Comments about parent job handling
   - `internal/interfaces/event_service.go` (lines 166, 177) - Event documentation
   - `internal/jobs/manager.go` (line 1687) - Method documentation
   - `test/api/places_job_document_test.go` (line 379) - Test comment

5. **Target Directory Ready:**
   - `internal/jobs/orchestrator/` exists with interfaces.go (created in ARCH-003)
   - ParentJobOrchestrator interface defined with 3 methods
   - Ready to receive implementation file

6. **Interface Signature Mismatch:**
   - Interface defines: `StartMonitoring(ctx context.Context, parentJobID string) error`
   - Implementation has: `StartMonitoring(ctx context.Context, job *models.JobModel)`
   - **Resolution needed:** Either update interface or adjust implementation signature

**Key Architectural Insight:**

The ParentJobExecutor is the monitoring layer of the Manager/Worker/Orchestrator architecture:
- **Managers** create parent jobs and enqueue children (orchestration)
- **Workers** execute individual jobs from queue (execution)
- **Orchestrator** monitors parent jobs and aggregates progress (monitoring)

This is the final piece to complete the architectural separation.

**Dependencies Analysis:**

- **jobMgr** (*jobs.Manager): Used for job status updates, child stats, metadata operations
- **eventService** (interfaces.EventService): Used for publishing progress events and subscribing to status changes
- **logger** (arbor.ILogger): Used for structured logging with correlation IDs

All dependencies are injected via constructor (good DI pattern).

**Risk Assessment:**

- **Low Risk**: File migration is mechanical transformation
- **Low Risk**: Only 2 files import it (limited scope)
- **Medium Risk**: Interface signature mismatch needs resolution (StartMonitoring parameter type)
- **Low Risk**: Breaking changes are acceptable per user request
- **Very Low Risk**: No backward compatibility needed
- **Low Risk**: Comment updates in 4 files (no functional impact)

**Success Criteria:**

1. New file created in internal/jobs/orchestrator/
2. ParentJobExecutor renamed to ParentJobOrchestrator throughout
3. Interface signature mismatch resolved
4. app.go successfully imports and uses orchestrator package
5. job_executor.go successfully imports and uses orchestrator package
6. All comments updated to use "orchestrator" terminology
7. Old file deleted from processor/ directory
8. Application compiles and runs successfully
9. All tests pass (especially parent job monitoring tests)
10. Parent job monitoring works correctly end-to-end
11. Child job progress aggregation works correctly
12. Real-time events publish correctly
13. Documentation updated to reflect ARCH-007 completion

### Approach

**Direct File Migration with Immediate Cleanup Strategy**

This phase migrates the ParentJobExecutor from `internal/jobs/processor/` to `internal/jobs/orchestrator/` as the final piece of the Manager/Worker/Orchestrator architecture. Since breaking changes are acceptable, we'll use a direct migration approach without backward compatibility.

**Key Principles:**

1. **Direct Migration**: Create new file, update imports, delete old file immediately
2. **No Backward Compatibility**: User explicitly stated breaking changes are OK
3. **Complete Transformation**: Rename all references to use "orchestrator" terminology
4. **Interface Compliance**: Ensure implementation matches ParentJobOrchestrator interface from ARCH-003
5. **Consistent Naming**: Follow orchestrator naming convention (receiver variable `o` instead of `e`)

**Why This Approach:**

- **Clean Break**: No temporary duplication or deprecation notices needed
- **Immediate Clarity**: Developers see the new structure immediately
- **Reduced Complexity**: No dual import strategy or transition period
- **Architectural Completion**: Completes the Manager/Worker/Orchestrator separation
- **User Preference**: Explicitly requested immediate deletion of old files

**Migration Sequence:**

1. **Create ParentJobOrchestrator** in orchestrator/ directory
2. **Update app.go** imports and initialization
3. **Update job_executor.go** imports and field references
4. **Update comments** in 4 files that mention ParentJobExecutor
5. **Delete old file** immediately (no deprecation period)
6. **Update documentation** to reflect ARCH-007 completion

**Key Transformations:**

**File Migration:**
- Source: `internal/jobs/processor/parent_job_executor.go`
- Target: `internal/jobs/orchestrator/parent_job_orchestrator.go`
- Package: `processor` → `orchestrator`
- Struct: `ParentJobExecutor` → `ParentJobOrchestrator`
- Constructor: `NewParentJobExecutor()` → `NewParentJobOrchestrator()`
- Receiver: `func (e *ParentJobExecutor)` → `func (o *ParentJobOrchestrator)`
- All references to `e.` → `o.` within method bodies

**Import Updates:**

Only 2 files import ParentJobExecutor:
- `internal/app/app.go` (line 22, 314, 319, 377)
- `internal/jobs/executor/job_executor.go` (line 11, 20, 25, 29, 370)

Both need:
- Import: `"github.com/ternarybob/quaero/internal/jobs/processor"` → `"github.com/ternarybob/quaero/internal/jobs/orchestrator"`
- Type references: `processor.ParentJobExecutor` → `orchestrator.ParentJobOrchestrator`
- Constructor calls: `processor.NewParentJobExecutor()` → `orchestrator.NewParentJobOrchestrator()`
- Variable names: `parentJobExecutor` → `parentJobOrchestrator`

**Comment Updates:**

4 files mention ParentJobExecutor in comments only:
- `internal/jobs/worker/job_processor.go` (lines 221, 227)
- `internal/interfaces/event_service.go` (lines 166, 177)
- `internal/jobs/manager.go` (line 1687)
- `test/api/places_job_document_test.go` (line 379)

All need: "ParentJobExecutor" → "ParentJobOrchestrator" in comments

**Interface Compliance:**

The orchestrator must implement the ParentJobOrchestrator interface created in ARCH-003:
```go
type ParentJobOrchestrator interface {
    StartMonitoring(ctx context.Context, parentJobID string) error
    StopMonitoring(parentJobID string) error
    GetMonitoringStatus(parentJobID string) bool
}
```

**Note:** Current implementation has `StartMonitoring(ctx, job *models.JobModel)` which differs from interface signature. This will need adjustment or interface update.

**Validation Strategy:**

After migration:
1. Verify new file compiles in orchestrator/ package
2. Verify app.go compiles with new import
3. Verify job_executor.go compiles with new import
4. Build application successfully
5. Run full test suite
6. Verify parent job monitoring works end-to-end
7. Verify child job progress aggregation works correctly

**Risk Assessment:**

- **Low Risk**: File migration is mechanical transformation
- **Low Risk**: Only 2 files import it (limited scope)
- **Medium Risk**: Interface signature mismatch needs resolution
- **Low Risk**: Breaking changes are acceptable per user request
- **Very Low Risk**: No backward compatibility needed

**Success Criteria:**

1. New file created in internal/jobs/orchestrator/
2. ParentJobExecutor renamed to ParentJobOrchestrator throughout
3. app.go successfully imports and uses orchestrator package
4. job_executor.go successfully imports and uses orchestrator package
5. All comments updated to use "orchestrator" terminology
6. Old file deleted from processor/ directory
7. Application compiles and runs successfully
8. All tests pass
9. Parent job monitoring works correctly
10. Documentation updated to reflect ARCH-007 completion

### Reasoning

I systematically explored the codebase to understand the migration requirements:

1. **Read target file** - Examined parent_job_executor.go (510 lines) to understand structure, dependencies, and responsibilities
2. **Searched for all references** - Used grep to find all files that mention ParentJobExecutor (found 20 matches across 6 files)
3. **Analyzed import locations** - Identified 2 files that actually import and use ParentJobExecutor (app.go, job_executor.go)
4. **Identified comment-only references** - Found 4 files that only mention ParentJobExecutor in comments
5. **Read import contexts** - Examined app.go and job_executor.go to understand how ParentJobExecutor is initialized and used
6. **Verified target directory** - Confirmed orchestrator/ directory exists with interfaces.go from ARCH-003
7. **Checked interface definition** - Reviewed ParentJobOrchestrator interface to ensure compliance

This comprehensive exploration revealed:
- Clear scope: 1 file to migrate, 2 files to update imports, 4 files to update comments
- Simple transformation: Rename struct, constructor, package, and receiver variable
- No complex dependencies: Only 3 dependencies (jobMgr, eventService, logger)
- Breaking changes acceptable: User explicitly requested immediate deletion
- Interface mismatch: StartMonitoring signature differs from interface (needs resolution)

## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant Old as processor/parent_job_executor.go
    participant New as orchestrator/parent_job_orchestrator.go
    participant Interface as orchestrator/interfaces.go
    participant App as internal/app/app.go
    participant JobExec as executor/job_executor.go
    participant Build as Go Build System
    
    Note over Dev,Build: Phase 1: Create Orchestrator File
    
    Dev->>New: Create parent_job_orchestrator.go
    Note right of New: Copy from parent_job_executor.go<br/>Package: processor → orchestrator<br/>Struct: ParentJobExecutor → ParentJobOrchestrator<br/>Constructor: NewParentJobExecutor → NewParentJobOrchestrator<br/>Receiver: (e *ParentJobExecutor) → (o *ParentJobOrchestrator)
    
    Dev->>Interface: Update ParentJobOrchestrator interface
    Note right of Interface: Match implementation signature:<br/>StartMonitoring(ctx, job *JobModel)<br/>SubscribeToChildStatusChanges()<br/>Remove speculative methods
    
    Dev->>Build: Compile new orchestrator file
    Build-->>Dev: ✓ parent_job_orchestrator.go compiles successfully
    
    Note over Dev,Build: Phase 2: Update Import Paths
    
    Dev->>App: Update imports and initialization
    Note right of App: Remove: processor import<br/>Add: orchestrator import<br/>orchestrator.NewParentJobOrchestrator()<br/>Variable: parentJobExecutor → parentJobOrchestrator
    
    Dev->>JobExec: Update imports and field references
    Note right of JobExec: Remove: processor import<br/>Add: orchestrator import<br/>Field: parentJobOrchestrator<br/>Call: o.parentJobOrchestrator.StartMonitoring()
    
    Dev->>Build: Build application
    Build-->>Dev: ✓ Application compiles successfully
    
    Note over Dev,Build: Phase 3: Update Comments (4 files)
    
    Dev->>Dev: Update job_processor.go comments
    Dev->>Dev: Update event_service.go comments
    Dev->>Dev: Update manager.go comments
    Dev->>Dev: Update places_job_document_test.go comments
    
    Note over Dev,Build: Phase 4: Delete Old File
    
    Dev->>Old: Delete parent_job_executor.go
    Note right of Old: Breaking change acceptable<br/>No backward compatibility<br/>processor/ directory now empty
    
    Note over Dev,Build: Phase 5: Validation
    
    Dev->>Build: Run test suite
    Build-->>Dev: ✓ All tests pass
    
    Dev->>App: Start application
    App->>New: Initialize ParentJobOrchestrator
    App->>New: Subscribe to child status changes
    App-->>Dev: ✓ "Parent job orchestrator created"
    
    Dev->>App: Trigger crawler job via UI
    App->>JobExec: Execute job definition
    JobExec->>New: StartMonitoring(ctx, parentJobModel)
    New->>New: Start monitoring goroutine
    New->>New: Poll child job stats every 5s
    New->>New: Publish progress events
    New->>New: Detect completion (all children done)
    New->>New: Update parent job status
    New-->>JobExec: ✓ Monitoring started
    JobExec-->>App: ✓ Job execution complete
    
    Note over Dev,Build: Migration Complete<br/>Orchestrator migrated<br/>Old file deleted<br/>processor/ directory empty<br/>Manager/Worker/Orchestrator separation complete

## Proposed File Changes

### internal\jobs\orchestrator\parent_job_orchestrator.go(NEW)

References: 

- internal\jobs\processor\parent_job_executor.go(DELETE)
- internal\jobs\orchestrator\interfaces.go(MODIFY)

Create new ParentJobOrchestrator file by copying from `internal/jobs/processor/parent_job_executor.go` with the following transformations:

**Package Declaration:**
- Change: `package processor` → `package orchestrator`

**File Header Comment:**
- Add: "Parent Job Orchestrator - Monitors parent job progress and aggregates child job statistics"

**Imports:**
- Keep all imports unchanged:
  - Standard library: `context`, `fmt`, `time`
  - External: `github.com/ternarybob/arbor`
  - Internal: `github.com/ternarybob/quaero/internal/interfaces`, `github.com/ternarybob/quaero/internal/jobs`, `github.com/ternarybob/quaero/internal/models`
- No import changes needed (all use internal packages)

**Struct Rename:**
- Change: `type ParentJobExecutor struct` → `type ParentJobOrchestrator struct`
- Keep all 3 fields unchanged:
  - jobMgr *jobs.Manager
  - eventService interfaces.EventService
  - logger arbor.ILogger
- Update struct comment: "ParentJobOrchestrator monitors parent job progress and aggregates child job statistics. It runs in background goroutines (not via queue) and publishes real-time progress events."

**Constructor Rename:**
- Change: `func NewParentJobExecutor(...)` → `func NewParentJobOrchestrator(...)`
- Change return type: `*ParentJobExecutor` → `*ParentJobOrchestrator`
- Update struct initialization: `executor := &ParentJobExecutor{...}` → `orchestrator := &ParentJobOrchestrator{...}`
- Update return statement: `return executor` → `return orchestrator`
- Update comment: "NewParentJobOrchestrator creates a new parent job orchestrator for monitoring parent job lifecycle and aggregating child job progress"
- Keep all 3 parameters unchanged (same order, same types)

**Method Receivers (All Methods):**
- Change all method receivers: `func (e *ParentJobExecutor)` → `func (o *ParentJobOrchestrator)`
- Rename receiver variable from `e` to `o` for consistency (orchestrator convention)
- Update all references to `e.` → `o.` within all method bodies
- This applies to:
  - StartMonitoring() - public method (entry point)
  - validate() - private helper
  - monitorChildJobs() - private monitoring loop
  - checkChildJobProgress() - private progress checker
  - publishParentJobProgress() - private event publisher
  - publishChildJobStats() - private event publisher
  - SubscribeToChildStatusChanges() - public subscription method
  - formatProgressText() - private formatter
  - publishParentJobProgressUpdate() - private event publisher
  - calculateOverallStatus() - private status calculator

**Comments:**
- Update all comments referencing "executor" → "orchestrator"
- Update all comments referencing "ParentJobExecutor" → "ParentJobOrchestrator"
- Keep all existing detailed comments (especially in monitorChildJobs method)
- Update method comment for StartMonitoring: "StartMonitoring starts monitoring a parent job in a separate goroutine. This is the primary entry point for parent job orchestration - NOT via queue. Returns immediately after starting the goroutine."
- Update method comment for validate: "validate validates that the job model is compatible with this orchestrator"
- Update subscription log message (line 402): "ParentJobOrchestrator subscribed to child job status changes and document_saved events"

**Log Messages:**
- Update log messages: "executor" → "orchestrator" where referring to this component
- Update log message (line 71): "Parent job monitoring started in background goroutine" (already correct)
- Update log message (line 319): "Parent job orchestrator created (runs in background goroutines, not via queue)"
- Keep all other log messages unchanged (e.g., "Starting parent job execution", "All child jobs completed")

**Interface Compliance Note:**
- Current signature: `StartMonitoring(ctx context.Context, job *models.JobModel)`
- Interface signature: `StartMonitoring(ctx context.Context, parentJobID string) error`
- **Keep current signature** - it's more flexible and already used by job_executor.go
- The interface in orchestrator/interfaces.go should be updated to match implementation (not part of this file)

**Validation:**
- Verify file compiles independently: `go build internal/jobs/orchestrator/parent_job_orchestrator.go`
- Verify all method signatures are correct
- Total lines: ~510 (same as original)
- Verify all 3 dependencies are properly injected via constructor
- Verify all event publishing methods work correctly
- Verify subscription logic is preserved

### internal\jobs\orchestrator\interfaces.go(MODIFY)

References: 

- internal\jobs\processor\parent_job_executor.go(DELETE)
- internal\jobs\executor\job_executor.go(MODIFY)

Update ParentJobOrchestrator interface to match the actual implementation signature.

**Current Interface (from ARCH-003):**
```go
type ParentJobOrchestrator interface {
    StartMonitoring(ctx context.Context, parentJobID string) error
    StopMonitoring(parentJobID string) error
    GetMonitoringStatus(parentJobID string) bool
}
```

**Updated Interface:**
```go
type ParentJobOrchestrator interface {
    StartMonitoring(ctx context.Context, job *models.JobModel)
    SubscribeToChildStatusChanges()
}
```

**Rationale for Changes:**

1. **StartMonitoring signature:**
   - Change from: `StartMonitoring(ctx context.Context, parentJobID string) error`
   - Change to: `StartMonitoring(ctx context.Context, job *models.JobModel)`
   - Reason: Implementation needs full job model (not just ID) to access config fields like source_type, entity_type, and metadata
   - Reason: Implementation doesn't return error - it starts goroutine and returns immediately
   - This matches how job_executor.go calls it: `e.parentJobExecutor.StartMonitoring(ctx, parentJobModel)`

2. **Remove StopMonitoring and GetMonitoringStatus:**
   - These methods don't exist in the implementation
   - They were speculative additions in ARCH-003
   - Current implementation uses context cancellation for stopping (via ctx.Done())
   - No need for explicit stop/status methods at this time

3. **Add SubscribeToChildStatusChanges:**
   - This method exists in implementation and is called during initialization
   - It's a public method that sets up event subscriptions
   - Should be part of the interface for completeness

**Updated Interface Comment:**
- Update comment: "ParentJobOrchestrator monitors parent job progress and aggregates child job statistics. It runs in background goroutines (not via queue) and publishes real-time progress events. Orchestrators subscribe to child job status changes for real-time tracking."

**Import Updates:**
- Add import: `"github.com/ternarybob/quaero/internal/models"` (needed for JobModel type)

**Validation:**
- Verify interface compiles with new signature
- Verify ParentJobOrchestrator implementation satisfies updated interface
- Verify job_executor.go usage matches new interface signature

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\orchestrator\parent_job_orchestrator.go(NEW)
- internal\jobs\executor\job_executor.go(MODIFY)

Update app.go to import and use the new orchestrator package for ParentJobOrchestrator.

**Import Section Updates (line 22):**

Remove processor import (no longer needed after this migration):
```go
"github.com/ternarybob/quaero/internal/jobs/processor"  // DELETE - migrated to orchestrator
```

Add orchestrator import:
```go
"github.com/ternarybob/quaero/internal/jobs/orchestrator"  // NEW - for ParentJobOrchestrator
```

**Note:** After this change, processor import should be completely removed since parent_job_executor.go was the last file in processor/ directory.

**ParentJobOrchestrator Initialization (lines 311-319):**

Replace:
```go
// Create parent job executor for managing parent job lifecycle
// NOTE: Parent jobs are NOT registered with JobProcessor - they run in separate goroutines
// to avoid blocking queue workers with long-running monitoring loops
parentJobExecutor := processor.NewParentJobExecutor(
    jobMgr,
    a.EventService,
    a.Logger,
)
a.Logger.Info().Msg("Parent job executor created (runs in background goroutines, not via queue)")
```

With:
```go
// Create parent job orchestrator for monitoring parent job lifecycle
// NOTE: Parent jobs are NOT registered with JobProcessor - they run in separate goroutines
// to avoid blocking queue workers with long-running monitoring loops
parentJobOrchestrator := orchestrator.NewParentJobOrchestrator(
    jobMgr,
    a.EventService,
    a.Logger,
)
a.Logger.Info().Msg("Parent job orchestrator created (runs in background goroutines, not via queue)")
```

**JobExecutor Initialization (line 377):**

Replace:
```go
// 6.9. Initialize JobExecutor for job definition execution
// Pass parentJobExecutor so it can start monitoring goroutines for crawler jobs
a.JobExecutor = executor.NewJobExecutor(jobMgr, parentJobExecutor, a.Logger)
```

With:
```go
// 6.9. Initialize JobExecutor for job definition execution
// Pass parentJobOrchestrator so it can start monitoring goroutines for crawler jobs
a.JobExecutor = executor.NewJobExecutor(jobMgr, parentJobOrchestrator, a.Logger)
```

**Variable Naming:**
- Changed from `parentJobExecutor` to `parentJobOrchestrator` for clarity and consistency
- Constructor call: `processor.NewParentJobExecutor()` → `orchestrator.NewParentJobOrchestrator()`
- All 3 parameters remain in same order (no changes to parameter list)

**Comment Updates:**
- "Create parent job executor" → "Create parent job orchestrator"
- "Pass parentJobExecutor" → "Pass parentJobOrchestrator"
- Keep note about NOT registering with JobProcessor (still accurate)

**Log Message:**
- "Parent job executor created" → "Parent job orchestrator created"
- Keep rest of message unchanged ("runs in background goroutines, not via queue")

**Validation:**
- Verify application compiles successfully
- Verify ParentJobOrchestrator is initialized correctly
- Verify it's passed to JobExecutor correctly
- Run application and check startup logs for "Parent job orchestrator created (runs in background goroutines, not via queue)"
- Verify parent job monitoring works correctly via UI or API

### internal\jobs\executor\job_executor.go(MODIFY)

References: 

- internal\jobs\orchestrator\parent_job_orchestrator.go(NEW)

Update job_executor.go to import and use the new orchestrator package for ParentJobOrchestrator.

**Import Section Updates (line 11):**

Remove processor import:
```go
"github.com/ternarybob/quaero/internal/jobs/processor"  // DELETE - migrated to orchestrator
```

Add orchestrator import:
```go
"github.com/ternarybob/quaero/internal/jobs/orchestrator"  // NEW - for ParentJobOrchestrator
```

**Field Declaration Update (line 20):**

Replace:
```go
parentJobExecutor *processor.ParentJobExecutor
```

With:
```go
parentJobOrchestrator *orchestrator.ParentJobOrchestrator
```

**Constructor Parameter Update (line 25):**

Replace:
```go
func NewJobExecutor(jobManager *jobs.Manager, parentJobExecutor *processor.ParentJobExecutor, logger arbor.ILogger) *JobExecutor {
```

With:
```go
func NewJobExecutor(jobManager *jobs.Manager, parentJobOrchestrator *orchestrator.ParentJobOrchestrator, logger arbor.ILogger) *JobExecutor {
```

**Constructor Body Update (line 29):**

Replace:
```go
return &JobExecutor{
    stepExecutors:     make(map[string]JobManager),
    jobManager:        jobManager,
    parentJobExecutor: parentJobExecutor,
    logger:            logger,
}
```

With:
```go
return &JobExecutor{
    stepExecutors:        make(map[string]JobManager),
    jobManager:           jobManager,
    parentJobOrchestrator: parentJobOrchestrator,
    logger:               logger,
}
```

**Method Call Update (line 370):**

Replace:
```go
// Start monitoring in background goroutine
e.parentJobExecutor.StartMonitoring(ctx, parentJobModel)
```

With:
```go
// Start monitoring in background goroutine
e.parentJobOrchestrator.StartMonitoring(ctx, parentJobModel)
```

**Comment Updates:**

Update comments that reference "ParentJobExecutor" to "ParentJobOrchestrator":

- Line 335: "leaving in running state for ParentJobOrchestrator to monitor child jobs"
- Line 375: "NOTE: Do NOT set finished_at for crawler jobs - ParentJobOrchestrator will handle this"

**Variable Naming:**
- Changed from `parentJobExecutor` to `parentJobOrchestrator` throughout
- Field name: `parentJobExecutor` → `parentJobOrchestrator`
- Parameter name: `parentJobExecutor` → `parentJobOrchestrator`
- All references updated consistently

**Validation:**
- Verify file compiles successfully
- Verify JobExecutor constructor accepts ParentJobOrchestrator parameter
- Verify StartMonitoring call works correctly
- Verify all references to parentJobOrchestrator are correct
- Run application and verify job definition execution works correctly
- Verify parent job monitoring starts correctly for crawler jobs

### internal\jobs\worker\job_processor.go(MODIFY)

Update comments to use "ParentJobOrchestrator" terminology.

**Comment Update (line 221):**

Replace:
```go
// For parent jobs, do NOT mark as completed here - ParentJobExecutor will handle completion
```

With:
```go
// For parent jobs, do NOT mark as completed here - ParentJobOrchestrator will handle completion
```

**Comment Update (line 227):**

Replace:
```go
// Parent job remains in "running" state and will be re-enqueued by ParentJobExecutor
```

With:
```go
// Parent job remains in "running" state and will be re-enqueued by ParentJobOrchestrator
```

**No Code Changes:**
- Only comment updates
- No import changes needed (file doesn't import ParentJobOrchestrator)
- No functional changes

**Purpose:**
- Update terminology to match new architecture
- Maintain consistency across codebase
- Help developers understand the orchestrator's role

### internal\interfaces\event_service.go(MODIFY)

Update comments to use "ParentJobOrchestrator" terminology.

**Comment Update (line 166):**

Replace:
```go
// Used by ParentJobExecutor to track child job progress in real-time.
```

With:
```go
// Used by ParentJobOrchestrator to track child job progress in real-time.
```

**Comment Update (line 177):**

Replace:
```go
// Used by ParentJobExecutor to track document count for parent jobs in real-time.
```

With:
```go
// Used by ParentJobOrchestrator to track document count for parent jobs in real-time.
```

**No Code Changes:**
- Only comment updates in event documentation
- No import changes needed (file doesn't import ParentJobOrchestrator)
- No functional changes

**Purpose:**
- Update event documentation to reflect new architecture
- Clarify which component subscribes to these events
- Maintain consistency across codebase

### internal\jobs\manager.go(MODIFY)

Update comment to use "ParentJobOrchestrator" terminology.

**Comment Update (line 1687):**

Replace:
```go
// This is used by the ParentJobExecutor to monitor child job progress
```

With:
```go
// This is used by the ParentJobOrchestrator to monitor child job progress
```

**No Code Changes:**
- Only comment update in method documentation
- No import changes needed (file doesn't import ParentJobOrchestrator)
- No functional changes

**Purpose:**
- Update method documentation to reflect new architecture
- Clarify which component calls this method
- Maintain consistency across codebase

### test\api\places_job_document_test.go(MODIFY)

Update comment to use "ParentJobOrchestrator" terminology.

**Comment Update (line 379):**

Replace:
```go
// This is set by the event-driven ParentJobExecutor when EventDocumentSaved is published
```

With:
```go
// This is set by the event-driven ParentJobOrchestrator when EventDocumentSaved is published
```

**No Code Changes:**
- Only comment update in test documentation
- No import changes needed (test doesn't import ParentJobOrchestrator)
- No functional changes to test logic

**Purpose:**
- Update test documentation to reflect new architecture
- Clarify which component handles document count tracking
- Maintain consistency across codebase including tests

### internal\jobs\processor\parent_job_executor.go(DELETE)

Delete the old parent_job_executor.go file immediately after migration.

**Rationale:**
- User explicitly stated: "delete the old file immediately after migration. No backward compatibility needed."
- Breaking changes are acceptable for this project
- No deprecation period or backward compatibility required
- Clean break ensures developers use new orchestrator package
- Prevents confusion about which file to use

**Validation Before Deletion:**
- Verify new file exists in orchestrator/ directory
- Verify app.go compiles with new import
- Verify job_executor.go compiles with new import
- Verify application builds successfully
- Verify all tests pass

**Note:**
After this deletion, the processor/ directory should be empty and can be deleted in the next phase (ARCH-008).

### AGENTS.md(MODIFY)

References: 

- docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update AGENTS.md to document the completion of ARCH-007 (parent job orchestrator migration).

**Section to Update: "Directory Structure (In Transition - ARCH-006)"**

Update the section title and content to reflect ARCH-007 completion:

Change from:
```markdown
### Directory Structure (In Transition - ARCH-006)
```

To:
```markdown
### Directory Structure (In Transition - ARCH-007)
```

Update the orchestrator directory listing:

```markdown
- `internal/jobs/orchestrator/` - Parent job orchestrator (monitoring layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `parent_job_orchestrator.go` (ARCH-007)
```

Update the old directories listing:

```markdown
**Old Directories (Still Active - Will be removed in ARCH-008):**
- `internal/jobs/executor/` - Old manager implementations (6 remaining files)
- `internal/jobs/processor/` - EMPTY (all files migrated, directory will be deleted in ARCH-008)
```

Update the migration progress:

```markdown
**Migration Progress:**
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated (merged crawler_executor.go + crawler_executor_auth.go)
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker.go, job_processor.go)
- Phase ARCH-007: ✅ Parent job orchestrator migrated (YOU ARE HERE)
- Phase ARCH-008: ⏳ Database maintenance worker split (pending)
```

**Section to Update: "Core Components"**

Update to reflect that ParentJobOrchestrator is now in orchestrator/ package:

```markdown
**Core Components:**
- `JobProcessor` - `internal/jobs/worker/job_processor.go` (ARCH-006)
  - Routes jobs from queue to registered workers
  - Manages worker pool lifecycle (Start/Stop)
- `ParentJobOrchestrator` - `internal/jobs/orchestrator/parent_job_orchestrator.go` (ARCH-007)
  - Monitors parent job progress in background goroutines
  - Aggregates child job statistics
  - Publishes real-time progress events
```

**Implementation Notes:**
- Update migration status to show ARCH-007 complete
- Add checkmark (✅) for parent_job_orchestrator.go
- Mark processor/ directory as EMPTY (all files migrated)
- Add ParentJobOrchestrator to Core Components section
- Clarify that orchestrator runs in background goroutines (not via queue)

### docs\architecture\MANAGER_WORKER_ARCHITECTURE.md(MODIFY)

Update MANAGER_WORKER_ARCHITECTURE.md to document the completion of ARCH-007 (parent job orchestrator migration).

**Section to Update: "Current Status (After ARCH-006)"**

Change the section title and content to reflect ARCH-007 completion:

```markdown
### Current Status (After ARCH-007)

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
  - ✅ `parent_job_orchestrator.go` - Migrated from processor/ (ARCH-007)

**Old Directories (Still Active):**
- `internal/jobs/executor/` - Contains 6 remaining implementation files:
  - `transform_step_executor.go` (pending migration)
  - `reindex_step_executor.go` (pending migration)
  - `places_search_step_executor.go` (pending migration)
  - `job_executor.go` (orchestrator - will be refactored separately)
  - `base_executor.go` (shared utilities - will be refactored separately)
  - `database_maintenance_executor.go` (old worker - will be deleted in ARCH-008)
- `internal/jobs/processor/` - EMPTY (all files migrated, directory will be deleted in ARCH-008)

**Migration Status:**
- Phase ARCH-001: ✅ Documentation created
- Phase ARCH-002: ✅ Interfaces renamed
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ Manager files migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated and merged
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker, job_processor)
- Phase ARCH-007: ✅ Parent job orchestrator migrated (YOU ARE HERE)
- Phase ARCH-008: ⏳ Database maintenance worker split (pending)
- Phase ARCH-009: ⏳ Import path updates and cleanup (pending)
- Phase ARCH-010: ⏳ End-to-end validation (pending)
```

**Section to Add: "Parent Job Orchestrator Migrated (ARCH-007)"**

Add a new subsection after "Remaining Worker Files Migrated (ARCH-006)":

```markdown
### Parent Job Orchestrator Migrated (ARCH-007)

**File Moved from processor/ to orchestrator/:**

1. **ParentJobOrchestrator** (`parent_job_orchestrator.go`)
   - Old: `internal/jobs/processor/parent_job_executor.go`
   - New: `internal/jobs/orchestrator/parent_job_orchestrator.go`
   - Struct: `ParentJobExecutor` → `ParentJobOrchestrator`
   - Constructor: `NewParentJobExecutor()` → `NewParentJobOrchestrator()`
   - Receiver: `func (e *ParentJobExecutor)` → `func (o *ParentJobOrchestrator)`
   - Dependencies: JobManager, EventService, Logger (3 dependencies)
   - Purpose: Monitors parent job progress in background goroutines, aggregates child job statistics, publishes real-time events

**Transformations Applied:**

- **Package**: `processor` → `orchestrator`
- **Struct**: `ParentJobExecutor` → `ParentJobOrchestrator`
- **Constructor**: `NewParentJobExecutor()` → `NewParentJobOrchestrator()`
- **Receiver**: `func (e *ParentJobExecutor)` → `func (o *ParentJobOrchestrator)`
- **Interface**: Updated `ParentJobOrchestrator` interface to match implementation signature

**Key Features Preserved:**

- **Background Monitoring**: Runs in separate goroutines (NOT via queue) to avoid blocking workers
- **Child Job Progress Tracking**: Polls child job statistics every 5 seconds
- **Real-Time Events**: Publishes progress updates via EventService for WebSocket streaming
- **Status Aggregation**: Calculates overall parent job status from child states
- **Event Subscriptions**: Subscribes to child job status changes and document_saved events
- **Document Count Tracking**: Increments document count in parent job metadata
- **Timeout Handling**: 30-minute maximum wait time for child jobs
- **Graceful Cancellation**: Respects context cancellation for clean shutdown
- **Comprehensive Logging**: Structured logging with correlation IDs for parent job aggregation

**Interface Updates:**

- Updated `ParentJobOrchestrator` interface in `orchestrator/interfaces.go` to match implementation:
  - `StartMonitoring(ctx context.Context, job *models.JobModel)` - Takes full job model (not just ID)
  - `SubscribeToChildStatusChanges()` - Sets up event subscriptions
  - Removed speculative methods: `StopMonitoring()`, `GetMonitoringStatus()` (not implemented)

**Import Path Updates:**

- `internal/app/app.go` (lines 22, 314, 319, 377) - Updated to import and use `orchestrator.NewParentJobOrchestrator()`
- `internal/jobs/executor/job_executor.go` (lines 11, 20, 25, 29, 370) - Updated field, parameter, and method call
- Variable renamed: `parentJobExecutor` → `parentJobOrchestrator`

**Comment Updates:**

- `internal/jobs/worker/job_processor.go` (lines 221, 227) - Updated comments
- `internal/interfaces/event_service.go` (lines 166, 177) - Updated event documentation
- `internal/jobs/manager.go` (line 1687) - Updated method documentation
- `test/api/places_job_document_test.go` (line 379) - Updated test comment

**Breaking Changes:**

- Old file deleted immediately (no backward compatibility)
- processor/ directory now empty (will be deleted in ARCH-008)
- All references updated to use orchestrator package

**Architectural Completion:**

This migration completes the Manager/Worker/Orchestrator separation:
- **Managers** (`internal/jobs/manager/`) - Create parent jobs, enqueue children (orchestration)
- **Workers** (`internal/jobs/worker/`) - Execute individual jobs from queue (execution)
- **Orchestrator** (`internal/jobs/orchestrator/`) - Monitor parent jobs, aggregate progress (monitoring)

The three layers are now clearly separated with distinct responsibilities.
```

**Implementation Notes:**
- Update migration status to show ARCH-007 complete
- Add detailed documentation of orchestrator migration
- Document interface updates and signature changes
- Clarify key features preserved during migration
- Emphasize architectural completion (Manager/Worker/Orchestrator separation)
- Provide context for developers working during transition
- Note that processor/ directory is now empty