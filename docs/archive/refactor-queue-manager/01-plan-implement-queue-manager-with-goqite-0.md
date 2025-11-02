I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Architecture Analysis

**Queue System:**
- Custom `URLQueue` in `internal/services/crawler/queue.go` with priority heap, deduplication, and blocking operations
- Per-job URL queue managed by `crawler.Service` with worker pool pattern
- Jobs stored in `crawl_jobs` table with config snapshots, progress tracking, and logs as JSON array

**Job Execution:**
- `JobExecutor` orchestrates `JobDefinition` execution through registered action handlers
- `JobTypeRegistry` maps job types (crawler, summarizer) to action handlers
- Crawler service spawns workers that process URLs from the queue
- Workers discover links and enqueue them back to the queue (depth tracking)

**Storage:**
- `crawl_jobs` table stores job metadata, config, progress, and logs
- `job_definitions` table stores user-defined workflows
- No separate job logs table currently

**Key Integration Points:**
- `crawler.Service.StartCrawl()` creates jobs and starts workers
- `crawler.Service.workerLoop()` processes URLs from queue
- `crawler.Service.enqueueLinks()` adds discovered URLs to queue
- `JobExecutor.Execute()` runs job definitions with async polling for crawl jobs
- Handlers trigger jobs via `CrawlerService` and `JobExecutor`

## goqite Library Capabilities

- Persistent queue backed by SQLite table
- Multiple named queues in same table
- Visibility timeout for message redelivery
- `goqite.Setup()` creates schema programmatically
- `goqite/jobs.Runner` provides worker pool with concurrency control
- Message body as `[]byte` (JSON encoding required)
- Delay support for scheduled messages

### Approach

## Refactoring Strategy

**Phase 1: Queue Infrastructure**
Replace custom `URLQueue` with goqite-backed queue manager. Create new `internal/queue/` package with manager, worker pool, and job message types. Initialize goqite schema during app startup.

**Phase 2: Job Message Architecture**
Define `JobMessage` struct to represent both parent jobs and spawned child jobs (URLs). Include parent-child tracking, depth management, and job type discrimination. Use JSON encoding for message body.

**Phase 3: Job Logs Separation**
Create `job_logs` table and service to replace JSON array storage. Integrate with existing arbor logging context channel for automatic persistence.

**Phase 4: Crawler Integration**
Refactor `crawler.Service` to enqueue jobs to goqite instead of custom queue. Update worker loop to receive from goqite. Maintain parent job tracking for UI display.

**Phase 5: Job Manager Service**
Create dedicated job manager for CRUD operations on jobs. Separate concerns: queue manager handles queue lifecycle, job manager handles job metadata and operations.

**Phase 6: Cleanup**
Remove custom `URLQueue`, update handlers, and verify all functionality works with new architecture.

### Reasoning

Explored the codebase to understand the current queue and job architecture. Read the custom `URLQueue` implementation with priority heap and deduplication. Examined `crawler.Service` worker pool pattern and URL processing flow. Analyzed `JobExecutor` orchestration of job definitions. Reviewed storage schema for `crawl_jobs` and `job_definitions` tables. Checked handlers to understand job triggering and management. Researched goqite library capabilities including persistent queue, visibility timeout, and worker pool features.

## Mermaid Diagram

