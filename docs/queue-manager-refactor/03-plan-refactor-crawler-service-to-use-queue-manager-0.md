I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State

**Custom Queue Architecture:**
- `Service` struct has `queue *URLQueue` field (service.go:44)
- `StartCrawl` seeds URLQueue with URLQueueItems (service.go:447-462)
- `startWorkers` launches goroutines running `workerLoop` (orchestrator.go:16-24)
- `workerLoop` pops from queue, processes URLs, discovers links, enqueues back (worker.go:26-410)
- Progress tracked in-memory via `activeJobs` map (service.go:55)

**New Queue Architecture (Already Implemented):**
- `queue.Manager` with goqite backend (queue/manager.go)
- `queue.WorkerPool` with registered handlers (queue/worker.go)
- `CrawlerJob.Execute` handles URL processing (jobs/types/crawler.go)
- `JobManager` for CRUD operations (jobs/manager.go)
- Job handlers registered in app.go (lines 380-423)

**Helper Functions to Preserve:**
- `makeRequest`, `executeRequest` - HTTP scraping with retry (worker.go:412-722)
- `discoverLinks`, `extractLinksFromHTML` - link discovery (worker.go:751-1045)
- `filterJiraLinks`, `filterConfluenceLinks` - source-specific filtering (worker.go:1047-1199)
- `filterLinks` - pattern-based filtering (orchestrator.go:26-121)
- `BuildHTTPClientFromAuth` - auth-aware HTTP client (service.go:921-1022)

**Integration Points:**
- app.go line 351: CrawlerService initialization (needs QueueManager parameter)
- service.go line 66: NewService constructor (needs QueueManager parameter)
- service.go line 196: StartCrawl (needs to enqueue JobMessages)
- orchestrator.go: Remove worker management, keep filterLinks
- worker.go: Keep helper functions, remove workerLoop

**Progress Tracking:**
- Current: in-memory via activeJobs map
- New: JobStorage + queue stats for UI display
- monitorCompletion needs update to track via storage

### Approach

Replace custom URLQueue with goqite-backed queue manager. Refactor CrawlerService to enqueue JobMessages instead of managing workers directly. Worker pool and job execution are already implemented via queue.WorkerPool and CrawlerJob.Execute. Focus on updating service initialization, StartCrawl method, and removing obsolete worker management code while preserving helper functions.

### Reasoning

Explored the codebase to understand the current crawler architecture with custom URLQueue, worker pool, and job management. Read the queue manager implementation (goqite-backed), worker pool with registered handlers, and CrawlerJob.Execute implementation. Analyzed service initialization in app.go to understand dependency injection. Examined helper functions in worker.go and orchestrator.go to identify reusable code. Reviewed job storage and progress tracking mechanisms.

## Mermaid Diagram

