I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Architecture:**
- Child URL jobs exist ONLY as queue messages (`crawler_url` type), not persisted to `crawl_jobs` table
- Parent jobs created in StartCrawl() don't set `job_type` field (defaults to 'parent' in SaveJob)
- SaveJob already supports `job_type` field (Phase 1 complete)
- Progress updates happen on EVERY URL (lines 691-732 in crawler.go) causing database contention
- DeleteJob has no idempotency check - fails on double-delete
- Foreign key CASCADE DELETE exists: `crawl_jobs.parent_id → crawl_jobs.id`, `job_logs.job_id → crawl_jobs.id`, `job_seen_urls.job_id → crawl_jobs.id`
- manager.go imported as `jobmgr` in app.go (line 20), needs relocation to `internal/services/jobs/`

**Key Integration Points:**
1. CrawlerJob.Execute() lines 627-671: Child job enqueueing loop - INSERT child CrawlJob records here
2. StartCrawl() line 314-330: Parent job creation - explicitly set JobType to JobTypeParent
3. Progress updates lines 691-732: Batch updates every 10 URLs or on completion/failure
4. DeleteJob line 536-543: Add existence check before DELETE
5. Import paths: Update `internal/jobs` → `internal/services/jobs` in app.go and other files

### Approach

Persist child crawler_url jobs to database with proper hierarchy, set job_type fields, implement batched progress updates to reduce database contention, add idempotency to DeleteJob, and reorganize folder structure by moving manager.go to services/jobs/.

### Reasoning

Read the complete job execution flow in CrawlerJob.Execute(), examined StartCrawl() parent job creation, reviewed JobStorage SaveJob/DeleteJob implementations, checked schema for CASCADE DELETE constraints, analyzed app.go initialization order, and identified all import paths for the manager.go relocation.

## Mermaid Diagram

sequenceDiagram
    participant UI as Queue Management UI
    participant Handler as JobHandler
    participant Manager as JobManager
    participant Storage as JobStorage
    participant Service as CrawlerService
    participant Worker as WorkerPool
    participant CrawlerJob as CrawlerJob.Execute()
    participant Queue as QueueManager

    Note over UI,Queue: Phase 1: Parent Job Creation
    UI->>Handler: POST /api/jobs/create
    Handler->>Service: StartCrawl(sourceType, seedURLs, config)
    Service->>Service: Create parent job with JobType='parent'
    Service->>Storage: SaveJob(parentJob)
    Storage-->>Service: Job persisted
    Service->>Queue: Enqueue seed URLs as crawler_url messages
    Queue-->>Service: Messages enqueued
    Service-->>Handler: Return parentJobID
    Handler-->>UI: 201 Created {job_id}

    Note over UI,Queue: Phase 2: Child Job Discovery & Persistence
    Worker->>Queue: Receive crawler_url message
    Worker->>CrawlerJob: Execute(ctx, msg)
    CrawlerJob->>CrawlerJob: Scrape URL, discover 35 links
    
    loop For each discovered URL
        CrawlerJob->>Storage: MarkURLSeen(parentID, childURL)
        Storage-->>CrawlerJob: isNew=true
        CrawlerJob->>CrawlerJob: Create CrawlJob record (JobType='crawler_url')
        CrawlerJob->>Storage: SaveJob(childJob)
        Storage-->>CrawlerJob: Child job persisted
        CrawlerJob->>Queue: Enqueue crawler_url message
        Queue-->>CrawlerJob: Message enqueued
    end

    CrawlerJob->>CrawlerJob: Batch progress update (every 10 URLs)
    CrawlerJob->>Storage: SaveJob(parentJob) [batched]
    Storage-->>CrawlerJob: Progress updated
    CrawlerJob-->>Worker: URL processing complete

    Note over UI,Queue: Phase 3: UI Hierarchy Display
    UI->>Handler: GET /api/jobs?parent_id={parentID}&grouped=true
    Handler->>Manager: ListJobs(opts)
    Manager->>Storage: ListJobs(parent_id filter)
    Storage-->>Manager: [parentJob, 35 childJobs]
    Manager->>Storage: GetJobChildStats([parentID])
    Storage-->>Manager: {childCount: 35, completed: 5, failed: 2}
    Manager-->>Handler: Jobs with stats
    Handler-->>UI: Hierarchical job tree

    Note over UI,Queue: Phase 4: Cascade Delete
    UI->>Handler: DELETE /api/jobs/{parentID}
    Handler->>Manager: DeleteJob(parentID)
    Manager->>Storage: DeleteJob(parentID) [idempotent check]
    Storage->>Storage: Check if job exists
    Storage->>Storage: DELETE FROM crawl_jobs WHERE id=parentID
    Storage->>Storage: CASCADE DELETE children (FK constraint)
    Storage->>Storage: CASCADE DELETE job_logs (FK constraint)
    Storage->>Storage: CASCADE DELETE job_seen_urls (FK constraint)
    Storage-->>Manager: Deletion complete
    Manager-->>Handler: Success
    Handler-->>UI: 200 OK {message: "Job deleted"}

