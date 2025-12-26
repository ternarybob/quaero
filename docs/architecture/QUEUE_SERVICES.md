# Queue Services Architecture

> **For AI Agents:** This document describes the supporting services for the queue system.
> Read this before modifying event handling, storage, or service initialization.

## Overview

The queue system relies on several supporting services for event broadcasting, storage, and coordination.

## Service Dependency Graph

```
┌─────────────────────────────────────────────────────────────────┐
│                        APP INITIALIZATION                        │
│  internal/app/app.go                                             │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                         STORAGE LAYER                            │
│  BadgerDB (embedded key-value store)                             │
│  ├── queue_storage.go (jobs table)                               │
│  ├── log_storage.go (job_logs table)                             │
│  └── queue_message_storage.go (message queue)                    │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                         EVENT SERVICE                            │
│  internal/services/events/service.go                             │
│  ├── Pub/Sub pattern for system events                           │
│  ├── Subscribers: WebSocket, Monitors, Coordinators              │
│  └── Events: EventJobLog, EventJobStatusChange, etc.             │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                         QUEUE SERVICES                           │
│  ├── JobManager (job CRUD, logging)                              │
│  ├── QueueManager (message queue operations)                     │
│  ├── StepManager (worker routing)                                │
│  ├── Orchestrator (job definition execution)                     │
│  └── Monitors (JobMonitor, StepMonitor)                          │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                         WORKER POOL                              │
│  internal/queue/workers/job_processor.go                         │
│  ├── Polls queue for messages                                    │
│  ├── Routes to registered JobWorkers                             │
│  └── Manages concurrency and retries                             │
└─────────────────────────────────────────────────────────────────┘
```

## Event Service

### Purpose

The EventService provides pub/sub messaging for decoupled communication between components.

### Key Events

| Event | Publisher | Subscribers | Purpose |
|-------|-----------|-------------|---------|
| `EventJobLog` | JobManager | WebSocketHandler | Real-time log streaming |
| `EventJobStatusChange` | JobManager | WebSocket, Monitors | Status updates |
| `EventJobUpdate` | JobManager | WebSocket | Metadata changes |
| `EventRefreshLogs` | StepMonitor | WebSocket | Trigger log refetch |

### Event Publishing

```go
// JobManager publishes log event
eventService.Publish(events.EventJobLog, &events.JobLogPayload{
    JobID:     jobID,
    ManagerID: managerID,
    StepName:  stepName,
    Entry:     logEntry,
})
```

### Event Subscription

```go
// WebSocketHandler subscribes to log events
eventService.Subscribe(events.EventJobLog, func(payload interface{}) {
    logPayload := payload.(*events.JobLogPayload)
    wsHandler.BroadcastLog(logPayload)
})
```

## Storage Services

### LogService Interface

```go
type LogService interface {
    AppendLog(ctx, jobID, entry) error
    AppendLogs(ctx, jobID, entries) error
    GetLogs(ctx, jobID, limit) ([]LogEntry, error)
    GetLogsWithOffset(ctx, jobID, limit, offset) ([]LogEntry, error)
    GetAggregatedLogs(ctx, parentJobID, includeChildren, level, limit, cursor, order) ([]LogEntry, map[string]*AggregatedJobMeta, string, error)
    DeleteLogs(ctx, jobID) error
    CountLogs(ctx, jobID) (int, error)
}
```

### JobManager Interface

```go
type JobManager interface {
    CreateJob(ctx, sourceType, sourceID, config) (string, error)
    GetJob(ctx, jobID) (interface{}, error)
    ListJobs(ctx, opts) ([]*QueueJobState, error)
    UpdateJob(ctx, job) error
    DeleteJob(ctx, jobID) (int, error)
    CopyJob(ctx, jobID) (string, error)
    GetJobChildStats(ctx, parentIDs) (map[string]*JobChildStats, error)
    StopAllChildJobs(ctx, parentID) (int, error)
}
```

### QueueManager Interface

```go
type QueueManager interface {
    Enqueue(ctx, msg) error
    Receive(ctx) (*QueueMessage, func() error, error)
    Extend(ctx, messageID, duration) error
    DeleteByJobID(ctx, jobID) (int, error)
    DeleteByJobIDs(ctx, jobIDs) (int, error)
    Close() error
}
```

## Job Definition Change Detection & Document Cleanup

The system automatically detects when job definition TOML files change and cleans up stale documents.

### How It Works

1. **Content Hash Computation:** When loading job definitions, an MD5 hash (8-char hex) is computed from the TOML content
2. **Change Detection:** The new hash is compared with the stored `ContentHash` on the existing job definition
3. **Document Cleanup:** If hashes differ, all documents with `jobdef:{id}` cache tag are deleted
4. **Updated Flag:** The job definition is marked with `Updated=true` for UI display