sequenceDiagram
    participant UI as Web UI
    participant Handler as Job Handler
    participant JobMgr as Job Manager
    participant QueueMgr as Queue Manager
    participant Worker as Queue Worker
    participant CrawlerJob as Crawler Job Type
    participant Storage as Job Storage
    participant LogSvc as Log Service

    UI->>Handler: POST /api/jobs (start crawl)
    Handler->>JobMgr: CreateJob(crawlJob)
    JobMgr->>Storage: SaveJob(metadata)
    JobMgr->>QueueMgr: Enqueue(parent message)
    QueueMgr->>QueueMgr: Store in goqite table
    JobMgr-->>Handler: Return jobID
    Handler-->>UI: 201 Created {jobID}

    Note over Worker,CrawlerJob: Background Processing

    Worker->>QueueMgr: Receive() message
    QueueMgr-->>Worker: JobMessage (type=parent)
    Worker->>Worker: Route by message type
    Worker->>CrawlerJob: Execute(message)
    
    loop For each seed URL
        CrawlerJob->>QueueMgr: Enqueue(child message, depth=1)
    end

    Worker->>QueueMgr: Receive() message
    QueueMgr-->>Worker: JobMessage (type=crawler_url)
    Worker->>CrawlerJob: Execute(message)
    CrawlerJob->>CrawlerJob: Fetch & process URL
    CrawlerJob->>Storage: SaveDocument(result)
    CrawlerJob->>LogSvc: Log processing event
    CrawlerJob->>CrawlerJob: Discover links
    
    loop For each discovered link
        CrawlerJob->>QueueMgr: Enqueue(child message, depth=2)
    end
    
    CrawlerJob->>Storage: UpdateJobProgress(jobID)
    CrawlerJob-->>Worker: Success
    Worker->>QueueMgr: Delete(message)

    UI->>Handler: GET /api/jobs/{id}
    Handler->>JobMgr: GetJob(jobID)
    JobMgr->>Storage: GetJob(jobID)
    Storage-->>JobMgr: CrawlJob with progress
    JobMgr-->>Handler: Job data
    Handler-->>UI: 200 OK {job}

    UI->>Handler: GET /api/jobs/{id}/logs
    Handler->>LogSvc: GetLogs(jobID)
    LogSvc->>Storage: Query job_logs table
    Storage-->>LogSvc: Log entries
    LogSvc-->>Handler: Logs
    Handler-->>UI: 200 OK {logs}

## Proposed File Changes

### go.mod(MODIFY)

Add `maragu.dev/goqite` dependency to the module. This library provides persistent queue functionality backed by SQLite with visibility timeout and worker pool support.

### internal\queue(NEW)

Create new directory for queue management infrastructure. This will contain the queue manager, worker pool, configuration, and job message types.

### internal\queue\config.go(NEW)

References: 

- internal\common\config.go(MODIFY)

Create queue configuration struct with fields for:
- `PollInterval` (time.Duration) - how often workers poll for messages (default: 1s)
- `Concurrency` (int) - number of concurrent workers (default: 5)
- `VisibilityTimeout` (time.Duration) - message visibility timeout (default: 5m)
- `MaxReceive` (int) - max times a message can be received before dead-letter (default: 3)
- `QueueName` (string) - name of the queue in goqite table (default: "quaero_jobs")

Add configuration to `internal/common/config.go` under new `[queue]` section in TOML. Provide sensible defaults in `NewDefaultConfig()`.

### internal\queue\types.go(NEW)

References: 

- internal\services\crawler\types.go
- internal\models\job_definition.go

Define `JobMessage` struct to represent queue messages:
- `ID` (string) - unique message ID (UUID)
- `Type` (string) - job type: "parent", "crawler_url", "summarizer", "cleanup"
- `ParentID` (string) - parent job ID for child jobs (empty for parent jobs)
- `JobDefinitionID` (string) - reference to job definition if applicable
- `Depth` (int) - crawl depth for URL jobs
- `URL` (string) - URL for crawler_url type
- `SourceType` (string) - "jira", "confluence", etc.
- `EntityType` (string) - "projects", "issues", etc.
- `Config` (map[string]interface{}) - job-specific configuration
- `Metadata` (map[string]interface{}) - additional metadata
- `Status` (string) - "pending", "running", "completed", "failed"
- `CreatedAt` (time.Time)
- `StartedAt` (time.Time)
- `CompletedAt` (time.Time)

Provide JSON marshaling/unmarshaling methods. Add helper functions to create messages for different job types.

### internal\queue\manager.go(NEW)

References: 

- internal\storage\sqlite\connection.go(MODIFY)
- internal\app\app.go(MODIFY)

Create `QueueManager` struct with:
- `queue` (*goqite.Queue) - goqite queue instance
- `runner` (*jobs.Runner) - goqite worker pool
- `config` (QueueConfig) - queue configuration
- `logger` (arbor.ILogger)
- `ctx` (context.Context) - for lifecycle management
- `cancel` (context.CancelFunc)

