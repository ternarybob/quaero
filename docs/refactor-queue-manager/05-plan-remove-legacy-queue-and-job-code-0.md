I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**✅ Already Completed:**
1. Custom URLQueue removed - `queue.go` is empty, no imports reference it
2. Worker management removed - `orchestrator.go` has verification comments confirming removal of startWorkers, monitorCompletion, enqueueLinks, logQueueDiagnostics
3. Progress tracking removed - updateProgress, updateCurrentURL, updatePendingCount, emitProgress functions removed
4. Migration exists - `migrateRemoveLogsColumn()` implemented in schema.go (lines 1107-1237) and called in runMigrations() (line 324)
5. New architecture integrated - app.go shows QueueManager, WorkerPool, JobManager, and job handlers fully initialized

**❌ Should NOT Be Removed:**
1. **JobExecutor** (`internal/services/jobs/executor.go`) - Still actively used for:
   - JobDefinition execution with step-based workflows
   - Retry logic with exponential backoff
   - Async polling of crawl jobs with wait_for_completion
   - Error handling strategies (fail/continue/retry)
   - Progress event publishing for UI updates
   - This is complementary to the queue system, not replaced by it

2. **crawl_jobs table** (schema.go lines 103-127) - Still needed for:
   - Job metadata storage (name, description, config, status)
   - Progress tracking (total_urls, completed_urls, failed_urls, percentage)
   - Job history and results
   - Auth and source config snapshots
   - The queue system uses this table for persistence

**🔧 Needs Cleanup:**
1. Delete empty `queue.go` file
2. Update stale comments in test files (service_test.go references to workerLoop)
3. Verify no unused imports remain
4. Confirm migration runs successfully
5. Update architecture documentation

**📊 Architecture Verification:**
- Queue-based jobs: CrawlerJob, SummarizerJob, CleanupJob (handle individual tasks)
- JobExecutor: Orchestrates multi-step workflows defined by users
- Both systems coexist and serve different purposes

### Approach

## Cleanup Strategy

The queue/jobs refactoring is **95% complete**. The remaining work involves:

1. **File Deletion**: Remove the empty `queue.go` file
2. **Documentation Updates**: Update comments to reflect the new architecture
3. **Verification**: Confirm the migration runs successfully and tests pass
4. **Preservation**: Keep JobExecutor and crawl_jobs table (still actively used)

**Key Principle**: This is a cleanup task, not a refactoring task. The new architecture is already functional and integrated.

### Reasoning

Explored the codebase to assess the cleanup requirements. Read the four files mentioned by the user:
- `queue.go` - Empty file (already deleted)
- `executor.go` - Still actively used for JobDefinition execution with polling
- `schema.go` - Contains migration to remove logs column (already implemented)
- `app.go` - Shows full integration of new queue-based architecture

Searched for references to removed code (URLQueue, workerLoop, startWorkers, monitorCompletion) and found only comments documenting the removal. Verified that JobExecutor is still needed for step-based job workflows and that crawl_jobs table stores essential job metadata.

## Proposed File Changes

### internal\services\crawler\types.go(MODIFY)

References: 

- internal\queue\types.go

**Update Documentation Comments:**

The file already has a verification comment at line 40-41 documenting the removal of URLQueueItem. Enhance this comment to provide more context:

Change from:
```go
// VERIFICATION COMMENT 2: URLQueueItem removed - legacy type from old worker-based architecture
// The new queue system uses queue.JobMessage instead (see internal/queue/types.go)
```

To:
```go
// URLQueueItem was removed during queue refactoring (replaced by queue.JobMessage)
// The custom URLQueue with priority heap and deduplication has been replaced by
// goqite-backed queue manager with persistent storage and worker pool.
// See internal/queue/types.go for the new message types.
```

**No Code Changes**: This file contains only type definitions and is still actively used.

### internal\services\crawler\orchestrator.go(MODIFY)

References: 

- internal\queue\worker.go
- internal\jobs\types\crawler.go
- internal\services\crawler\filters.go

**Update File Header Comment:**

The file already has good documentation (lines 3-8) explaining the migration. Enhance the header to clarify what remains:

