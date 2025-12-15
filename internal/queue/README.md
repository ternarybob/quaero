# Queue Domain

This package contains the **Queue Domain** - all job execution operations and runtime state tracking.

## Architecture

The Queue Domain is part of the Manager/Worker/Monitor architecture. See `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` for the complete architecture documentation.

### Key Concept

The Queue Domain manages HOW work gets executed. It handles:
- Job creation and lifecycle (immutable operations)
- Queue message management (enqueue, dequeue, extend)
- Worker execution (processing queued jobs)
- Runtime state tracking (status, progress, monitoring)

### Separation of Concerns

| Domain | Location | Purpose |
|--------|----------|---------|
| **Jobs** | `internal/jobs/` | User-defined workflows (WHAT to do) |
| **Queue** | `internal/queue/` | Job execution and runtime state (HOW to do it) |

## Package Structure

```
internal/queue/
├── badger_manager.go      # Badger-backed queue manager
├── manager.go             # Job creation and retrieval (immutable)
├── orchestrator.go        # Orchestrator - routes job steps to managers
├── types.go               # Shared types and aliases
├── managers/              # StepManager implementations
│   ├── crawler_manager.go
│   ├── agent_manager.go
│   ├── transform_manager.go
│   ├── reindex_manager.go
│   ├── places_search_manager.go
├── workers/               # JobWorker implementations
│   ├── job_processor.go   # Routes jobs to appropriate workers
│   ├── crawler_worker.go
│   ├── agent_worker.go
│   └── github_log_worker.go
└── state/                 # Runtime state tracking
    ├── monitor.go         # Job progress monitoring
    ├── progress.go        # Progress tracking
    ├── runtime.go         # Status management
    └── stats.go           # Statistics aggregation
```

## Core Components

### Queue Operations (Immutable)

#### badger_manager.go

The `BadgerQueueManager` handles queue message operations:

- `Enqueue()` - Add a message to the queue
- `Receive()` - Get the next message from the queue
- `Extend()` - Extend visibility timeout for a message
- `Close()` - Shutdown the queue

#### lifecycle.go

The job lifecycle manager handles job CRUD operations:

- `CreateJobRecord()` - Create a new job in the database
- `GetJob()` - Retrieve a job by ID
- `ListJobs()` - List jobs with filtering
- `UpdateJob()` - Update job fields

**Important:** These operations work with immutable `QueueJob` records. Runtime state is tracked separately.

### Managers (StepManager Interface)

Managers create parent jobs and orchestrate workflows. Each manager handles a specific action type:

| Manager | Action Type | Description |
|---------|-------------|-------------|
| `CrawlerManager` | `"crawl"` | Web crawling workflows |
| `AgentManager` | `"agent"` | AI-powered document processing |
| `TransformManager` | `"transform"` | Document transformation |
| `ReindexManager` | `"reindex"` | Search index rebuilding |
| `PlacesSearchManager` | `"places_search"` | Google Places API searches |

### Workers (JobWorker Interface)

Workers execute individual jobs dequeued from the queue:

| Worker | Job Type | Description |
|--------|----------|-------------|
| `CrawlerWorker` | `"crawler"` | Executes web crawling |
| `AgentWorker` | `"agent"` | Executes AI agent processing |
| `GitHubLogWorker` | `"github_log"` | Executes GitHub log fetching |
| `JobProcessor` | (router) | Routes jobs to appropriate workers |

### State Tracking (Mutable)

Runtime state is tracked separately from immutable queue jobs:

#### monitor.go

The `JobMonitor` watches job events and aggregates statistics:

- `StartMonitoring()` - Begin monitoring a parent job
- `StopMonitoring()` - End monitoring
- `GetJobProgress()` - Get current progress

#### runtime.go

Status management operations:

- `UpdateJobStatus()` - Change job status
- `MarkJobStarted/Completed/Failed()` - Status transitions
- `SetJobError()` - Record error details

#### progress.go

Progress tracking operations:

- `UpdateJobProgress()` - Update progress counters
- `IncrementProcessed()` - Increment success counter
- `IncrementFailed()` - Increment failure counter

#### stats.go

Statistics aggregation:

- `GetJobStats()` - Aggregate child job statistics
- `CalculateCompletionPercentage()` - Calculate progress

## Data Types

### QueueJob (Immutable)

The `QueueJob` model (`internal/models/job_model.go`) represents immutable work:

```go
type QueueJob struct {
    ID       string  // Unique job ID
    ParentID *string // Parent job (nil for root)
    Type     string  // Job type: "crawler", "agent", etc.
    Name     string  // Human-readable name
    Config   map[string]interface{} // Configuration snapshot
    Metadata map[string]interface{} // Additional metadata
    Depth    int     // Hierarchy depth (0 for root)
}
```

### QueueJobState (In-Memory)

The `QueueJobState` model represents runtime state (not persisted):

```go
type QueueJobState struct {
    // Embedded QueueJob fields
    Status      JobStatus   // pending, running, completed, failed
    Progress    JobProgress // Execution progress
    StartedAt   *time.Time
    CompletedAt *time.Time
    Error       string
}
```

### QueueMessage

Messages stored in the queue for worker processing:

```go
type QueueMessage struct {
    JobID   string          // References jobs.id
    Type    string          // Job type for routing
    Payload json.RawMessage // Job-specific data
}
```

## Usage

### Creating and Enqueuing a Job

```go
// Create a queue job
job := models.NewQueueJob("crawler", "Crawl Example.com", config, metadata)

// Create job record
if err := jobManager.CreateJobRecord(ctx, &queue.Job{...}); err != nil {
    return err
}

// Enqueue for processing
msg := models.QueueMessage{
    JobID:   job.ID,
    Type:    job.Type,
    Payload: payloadBytes,
}
if err := queueMgr.Enqueue(ctx, msg); err != nil {
    return err
}
```

### Processing Jobs

```go
// Receive from queue
msg, ack, err := queueMgr.Receive(ctx)
if err != nil {
    return err
}

// Process with job processor
if err := processor.Process(ctx, msg); err != nil {
    // Handle error, message will be requeued after visibility timeout
    return err
}

// Acknowledge successful processing
if err := ack(); err != nil {
    return err
}
```

## Design Principles

1. **Immutability:** `QueueJob` records are immutable after creation
2. **Event-Driven State:** Runtime state tracked via job events/logs
3. **Separation of Concerns:** Queue operations vs state tracking
4. **Single Responsibility:** Each component has a focused purpose
5. **Interface-Driven:** All components implement defined interfaces

## Related Documentation

- [Manager/Worker Architecture](../../docs/architecture/MANAGER_WORKER_ARCHITECTURE.md)
- [Jobs Domain README](../jobs/README.md)
- [Agent Framework](../../AGENTS.md)
