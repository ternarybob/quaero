I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Implementation Status

**Queue Infrastructure (✅ Complete):**
- `queue.Manager` - goqite-backed queue with lifecycle management
- `queue.WorkerPool` - worker pool with handler registration
- `queue.JobMessage` - message types with factory methods
- Job handlers already registered in `internal/app/app.go` (lines 379-419)

**Job Types (⚠️ Skeleton Only):**
- `internal/jobs/types/base.go` - BaseJob with helper methods (UpdateJobStatus, EnqueueChildJob, LogJobEvent)
- `internal/jobs/types/crawler.go` - CrawlerJob with TODO in Execute method
- `internal/jobs/types/summarizer.go` - SummarizerJob with TODO in Execute method
- `internal/jobs/types/cleanup.go` - CleanupJob with TODO in Execute method

**Existing Logic to Reuse:**
- `internal/services/crawler/worker.go` - makeRequest, executeRequest, discoverLinks, extractLinksFromHTML, filterJiraLinks, filterConfluenceLinks
- `internal/services/jobs/actions/crawler_actions.go` - crawl action logic
- `internal/services/jobs/actions/summarizer_actions.go` - summarize, scan, extract_keywords actions

**Key Issues Found:**
1. BaseJob constructor signature mismatch between `app.go` (lines 388, 401, 410) and `base.go` (line 29)
2. Dependency structs (CrawlerJobDeps, SummarizerJobDeps) referenced in `app.go` but not defined in job types files
3. Job type Execute methods have TODOs instead of actual implementation
4. No integration between job types and existing crawler/summarizer service logic

### Approach

## Implementation Strategy

**Phase 1: Fix Architecture Mismatches**
Update BaseJob constructor and define dependency structs to match the usage in `app.go`.

**Phase 2: Implement CrawlerJob**
Complete CrawlerJob.Execute() by integrating with existing crawler service methods. The job should:
- Fetch URL content using crawler service's makeRequest
- Save document to storage
- Discover and filter links
- Enqueue child jobs for discovered links (respecting depth limits)
- Update parent job progress

**Phase 3: Implement SummarizerJob**
Complete SummarizerJob.Execute() by adapting logic from existing summarizer actions. The job should:
- Retrieve documents from storage based on config
- Generate summaries using LLM service
- Update document metadata with summaries
- Handle batch processing

**Phase 4: Implement CleanupJob**
Complete CleanupJob.Execute() with job/log cleanup logic. The job should:
- Query old jobs based on age threshold and status filter
- Delete old logs from job_logs table
- Delete old jobs from crawl_jobs table
- Log cleanup statistics

**Phase 5: Update Registry (Optional)**
Add clarifying comments to distinguish between JobTypeRegistry (for JobDefinitions) and queue-based job types (for queue workers).

### Reasoning

Explored the codebase structure and identified that queue infrastructure is complete but job type implementations are skeletal. Read existing crawler and summarizer action code to understand the business logic that needs to be integrated. Analyzed `app.go` to see how job handlers are registered and discovered architecture mismatches. Examined crawler service worker methods to identify reusable functions for URL processing and link discovery.

## Mermaid Diagram

sequenceDiagram
    participant WP as Worker Pool
    participant CJ as CrawlerJob
    participant CS as Crawler Service
    participant DS as Document Storage
    participant QM as Queue Manager
    participant SJ as SummarizerJob
    participant LLM as LLM Service

    Note over WP,QM: Crawler Job Processing

    WP->>CJ: Execute(ctx, JobMessage)
    CJ->>CJ: Validate message (URL, ParentID, Config)
    CJ->>CJ: Extract config (max_depth, follow_links, patterns)
    
    alt Depth > MaxDepth
        CJ-->>WP: Skip (depth limit reached)
    else Process URL
        CJ->>CS: makeRequest(URL, auth, config)
        CS-->>CJ: CrawlResult (content, links, metadata)
        
        CJ->>DS: SaveDocument(document)
        DS-->>CJ: Success
        
        alt follow_links == true
            CJ->>CJ: discoverLinks(result, patterns)
            CJ->>CJ: filterLinks(links, include/exclude)
            
            loop For each discovered link
                CJ->>QM: Enqueue(childMessage, depth+1)
            end
        end
        
        CJ->>DS: UpdateJobProgress(parentID, +1 completed)
        CJ-->>WP: Success
    end

    Note over WP,LLM: Summarizer Job Processing

    WP->>SJ: Execute(ctx, JobMessage)
    SJ->>SJ: Extract config (action, batch_size, filters)
    
    alt action == "summarize"
        loop Batch processing
            SJ->>DS: ListDocuments(batch_size, offset, filters)
            DS-->>SJ: Documents[]
            
            loop For each document
                SJ->>SJ: Limit content (content_limit)
                SJ->>LLM: Chat(system_prompt, content)
                LLM-->>SJ: Summary text
                
                SJ->>SJ: Update metadata (summary, keywords, word_count)
                SJ->>DS: UpdateDocument(document)
            end
        end
        SJ-->>WP: Success (processed count, errors)
    else action == "scan"
        SJ->>DS: ListDocuments(filters)
        SJ->>SJ: Identify documents needing summarization
        SJ-->>WP: Success (scan results)
    else action == "extract_keywords"
        SJ->>DS: ListDocuments(filters)
        loop For each document
            SJ->>SJ: extractKeywords(content, top_n)
            SJ->>DS: UpdateDocument(keywords)
        end
        SJ-->>WP: Success
    end