Change from:
```go
// orchestrator.go contains link filtering functions for URL processing.
// Worker management has been migrated to queue.WorkerPool.
// Job execution is handled by queue-based job types (internal/jobs/types/crawler.go).
// VERIFICATION COMMENT 9: Progress tracking functions removed (now handled by queue-based jobs).
// VERIFICATION COMMENT 3: Regex imports removed (filtering now handled by shared LinkFilter helper).
```

To:
```go
// orchestrator.go contains link filtering utilities for the crawler service.
//
// ARCHITECTURE NOTES:
// - Worker management: Migrated to queue.WorkerPool (internal/queue/worker.go)
// - Job execution: Handled by CrawlerJob type (internal/jobs/types/crawler.go)
// - Progress tracking: Managed by JobStorage and queue stats (removed from this file)
// - Link filtering: Uses shared LinkFilter helper (internal/services/crawler/filters.go)
//
// REMAINING FUNCTIONS:
// - filterLinks(): Applies include/exclude patterns using LinkFilter helper
```

**Verify No Dead Code**: The file should only contain the filterLinks() function. All other functions (startWorkers, monitorCompletion, enqueueLinks, logQueueDiagnostics, updateProgress, updateCurrentURL, updatePendingCount, emitProgress) should already be removed.

**No Code Changes**: The filterLinks() function is still actively used by the crawler service.

### internal\services\crawler\service_test.go(MODIFY)

References: 

- internal\jobs\types\crawler.go

**Update Test Comments:**

The test file has several comments referencing the old workerLoop (lines 1256, 1269, 1281, 1289, 1297, 1311, 1391). Update these comments to reflect the new architecture:

Change references like:
- "what workerLoop would produce" → "what CrawlerJob.Execute() produces"
- "as workerLoop does" → "as CrawlerJob does"
- "Simulate what workerLoop does" → "Simulate what CrawlerJob.Execute() does"

**Example Update (line 1256):**
```go
// Create a simulated crawl result (what CrawlerJob.Execute() produces)
```

**Example Update (line 1269):**
```go
// Simulate what CrawlerJob.Execute() does: extract markdown and save document
```

**Rationale**: These are documentation updates only. The test logic remains valid - it's testing the same document creation and storage behavior, just with updated terminology.

**No Test Logic Changes**: The tests are still valid and should continue to pass. Only update comments for clarity.

### internal\services\jobs\executor.go(MODIFY)

References: 

- internal\jobs\types\crawler.go
- internal\models\job_definition.go
- internal\app\app.go(MODIFY)

**Add Clarifying Documentation:**

Add a comprehensive file header comment explaining the role of JobExecutor in the new architecture:

```go
// Package jobs provides the JobExecutor for orchestrating user-defined job workflows.
//
// ARCHITECTURE NOTES:
// JobExecutor is NOT replaced by the queue system - it serves a different purpose:
//
// - JobExecutor: Orchestrates multi-step workflows defined by users (JobDefinitions)
//   - Executes steps sequentially with retry logic and error handling
//   - Polls crawl jobs asynchronously when wait_for_completion is enabled
//   - Publishes progress events for UI updates
//   - Supports error strategies: fail, continue, retry
//
// - Queue System: Handles individual task execution (CrawlerJob, SummarizerJob, CleanupJob)
//   - Processes URLs, generates summaries, cleans up old jobs
//   - Provides persistent queue with worker pool
//   - Enables job spawning and depth tracking
//
// Both systems coexist and complement each other:
// - JobDefinitions can trigger crawl jobs via the crawl action
// - JobExecutor polls those crawl jobs until completion
// - Crawl jobs are executed by the queue-based CrawlerJob type
```

**No Code Changes**: The JobExecutor implementation is correct and actively used. This is documentation only.

**Verification**: Confirmed that JobExecutor is initialized in app.go (line 506) and used by JobDefinitionHandler (app.go line 628).

### internal\storage\sqlite\schema.go(MODIFY)

References: 

- internal\logs\service.go
- internal\storage\sqlite\job_log_storage.go

**Verify Migration Documentation:**

The migration to remove the logs column is already implemented (lines 1107-1237) and called in runMigrations() (line 324). Verify the documentation is clear:

**At line 323-326**, ensure the comment explains the migration:
```go
// MIGRATION 13: Remove deprecated logs column from crawl_jobs table
// Logs are now stored in the dedicated job_logs table with unlimited history
// and better query performance. This migration recreates the crawl_jobs table
// without the logs column while preserving all other data.
if err := s.migrateRemoveLogsColumn(); err != nil {
    return err
}
```

**At line 1107**, ensure the function comment is comprehensive:
```go
// migrateRemoveLogsColumn removes the deprecated logs column from crawl_jobs table.
// The logs column stored job logs as a JSON array with a 100-entry limit.
// Logs are now stored in the dedicated job_logs table (see lines 145-158) which provides:
// - Unlimited log history (no truncation)
// - Better query performance with indexes
// - Automatic CASCADE DELETE when jobs are deleted
// - Batched writes via LogService for efficiency
```

**Verify crawl_jobs Table Documentation (line 101-103):**

Ensure the table comment explains its purpose in the new architecture:
```sql
-- Crawler job history with configuration snapshots for re-runnable jobs
-- Inspired by Firecrawl's async job model
-- Used by both JobExecutor (for JobDefinition workflows) and queue-based jobs
-- The 'logs' column was removed in MIGRATION 13 (logs now in job_logs table)
```

**No Schema Changes**: The migration is already implemented correctly. This is documentation verification only.

### internal\app\app.go(MODIFY)

References: 

- internal\queue\manager.go
- internal\jobs\manager.go
- internal\queue\worker.go
- internal\services\jobs\executor.go(MODIFY)

**Add Architecture Documentation Comment:**

Add a comprehensive comment at the top of the initServices() method (after line 183) explaining the service initialization order and architecture:

```go
// initServices initializes all business services in dependency order.
//
// QUEUE-BASED JOB ARCHITECTURE:
// 1. QueueManager (goqite-backed) - Persistent queue with worker pool
// 2. JobManager - CRUD operations for jobs
// 3. WorkerPool - Registers handlers for job types (crawler_url, summarizer, cleanup, parent)
// 4. Job Types - CrawlerJob, SummarizerJob, CleanupJob (handle individual tasks)
//
// JOB DEFINITION ARCHITECTURE:
// 1. JobRegistry - Maps job types to action handlers
// 2. JobExecutor - Orchestrates multi-step workflows with retry and polling
// 3. Action Handlers - CrawlerActions, SummarizerActions (registered with JobRegistry)
//
// Both systems coexist:
// - Queue system: Handles individual task execution (URLs, summaries, cleanup)
// - JobExecutor: Orchestrates user-defined workflows (JobDefinitions)
// - JobDefinitions can trigger crawl jobs, which are executed by the queue system
```

**Verify Service Initialization Order:**

Confirm the initialization order is correct (no changes needed, just verification):
1. LLM Service (line 187)
2. Log Service (line 203)
3. Context Logging (line 210)
4. Document Service (line 267)
5. Search Service (line 273)
6. Chat Service (line 279)
7. Event Service (line 287)
8. Status Service (line 290)
9. Source Service (line 295)
10. **Queue Manager (line 323)** ✓
11. **Job Manager (line 334)** ✓
12. **Worker Pool (line 339)** ✓
13. Auth Service (line 344)
14. Crawler Service (line 353)
15. Transformers (line 361, 371)
16. **Job Handlers Registration (line 382-469)** ✓
17. Summary Service (line 492)
18. **Job Registry & Executor (line 505-534)** ✓
19. Scheduler Service (line 537)

**No Code Changes**: The initialization is correct. This is documentation only.

### docs\QUEUE_MANAGER_IMPLEMENTATION_STATUS.md(MODIFY)

References: 

- docs\architecture.md

**Update Implementation Status Document:**

If this documentation file exists, update it to reflect the completed refactoring:

1. Mark all phases as **COMPLETED**
2. Add a "Cleanup Completed" section documenting:
   - Custom URLQueue removed
   - Worker management migrated to queue.WorkerPool
   - Logs migrated to dedicated job_logs table
   - JobExecutor preserved (still needed for JobDefinitions)
   - crawl_jobs table preserved (still needed for job metadata)

