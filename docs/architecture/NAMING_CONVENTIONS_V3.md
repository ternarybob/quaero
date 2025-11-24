# Naming Conventions V3 (Revised)

**Version:** 3.0
**Last Updated:** 2025-11-24
**Status:** Approved - Ready for Implementation

## Executive Summary

This document defines the **final** naming conventions for Quaero's job system, providing clear separation between three domains:

1. **Jobs Domain** - Job definitions and workflows
2. **Queue Domain** - Queued work items
3. **Queue State Domain** - Runtime execution information

## Three Clear Domains

### 1. Jobs Domain (Definitions)

**Purpose:** User-defined workflows and job configurations

**Naming Pattern:** `Job` or `JobDefinition` prefix

**Examples:**
- `Job` or `JobDefinition` - User-defined workflow
- `JobStep` - Step in a job definition
- `JobDefinitionType` - Type of job definition (crawler, agent, etc.)

**Storage:** `job_definitions` table in BadgerDB

**Managed By:** Jobs page UI, JobDefinitionOrchestrator

**Key Characteristics:**
- User-editable via UI
- Defines WHAT work to do
- Contains workflow steps and configuration
- Can be scheduled (cron expressions)

### 2. Queue Domain (Queued Work)

**Purpose:** Immutable work items sent to queue for execution

**Naming Pattern:** `Queue` prefix for all queue-related types and operations

**Examples:**
- `QueueJob` - Immutable job sent to queue
- `NewQueueJob()` - Create root queued job
- `NewQueueJobChild()` - Create child queued job
- `QueueJobFromJSON()` - Deserialize queued job

**Storage:** `jobs` table in BadgerDB (stores ONLY QueueJob, no runtime state)

**Managed By:** StepManagers (create), JobWorkers (execute)

**Key Characteristics:**
- IMMUTABLE after creation
- Contains snapshot of configuration at creation time
- Defines work to be done
- No runtime state (Status, Progress)

### 3. Queue State Domain (Runtime Information)

**Purpose:** Runtime execution state and progress tracking

**Naming Pattern:** `QueueJobState` prefix for runtime state

**Examples:**
- `QueueJobState` - Runtime execution state (in-memory)
- `NewQueueJobState()` - Create state from queued job
- `QueueJobState.ToQueueJob()` - Extract immutable job
- `QueueJobState.MarkStarted()` - Update runtime status

**Storage:** NOT stored in database (reconstructed from QueueJob + job logs)

**Managed By:** JobWorkers (update), JobMonitor (aggregate)

**Key Characteristics:**
- Mutable during execution
- Combines QueueJob fields + runtime state
- Tracked via job logs/events
- In-memory only

## Complete Type Mapping

### V1 → V2 → V3 (Final)

| V1 Name | V2 Name (Phase 1) | **V3 Name (Final)** | Domain | Purpose |
|---------|-------------------|---------------------|--------|---------|
| `JobModel` | `JobQueued` | **`QueueJob`** | Queue | Immutable queued job |
| `Job` | `JobExecutionState` | **`QueueJobState`** | Queue State | Runtime state |
| `NewJobModel()` | `NewJobQueued()` | **`NewQueueJob()`** | Queue | Create queued job |
| `NewChildJobModel()` | `NewJobQueuedChild()` | **`NewQueueJobChild()`** | Queue | Create child job |
| `NewJob()` | `NewJobExecutionState()` | **`NewQueueJobState()`** | Queue State | Create state |
| `Job.ToJobModel()` | `JobExecutionState.ToJobQueued()` | **`QueueJobState.ToQueueJob()`** | Queue State | Extract queued job |
| `JobModel.FromJSON()` | `JobQueued.FromJSON()` | **`QueueJobFromJSON()`** | Queue | Deserialize |

### Jobs Domain Types (Unchanged)

| Type | Purpose |
|------|---------|
| `Job` or `JobDefinition` | User-defined workflow |
| `JobStep` | Step in job definition |
| `JobDefinitionType` | Type of job definition |
| `JobStatus` | Job status enum (pending, running, completed, failed) |
| `JobProgress` | Progress counters struct |

## Naming Rules

### Rule 1: Jobs vs Queue Separation

**Jobs** = Definitions (what to do)
**Queue** = Execution (doing it)

```go
// ✅ CORRECT: Clear separation
type Job struct { ... }              // Jobs domain - definition
type QueueJob struct { ... }         // Queue domain - queued work
type QueueJobState struct { ... }    // Queue State domain - runtime info

// ❌ WRONG: Ambiguous
type Job struct { ... }              // Which domain?
type JobModel struct { ... }         // Not clear it's queued work
type JobExecutionState struct { ... } // Too verbose, not clear it's queue-related
```

### Rule 2: Queue Prefix for All Queue Operations

**All queue-related types and functions use `Queue` prefix**

```go
// ✅ CORRECT: Queue prefix
func NewQueueJob() *QueueJob
func NewQueueJobChild() *QueueJob
func (q *QueueJobState) ToQueueJob() *QueueJob
func QueueJobFromJSON(data []byte) (*QueueJob, error)

// ❌ WRONG: Missing Queue prefix
func NewJob() *Job                   // Ambiguous - which domain?
func NewJobModel() *JobModel         // Not clear it's queue-related
func (j *Job) ToJobModel() *JobModel // Confusing naming
```