sequenceDiagram
    participant UI as Web UI
    participant Handler as Job Handler
    participant CrawlerSvc as Crawler Service
    participant QueueMgr as Queue Manager
    participant Worker as Queue Worker
    participant CrawlerJob as Crawler Job Type
    participant JobStorage as Job Storage

    Note over UI,JobStorage: OLD: Custom URLQueue Architecture (TO BE REMOVED)
    UI->>Handler: POST /api/crawl
    Handler->>CrawlerSvc: StartCrawl(seedURLs, config)
    CrawlerSvc->>CrawlerSvc: Create CrawlJob
    CrawlerSvc->>CrawlerSvc: queue.Push(URLQueueItem) [REMOVE]
    CrawlerSvc->>CrawlerSvc: startWorkers() [REMOVE]
    Note over CrawlerSvc: workerLoop goroutines [REMOVE]
    CrawlerSvc-->>Handler: jobID
    
    Note over UI,JobStorage: NEW: goqite Queue Architecture (IMPLEMENT)
    UI->>Handler: POST /api/crawl
    Handler->>CrawlerSvc: StartCrawl(seedURLs, config)
    CrawlerSvc->>CrawlerSvc: Create CrawlJob
    CrawlerSvc->>JobStorage: SaveJob(job)
    
    loop For each seed URL
        CrawlerSvc->>QueueMgr: Enqueue(JobMessage{type=crawler_url, depth=0})
    end
    
    CrawlerSvc-->>Handler: jobID
    Handler-->>UI: 201 Created {jobID}
    
    Note over Worker,CrawlerJob: Background Processing (Already Implemented)
    
    Worker->>QueueMgr: Receive() message
    QueueMgr-->>Worker: JobMessage
    Worker->>CrawlerJob: Execute(ctx, message)
    
    CrawlerJob->>CrawlerJob: Fetch URL (makeRequest)
    CrawlerJob->>CrawlerJob: Save document
    CrawlerJob->>CrawlerJob: Discover links
    
    alt follow_links && depth < max_depth
        loop For each discovered link
            CrawlerJob->>QueueMgr: Enqueue(child message, depth+1)
        end
    end
    
    CrawlerJob->>JobStorage: Update job progress
    CrawlerJob-->>Worker: Success
    Worker->>QueueMgr: Delete(message)
    
    Note over UI,JobStorage: Progress Tracking
    UI->>Handler: GET /api/jobs/{id}
    Handler->>CrawlerSvc: GetJobStatus(jobID)
    CrawlerSvc->>JobStorage: GetJob(jobID)
    JobStorage-->>CrawlerSvc: CrawlJob with progress
    CrawlerSvc-->>Handler: Job data
    Handler-->>UI: 200 OK {job}

## Proposed File Changes

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\queue\types.go
- internal\jobs\types\crawler.go(MODIFY)
- internal\interfaces\queue_service.go(MODIFY)

**Update Service struct (lines 30-63):**

1. Remove `queue *URLQueue` field (line 44)
2. Add `queueManager interfaces.QueueManager` field after `documentStorage`
3. Keep all other fields unchanged (activeJobs, jobResults, jobClients, etc.)

**Update NewService constructor (lines 65-89):**

1. Add `queueManager interfaces.QueueManager` parameter after `documentStorage`
2. Remove `queue: NewURLQueue()` initialization (line 78)
3. Add `queueManager: queueManager` field assignment
4. Keep all other initializations unchanged

**Refactor StartCrawl method (lines 196-521):**

1. Keep all validation, config handling, snapshot logic (lines 196-429) - NO CHANGES
2. Keep job creation and persistence (lines 229-436) - NO CHANGES
3. **Replace queue seeding (lines 443-468)** with JobMessage enqueueing:
   - For each seed URL, create `queue.NewCrawlerURLMessage(jobID, url, 0, sourceType, entityType)`
   - Set Config map with max_depth, max_pages, follow_links, include_patterns, exclude_patterns, rate_limit
   - Copy auth_snapshot and source_config_snapshot to message metadata
   - Call `queueManager.Enqueue(ctx, msg)` instead of `queue.Push(item)`
   - Track actuallyEnqueued count for progress initialization
4. **Remove startWorkers call (line 518)** - workers are managed by queue.WorkerPool
5. Keep browser pool initialization (lines 509-515) if needed for future use
6. Keep job status updates and event emission (lines 470-507)

**Update GetJobStatus (lines 523-560):**
- Keep current implementation - already checks activeJobs then jobStorage
- No changes needed

**Update CancelJob (lines 562-627):**
- Keep current implementation - updates job status and persists
- No changes needed

**Keep helper methods unchanged:**
- `BuildHTTPClientFromAuth` (lines 921-1022)
- `buildHTTPClientFromAuth` (standalone function)
- `initBrowserPool`, `getBrowserFromPool`, `shutdownBrowserPool` (lines 97-194)

**Add new helper method:**
```go
// GetConfig returns the crawler configuration for use by job types
func (s *Service) GetConfig() *common.Config {
    return s.config
}
```

This allows CrawlerJob.Execute to access crawler config for HTMLScraper initialization.