Implement lifecycle methods:
- `NewQueueManager(db *sql.DB, config QueueConfig, logger arbor.ILogger)` - initialize goqite queue with `goqite.Setup()` and create queue instance with `goqite.New()`
- `Start()` - start the goqite worker runner with registered job handlers
- `Stop()` - gracefully stop the worker runner and cancel context
- `Restart()` - stop and start the queue manager

Implement message operations:
- `Enqueue(ctx context.Context, msg *JobMessage)` - send message to queue with JSON encoding
- `EnqueueWithDelay(ctx context.Context, msg *JobMessage, delay time.Duration)` - send delayed message
- `GetQueueLength()` - return current queue length
- `GetQueueStats()` - return queue statistics

Reference `internal/storage/sqlite/connection.go` for database access pattern.

### internal\queue\worker.go(NEW)

References: 

- internal\services\crawler\worker.go(MODIFY)
- internal\services\crawler\orchestrator.go(MODIFY)
- internal\jobs\types\crawler.go(NEW)
- internal\jobs\types\summarizer.go(NEW)
- internal\jobs\types\cleanup.go(NEW)
- internal\app\app.go(MODIFY)

Create worker pool implementation using goqite/jobs.Runner:
- `WorkerPool` struct with runner, handlers map, and logger
- `NewWorkerPool(queueMgr *QueueManager, logger arbor.ILogger)` - create worker pool
- `RegisterHandler(jobType string, handler JobHandler)` - register job type handlers
- `Start(ctx context.Context)` - start worker pool with goqite runner
- `Stop()` - gracefully stop worker pool

Define `JobHandler` function type:
```go
type JobHandler func(ctx context.Context, msg *JobMessage) error
```

Implement message processing:
- Decode JSON message body to `JobMessage`
- Route to appropriate handler based on `Type` field
- Update job status in database
- Handle errors and retries via goqite visibility timeout
- Log processing events with correlation ID

Reference `internal/services/crawler/worker.go` for worker loop pattern and `internal/services/crawler/orchestrator.go` for worker coordination.
Update worker pool to register job type handlers during initialization:

In `NewWorkerPool()` or a separate `RegisterJobTypes()` method:
1. Create instances of each job type (CrawlerJob, SummarizerJob, CleanupJob)
2. Register handlers with the worker pool:
   ```go
   pool.RegisterHandler("crawler_url", crawlerJob.Execute)
   pool.RegisterHandler("summarizer", summarizerJob.Execute)
   pool.RegisterHandler("cleanup", cleanupJob.Execute)
   ```
3. Register parent job handler for initial job creation

The worker pool routes incoming messages to the appropriate handler based on the `Type` field in `JobMessage`.

Reference `internal/app/app.go` for dependency injection pattern to pass required services to job type constructors.

### internal\storage\sqlite\connection.go(MODIFY)

In `NewSQLiteDB()` function after opening database connection and before calling `configure()`, add goqite schema initialization:

```go
// Initialize goqite queue schema
if err := goqite.Setup(context.Background(), db); err != nil {
    db.Close()
    return nil, fmt.Errorf("failed to initialize goqite schema: %w", err)
}
logger.Info().Msg("goqite queue schema initialized")
```

This creates the goqite queue table in the same SQLite database used for application data. Import `maragu.dev/goqite` at the top of the file.

### internal\app\app.go(MODIFY)

References: 

- internal\queue\manager.go(NEW)
- internal\common\config.go(MODIFY)
- internal\logs\service.go(NEW)
- internal\jobs\manager.go(NEW)
- internal\jobs\types\base.go(NEW)
- internal\jobs\types\crawler.go(NEW)

Add `QueueManager` field to `App` struct:
```go
QueueManager *queue.QueueManager
```

In `initServices()` method, initialize queue manager after storage layer (around line 170, after database initialization):

1. Create queue configuration from `Config.Queue` (new config section)
2. Initialize `QueueManager` with `queue.NewQueueManager(db, queueConfig, logger)`
3. Start queue manager with `QueueManager.Start()`
4. Log initialization

In `Close()` method, add queue manager shutdown before closing storage (around line 490):
```go
if a.QueueManager != nil {
    if err := a.QueueManager.Stop(); err != nil {
        a.Logger.Warn().Err(err).Msg("Failed to stop queue manager")
    }
}
```