## Proposed File Changes

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\models\crawler_job.go

**Update StartCrawl method to set job_type='parent' on root job (lines 314-330):**

1. After creating the `job` struct (line 314), explicitly set the JobType field:
   - Add `JobType: JobTypeParent,` to the CrawlJob initialization
   - This ensures parent jobs are properly marked in the hierarchy

2. Import the models package if not already imported to access `models.JobTypeParent` constant

**Rationale:** Currently job_type defaults to 'parent' in SaveJob validation, but explicit assignment at creation is clearer and prevents ambiguity.

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\models\crawler_job.go
- internal\storage\sqlite\job_storage.go(MODIFY)

**Persist child crawler_url jobs to database with proper hierarchy (lines 627-671):**

1. **After MarkURLSeen succeeds (line 589-604)**, create a CrawlJob record for each discovered child URL:
   - Generate child job ID: use the queue message ID format `fmt.Sprintf("%s-child-%d", msg.ID, enqueuedCount)`
   - Create CrawlJob struct with:
     - `ID`: generated child job ID
     - `ParentID`: msg.ParentID (root job ID for flat hierarchy)
     - `JobType`: models.JobTypeCrawlerURL
     - `Name`: fmt.Sprintf("URL: %s", childURL)
     - `SourceType`: sourceType (from msg.Config)
     - `EntityType`: entityType (from msg.Config)
     - `Config`: inherit from msg.Config (convert map to CrawlConfig)
     - `Status`: models.JobStatusPending
     - `Progress`: CrawlProgress with TotalURLs=1, PendingURLs=1
     - `CreatedAt`: time.Now()
   - Call `c.deps.JobStorage.SaveJob(ctx, childJob)` to persist
   - Log success/failure with child_url and child_id fields
   - Continue on save error (don't block enqueueing)

2. **Update child job status to running** when processing starts (line 168-199):
   - Check if current message is a child job (msg.ParentID != "" and msg.ID contains "-child-")
   - If yes, load child job from storage and update status to JobStatusRunning
   - Set StartedAt timestamp
   - Save updated child job

3. **Update child job status on completion/failure** (lines 744-765 and 313-331):
   - After successful URL processing, load child job and set Status=JobStatusCompleted, CompletedAt=time.Now()
   - On failure, set Status=JobStatusFailed, Error=formatJobError(...), CompletedAt=time.Now()
   - Save updated child job

**Implement batched progress updates to reduce database contention (lines 691-732):**

4. **Add progress update batching logic:**
   - Add counter field to CrawlerJob struct (or use atomic counter in deps): `progressUpdateCounter int`
   - After incrementing progress counters (lines 700-709), check if batch threshold reached:
     - `if progressUpdateCounter % 10 == 0 || job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed`
   - Only call `SaveJob` when threshold met or job reaches terminal status
   - Always save on completion/failure to ensure final state persisted
   - Log batched updates: "Progress update batched (10 URLs processed)"

5. **Ensure heartbeat updates are separate from progress saves:**
   - UpdateJobHeartbeat (line 685) is already a separate lightweight call
   - Keep heartbeat updates on every URL to track activity
   - Only batch the full SaveJob calls that update progress_json

**Rationale:** Persisting child jobs enables UI hierarchy display, progress tracking per URL, and proper CASCADE DELETE. Batching reduces write contention from concurrent workers while maintaining accurate progress via heartbeat.

### internal\storage\sqlite\job_storage.go(MODIFY)

**Add idempotency check to DeleteJob to prevent double-delete errors (lines 535-543):**

1. Before executing DELETE query, check if job exists:
   - Add query: `SELECT COUNT(*) FROM crawl_jobs WHERE id = ?`
   - Execute with jobID parameter
   - If count is 0, log debug message "Job already deleted" and return nil (success)
   - If count > 0, proceed with DELETE query

2. Update error handling:
   - If DELETE returns error, check if it's "no rows affected" (not an error in our case)
   - Log successful deletion with job_id field
   - Return nil on success or non-existent job

3. Add logging:
   - Debug log when job doesn't exist: "Job not found for deletion (already deleted or never existed)"
   - Info log on successful deletion: "Job deleted from storage"

**Rationale:** Idempotent deletes prevent errors when UI sends duplicate delete requests or when CASCADE DELETE already removed the job. This aligns with REST API best practices where DELETE should be idempotent.

### internal\jobs\manager.go → internal\services\jobs\manager.go

**Move manager.go to services/jobs/ directory for architectural consistency:**

1. Rename file from `internal/jobs/manager.go` to `internal/services/jobs/manager.go`
2. Update package declaration from `package jobs` to `package jobs` (no change needed)
3. This aligns with the existing `internal/services/jobs/executor.go` location
4. Both JobManager and JobExecutor now colocated in same package

**Rationale:** The manager.go file contains service-level CRUD operations and should be in the services layer alongside executor.go. The current location in internal/jobs/ is inconsistent since that directory primarily contains job type implementations (internal/jobs/types/).

### internal\app\app.go(MODIFY)

References: 

- internal\services\jobs\manager.go

**Update import path for relocated manager.go (line 20):**

1. Change import statement:
   - FROM: `jobmgr "github.com/ternarybob/quaero/internal/jobs"`
   - TO: `jobmgr "github.com/ternarybob/quaero/internal/services/jobs"`

2. Update NewManager call (line 374):
   - No change needed - still uses `jobmgr.NewManager(...)`
   - The alias `jobmgr` remains the same

3. Verify no other references to `internal/jobs` package exist in this file
   - The `jobtypes` import (line 21) remains unchanged as it points to `internal/jobs/types`

**Rationale:** After moving manager.go to services/jobs/, the import path must be updated to reflect the new location. Using the alias `jobmgr` avoids naming conflicts with the `jobs` package.

### internal\handlers\job_handler.go(MODIFY)

**Verify child job retrieval works with persisted child jobs:**

1. **GetJobHandler (lines 283-345)** already supports fetching jobs by ID:
   - No changes needed - child jobs will be fetchable via `/api/jobs/{child_id}`
   - Child statistics already enriched for parent jobs (lines 317-336)

2. **ListJobsHandler (lines 59-279)** already supports hierarchical queries:
   - `parent_id` filter parameter already implemented (line 68, 106)
   - Grouped view already implemented (lines 202-278)
   - Child statistics already calculated (lines 142-153)
   - No changes needed

3. **Add validation logging** for child job queries:
   - When fetching child jobs, log the parent_id and job_type for debugging
   - This helps verify the hierarchy is working correctly

**Rationale:** The handler layer already supports hierarchical job queries. Once child jobs are persisted, the existing endpoints will automatically return them. No structural changes needed, only verification that the flow works end-to-end.

### internal\jobs\types\base.go(MODIFY)

References: 

- internal\models\crawler_job.go

**Add helper method for creating child job records:**

1. Add new method `CreateChildJobRecord` to BaseJob:
   - Parameters: ctx, parentID, childID, url, depth, sourceType, entityType, config
   - Creates CrawlJob struct with proper hierarchy fields
   - Calls JobManager.UpdateJob (or JobStorage.SaveJob via JobManager)
   - Returns error if save fails
   - Logs creation with structured fields

2. This centralizes child job creation logic and ensures consistency

3. CrawlerJob.Execute can call this helper instead of duplicating logic

**Rationale:** Extracting child job creation to BaseJob promotes code reuse and ensures all job types create child records consistently. This is especially useful if future job types (PreValidationJob, PostSummaryJob) also need to spawn children.