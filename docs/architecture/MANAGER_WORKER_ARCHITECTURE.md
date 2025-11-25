# Manager/Worker Architecture

**Version:** 2.0 (Revised Naming - IMPLEMENTED)
**Last Updated:** 2025-11-25
**Migration Status:** ✅ Complete - All phases finished (Steps 0-13)
**Migration Details:** See `docs/features/refactor-job-queues/MIGRATION_COMPLETE_SUMMARY.md`

## Executive Summary

Quaero's job system implements a **Manager/Worker/Monitor pattern** with clear separation between job states and operations:

### Three Domains

1. **Jobs Domain** - User-defined workflows (JobDefinition or Job)
   - Located in: `internal/jobs/definitions/`
2. **Queue Domain** - Immutable queued work (QueueJob)
   - Located in: `internal/jobs/queue/`
3. **Queue State Domain** - Runtime execution information (QueueJobState)
   - Located in: `internal/jobs/state/`

### Three Job Operations

1. **JobManager (StepManager)** - Orchestrates job execution, creates parent jobs
2. **JobWorker** - Executes individual jobs from queue
3. **JobMonitor** - Watches job logs/events, stores runtime state against worker ID

## Key Architectural Principles

**Immutability:** Once a job is enqueued (`QueueJob`), it is immutable. Runtime state (Status, Progress) is tracked via job logs/events, NOT in the stored job.

**Separation of Concerns:**
- **Job/JobDefinition** = What to do (user-defined workflow)
- **QueueJob** = Work to be done (immutable task definition)
- **QueueJobState** = How it's going (runtime state, in-memory only)

**Clear Naming:**
- **Jobs** prefix = Job definitions and workflows
- **Queue** prefix = Queue-related operations and state

**Event-Driven State:** Job status changes are published as events and stored in job logs. The `JobMonitor` aggregates these events to track overall job progress.

**Domain-Based Organization:** The folder structure enforces the three-domain model with clear boundaries between job definitions, queue operations, and runtime state.

## Folder Structure

The job system is organized into three distinct domains:

### Jobs Domain - Definitions (`internal/jobs/definitions/`)
- **`orchestrator.go`** - `JobDefinitionOrchestrator`
  - Routes job definition steps to appropriate StepManagers
  - Coordinates overall workflow execution

### Queue Domain - Immutable Operations (`internal/jobs/queue/`)
- **`lifecycle.go`** - Job lifecycle management (immutable operations)
  - Job creation (`CreateJob`, `CreateChildJob`)
  - Job retrieval (`GetJob`, `ListJobs`)
  - Queue enqueue/dequeue operations
- **`managers/`** - StepManager implementations
  - `crawler_manager.go` - Handles "crawl" action
  - `transform_manager.go` - Handles "transform" action
  - `reindex_manager.go` - Handles "reindex" action
  - `places_search_manager.go` - Handles "places_search" action
  - `agent_manager.go` - Handles "agent" action
  - `database_maintenance_manager.go` - Handles "database_maintenance" action
- **`workers/`** - JobWorker implementations
  - `crawler_worker.go` - Executes crawler jobs
  - `agent_worker.go` - Executes agent jobs
  - `github_log_worker.go` - Executes GitHub logging jobs
  - `database_maintenance_worker.go` - Executes maintenance jobs
  - `job_processor.go` - Routes jobs to appropriate workers

### Queue State Domain - Runtime Operations (`internal/jobs/state/`)
- **`runtime.go`** - Status and error management (mutable operations)
  - `UpdateJobStatus` - Update job execution status
  - `MarkJobStarted/Completed/Failed` - Status transitions
  - `SetJobError` - Error tracking
- **`progress.go`** - Progress tracking
  - `UpdateJobProgress` - Update progress counters
  - `IncrementProcessed/Failed` - Atomic counter updates
- **`stats.go`** - Statistics aggregation
  - `GetJobStats` - Aggregate statistics for parent jobs
  - `CalculateCompletionPercentage` - Progress calculations