Update service initialization order:
1. Storage layer
2. **Queue manager** (new)
3. LLM service
4. Document service
5. ... (rest of services)

Pass `QueueManager` to services that need to enqueue jobs (crawler service, job executor).
Add `LogService` field to `App` struct:
```go
LogService *logs.LogService
```

In `initServices()` method, replace the existing context logging setup (lines 192-239) with:

1. Initialize `LogService` with `logs.NewLogService(StorageManager.JobLogStorage(), logger)`
2. Start log service with `LogService.Start()`
3. Configure arbor context channel to send logs to `LogService` instead of directly to database
4. Update the consumer goroutine to call `LogService.AppendLog()` instead of `JobStorage.AppendJobLog()`

In `Close()` method, add log service shutdown:
```go
if a.LogService != nil {
    if err := a.LogService.Stop(); err != nil {
        a.Logger.Warn().Err(err).Msg("Failed to stop log service")
    }
}
```

This separates log management from job storage and enables batching for better performance.
Update service initialization to wire up the new queue-based architecture:

1. After initializing `QueueManager` (added earlier), create job type instances:
   ```go
   // Initialize job types
   baseJob := jobs.NewBaseJob(app.QueueManager, app.StorageManager.JobStorage(), logger)
   crawlerJob := jobs.NewCrawlerJob(baseJob, app.CrawlerService, app.StorageManager.DocumentStorage())
   summarizerJob := jobs.NewSummarizerJob(baseJob, app.LLMService, app.StorageManager.DocumentStorage())
   cleanupJob := jobs.NewCleanupJob(baseJob, app.StorageManager.JobStorage(), app.StorageManager.JobLogStorage())
   ```

2. Register job types with queue worker pool:
   ```go
   app.QueueManager.RegisterJobType("crawler_url", crawlerJob)
   app.QueueManager.RegisterJobType("summarizer", summarizerJob)
   app.QueueManager.RegisterJobType("cleanup", cleanupJob)
   ```

3. Update `CrawlerService` initialization (line 297) to pass `QueueManager`:
   ```go
   a.CrawlerService = crawler.NewService(a.AuthService, a.SourceService, a.StorageManager.AuthStorage(), a.EventService, a.StorageManager.JobStorage(), a.StorageManager.DocumentStorage(), a.QueueManager, a.Logger, a.Config)
   ```

4. Initialize `JobManager` after queue manager:
   ```go
   a.JobManager = jobs.NewJobManager(a.QueueManager, a.StorageManager.JobStorage(), a.Logger)
   ```

This wires up the complete queue-based job processing pipeline.

### internal\storage\sqlite\schema.go(MODIFY)

Add `job_logs` table to schema SQL (around line 124, after `crawl_jobs` table):

```sql
-- Job logs table for structured log storage
CREATE TABLE IF NOT EXISTS job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (job_id) REFERENCES crawl_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_job_logs_job_id ON job_logs(job_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_logs_level ON job_logs(level, created_at DESC);
```

Add migration in `runMigrations()` to create the table if it doesn't exist. Follow the pattern of existing migrations like `migrateAddJobLogsColumn()`. The migration should check if the table exists and create it if not.

Note: The existing `logs` column in `crawl_jobs` table will be removed in a later phase after migration is complete.

### internal\logs(NEW)

Create new directory for job logging infrastructure. This will contain the log service and storage interface for managing job logs separately from job metadata.

### internal\logs\storage.go(NEW)

References: 

- internal\models\job_log.go

Define `JobLogStorage` interface:
- `AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry) error` - append single log entry
- `AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry) error` - batch append
- `GetLogs(ctx context.Context, jobID string, limit int) ([]models.JobLogEntry, error)` - retrieve logs with limit
- `GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.JobLogEntry, error)` - filter by level
- `DeleteLogs(ctx context.Context, jobID string) error` - delete all logs for a job
- `CountLogs(ctx context.Context, jobID string) (int, error)` - count logs for a job

This interface will be implemented by SQLite storage in `internal/storage/sqlite/job_log_storage.go`.

### internal\logs\service.go(NEW)

References: 

- internal\app\app.go(MODIFY)