Reference `internal/jobs/types/crawler.go` line 109 which calls `GetConfig()`.

### internal\services\crawler\orchestrator.go(MODIFY)

References: 

- internal\queue\worker.go
- internal\jobs\types\crawler.go(MODIFY)

**Remove worker management functions:**

1. **Delete `startWorkers` function (lines 15-24)** - replaced by queue.WorkerPool
2. **Delete `monitorCompletion` function (lines 271-401)** - job completion tracked via JobStorage and queue stats
3. **Delete `enqueueLinks` function (lines 123-186)** - replaced by CrawlerJob enqueueing child jobs
4. **Delete `logQueueDiagnostics` function (lines 403-486)** - queue stats available via QueueManager.GetQueueStats

**Keep filtering functions:**

1. **Keep `filterLinks` function (lines 26-121)** - used by CrawlerJob.Execute for pattern filtering
   - This function is reusable and contains pattern compilation and filtering logic
   - Referenced by `internal/jobs/types/crawler.go` for link filtering

**Keep progress tracking helpers:**

1. **Keep `updateProgress` function (lines 188-216)** - updates in-memory job progress
2. **Keep `updateCurrentURL` function (lines 218-229)** - updates current URL being processed
3. **Keep `updatePendingCount` function (lines 231-243)** - updates pending URL count
4. **Keep `emitProgress` function (lines 245-269)** - publishes progress events

These progress tracking functions are still used by GetJobStatus and other methods that read from activeJobs map.

**Add comment at top of file:**
```go
// orchestrator.go contains progress tracking and filtering functions.
// Worker management has been migrated to queue.WorkerPool.
// Job execution is handled by queue-based job types (internal/jobs/types/crawler.go).
```

### internal\services\crawler\worker.go(MODIFY)

References: 

- internal\jobs\types\crawler.go(MODIFY)
- internal\queue\worker.go

**Remove worker loop:**

1. **Delete `workerLoop` function (lines 26-410)** - replaced by queue.WorkerPool + CrawlerJob.Execute

**Keep all helper functions (these are reused by CrawlerJob.Execute):**

1. **Keep `executeRequest` function (lines 412-496)** - wraps makeRequest with retry logic
2. **Keep `makeRequest` function (lines 498-722)** - performs HTML scraping with HTMLScraper
3. **Keep `extractCookiesFromClient` function (lines 724-749)** - extracts cookies from HTTP client
4. **Keep `discoverLinks` function (lines 751-935)** - extracts and filters links from crawl results
5. **Keep `extractLinksFromHTML` function (lines 937-1045)** - parses HTML for links
6. **Keep `filterJiraLinks` function (lines 1047-1124)** - Jira-specific link filtering
7. **Keep `filterConfluenceLinks` function (lines 1126-1199)** - Confluence-specific link filtering

**Update file header comment:**
```go
// worker.go contains reusable crawler utility functions for URL processing and content extraction.
// The worker loop has been replaced by queue-based job processing.
// See internal/jobs/types/crawler.go for the new crawler job implementation.
```

These helper functions are called by `CrawlerJob.Execute` in `internal/jobs/types/crawler.go` (lines 99-122 for makeRequest usage, lines 211-377 for link discovery).

### internal\services\crawler\queue.go(DELETE)

References: 

- internal\queue\manager.go
- internal\queue\types.go

Delete the entire custom URLQueue implementation. This file is replaced by goqite-backed queue manager in `internal/queue/manager.go`.

The URLQueue provided:
- Priority heap for URL ordering
- Deduplication via seen map
- Blocking Pop/Push operations
- Per-job deduplication keys

These features are now handled by:
- goqite's persistent queue (priority via message ordering)
- Deduplication via queue message IDs
- Blocking via goqite.Receive with visibility timeout
- Per-job tracking via JobMessage.ParentID field

### internal\app\app.go(MODIFY)

References: 

- internal\services\crawler\service.go(MODIFY)

**Update CrawlerService initialization (line 351):**