- **`monitor.go`** - `JobMonitor` implementation
  - Job completion monitoring
  - Event aggregation and WebSocket publishing

### Responsibility Separation

**Queue Manager (`queue/lifecycle.go`)** - Immutable operations:
- Creating jobs (parent and child)
- Retrieving jobs
- Enqueuing/dequeuing messages
- NO status updates or mutations

**State Manager (`state/runtime.go`, `state/progress.go`, `state/stats.go`)** - Mutable operations:
- Status updates and transitions
- Progress tracking and counters
- Error recording
- Statistics aggregation
- NO job creation or retrieval

This separation ensures:
- Clear domain boundaries
- Immutable queue operations separate from mutable state tracking
- Easy testing and maintenance
- Enforcement of architectural principles at the folder level

## Architecture Overview

```mermaid
graph TB
    subgraph "Jobs Domain - Definitions"
        UI[Web UI] -->|Create/Edit| JobDef[Job/JobDefinition]
        JobDef -->|Stored in| DefDB[(Job Definitions DB)]
    end

    subgraph "Queue Domain - Orchestration"
        UI -->|Trigger| Orchestrator[JobDefinitionOrchestrator]
        Orchestrator -->|Route Step| StepMgr[StepManager]
        StepMgr -->|Create Parent| QueueJob[QueueJob Record]
        QueueJob -->|Store| QueueDB[(BadgerDB - Jobs)]
        StepMgr -->|Enqueue Children| Queue[BadgerDB - Queue]
    end

    subgraph "Queue Domain - Execution"
        Queue -->|Dequeue| Processor[JobProcessor]
        Processor -->|Route by Type| Worker[JobWorker]
        Worker -->|Load| QueueJob
        Worker -->|Create| QueueState[QueueJobState]
        Worker -->|Execute| Service[External Service]
        Worker -->|Save Results| Storage[Data Storage]
        Worker -->|Spawn Children| Queue
        Worker -->|Publish Events| Events[Job Events]
    end

    subgraph "Queue State Domain - Monitoring"
        Monitor[JobMonitor] -->|Watch| Events
        Monitor -->|Aggregate Stats| JobLogs[(Job Logs DB)]
        Monitor -->|Publish Progress| WS[WebSocket]
        WS -->|Real-time Updates| UI
    end
```

## Job State Lifecycle

```
1. User creates Job/JobDefinition via UI (Jobs Domain)
   ↓
2. User triggers job execution
   ↓
3. JobDefinitionOrchestrator routes to StepManager
   ↓
4. StepManager creates QueueJob (parent) and stores in BadgerDB (Queue Domain)
   ↓
5. StepManager enqueues child QueueJob records to queue
   ↓
6. JobProcessor dequeues QueueJob from queue
   ↓
7. JobWorker loads QueueJob and creates QueueJobState (in-memory, Queue State Domain)
   ↓
8. JobWorker executes task, publishes status events
   ↓
9. JobMonitor watches events, updates job logs (Queue State Domain)
   ↓
10. JobWorker completes, publishes completion event
   ↓
11. JobMonitor aggregates child stats, determines parent completion
```

## Core Data Structures

### JobDefinition (User-Defined Workflow)

**File:** `internal/models/job_definition.go`