### Flow

```
LoadJobDefinitionsFromFiles()
    ↓
Compute MD5 hash of TOML content
    ↓
Compare with existingJobDef.ContentHash
    ↓
If different:
    - jobDef.Updated = true
    - cacheService.CleanupByJobDefID(jobDef.ID)
    - Log "Job definition content changed"
    ↓
Save updated job definition with new ContentHash
```

### Key Fields

| Field | Type | Persisted | Description |
|-------|------|-----------|-------------|
| `ContentHash` | string | Yes | MD5 hash (8-char hex) of TOML content |
| `Updated` | bool | No | True if content changed since last load |

### Cache Service Integration

The `CacheService.CleanupByJobDefID()` method queries all documents with the `jobdef:{id}` tag and deletes them, forcing regeneration on next job execution.

## Service Initialization Order

**CRITICAL:** Services must be initialized in this order:

```go
// 1. Storage Layer
badgerDB := badger.Open(config.Database.Path)
logStorage := badger.NewLogStorage(badgerDB)
jobStorage := badger.NewQueueStorage(badgerDB)
documentStorage := badger.NewDocumentStorage(badgerDB)

// 2. Cache Service (for document cleanup during job definition loading)
cacheService := cache.NewService(documentStorage, logger)
storageManager.SetCacheService(cacheService)

// 3. Load Job Definitions (cleanup happens here if content changed)
storageManager.LoadJobDefinitionsFromFiles(ctx, definitionsDir)

// 4. Event Service
eventService := events.NewService(logger)

// 3. Queue Services
queueManager := queue.NewBadgerQueueManager(badgerDB, config.Queue)
jobManager := queue.NewManager(jobStorage, logStorage, eventService, logger)
stepManager := queue.NewStepManager(logger)

// 4. Monitors
jobMonitor := state.NewJobMonitor(jobManager, eventService, logger)
stepMonitor := state.NewStepMonitor(jobManager, eventService, logger)

// 5. Orchestrator
orchestrator := queue.NewOrchestrator(jobManager, stepManager, queueManager, eventService, logger)

// 6. Worker Pool
jobProcessor := workers.NewJobProcessor(queueManager, jobManager, eventService, logger)

// 7. Register Workers
jobProcessor.RegisterWorker(workers.NewCrawlerWorker(...))
jobProcessor.RegisterWorker(workers.NewAgentWorker(...))
// ... more workers

// 8. Start Processing
jobProcessor.Start()
```

## Monitor Services

### JobMonitor

Monitors parent (manager) job progress:

```go
type JobMonitor interface {
    StartMonitoring(ctx, job *QueueJob)
    SubscribeToJobEvents()
}
```

**Responsibilities:**
- Track child job completion
- Aggregate statistics (completed, failed, total)
- Update manager job progress
- Publish completion events

### StepMonitor

Monitors step job children:

```go
type StepMonitor interface {
    StartMonitoring(ctx, stepJob *QueueJob)
}
```

**Responsibilities:**
- Track worker job completion under a step
- Mark step as completed when all workers finish
- Update step_stats in manager job
- Publish refresh_logs event

## Configuration

### Queue Configuration

```toml
[queue]
queue_name = "quaero-jobs"
concurrency = 4              # Worker pool size
poll_interval = "1s"         # Queue polling interval
visibility_timeout = "5m"    # Message visibility timeout
max_receive = 3              # Max retries before dead-letter
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `QUAERO_QUEUE_CONCURRENCY` | Override worker pool size |
| `QUAERO_QUEUE_POLL_INTERVAL` | Override poll interval |
| `QUAERO_QUEUE_VISIBILITY_TIMEOUT` | Override visibility timeout |

## Error Handling

### Job Failure Flow

```
Worker.Execute() returns error
    ↓
JobProcessor catches error
    ↓
jobManager.SetJobError(jobID, errorMsg)
    ↓
EventService publishes EventJobStatusChange
    ↓
StepMonitor receives event, checks if step should fail
    ↓
If error tolerance exceeded: StopAllChildJobs()
```

### Retry Logic

- Messages reappear after visibility timeout if not acknowledged
- `max_receive` limits retry attempts
- Failed jobs are marked with error status (not retried)

## Related Documents

- **Manager/Worker Architecture:** `docs/architecture/manager_worker_architecture.md`
- **Logging Architecture:** `docs/architecture/QUEUE_LOGGING.md`
- **UI Architecture:** `docs/architecture/QUEUE_UI.md`
- **Workers Reference:** `docs/architecture/workers.md`