Change from:
```go
a.CrawlerService = crawler.NewService(a.AuthService, a.SourceService, a.StorageManager.AuthStorage(), a.EventService, a.StorageManager.JobStorage(), a.StorageManager.DocumentStorage(), a.Logger, a.Config)
```

To:
```go
a.CrawlerService = crawler.NewService(a.AuthService, a.SourceService, a.StorageManager.AuthStorage(), a.EventService, a.StorageManager.JobStorage(), a.StorageManager.DocumentStorage(), a.QueueManager, a.Logger, a.Config)
```

Add `a.QueueManager` parameter after `a.StorageManager.DocumentStorage()`.

**Verify QueueManager is initialized before CrawlerService:**
- QueueManager is initialized at lines 295-323
- CrawlerService is initialized at line 351
- Order is correct - no changes needed

**No other changes needed:**
- Job handler registration (lines 380-423) is already correct
- WorkerPool is already started (line 422)
- All dependencies are properly wired

### internal\services\crawler\helpers.go(MODIFY)

**No changes needed.**

This file contains pure utility functions for HTML parsing and data extraction:
- `CreateDocument` - creates goquery.Document from HTML
- `ExtractTextFromDoc`, `ExtractMultipleTextsFromDoc` - text extraction
- `ExtractCleanedHTML` - HTML cleaning
- `ExtractDateFromDoc` - date extraction and normalization
- `ParseJiraIssueKey`, `ParseConfluencePageID`, `ParseSpaceKey` - ID extraction
- `NormalizeStatus` - status normalization

These functions are used by transformers (jira_transformer, confluence_transformer) and are independent of the queue refactoring.

Add clarifying comment at top if desired:
```go
// Package crawler provides HTML parsing utilities and helpers.
// These helpers are used by both the crawler service and specialized transformers
// (jira_transformer, confluence_transformer) for extracting structured data from HTML.
// These utilities are independent of the queue/worker architecture.
```

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\services\crawler\types.go
- internal\app\app.go(MODIFY)

**Update progress tracking (lines 386-398):**

Replace the TODO comment with actual implementation:

1. Get parent job from JobStorage using `msg.ParentID`
2. Increment `Progress.CompletedURLs`
3. Decrement `Progress.PendingURLs`
4. Update `Progress.Percentage` calculation
5. Save updated job via JobStorage
6. Optionally emit progress event via EventService if available

Implementation approach:
```go
// Update parent job progress
if c.deps.JobStorage != nil {
    // Get parent job
    jobInterface, err := c.deps.JobStorage.GetJob(ctx, msg.ParentID)
    if err != nil {
        c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to get parent job for progress update")
    } else if job, ok := jobInterface.(*crawler.CrawlJob); ok {
        // Update progress counters
        job.Progress.CompletedURLs++
        if job.Progress.PendingURLs > 0 {
            job.Progress.PendingURLs--
        }
        job.Progress.Percentage = float64(job.Progress.CompletedURLs) / float64(job.Progress.TotalURLs) * 100
        
        // Save updated job
        if err := c.deps.JobStorage.SaveJob(ctx, job); err != nil {
            c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to update parent job progress")
        }
    }
}
```

This replaces the in-memory progress tracking with persistent storage-based tracking.

**Add JobStorage to CrawlerJobDeps (line 19-24):**

The deps struct already has JobStorage field (line 22), so no changes needed to struct definition.

**Verify all dependencies are available:**
- CrawlerService: ✓ (line 20)
- LogService: ✓ (line 21)
- DocumentStorage: ✓ (line 22)
- QueueManager: ✓ (line 23)
- JobStorage: Need to add to deps struct

Update CrawlerJobDeps struct:
```go
type CrawlerJobDeps struct {
    CrawlerService  *crawler.Service
    LogService      interfaces.LogService
    DocumentStorage interfaces.DocumentStorage
    QueueManager    interfaces.QueueManager
    JobStorage      interfaces.JobStorage  // ADD THIS
}
```

