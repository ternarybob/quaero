# Plan: Reorganize internal/jobs According to Domain Architecture

## Dependency Analysis

The reorganization follows a clear dependency chain:

1. **Foundation** - Create new folder structure (no dependencies)
2. **Content Extraction** - Split manager.go into domain-specific files (depends on #1)
3. **File Migration** - Move existing files to new locations (depends on #1, can run in parallel with #2)
4. **Import Updates** - Update all references throughout codebase (depends on #2 and #3)
5. **Cleanup** - Remove old structure (depends on #4)
6. **Verification** - Compile and test (depends on #5)

**Critical Path:** Steps 1 → 2 → 4 → 5 → 6 (manager.go split blocks import updates)

**Parallelization Opportunity:** Step 3 (file migration) can run in parallel with Step 2 (manager.go split)

## Critical Path Flags

- Step 2: high complexity (requires careful extraction of domain logic)
- Step 4: high complexity (breaking change to imports throughout codebase)
- Step 6: Critical validation step

## Execution Groups

### Group 1 (Sequential - Foundation)

These must run first, in order:

#### 1. Create New Folder Structure
- **Skill:** @go-coder
- **Files:** `internal/jobs/definitions/`, `internal/jobs/queue/managers/`, `internal/jobs/queue/workers/`, `internal/jobs/state/`
- **Complexity:** low
- **Critical:** no
- **Depends on:** none
- **User decision:** no
- **Description:** Create the three-domain folder structure: `definitions/`, `queue/managers/`, `queue/workers/`, `state/`

### Group 2 (Parallel - Content Preparation)

These can run simultaneously after Group 1:

#### 2a. Split manager.go into Domain-Specific Files
- **Skill:** @go-coder
- **Files:** `internal/jobs/manager.go` → `internal/jobs/queue/lifecycle.go`, `internal/jobs/state/runtime.go`, `internal/jobs/state/progress.go`, `internal/jobs/state/stats.go`
- **Complexity:** high
- **Critical:** yes:api-breaking
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-a
- **Description:** Extract domain-specific logic from manager.go into separate files aligned with the three-domain model. Queue domain gets immutable operations, State domain gets mutable runtime operations.

#### 2b. Move job_definition_orchestrator.go to definitions/
- **Skill:** @go-coder
- **Files:** `internal/jobs/job_definition_orchestrator.go` → `internal/jobs/definitions/orchestrator.go`
- **Complexity:** low
- **Critical:** no
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-b
- **Description:** Move orchestrator to definitions/ domain and update package declaration to `package definitions`

#### 2c. Move StepManager Implementations to queue/managers/
- **Skill:** @go-coder
- **Files:** `internal/jobs/manager/*.go` → `internal/jobs/queue/managers/*.go`
- **Complexity:** low
- **Critical:** no
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-c
- **Description:** Move all manager implementations (crawler, agent, etc.) to queue/managers/ and update package declarations to `package managers`

#### 2d. Move JobWorker Implementations to queue/workers/
- **Skill:** @go-coder
- **Files:** `internal/jobs/worker/*.go` → `internal/jobs/queue/workers/*.go`
- **Complexity:** low
- **Critical:** no
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-d
- **Description:** Move all worker implementations to queue/workers/ and update package declarations to `package workers`

#### 2e. Move JobMonitor to state/
- **Skill:** @go-coder
- **Files:** `internal/jobs/monitor/job_monitor.go` → `internal/jobs/state/monitor.go`
- **Complexity:** low
- **Critical:** no
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-e
- **Description:** Move monitor to state/ domain and update package declaration to `package state`

### Group 3 (Sequential - Integration)

Runs after Group 2 completes:

#### 3. Update All Import Statements Throughout Codebase
- **Skill:** @go-coder
- **Files:** All Go files importing from `internal/jobs`
- **Complexity:** high
- **Critical:** yes:api-breaking
- **Depends on:** 2a, 2b, 2c, 2d, 2e
- **User decision:** no
- **Description:** Update all import statements throughout the codebase to reference the new folder structure. Use grep to find all references and update systematically.

#### 4. Remove Old Folder Structure
- **Skill:** @go-coder
- **Files:** `internal/jobs/manager/`, `internal/jobs/worker/`, `internal/jobs/monitor/`, `internal/jobs/manager.go`, `internal/jobs/job_definition_orchestrator.go`
- **Complexity:** low
- **Critical:** no
- **Depends on:** Step 3
- **User decision:** no
- **Description:** Delete the old folder structure and files after verifying all imports are updated

#### 5. Verification and Testing
- **Skill:** @go-coder
- **Files:** All Go files in project
- **Complexity:** medium
- **Critical:** yes:build-verification
- **Depends on:** Step 4
- **User decision:** no
- **Description:** Compile the entire codebase and run tests to verify the reorganization is complete and functional

### Group 4 (Sequential - Documentation)

Runs after Group 3 completes:

#### 6. Update Architecture Documentation
- **Skill:** @none
- **Files:** `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
- **Complexity:** low
- **Critical:** no
- **Depends on:** Step 5
- **User decision:** no
- **Description:** Update the architecture documentation to reflect the new folder structure and file locations

## Parallel Execution Map

```
[Step 1: Create Folders] ──┬──> [Step 2a: Split manager.go*] ──┐
                           ├──> [Step 2b: Move orchestrator]  ──┤
                           ├──> [Step 2c: Move managers]      ──┤
                           ├──> [Step 2d: Move workers]       ──┼──> [Step 3: Update Imports*] ──> [Step 4: Cleanup] ──> [Step 5: Verify*] ──> [Step 6: Docs]
                           └──> [Step 2e: Move monitor]       ──┘

* = High complexity (extended attention required)
```

## Final Review Triggers

Steps flagged for careful review:
- 2a: api-breaking (complex extraction logic)
- 3: api-breaking (system-wide import changes)
- 5: build-verification (ensures nothing broke)

## Success Criteria

1. **Folder Structure Aligned:**
   - `internal/jobs/definitions/` contains job definition orchestration
   - `internal/jobs/queue/managers/` contains all StepManager implementations
   - `internal/jobs/queue/workers/` contains all JobWorker implementations
   - `internal/jobs/state/` contains JobMonitor and runtime state management

2. **manager.go Properly Split:**
   - `queue/lifecycle.go` - Immutable queue operations (CreateQueueJob, EnqueueJob, GetQueueJob)
   - `state/runtime.go` - Mutable state operations (UpdateJobStatus, SetJobError, UpdateJobTimestamps)
   - `state/progress.go` - Progress tracking (UpdateJobProgress, IncrementDocumentCount)
   - `state/stats.go` - Statistics aggregation (GetJobTreeStatus, GetChildJobStats)

3. **Package Names Match Domains:**
   - `package definitions` for job definition orchestration
   - `package managers` for StepManager implementations
   - `package workers` for JobWorker implementations
   - `package state` for runtime state management

4. **All Imports Updated:**
   - No references to old paths (`internal/jobs/manager`, `internal/jobs/worker`, `internal/jobs/monitor`)
   - All imports use new domain-aligned paths

5. **Clean Compilation:**
   - `go build ./...` succeeds without errors
   - All tests pass: `go test ./test/api/...` and `go test ./test/ui/...`

6. **Old Structure Removed:**
   - No old folders remain (`manager/`, `worker/`, `monitor/`)
   - Original `manager.go` and `job_definition_orchestrator.go` deleted from root

7. **Documentation Updated:**
   - Architecture document reflects new structure
   - File paths in documentation match actual structure

## Implementation Notes

### Domain Separation Principles

**Jobs Domain** (`definitions/`):
- User-defined workflows
- Job definition orchestration
- NO queue operations, NO runtime state

**Queue Domain** (`queue/`):
- Immutable queued work
- Job creation and enqueueing
- Job execution by workers
- NO mutable runtime state

**Queue State Domain** (`state/`):
- Runtime execution information
- Progress tracking and statistics
- Job status and error management
- Event-driven state updates

### manager.go Split Strategy

The key challenge is extracting logic while maintaining functionality. The split must preserve:

1. **Public API:** All exported functions must remain accessible through the new structure
2. **Dependencies:** Functions that call each other must be in the same package or properly imported
3. **Interface Compliance:** Types implementing interfaces must maintain compatibility

**Analysis of manager.go functions:**

**Queue Domain (Immutable):**
- `CreateJobRecord()` - Creates QueueJob
- `GetJob()` - Retrieves QueueJob
- `GetJobs()` - Lists QueueJobs
- `GetJobsByParent()` - Queries by parent

**State Domain (Mutable):**
- `UpdateJobStatus()` - Updates status
- `GetJobWithStatus()` - Retrieves with status
- `SetJobError()` - Sets error message
- `UpdateJobStarted/Completed/Finished()` - Timestamp management
- `UpdateJobProgress()` - Progress counters
- `IncrementDocumentCount()` - Result tracking
- `GetJobTreeStatus()` - Aggregated status
- `GetChildJobStats()` - Child statistics
- `GetFailedChildCount()` - Failed child count

### Import Update Strategy

Use systematic grep-based find and replace:

```bash
# Find all imports
grep -r "internal/jobs/manager" --include="*.go" .
grep -r "internal/jobs/worker" --include="*.go" .
grep -r "internal/jobs/monitor" --include="*.go" .

# Update imports
find . -name "*.go" -exec sed -i 's|internal/jobs/manager|internal/jobs/queue/managers|g' {} +
find . -name "*.go" -exec sed -i 's|internal/jobs/worker|internal/jobs/queue/workers|g' {} +
find . -name "*.go" -exec sed -i 's|internal/jobs/monitor|internal/jobs/state|g' {} +
```

## Risk Assessment

**Low Risk:**
- Folder creation (Step 1)
- Simple file moves with package updates (Steps 2b, 2c, 2d, 2e)
- Documentation updates (Step 6)

**Medium Risk:**
- Import updates (Step 3) - Many files affected, but mechanical change
- Old structure removal (Step 4) - Can be undone if issues found

**High Risk:**
- manager.go split (Step 2a) - Complex logic extraction, potential for breaking interfaces

**Mitigation:**
- Keep original manager.go during development
- Compile after each domain file is complete
- Only remove originals after full verification
- Git commit after each major step for easy rollback