```go
type JobDefinition struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Type        JobDefinitionType `json:"type"` // crawler, agent, places, custom
    Description string    `json:"description"`
    Schedule    string    `json:"schedule"` // Cron expression (optional)
    Steps       []JobStep `json:"steps"`    // Workflow steps
    Enabled     bool      `json:"enabled"`
    AuthID      string    `json:"auth_id"`  // Authentication credentials
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

**Purpose:** Defines WHAT work to do and HOW to orchestrate it
**Storage:** BadgerDB (job_definitions table)
**Mutability:** Editable by user via UI

### QueueJob (Immutable Queued Job)

**File:** `internal/models/job_model.go`

```go
// QueueJob represents the immutable job sent to the queue and stored in the database.
// Once created and enqueued, this job should not be modified.
type QueueJob struct {
    // Core identification
    ID       string  `json:"id"`        // Unique job ID (UUID)
    ParentID *string `json:"parent_id"` // Parent job ID (nil for root)

    // Job classification
    Type string `json:"type"` // Job type: "crawler", "agent", etc.
    Name string `json:"name"` // Human-readable name

    // Configuration (immutable snapshot at creation time)
    Config   map[string]interface{} `json:"config"`
    Metadata map[string]interface{} `json:"metadata"`

    // Timestamps
    CreatedAt time.Time `json:"created_at"`

    // Hierarchy tracking
    Depth int `json:"depth"` // 0 for root, 1+ for children
}
```

**Purpose:** Immutable work definition sent to queue
**Storage:** BadgerDB (jobs table) - stores ONLY this, no runtime state
**Mutability:** IMMUTABLE after creation
**Key Methods:**
- `NewQueueJob()` - Create root job
- `NewQueueJobChild()` - Create child job
- `Validate()` - Validate job structure
- `GetConfigString/Int/Bool()` - Extract config values

### QueueJobState (Runtime Execution State)

**File:** `internal/models/job_model.go`

```go
// QueueJobState represents runtime execution state for a queued job (in-memory only)
// This combines the immutable QueueJob fields with mutable runtime state
// Runtime state (Status, Progress) should be tracked via job logs/events, not stored in database
type QueueJobState struct {
    // Fields from QueueJob (immutable)
    ID        string
    ParentID  *string
    Type      string
    Name      string
    Config    map[string]interface{}
    Metadata  map[string]interface{}
    CreatedAt time.Time
    Depth     int

    // Mutable runtime state (tracked via job logs/events)
    Status        JobStatus   `json:"status"`        // pending, running, completed, failed
    Progress      JobProgress `json:"progress"`      // Execution progress
    StartedAt     *time.Time  `json:"started_at"`
    CompletedAt   *time.Time  `json:"completed_at"`
    Error         string      `json:"error"`
    ResultCount   int         `json:"result_count"`
    FailedCount   int         `json:"failed_count"`
}
```

**Purpose:** In-memory runtime state during execution
**Storage:** NOT stored in database (reconstructed from QueueJob + job logs)
**Mutability:** Mutable during execution
**Key Methods:**
- `NewQueueJobState(queueJob *QueueJob)` - Create from queued job
- `ToQueueJob()` - Extract immutable job
- `MarkStarted/Completed/Failed()` - Update status
- `UpdateProgress()` - Update progress counters

## Interface Definitions

### StepManager (Job Orchestration)

**File:** `internal/interfaces/job_interfaces.go`

```go
// StepManager creates parent jobs and orchestrates job definition steps
type StepManager interface {
    // CreateParentJob creates a parent job and spawns initial child jobs
    CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (jobID string, err error)

    // GetManagerType returns the action type this manager handles (e.g., "crawl")
    GetManagerType() string
}
```

**Implementations:** (Located in `internal/jobs/queue/managers/`)
- `CrawlerManager` (`crawler_manager.go`) - Handles "crawl" action
- `AgentManager` (`agent_manager.go`) - Handles "agent" action
- `DatabaseMaintenanceManager` (`database_maintenance_manager.go`) - Handles "database_maintenance" action
- `TransformManager` (`transform_manager.go`) - Handles "transform" action
- `ReindexManager` (`reindex_manager.go`) - Handles "reindex" action
- `PlacesSearchManager` (`places_search_manager.go`) - Handles "places_search" action

**Responsibilities:**
1. Create parent `QueueJob` record in database (via `queue/lifecycle.go`)
2. Define work items (e.g., URLs to crawl, documents to process)
3. Enqueue child `QueueJob` records to queue (via `queue/lifecycle.go`)
4. NO direct execution - delegates to workers
5. NO status updates - delegates to state management

### JobWorker (Job Execution)

**File:** `internal/interfaces/job_interfaces.go`

```go
// JobWorker executes individual jobs from the queue
type JobWorker interface {
    // Execute processes a single job from the queue
    Execute(ctx context.Context, job *models.QueueJob) error

    // GetWorkerType returns the job type this worker handles
    GetWorkerType() string

    // Validate validates that the job is compatible with this worker
    Validate(job *models.QueueJob) error
}
```

**Implementations:** (Located in `internal/jobs/queue/workers/`)
- `CrawlerWorker` (`crawler_worker.go`) - Executes crawler jobs
- `AgentWorker` (`agent_worker.go`) - Executes agent jobs
- `DatabaseMaintenanceWorker` (`database_maintenance_worker.go`) - Executes maintenance jobs
- `GitHubLogWorker` (`github_log_worker.go`) - Executes GitHub logging jobs
- `JobProcessor` (`job_processor.go`) - Routes jobs to appropriate workers

**Responsibilities:**
1. Load `QueueJob` from queue (via `queue/lifecycle.go`)
2. Create `QueueJobState` for in-memory tracking
3. Execute task (fetch URL, run agent, etc.)
4. Publish status events (started, progress, completed) - tracked by `state/monitor.go`
5. Update job status via `state/runtime.go` (started, completed, failed)
6. Update progress counters via `state/progress.go`
7. Spawn child jobs if needed (URL discovery) via `queue/lifecycle.go`
8. Save results to storage

### JobMonitor (Progress Monitoring)

**File:** `internal/interfaces/job_interfaces.go`

```go
// JobMonitor monitors parent job progress and aggregates child statistics
type JobMonitor interface {
    // StartMonitoring begins monitoring a parent job
    StartMonitoring(ctx context.Context, parentJobID string) error

    // StopMonitoring stops monitoring a parent job
    StopMonitoring(parentJobID string) error

    // GetJobProgress returns current progress for a job
    GetJobProgress(ctx context.Context, jobID string) (*JobProgress, error)
}
```

**Implementation:** `internal/jobs/state/monitor.go`

**Responsibilities:**
1. Subscribe to job events (started, progress, completed)
2. Aggregate child job statistics (via `state/stats.go`)
3. Determine parent job completion
4. Publish progress updates via WebSocket
5. Store job logs in database
6. Coordinate with `state/runtime.go` for status updates
7. Use `state/progress.go` for progress tracking

## Data Flow Between Domains

```
┌─────────────────────────────────────────────────────────────────────┐
│ Jobs Domain (internal/jobs/definitions/)                            │
│                                                                      │
│ orchestrator.go - JobDefinitionOrchestrator                         │
│   └─> Routes job steps to appropriate StepManagers                  │
└──────────────────────────┬───────────────────────────────────────────┘
                           │
                           v