## Proposed File Changes

### internal\jobs\types\base.go(MODIFY)

References: 

- internal\app\app.go(MODIFY)
- internal\interfaces\queue_service.go

**Fix BaseJob Constructor Signature:**

Update `NewBaseJob` function signature to match the usage in `internal/app/app.go` (lines 388, 401, 410):

Change from:
```go
func NewBaseJob(queueMgr *queue.Manager, jobStorage interfaces.JobStorage, logger arbor.ILogger) *BaseJob
```

To:
```go
func NewBaseJob(messageID, jobDefinitionID string, logger arbor.ILogger, jobManager interfaces.JobManager, queueManager interfaces.QueueManager) *BaseJob
```

Update BaseJob struct fields accordingly:
- Remove `queueManager *queue.Manager` and `jobStorage interfaces.JobStorage`
- Add `messageID string`, `jobDefinitionID string`, `jobManager interfaces.JobManager`, `queueManager interfaces.QueueManager`

**Update Helper Methods:**

Modify `UpdateJobStatus` to use `jobManager.GetJob()` and `jobManager.UpdateJob()` instead of direct storage access.

Modify `EnqueueChildJob` to use `queueManager.Enqueue()` (already correct interface).

Modify `LogJobEvent` to use a log service interface instead of direct job storage access. Consider adding `logService interfaces.LogService` to BaseJob struct and passing it in constructor.

**Rationale:** The current constructor doesn't match how it's being called in `app.go`, causing compilation errors. The new signature aligns with the factory pattern used in the handler closures.

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\services\crawler\worker.go
- internal\services\crawler\service.go
- internal\app\app.go(MODIFY)

**Define CrawlerJobDeps Struct:**

Add dependency struct at the top of the file:
```go
type CrawlerJobDeps struct {
    CrawlerService *crawler.Service
    LogService     interfaces.LogService
    JobStorage     interfaces.JobStorage
    QueueManager   interfaces.QueueManager
}
```

**Update CrawlerJob Struct:**

Replace individual dependency fields with:
```go
type CrawlerJob struct {
    *BaseJob
    deps *CrawlerJobDeps
}
```

**Update Constructor:**

Change `NewCrawlerJob` to accept `deps *CrawlerJobDeps` instead of individual parameters.

**Complete Execute Method Implementation:**

Replace the TODO section with actual implementation:

1. **Validate and Extract Config:**
   - Extract max_depth, follow_links, include_patterns, exclude_patterns from `msg.Config`
   - Check depth limit (if `msg.Depth > maxDepth`, skip processing)

2. **Fetch URL Content:**
   - Call `deps.CrawlerService` methods to fetch URL (reuse `makeRequest` or `executeRequest` logic from `internal/services/crawler/worker.go`)
   - Handle authentication using auth snapshot from parent job
   - Apply rate limiting

3. **Save Document:**
   - Extract content, title, metadata from crawl result
   - Create `models.Document` with source_type, source_id from message
   - Save to document storage via `deps.JobStorage` or document storage interface

4. **Discover Links (if follow_links is true):**
   - Call `discoverLinks` function from crawler service (or reimplement using `extractLinksFromHTML`, `filterJiraLinks`, `filterConfluenceLinks` from `internal/services/crawler/worker.go`)
   - Apply include/exclude pattern filtering
   - Filter by source type (Jira/Confluence specific filters)

5. **Enqueue Child Jobs:**
   - For each discovered link, create new `queue.JobMessage` with:
     - Type: "crawler_url"
     - ParentID: msg.ParentID (same parent as current job)
     - URL: discovered link
     - Depth: msg.Depth + 1
     - Config: copy from parent message
   - Call `deps.QueueManager.Enqueue(ctx, childMsg)`

6. **Update Parent Job Progress:**
   - Increment completed URLs count
   - Update current URL being processed
   - Call `deps.JobStorage` to update job progress