Create `LogService` struct:
- `storage` (JobLogStorage) - storage interface
- `logger` (arbor.ILogger)
- `batchChannel` (chan LogBatch) - channel for batching logs
- `ctx` (context.Context)
- `cancel` (context.CancelFunc)

Implement methods:
- `NewLogService(storage JobLogStorage, logger arbor.ILogger)` - create service and start batch processor
- `AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry)` - send to batch channel
- `GetLogs(ctx context.Context, jobID string, limit int)` - retrieve logs
- `Start()` - start background batch processor goroutine
- `Stop()` - stop batch processor and flush pending logs

Batch processor:
- Collect logs for 1 second or until batch size reaches 100
- Write batch to storage
- Handle errors gracefully

This service will be integrated with the arbor logging context channel in `internal/app/app.go` (lines 193-239) to automatically persist logs.

### internal\storage\sqlite\job_log_storage.go(NEW)

References: 

- internal\storage\sqlite\job_storage.go(MODIFY)
- internal\logs\storage.go(NEW)

Implement `JobLogStorage` interface for SQLite:

Create `JobLogStorage` struct:
- `db` (*SQLiteDB) - database connection
- `logger` (arbor.ILogger)

Implement interface methods:
- `AppendLog()` - insert single log entry into `job_logs` table
- `AppendLogs()` - batch insert using transaction
- `GetLogs()` - query logs ordered by created_at DESC with limit
- `GetLogsByLevel()` - query logs filtered by level
- `DeleteLogs()` - delete all logs for a job (CASCADE handles this)
- `CountLogs()` - count logs for a job

Use prepared statements for performance. Handle SQL errors gracefully. Follow the pattern of `internal/storage/sqlite/job_storage.go` for consistency.

### internal\storage\sqlite\manager.go(MODIFY)

References: 

- internal\interfaces\storage.go

Add `JobLogStorage` field to `Manager` struct:
```go
jobLogStorage *JobLogStorage
```

In `NewManager()` function, initialize job log storage:
```go
jobLogStorage := NewJobLogStorage(sqliteDB, logger)
```

Add getter method:
```go
func (m *Manager) JobLogStorage() interfaces.JobLogStorage {
    return m.jobLogStorage
}
```

Update `interfaces.StorageManager` interface in `internal/interfaces/storage.go` to include `JobLogStorage() JobLogStorage` method.

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\logs\service.go(NEW)
- internal\jobs\manager.go(NEW)

Update `GetJobLogsHandler()` method (around line 254) to use the new log service instead of job storage:

1. Add `LogService` field to `JobHandler` struct
2. Update `NewJobHandler()` to accept `LogService` parameter
3. In `GetJobLogsHandler()`, replace `h.jobStorage.GetJobLogs()` call with `h.logService.GetLogs(ctx, jobID, 1000)` (limit to 1000 most recent logs)
4. Keep the same response format for backward compatibility

This change routes log retrieval through the dedicated log service instead of the job storage layer.
Update handler to use `JobManager` instead of directly calling `CrawlerService`:

1. Add `jobManager` (*jobs.JobManager) field to `JobHandler` struct
2. Update `NewJobHandler()` to accept `jobManager` parameter
3. Update methods to use job manager:
   - `ListJobsHandler()` - call `jobManager.ListJobs()` instead of `crawlerService.ListJobs()`
   - `GetJobHandler()` - call `jobManager.GetJob()` instead of `crawlerService.GetJobStatus()`
   - `DeleteJobHandler()` - call `jobManager.DeleteJob()` (if exists)
   - Add `CopyJobHandler()` - call `jobManager.CopyJob()` for job duplication

4. Keep `GetJobResultsHandler()` using crawler service as it retrieves crawl-specific results
5. Keep `RerunJobHandler()` using crawler service for backward compatibility

This provides a cleaner separation between job management (CRUD) and job execution (crawler service).

### internal\jobs(NEW)

Create new directory for job management services. This will contain the job manager for CRUD operations, separate from the job executor which handles execution logic.

### internal\jobs\manager.go(NEW)

References: 

- internal\queue\manager.go(NEW)
- internal\services\crawler\types.go