### Rule 3: State Suffix for Runtime Information

**Runtime state uses `State` suffix**

```go
// ✅ CORRECT: State suffix
type QueueJobState struct { ... }    // Clear it's runtime state
func NewQueueJobState(qj *QueueJob) *QueueJobState

// ❌ WRONG: No State suffix
type QueueJob struct {               // Looks like immutable job
    Status JobStatus                 // But has runtime state!
    Progress JobProgress
}
```

## Code Examples

### Creating a Queued Job

```go
// Create root queued job
queueJob := models.NewQueueJob(
    "crawler",
    "Crawl example.com",
    config,
    metadata,
)

// Create child queued job
childJob := models.NewQueueJobChild(
    parentJob,
    "crawler_url",
    "Crawl /page1",
    childConfig,
)
```

### Worker Execution

```go
// Worker receives QueueJob from queue
func (w *CrawlerWorker) Execute(ctx context.Context, queueJob *models.QueueJob) error {
    // Create runtime state
    state := models.NewQueueJobState(queueJob)
    
    // Update state during execution
    state.MarkStarted()
    state.UpdateProgress(10, 0, 100, 0)
    
    // Execute work...
    
    // Mark completed
    state.MarkCompleted()
    
    return nil
}
```

### Storage Operations

```go
// Storage saves ONLY QueueJob (immutable)
func (s *JobStorage) SaveJob(ctx context.Context, job interface{}) error {
    state, ok := job.(*models.QueueJobState)
    if !ok {
        return fmt.Errorf("invalid job type")
    }
    
    // Extract immutable QueueJob
    queueJob := state.ToQueueJob()
    
    // Store ONLY QueueJob (no runtime state)
    return s.db.Store().Upsert(queueJob.ID, queueJob)
}

// Storage loads QueueJob and converts to QueueJobState
func (s *JobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
    var queueJob models.QueueJob
    if err := s.db.Store().Get(jobID, &queueJob); err != nil {
        return nil, err
    }
    
    // Convert to QueueJobState for in-memory use
    return models.NewQueueJobState(&queueJob), nil
}
```

## Interface Signatures

### StepManager

```go
type StepManager interface {
    CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (jobID string, err error)
    GetManagerType() string
    ReturnsChildJobs() bool
}
```

### JobWorker

```go
type JobWorker interface {
    Execute(ctx context.Context, job *models.QueueJob) error
    GetWorkerType() string
    Validate(job *models.QueueJob) error
}
```

### JobMonitor

```go
type JobMonitor interface {
    StartMonitoring(ctx context.Context, job *models.QueueJob)
    SubscribeToJobEvents()
}
```

### JobStorage

```go
type JobStorage interface {
    SaveJob(ctx context.Context, job interface{}) error
    GetJob(ctx context.Context, jobID string) (interface{}, error)
    ListJobs(ctx context.Context, opts *JobListOptions) ([]*models.QueueJobState, error)
    GetChildJobs(ctx context.Context, parentID string) ([]*models.QueueJob, error)
    GetJobsByStatus(ctx context.Context, status string) ([]*models.QueueJob, error)
    GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]*models.QueueJob, error)
    // ... other methods
}
```

## Benefits of This Naming

### 1. Clear Domain Separation

**Before (V1):**
```go
type Job struct { ... }              // Which domain? Definition or execution?
type JobModel struct { ... }         // What's a "model"?
```

**After (V3):**
```go
type Job struct { ... }              // Jobs domain - definition
type QueueJob struct { ... }         // Queue domain - queued work
type QueueJobState struct { ... }    // Queue State domain - runtime info
```

### 2. Intuitive Naming

**Before (V1):**
```go
NewJobModel()                        // What's a "model"?
NewChildJobModel()                   // Confusing
job.ToJobModel()                     // Which direction?
```

**After (V3):**
```go
NewQueueJob()                        // Clear: creates queued job
NewQueueJobChild()                   // Clear: creates child queued job
state.ToQueueJob()                   // Clear: extracts queued job from state
```

### 3. Consistent Prefixes

**All queue-related operations use `Queue` prefix:**
- `QueueJob` - The job itself
- `QueueJobState` - Runtime state
- `NewQueueJob()` - Constructor
- `QueueJobFromJSON()` - Deserializer

**All job definition operations use `Job` prefix:**
- `Job` or `JobDefinition` - The definition
- `JobStep` - Step in definition
- `JobDefinitionType` - Type enum

## Migration Impact

### Files Requiring Updates

**Core Models:**
- `internal/models/job_model.go` - Rename structs and methods

**Interfaces:**
- `internal/interfaces/job_interfaces.go` - Update signatures
- `internal/interfaces/queue_service.go` - Update signatures
- `internal/interfaces/storage.go` - Update signatures

**Implementation (50+ files):**
- All managers, workers, monitors
- All services, handlers
- All storage implementations
- All tests

### Estimated Effort

**Total Time:** 3-4 hours
**Breaking Changes:** Yes (acceptable)
**Backward Compatibility:** JSON API unchanged (field names same)

## Approval Status

**Status:** ✅ Approved by user on 2025-11-24

**Rationale:** Provides clear separation between:
1. Jobs (definitions) - what to do
2. Queue (queued work) - work to be done
3. QueueState (runtime info) - how it's going

**Next Steps:** Execute Phase 2 migration plan (see PHASE2_REVISED_PLAN.md)