Update app.go (line 381-386) to include JobStorage in deps:
```go
crawlerJobDeps := &jobtypes.CrawlerJobDeps{
    CrawlerService:  a.CrawlerService,
    LogService:      a.LogService,
    DocumentStorage: a.StorageManager.DocumentStorage(),
    QueueManager:    a.QueueManager,
    JobStorage:      a.StorageManager.JobStorage(),  // ADD THIS
}
```

### internal\interfaces\queue_service.go(MODIFY)

References: 

- internal\queue\manager.go

**Fix EnqueueWithDelay signature mismatch (line 15):**

Change from:
```go
EnqueueWithDelay(ctx context.Context, msg *queue.JobMessage, delay int) error
```

To:
```go
EnqueueWithDelay(ctx context.Context, msg *queue.JobMessage, delay time.Duration) error
```

The implementation in `internal/queue/manager.go` (line 107) uses `time.Duration`, but the interface declares `int`. This causes a type mismatch.

**Fix Receive signature (line 16):**

Change from:
```go
Receive(ctx context.Context) (*queue.JobMessage, error)
```

To:
```go
Receive(ctx context.Context) (*goqite.Message, error)
```

The implementation returns `*goqite.Message`, not `*queue.JobMessage`. The worker pool then decodes the message body to JobMessage.

**Fix Delete and Extend signatures (lines 17-18):**

Change from:
```go
Delete(ctx context.Context, id string) error
Extend(ctx context.Context, id string) error
```

To:
```go
Delete(ctx context.Context, msg goqite.Message) error
Extend(ctx context.Context, msg goqite.Message, duration time.Duration) error
```

The implementation uses `goqite.Message` objects, not string IDs.

**Add missing import:**
```go
import (
    "context"
    "time"  // ADD THIS
    "maragu.dev/goqite"  // ADD THIS
    "github.com/ternarybob/quaero/internal/queue"
)
```

### internal\jobs\manager.go(MODIFY)

References: 

- internal\interfaces\queue_service.go(MODIFY)
- internal\app\app.go(MODIFY)

**Fix constructor signature (line 30):**

Change from:
```go
func NewManager(queueMgr *queue.Manager, jobStorage interfaces.JobStorage, logger arbor.ILogger) *Manager
```

To:
```go
func NewManager(jobStorage interfaces.JobStorage, queueMgr interfaces.QueueManager, logService interfaces.LogService, logger arbor.ILogger) *Manager
```

This matches the call in `internal/app/app.go` line 332.

**Update Manager struct (lines 14-19):**

Change from:
```go
type Manager struct {
    queueManager *queue.Manager
    jobStorage   interfaces.JobStorage
    logger       arbor.ILogger
}
```

To:
```go
type Manager struct {
    queueManager interfaces.QueueManager
    jobStorage   interfaces.JobStorage
    logService   interfaces.LogService
    logger       arbor.ILogger
}
```

Use interface types instead of concrete types for better testability.

**Update CreateJob method (lines 38-73):**

The method signature doesn't match the interface. Update to:
```go
func (m *Manager) CreateJob(ctx context.Context, sourceType, sourceID string, config map[string]interface{}) (string, error)
```

This matches `interfaces.JobManager` interface (line 32 in queue_service.go).

Implementation should:
1. Create CrawlJob from parameters
2. Save to storage
3. Enqueue parent + seed URL messages
4. Return job ID

**Update GetJob return type (line 76):**

Change from:
```go
func (m *Manager) GetJob(ctx context.Context, jobID string) (*crawler.CrawlJob, error)
```

To:
```go
func (m *Manager) GetJob(ctx context.Context, jobID string) (interface{}, error)
```

This matches the interface definition.

**Update other method signatures to match interface:**
- `ListJobs` should return `[]interface{}` not `[]*crawler.CrawlJob`
- `UpdateJob` should accept `interface{}` not `*crawler.CrawlJob`
- `GetJobWithChildren` should return `interface{}` not `*JobTree`

Alternatively, update the interface to use concrete types if type safety is preferred.