Create `JobManager` struct for job CRUD operations:
- `queueManager` (*queue.QueueManager) - for enqueueing jobs
- `jobStorage` (interfaces.JobStorage) - for job metadata persistence
- `logger` (arbor.ILogger)

Implement methods:
- `NewJobManager(queueMgr *queue.QueueManager, jobStorage interfaces.JobStorage, logger arbor.ILogger)` - constructor
- `CreateJob(ctx context.Context, job *crawler.CrawlJob) (string, error)` - create job and enqueue parent message
- `GetJob(ctx context.Context, jobID string) (*crawler.CrawlJob, error)` - retrieve job from storage or active jobs
- `ListJobs(ctx context.Context, opts *interfaces.ListOptions) ([]*crawler.CrawlJob, error)` - list jobs with filters
- `UpdateJob(ctx context.Context, job *crawler.CrawlJob) error` - update job metadata
- `DeleteJob(ctx context.Context, jobID string) error` - delete job and cancel if running
- `CopyJob(ctx context.Context, jobID string, newName string) (string, error)` - duplicate job with new ID
- `GetJobWithChildren(ctx context.Context, jobID string) (*JobTree, error)` - get job with child job hierarchy

Define `JobTree` struct for parent-child relationships:
- `Job` (*crawler.CrawlJob)
- `Children` ([]*JobTree)
- `TotalURLs` (int)
- `CompletedURLs` (int)

This manager provides a clean API for job operations, separating concerns from the crawler service.

### internal\jobs\types(NEW)

Create new directory for job type implementations. This will contain the base job interface and concrete implementations for crawler, summarizer, and cleanup job types.

### internal\jobs\types\base.go(NEW)

References: 

- internal\queue\types.go(NEW)

Define `Job` interface for job type implementations:
```go
type Job interface {
    Execute(ctx context.Context, msg *queue.JobMessage) error
    Validate(msg *queue.JobMessage) error
    GetType() string
}
```

Define `BaseJob` struct with common fields:
- `queueManager` (*queue.QueueManager)
- `jobStorage` (interfaces.JobStorage)
- `logger` (arbor.ILogger)

Provide helper methods:
- `UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string)` - update job status in storage
- `EnqueueChildJob(ctx context.Context, msg *queue.JobMessage)` - enqueue child job to queue
- `LogJobEvent(ctx context.Context, jobID string, level string, message string)` - log job event

This provides a common base for all job type implementations.

### internal\jobs\types\crawler.go(NEW)

References: 

- internal\services\crawler\worker.go(MODIFY)
- internal\services\crawler\orchestrator.go(MODIFY)
- internal\queue\types.go(NEW)

Implement `CrawlerJob` struct that embeds `BaseJob` and implements `Job` interface:

Add dependencies:
- `crawlerService` (*crawler.Service) - for URL processing
- `documentStorage` (interfaces.DocumentStorage) - for saving results

Implement methods:
- `NewCrawlerJob(base *BaseJob, crawlerSvc *crawler.Service, docStorage interfaces.DocumentStorage)` - constructor
- `Execute(ctx context.Context, msg *queue.JobMessage) error` - main execution logic
- `Validate(msg *queue.JobMessage) error` - validate message has required fields (URL, config)
- `GetType() string` - return "crawler"

Execution logic:
1. Extract URL and config from message
2. Process URL using crawler service's HTML scraper (reference `internal/services/crawler/worker.go` makeRequest function)
3. Save document to storage
4. Discover links from result
5. Filter links using include/exclude patterns (reference `internal/services/crawler/orchestrator.go` filterLinks)
6. For each discovered link, create child job message with incremented depth
7. Enqueue child jobs if depth < max_depth and follow_links is true
8. Update parent job progress
9. Log processing events

This replaces the URL queue with individual job messages for each URL, maintaining the same crawling behavior.

### internal\jobs\types\summarizer.go(NEW)

References: 

- internal\services\jobs\actions\summarizer_actions.go
- internal\queue\types.go(NEW)

Implement `SummarizerJob` struct that embeds `BaseJob` and implements `Job` interface:

Add dependencies:
- `llmService` (interfaces.LLMService) - for generating summaries
- `documentStorage` (interfaces.DocumentStorage) - for reading/writing documents