7. **Log Events:**
   - Log successful processing via `deps.LogService.AppendLog()`
   - Log discovered links count
   - Log any errors

**Reference Implementation:**
Reuse logic from `internal/services/crawler/worker.go` workerLoop function (lines 26-410), particularly:
- URL fetching and retry logic (executeRequest)
- Link discovery and filtering (discoverLinks, extractLinksFromHTML)
- Document saving and progress tracking

**Error Handling:**
- Return errors for critical failures (network errors, storage errors)
- Log warnings for non-critical issues (link extraction failures)
- Update job status to "failed" on unrecoverable errors

### internal\jobs\types\summarizer.go(MODIFY)

References: 

- internal\services\jobs\actions\summarizer_actions.go
- internal\app\app.go(MODIFY)

**Define SummarizerJobDeps Struct:**

Add dependency struct at the top of the file:
```go
type SummarizerJobDeps struct {
    LLMService      interfaces.LLMService
    DocumentStorage interfaces.DocumentStorage
    Logger          arbor.ILogger
}
```

**Update SummarizerJob Struct:**

Replace individual dependency fields with:
```go
type SummarizerJob struct {
    *BaseJob
    deps *SummarizerJobDeps
}
```

**Update Constructor:**

Change `NewSummarizerJob` to accept `deps *SummarizerJobDeps` instead of individual parameters.

**Complete Execute Method Implementation:**

Replace the TODO section with actual implementation based on `internal/services/jobs/actions/summarizer_actions.go`:

1. **Extract Configuration:**
   - Extract action type from `msg.Config["action"]` ("scan", "summarize", "extract_keywords")
   - Extract batch_size, offset, max_documents, filter_source_type
   - Extract action-specific config (content_limit, system_prompt, top_n_keywords, etc.)

2. **Route to Action Handler:**
   - Based on action type, call appropriate handler:
     - "scan": Scan documents to identify those needing summarization
     - "summarize": Generate summaries using LLM service
     - "extract_keywords": Extract keywords from documents

3. **Scan Action Implementation:**
   - Query documents from storage with ListOptions (batch_size, offset, filters)
   - Check if documents already have summaries (skip if skip_with_summary is true)
   - Skip empty content documents
   - Log scan progress and statistics

4. **Summarize Action Implementation:**
   - Query documents in batches
   - For each document:
     - Limit content to content_limit characters
     - Build LLM messages with system_prompt and document content
     - Call `deps.LLMService.Chat(ctx, messages)` to generate summary
     - Update document metadata with summary, word_count, keywords
     - Save updated document via `deps.DocumentStorage.UpdateDocument()`
   - Handle errors based on error strategy (continue, fail, retry)
   - Log progress every 10 documents

5. **Extract Keywords Action Implementation:**
   - Query documents in batches
   - For each document:
     - Extract keywords using frequency analysis (reuse `extractKeywords` function from `internal/services/jobs/actions/summarizer_actions.go`)
     - Update document metadata with keywords array
     - Save updated document
   - Log progress and statistics

6. **Update Job Status:**
   - Log completion statistics (processed count, skipped count, error count)
   - Update parent job status if ParentID is set

**Reference Implementation:**
Adapt logic from `internal/services/jobs/actions/summarizer_actions.go`:
- scanAction (lines 93-183)
- summarizeAction (lines 186-325)
- extractKeywordsAction (lines 328-448)
- Helper functions: generateSummary, extractKeywords, calculateWordCount

**Error Handling:**
- Continue processing on individual document failures (log warnings)
- Return aggregated errors at the end
- Update job status to "failed" if critical errors occur

### internal\jobs\types\cleanup.go(MODIFY)

References: 

- internal\app\app.go(MODIFY)
- internal\interfaces\queue_service.go

**Update CleanupJob Constructor:**

Change `NewCleanupJob` signature to match usage in `internal/app/app.go` (line 411):
```go
func NewCleanupJob(base *BaseJob, jobStorage interfaces.JobStorage, logService interfaces.LogService) *CleanupJob
```

Update struct fields:
- Keep `jobStorage interfaces.JobStorage`
- Change `logStorage logs.JobLogStorage` to `logService interfaces.LogService`

**Complete Execute Method Implementation:**

Replace the TODO section with actual cleanup logic:

1. **Extract Configuration:**
   - Extract age_threshold_days from `msg.Config` (default: 30)
   - Extract status_filter from `msg.Config` (default: "completed")
   - Extract dry_run flag from `msg.Config` (default: false)

2. **Calculate Cleanup Threshold:**
   - Calculate cutoff time: `time.Now().Add(-time.Duration(ageThreshold) * 24 * time.Hour)`
   - Log cleanup criteria