┌─────────────────────────────────────────────────────────────────────┐
│ Queue Domain (internal/jobs/queue/)                                 │
│                                                                      │
│ managers/ - StepManager implementations                             │
│   ├─> crawler_manager.go                                            │
│   ├─> agent_manager.go                                              │
│   ├─> database_maintenance_manager.go                               │
│   ├─> transform_manager.go                                          │
│   ├─> reindex_manager.go                                            │
│   └─> places_search_manager.go                                      │
│                                                                      │
│ lifecycle.go - Queue Manager (Immutable Operations)                 │
│   ├─> CreateJob() - Create parent QueueJob                          │
│   ├─> CreateChildJob() - Create child QueueJob                      │
│   ├─> EnqueueJob() - Add to queue                                   │
│   ├─> DequeueJob() - Get from queue                                 │
│   └─> GetJob() - Retrieve QueueJob                                  │
│                                                                      │
│ workers/ - JobWorker implementations                                │
│   ├─> job_processor.go - Routes to workers                          │
│   ├─> crawler_worker.go                                             │
│   ├─> agent_worker.go                                               │
│   ├─> github_log_worker.go                                          │
│   └─> database_maintenance_worker.go                                │
└──────────────────────────┬───────────────────────────────────────────┘
                           │
                           v
┌─────────────────────────────────────────────────────────────────────┐
│ Queue State Domain (internal/jobs/state/)                           │
│                                                                      │
│ runtime.go - State Manager (Mutable Operations)                     │
│   ├─> UpdateJobStatus() - Change status                             │
│   ├─> MarkJobStarted() - Start execution                            │
│   ├─> MarkJobCompleted() - Complete execution                       │
│   ├─> MarkJobFailed() - Record failure                              │
│   └─> SetJobError() - Store error                                   │
│                                                                      │
│ progress.go - Progress Tracking                                     │
│   ├─> UpdateJobProgress() - Update counters                         │
│   ├─> IncrementProcessed() - Increment success                      │
│   └─> IncrementFailed() - Increment failures                        │
│                                                                      │
│ stats.go - Statistics Aggregation                                   │
│   ├─> GetJobStats() - Aggregate child stats                         │
│   └─> CalculateCompletionPercentage()                               │
│                                                                      │
│ monitor.go - JobMonitor                                             │
│   ├─> StartMonitoring() - Begin monitoring                          │
│   ├─> StopMonitoring() - End monitoring                             │
│   ├─> GetJobProgress() - Get current progress                       │
│   └─> Uses runtime.go, progress.go, stats.go                        │
└─────────────────────────────────────────────────────────────────────┘
```

### Interaction Flow

1. **Job Creation Flow** (Jobs Domain -> Queue Domain)
   ```
   JobDefinitionOrchestrator -> StepManager -> queue/lifecycle.go
   ```

2. **Job Execution Flow** (Queue Domain -> Queue State Domain)
   ```
   queue/lifecycle.go -> JobWorker -> state/runtime.go -> state/progress.go
   ```

3. **Monitoring Flow** (Queue State Domain)
   ```
   state/monitor.go -> state/stats.go -> state/runtime.go -> state/progress.go
   ```

4. **Status Update Flow** (Queue State Domain only)
   ```
   JobWorker -> state/runtime.go -> state/progress.go -> state/monitor.go
   ```

## Storage Architecture

### BadgerDB Tables

**jobs** - Stores `QueueJob` (immutable queued jobs)
```
Key: job_id (string)
Value: QueueJob struct (JSON serialized)
```

**job_logs** - Stores job events and status changes
```
Key: log_id (string)
Value: JobLog struct with job_id, event_type, payload, timestamp
```

**queue** - Stores queued messages for worker processing
```
Key: message_id (string)
Value: QueueMessage struct with job_id, job_type, visibility_timeout
```

### Key Storage Principle

**CRITICAL:** BadgerDB stores ONLY `QueueJob` (immutable job definition), NOT `QueueJobState` (runtime state).

Runtime state is tracked via:
1. Job events published by workers
2. Job logs stored in `job_logs` table
3. JobMonitor aggregating events into progress statistics

This solves the BadgerHold serialization issue by avoiding complex nested structs with runtime state.

## Benefits of Domain-Based Organization

The new folder structure provides several key benefits:

### 1. Clear Separation of Concerns
- **Jobs Domain** (`internal/jobs/definitions/`) handles user-defined workflows
- **Queue Domain** (`internal/jobs/queue/`) manages immutable job operations
- **Queue State Domain** (`internal/jobs/state/`) tracks mutable runtime state

### 2. Enforced Immutability
- Queue operations (`queue/lifecycle.go`) cannot modify job state
- State operations (`state/runtime.go`, `state/progress.go`) cannot create jobs
- Architectural principles are enforced at the file system level

### 3. Easier Navigation and Maintenance
- Developers can quickly find the right file for their task
- Related functionality is grouped together
- Clear boundaries prevent cross-domain pollution

### 4. Improved Testability
- Each domain can be tested independently
- Mock implementations are easier to create
- Integration tests have clear boundaries

### 5. Scalability
- New managers/workers can be added without touching other domains
- State tracking can be extended without affecting queue operations
- Each domain can evolve independently

### 6. Documentation Alignment
- Folder structure matches architectural diagrams
- Code organization reflects conceptual model
- New developers can understand the system faster

## Migration Plan (Incremental Steps)

See separate document: `docs/architecture/MIGRATION_V1_TO_V2.md`