Implement methods:
- `NewSummarizerJob(base *BaseJob, llmSvc interfaces.LLMService, docStorage interfaces.DocumentStorage)` - constructor
- `Execute(ctx context.Context, msg *queue.JobMessage) error` - main execution logic
- `Validate(msg *queue.JobMessage) error` - validate message has required fields
- `GetType() string` - return "summarizer"

Execution logic:
1. Extract document IDs or query from message config
2. Retrieve documents from storage
3. Generate summary using LLM service
4. Save summary as new document or update existing
5. Update job status
6. Log processing events

Reference `internal/services/jobs/actions/summarizer_actions.go` for existing summarizer logic that can be adapted.

### internal\jobs\types\cleanup.go(NEW)

References: 

- internal\queue\types.go(NEW)

Implement `CleanupJob` struct that embeds `BaseJob` and implements `Job` interface:

Add dependencies:
- `jobStorage` (interfaces.JobStorage) - for job operations
- `logStorage` (interfaces.JobLogStorage) - for log operations

Implement methods:
- `NewCleanupJob(base *BaseJob, jobStorage interfaces.JobStorage, logStorage interfaces.JobLogStorage)` - constructor
- `Execute(ctx context.Context, msg *queue.JobMessage) error` - main execution logic
- `Validate(msg *queue.JobMessage) error` - validate message has required fields
- `GetType() string` - return "cleanup"

Execution logic:
1. Extract cleanup criteria from message config (age threshold, status filter)
2. Query jobs matching criteria (e.g., completed jobs older than 30 days)
3. Archive job metadata to separate table or export to file (optional)
4. Delete old logs from `job_logs` table
5. Delete old jobs from `crawl_jobs` table
6. Update cleanup statistics
7. Log cleanup summary

This job type can be scheduled to run periodically to prevent database bloat.

### internal\services\jobs\registry.go(MODIFY)

This file will remain largely unchanged as it handles `JobDefinition` action registration. The new queue-based job types are separate from the action registry.

However, add a comment at the top clarifying the distinction:
```go
// JobTypeRegistry manages action handlers for JobDefinition execution.
// This is separate from the queue-based job types (crawler, summarizer, cleanup)
// which are registered with the QueueManager's worker pool.
```

No structural changes needed - the registry continues to serve its purpose for job definition workflows.

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\queue\manager.go(NEW)
- internal\queue\types.go(NEW)

Refactor `StartCrawl()` method to use queue manager instead of custom queue:

1. Remove `queue` field from `Service` struct
2. Add `queueManager` (*queue.QueueManager) field
3. Update `NewService()` to accept `queueManager` parameter
4. In `StartCrawl()`, instead of creating URLQueue and starting workers:
   - Create parent job message with type="parent"
   - Enqueue parent message to queue manager
   - For each seed URL, create child job message with type="crawler_url"
   - Enqueue child messages to queue manager
   - Store job metadata in database
   - Return job ID

5. Remove `startWorkers()` method - workers are now managed by queue manager
6. Keep job tracking in `activeJobs` map for status queries
7. Update `GetJobStatus()` to check both active jobs and database

The crawler service becomes a job factory and status tracker rather than managing its own worker pool.

### internal\services\crawler\worker.go(MODIFY)

References: 

- internal\jobs\types\crawler.go(NEW)

This file's `workerLoop()` function will be replaced by the queue-based worker in `internal/queue/worker.go` and the crawler job type in `internal/jobs/types/crawler.go`.

However, keep the following functions as they contain reusable logic:
- `executeRequest()` - wraps makeRequest with retry logic
- `makeRequest()` - performs HTML scraping
- `extractCookiesFromClient()` - extracts cookies for auth
- `discoverLinks()` - extracts and filters links
- `extractLinksFromHTML()` - parses HTML for links
- `filterJiraLinks()` - Jira-specific link filtering
- `filterConfluenceLinks()` - Confluence-specific link filtering

These functions will be called by the `CrawlerJob.Execute()` method in `internal/jobs/types/crawler.go`.

Remove or mark as deprecated:
- `workerLoop()` - replaced by queue worker

Add comment at top of file:
```go
// This file contains reusable crawler utility functions.
// The worker loop has been replaced by queue-based job processing.
// See internal/jobs/types/crawler.go for the new crawler job implementation.
```