3. Add an "Architecture Summary" section:
   ```markdown
   ## Final Architecture
   
   ### Queue-Based Jobs
   - **Purpose**: Handle individual task execution
   - **Components**: QueueManager, WorkerPool, JobMessage
   - **Job Types**: CrawlerJob, SummarizerJob, CleanupJob
   - **Storage**: goqite queue table + crawl_jobs metadata
   
   ### Job Definitions
   - **Purpose**: Orchestrate multi-step workflows
   - **Components**: JobExecutor, JobRegistry, ActionHandlers
   - **Job Types**: User-defined workflows with steps
   - **Storage**: job_definitions table + crawl_jobs for results
   
   ### Coexistence
   - JobDefinitions can trigger crawl jobs via crawl action
   - JobExecutor polls crawl jobs until completion
   - Crawl jobs are executed by queue-based CrawlerJob type
   ```

**If File Doesn't Exist**: Create it with the above content to document the refactoring for future reference.

### README.md(MODIFY)

References: 

- docs\architecture.md

**Update Architecture Section:**

If the README has an architecture section, update it to reflect the queue-based job system:

1. Replace any references to "custom URLQueue" with "goqite-backed queue manager"
2. Add a section explaining the dual job architecture:
   - Queue-based jobs for individual tasks
   - JobExecutor for multi-step workflows
3. Update any diagrams or flowcharts to show the new architecture
4. Document the migration from logs column to job_logs table

**Example Section:**
```markdown
## Job Processing Architecture

Quaero uses a dual job processing architecture:

### Queue-Based Jobs
- **Purpose**: Execute individual tasks (URL crawling, summarization, cleanup)
- **Technology**: goqite (persistent SQLite-backed queue)
- **Components**: QueueManager, WorkerPool, JobMessage
- **Job Types**: CrawlerJob, SummarizerJob, CleanupJob

### Job Definitions
- **Purpose**: Orchestrate multi-step workflows defined by users
- **Technology**: JobExecutor with retry logic and async polling
- **Components**: JobRegistry, ActionHandlers
- **Job Types**: User-defined workflows with configurable steps

Both systems work together: JobDefinitions can trigger crawl jobs, which are then executed by the queue-based CrawlerJob type.
```

**No Breaking Changes**: This is documentation only to help users understand the architecture.

### AGENTS.md(MODIFY)

References: 

- internal\queue\manager.go
- internal\jobs\types\crawler.go
- internal\services\jobs\executor.go(MODIFY)

**Update Agent Instructions:**

Update the "Architecture Overview" section to reflect the completed queue refactoring:

1. **Replace Custom Queue References:**
   - Change "custom URLQueue" to "goqite-backed queue manager"
   - Update worker pool description to reference queue.WorkerPool

2. **Add Queue Architecture Section:**
   ```markdown
   ### Queue-Based Job Processing
   
   Quaero uses goqite for persistent job queue management:
   - **QueueManager** (`internal/queue/manager.go`) - Lifecycle management, message operations
   - **WorkerPool** (`internal/queue/worker.go`) - Worker pool with registered handlers
   - **JobMessage** (`internal/queue/types.go`) - Message types for different job types
   - **Job Types** (`internal/jobs/types/`) - CrawlerJob, SummarizerJob, CleanupJob
   
   Job execution flow:
   1. User triggers job via UI or JobDefinition
   2. Job message enqueued to goqite queue
   3. Worker pool receives message and routes to handler
   4. Handler executes job (fetch URL, generate summary, etc.)
   5. Job spawns child jobs if needed (URL discovery)
   6. Progress tracked in crawl_jobs table
   7. Logs stored in job_logs table
   ```

3. **Update JobExecutor Section:**
   ```markdown
   ### Job Definitions vs Queue Jobs
   
   **JobExecutor** (`internal/services/jobs/executor.go`):
   - Orchestrates multi-step workflows defined by users
   - Executes steps sequentially with retry logic
   - Polls crawl jobs asynchronously when wait_for_completion is enabled
   - NOT replaced by queue system - serves different purpose
   
   **Queue Jobs** (`internal/jobs/types/`):
   - Handle individual task execution
   - Process URLs, generate summaries, clean up old jobs
   - Spawned by JobExecutor or directly by user actions
   ```

4. **Update Storage Schema Section:**
   - Document that logs column was removed from crawl_jobs
   - Explain job_logs table with unlimited history
   - Note CASCADE DELETE for automatic cleanup

**Rationale**: Ensure AI agents understand the new architecture when working with the codebase.