3. **Query Old Jobs:**
   - Use `jobStorage.ListJobs()` with filters:
     - Status: status_filter ("completed", "failed", "cancelled")
     - CreatedAt < cutoff time
   - Count matching jobs

4. **Delete Old Logs:**
   - For each old job:
     - Call `logService.DeleteLogs(ctx, jobID)` to remove logs from job_logs table
     - Track deleted log count
   - Handle errors gracefully (log warnings, continue processing)

5. **Delete Old Jobs:**
   - For each old job:
     - Call `jobStorage.DeleteJob(ctx, jobID)` if interface supports deletion
     - Or mark jobs as archived/deleted
     - Track deleted job count
   - Handle errors gracefully

6. **Archive Jobs (Optional):**
   - Before deletion, optionally export job metadata to file or archive table
   - Store in configured archive location
   - Include job config, results, statistics

7. **Log Cleanup Summary:**
   - Log total jobs processed
   - Log total logs deleted
   - Log total jobs deleted
   - Log any errors encountered
   - Log cleanup duration

8. **Dry Run Mode:**
   - If dry_run is true, only log what would be deleted without actual deletion
   - Useful for testing cleanup criteria

**Error Handling:**
- Continue processing on individual job failures
- Log warnings for non-critical errors
- Return aggregated error summary at the end
- Don't fail entire cleanup if some jobs can't be deleted

**Safety Considerations:**
- Never delete jobs with status "running" or "pending"
- Verify age threshold is reasonable (minimum 7 days)
- Log detailed information before deletion for audit trail
- Consider adding confirmation mechanism for production use

### internal\services\jobs\registry.go(MODIFY)

References: 

- internal\app\app.go(MODIFY)
- internal\queue\worker.go

**Add Clarifying Documentation:**

Update the file header comment (after line 6) to clarify the distinction between two job systems:

```go
// Package jobs provides the JobTypeRegistry for managing action handlers
// used by JobDefinition execution (user-defined workflows).
//
// IMPORTANT: This registry is separate from the queue-based job types
// (crawler, summarizer, cleanup) which are registered with the QueueManager's
// WorkerPool in internal/app/app.go.
//
// JobTypeRegistry: Maps JobType → Action Name → Handler (for JobDefinitions)
// WorkerPool: Maps Job Type → Handler (for queue messages)
```

Update the comment at line 23-28 to clarify:

```go
// JobTypeRegistry manages the registration and retrieval of action handlers
// for different job types defined in JobDefinitions.
//
// This is used by JobExecutor to execute user-defined workflows with steps.
// For queue-based job processing, see internal/queue/worker.go and
// internal/jobs/types/ for job type implementations.
```

**No Functional Changes:**
This file continues to serve its purpose for JobDefinition action registration. The queue-based job types are a separate system that doesn't interact with this registry.

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\types\base.go(MODIFY)
- internal\jobs\types\crawler.go(MODIFY)
- internal\jobs\types\summarizer.go(MODIFY)

**Update Job Handler Registration (Lines 379-419):**

The job handler registration code is already correct, but needs minor adjustments to match the updated BaseJob constructor:

1. **Update BaseJob Constructor Calls:**
   - Line 388: Update to pass logService parameter
   - Line 401: Update to pass logService parameter  
   - Line 410: Update to pass logService parameter

2. **Verify Dependency Structs:**
   - Ensure CrawlerJobDeps struct matches the definition in `internal/jobs/types/crawler.go`
   - Ensure SummarizerJobDeps struct matches the definition in `internal/jobs/types/summarizer.go`
   - Add DocumentStorage to SummarizerJobDeps if needed

3. **Add Missing Dependencies:**
   - Add `a.StorageManager.DocumentStorage()` to SummarizerJobDeps (line 396-399)
   - Verify all required services are passed to dependency structs

**Example Updated Code Structure:**
```go
// Line 381-393: CrawlerJobDeps already correct
crawlerJobDeps := &jobtypes.CrawlerJobDeps{
    CrawlerService: a.CrawlerService,
    LogService:     a.LogService,
    JobStorage:     a.StorageManager.JobStorage(),
    QueueManager:   a.QueueManager,
}

// Line 388: Update BaseJob constructor
baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager)

// Line 395-406: Add DocumentStorage to SummarizerJobDeps
summarizerJobDeps := &jobtypes.SummarizerJobDeps{
    LLMService:      a.LLMService,
    DocumentStorage: a.StorageManager.DocumentStorage(), // ADD THIS
    Logger:          a.Logger,
}
```

**No Other Changes Needed:**
The handler registration pattern is correct. The factory closures properly create job instances and call Execute. Worker pool is started correctly at line 418.