### internal\services\crawler\orchestrator.go(MODIFY)

References: 

- internal\queue\manager.go(NEW)

Update orchestration functions to work with queue-based architecture:

1. Remove `startWorkers()` method - workers are managed by queue manager
2. Remove `monitorCompletion()` method - completion is tracked via job status updates from workers
3. Keep `filterLinks()` method - used by crawler job type
4. Remove `enqueueLinks()` method - replaced by enqueueing child job messages in crawler job type
5. Keep progress tracking methods:
   - `updateProgress()`
   - `updateCurrentURL()`
   - `updatePendingCount()`
   - `emitProgress()`
6. Update `logQueueDiagnostics()` to query queue manager for queue stats instead of custom queue

Add helper method:
```go
func (s *Service) GetQueueStats() map[string]interface{} {
    return s.queueManager.GetQueueStats()
}
```

These functions remain in the crawler service as they manage job-level state and progress tracking.

### internal\services\crawler\queue.go(DELETE)

Delete the custom URLQueue implementation as it's replaced by goqite-based queue manager. The priority queue, deduplication, and blocking operations are now handled by goqite's message queue with visibility timeout.

All queue operations are now performed through `internal/queue/manager.go` and job messages are processed by `internal/queue/worker.go`.

### internal\handlers\job_definition_handler.go(MODIFY)

References: 

- internal\queue\manager.go(NEW)
- internal\queue\types.go(NEW)

Update `ExecuteJobDefinitionHandler()` method (if it exists, or add it) to enqueue job execution via queue manager:

1. When a job definition is triggered (manually or by scheduler), create a parent job message
2. Enqueue the message to queue manager with type="parent"
3. The job executor will be invoked by the queue worker
4. Return job ID to caller for tracking

If the handler doesn't exist yet, add:
```go
func (h *JobDefinitionHandler) ExecuteJobDefinitionHandler(w http.ResponseWriter, r *http.Request) {
    // Extract job definition ID from path
    // Load job definition from storage
    // Create job message
    // Enqueue to queue manager
    // Return job ID
}
```

This integrates job definitions with the queue-based execution model.

### internal\storage\sqlite\job_storage.go(MODIFY)

References: 

- internal\storage\sqlite\job_log_storage.go(NEW)

Remove or deprecate the `AppendJobLog()` and `GetJobLogs()` methods as they're replaced by the dedicated job log storage:

1. Add deprecation comments:
   ```go
   // Deprecated: Use JobLogStorage.AppendLog() instead
   func (s *JobStorage) AppendJobLog(...) error {
       return fmt.Errorf("deprecated: use JobLogStorage.AppendLog()")
   }
   ```

2. Keep the methods for backward compatibility during transition but log warnings
3. Update documentation to point to new log storage

In a future migration, the `logs` column can be removed from the `crawl_jobs` table schema. For now, keep it to avoid breaking existing code during the transition.

### internal\common\config.go(MODIFY)

References: 

- internal\queue\config.go(NEW)

Add `QueueConfig` struct to the `Config` struct (around line 26):

```go
type Config struct {
    Environment string           `toml:"environment"`
    Server      ServerConfig     `toml:"server"`
    Queue       QueueConfig      `toml:"queue"` // NEW
    Sources     SourcesConfig    `toml:"sources"`
    // ... rest of fields
}

type QueueConfig struct {
    PollInterval      string `toml:"poll_interval"`       // e.g., "1s"
    Concurrency       int    `toml:"concurrency"`         // Number of workers
    VisibilityTimeout string `toml:"visibility_timeout"`  // e.g., "5m"
    MaxReceive        int    `toml:"max_receive"`         // Max delivery attempts
    QueueName         string `toml:"queue_name"`          // Queue name in goqite
}
```

In `NewDefaultConfig()` function (around line 180), add queue defaults:
```go
Queue: QueueConfig{
    PollInterval:      "1s",
    Concurrency:       5,
    VisibilityTimeout: "5m",
    MaxReceive:        3,
    QueueName:         "quaero_jobs",
},
```

Add environment variable overrides in `applyEnvOverrides()` function for queue configuration.