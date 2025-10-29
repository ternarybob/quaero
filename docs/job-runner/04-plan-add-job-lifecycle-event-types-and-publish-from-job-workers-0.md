I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State

**Event Infrastructure:**
- EventService interface exists with Publish/Subscribe methods
- Existing events: EventCrawlProgress, EventJobProgress, EventJobSpawn
- Missing: Job lifecycle events (created, started, completed, failed, cancelled)

**Job Lifecycle Flow:**
1. **Creation**: `service.go` StartCrawl() creates job → saves to DB → adds to activeJobs
2. **Start**: `crawler.go` Execute() processes first URL → logs start
3. **Completion**: `crawler.go` ExecuteCompletionProbe() verifies idle → marks complete
4. **Failure**: `service.go` FailJob() called by scheduler → marks failed
5. **Cancellation**: `service.go` CancelJob() called by user → marks cancelled

**EventService Access:**
- ✅ `crawler.go`: Has EventService via `c.deps.EventService` (line 23)
- ✅ `service.go`: Has EventService via `s.eventService` (line 109)
- ❌ `worker.go`: No EventService (not needed - handlers publish events)

**Key Insight:**
EventJobSpawn already published in `crawler.go` (lines 416-429) demonstrates the pattern. New events follow the same approach: create Event struct with type and payload, call EventService.Publish().

**Payload Consistency:**
All job lifecycle events should include consistent metadata:
- job_id, status, source_type, entity_type (always)
- result_count, failed_count (for terminal states)
- error (for failed events)
- timestamp (all events)

### Approach

**Event-Driven Job Lifecycle Tracking**

Add 5 new event types to the event service interface and publish them at key job lifecycle transitions. This enables real-time UI updates via WebSocket (handled in subsequent phase) without coupling business logic to transport layer.

**Strategy:**
1. Define event types with structured payload documentation
2. Publish events after database persistence (ensures consistency)
3. Use existing EventService infrastructure (no new dependencies)
4. Maintain clean architecture (services publish, subscribers consume)

### Reasoning

I read the three files mentioned by the user (`event_service.go`, `crawler.go`, `worker.go`), then examined `service.go` to find job cancellation/failure handlers and `crawler_job.go` to understand the job model structure. I identified that EventService already exists in both `crawler.go` (via deps) and `service.go` (as field), eliminating the need for dependency injection. I traced the job lifecycle through StartCrawl (creation), Execute (start), ExecuteCompletionProbe (completion), CancelJob (cancellation), and FailJob (failure) methods to pinpoint exact event publishing locations.

## Mermaid Diagram

sequenceDiagram
    participant User as User/Scheduler
    participant Service as Crawler Service
    participant Queue as Queue Manager
    participant Worker as Worker Pool
    participant Job as CrawlerJob
    participant EventBus as Event Service
    participant DB as Job Storage

    Note over User,DB: Job Creation Flow
    User->>Service: StartCrawl(seedURLs, config)
    Service->>Service: Create CrawlJob (status=pending)
    Service->>DB: SaveJob(job)
    DB-->>Service: Success
    Service->>EventBus: Publish(EventJobCreated)
    Note over EventBus: job_id, status="pending", seed_count
    Service->>Queue: Enqueue seed URLs
    Service-->>User: Return jobID

    Note over User,DB: Job Start Flow
    Queue->>Worker: Receive message (first URL)
    Worker->>Job: Execute(msg)
    Job->>Job: Log job start
    Job->>EventBus: Publish(EventJobStarted)
    Note over EventBus: job_id, status="running", url
    Job->>Job: Scrape URL, save document
    Job->>DB: UpdateJobHeartbeat()
    Job->>DB: Update Progress counters
    Job-->>Worker: Success

    Note over User,DB: Job Completion Flow
    Worker->>Job: Execute(last URL)
    Job->>Job: PendingURLs == 0
    Job->>Queue: Enqueue completion probe (5s delay)
    Queue->>Worker: Receive probe (after 5s)
    Worker->>Job: ExecuteCompletionProbe(msg)
    Job->>DB: GetJob(jobID)
    Job->>Job: Verify: PendingURLs=0, heartbeat>5s old
    Job->>Job: Mark status=completed, sync counts
    Job->>DB: SaveJob(job)
    DB-->>Job: Success
    Job->>EventBus: Publish(EventJobCompleted)
    Note over EventBus: job_id, status="completed", result_count, duration
    Job-->>Worker: Success

    Note over User,DB: Job Cancellation Flow
    User->>Service: CancelJob(jobID)
    Service->>Service: Mark status=cancelled, sync counts
    Service->>DB: SaveJob(job)
    DB-->>Service: Success
    Service->>EventBus: Publish(EventJobCancelled)
    Note over EventBus: job_id, status="cancelled", result_count
    Service-->>User: Success

    Note over User,DB: Job Failure Flow
    User->>Service: FailJob(jobID, reason)
    Service->>Service: Mark status=failed, sync counts
    Service->>DB: SaveJob(job)
    DB-->>Service: Success
    Service->>EventBus: Publish(EventJobFailed)
    Note over EventBus: job_id, status="failed", error, result_count
    Service-->>User: Success

## Proposed File Changes

### internal\interfaces\event_service.go(MODIFY)

**Location: Lines 8-94 (Event type constants)**

Add 5 new event type constants after EventJobSpawn (line 93):

1. **EventJobCreated** - Published when a new crawl job is created and persisted
   - Add constant: `EventJobCreated EventType = "job_created"`
   - Add documentation comment block (similar to EventJobSpawn format):
     - Describe: Published when a new job is created via StartCrawl
     - Payload structure: job_id (string), status ("pending"), source_type (string), entity_type (string), seed_url_count (int), timestamp (time.Time)
     - Note: Published after successful database persistence

2. **EventJobStarted** - Published when a job begins processing its first URL
   - Add constant: `EventJobStarted EventType = "job_started"`
   - Add documentation comment block:
     - Describe: Published when job transitions from pending to running (first URL processed)
     - Payload structure: job_id (string), status ("running"), source_type (string), entity_type (string), url (string - first URL), timestamp (time.Time)
     - Note: Published at the start of CrawlerJob.Execute for the first URL

3. **EventJobCompleted** - Published when a job successfully completes all URLs
   - Add constant: `EventJobCompleted EventType = "job_completed"`
   - Add documentation comment block:
     - Describe: Published when job completes after grace period verification
     - Payload structure: job_id (string), status ("completed"), source_type (string), entity_type (string), result_count (int), failed_count (int), total_urls (int), duration_seconds (float64), timestamp (time.Time)
     - Note: Published after marking job complete in ExecuteCompletionProbe

4. **EventJobFailed** - Published when a job fails due to system errors or timeout
   - Add constant: `EventJobFailed EventType = "job_failed"`
   - Add documentation comment block:
     - Describe: Published when job is marked as failed (stale job detection, system errors)
     - Payload structure: job_id (string), status ("failed"), source_type (string), entity_type (string), result_count (int), failed_count (int), error (string), timestamp (time.Time)
     - Note: Published after marking job failed in Service.FailJob

5. **EventJobCancelled** - Published when a user cancels a running job
   - Add constant: `EventJobCancelled EventType = "job_cancelled"`
   - Add documentation comment block:
     - Describe: Published when user cancels a running job via API
     - Payload structure: job_id (string), status ("cancelled"), source_type (string), entity_type (string), result_count (int), failed_count (int), timestamp (time.Time)
     - Note: Published after marking job cancelled in Service.CancelJob

**Rationale:** These events enable real-time job status tracking in the UI without polling. Consistent payload structure simplifies subscriber implementation (next phase). Documentation follows existing pattern (EventCrawlProgress, EventJobSpawn) for maintainability.

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\interfaces\event_service.go(MODIFY)
- internal\models\crawler_job.go

**Location 1: Lines 76-79 (Job Start Event)**

Publish EventJobStarted after logging job start:

1. After line 79 (LogJobEvent for job start), add event publishing logic
2. Check if EventService is available: `if c.deps.EventService != nil`
3. Create Event struct with type `interfaces.EventJobStarted`
4. Build payload map with keys:
   - "job_id": msg.ParentID
   - "status": "running" (job is now processing)
   - "source_type": sourceType (extracted from msg.Config)
   - "entity_type": entityType (if available in config, else "url")
   - "url": msg.URL (first URL being processed)
   - "depth": msg.Depth
   - "timestamp": time.Now()
5. Call `c.deps.EventService.Publish(ctx, event)`
6. Log warning if publish fails (non-fatal): `c.logger.Warn().Err(err).Msg("Failed to publish job started event")`

**Rationale:** Publishes when job transitions from pending to running. This is the first URL processed, indicating active work has begun. Non-blocking publish (fire-and-forget) ensures crawler performance isn't impacted.

**Location 2: Lines 654-678 (Job Completion Event)**

Publish EventJobCompleted after marking job complete in ExecuteCompletionProbe:

1. After line 663 (SaveJob for completed job), add event publishing logic
2. Check if EventService is available: `if c.deps.EventService != nil`
3. Calculate job duration: `duration := job.CompletedAt.Sub(job.CreatedAt)`
4. Create Event struct with type `interfaces.EventJobCompleted`
5. Build payload map with keys:
   - "job_id": msg.ParentID
   - "status": "completed"
   - "source_type": job.SourceType
   - "entity_type": job.EntityType
   - "result_count": job.ResultCount (synced at line 655)
   - "failed_count": job.FailedCount (synced at line 656)
   - "total_urls": job.Progress.TotalURLs
   - "duration_seconds": duration.Seconds()
   - "timestamp": time.Now()
6. Call `c.deps.EventService.Publish(ctx, event)`
7. Log warning if publish fails: `c.logger.Warn().Err(err).Msg("Failed to publish job completed event")`

**Rationale:** Publishes after grace period verification and database persistence. Includes comprehensive completion metrics (counts, duration) for UI display and analytics. Published after SaveJob ensures event reflects persisted state.

**Note:** Do NOT publish EventJobStarted on every URL - only on the first URL when job transitions to running. Check job status before publishing to avoid duplicate events.

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\interfaces\event_service.go(MODIFY)
- internal\models\crawler_job.go

**Location 1: Lines 497-507 (Job Creation Event)**

Publish EventJobCreated after persisting job to database in StartCrawl:

1. After line 502 (successful SaveJob), before line 504 (adding to activeJobs), add event publishing logic
2. Check if EventService is available: `if s.eventService != nil`
3. Create Event struct with type `interfaces.EventJobCreated`
4. Build payload map with keys:
   - "job_id": jobID
   - "status": "pending" (job.Status)
   - "source_type": sourceType
   - "entity_type": entityType
   - "seed_url_count": len(seedURLs)
   - "max_depth": config.MaxDepth
   - "max_pages": config.MaxPages
   - "follow_links": config.FollowLinks
   - "timestamp": time.Now()
5. Call `s.eventService.Publish(s.ctx, event)`
6. Log warning if publish fails: `contextLogger.Warn().Err(err).Msg("Failed to publish job created event")`

**Rationale:** Publishes after successful database persistence, ensuring event reflects committed state. Includes configuration summary for UI display. Published before adding to activeJobs to maintain event ordering (created → started).

**Location 2: Lines 668-689 (Job Cancellation Event)**

Publish EventJobCancelled after persisting cancellation status in CancelJob:

1. After line 681 (successful SaveJob), before line 684 (logging cancellation), add event publishing logic
2. Check if EventService is available: `if s.eventService != nil`
3. Create Event struct with type `interfaces.EventJobCancelled`
4. Build payload map with keys:
   - "job_id": jobID
   - "status": "cancelled" (job.Status)
   - "source_type": job.SourceType
   - "entity_type": job.EntityType
   - "result_count": job.ResultCount (synced at line 672)
   - "failed_count": job.FailedCount (synced at line 673)
   - "completed_urls": job.Progress.CompletedURLs
   - "pending_urls": job.Progress.PendingURLs
   - "timestamp": time.Now()
5. Call `s.eventService.Publish(s.ctx, event)`
6. Log warning if publish fails: `contextLogger.Warn().Err(err).Msg("Failed to publish job cancelled event")`

**Rationale:** Publishes after database persistence, ensuring event reflects committed cancellation. Includes progress metrics to show work completed before cancellation. User-initiated action, so immediate event publishing is important for UI responsiveness.

**Location 3: Lines 718-742 (Job Failure Event)**

Publish EventJobFailed after persisting failure status in FailJob:

1. After line 733 (successful SaveJob), before line 736 (logging failure), add event publishing logic
2. Check if EventService is available: `if s.eventService != nil`
3. Create Event struct with type `interfaces.EventJobFailed`
4. Build payload map with keys:
   - "job_id": jobID
   - "status": "failed" (job.Status)
   - "source_type": job.SourceType
   - "entity_type": job.EntityType
   - "result_count": job.ResultCount (synced at line 724)
   - "failed_count": job.FailedCount (synced at line 725)
   - "error": reason (failure reason from parameter)
   - "completed_urls": job.Progress.CompletedURLs
   - "pending_urls": job.Progress.PendingURLs
   - "timestamp": time.Now()
5. Call `s.eventService.Publish(s.ctx, event)`
6. Log warning if publish fails: `contextLogger.Warn().Err(err).Msg("Failed to publish job failed event")`

**Rationale:** Publishes after database persistence, ensuring event reflects committed failure state. Includes error reason for debugging and user notification. Called by scheduler for stale job detection, so event enables UI to show failure immediately.

**Note:** All three events use `s.eventService` (Service struct field, line 109). No dependency injection needed - EventService already available.

### internal\queue\worker.go(MODIFY)

References: 

- internal\jobs\types\crawler.go(MODIFY)
- internal\services\crawler\service.go(MODIFY)

**No changes required** - verification only.

**Verification Points:**

1. **Line 190**: Handler execution (`handlerErr := handler(wp.ctx, jobMsg)`)
   - Handler is the CrawlerJob.Execute method
   - CrawlerJob publishes EventJobStarted internally (added in crawler.go)
   - No event publishing needed in worker.go

2. **Lines 193-216**: Handler failure path
   - Job-level failures (scraping errors, validation errors) are handled by CrawlerJob.Execute
   - System-level failures (stale jobs) are handled by Service.FailJob
   - Both publish EventJobFailed in their respective locations
   - No event publishing needed in worker.go

3. **Lines 219-237**: Handler success path
   - Individual URL completion doesn't trigger job-level events
   - Job completion is detected by ExecuteCompletionProbe (publishes EventJobCompleted)
   - No event publishing needed in worker.go

**Rationale:** Worker pool is a generic task executor - it doesn't know about job lifecycle semantics. Job-specific events are published by job handlers (CrawlerJob) and service methods (Service.CancelJob, Service.FailJob). This maintains clean separation of concerns: worker pool handles task execution, job types handle business logic and events.

**Architecture Validation:**
- ✅ Worker pool remains generic and reusable
- ✅ Job lifecycle events published by domain logic (crawler service, crawler job)
- ✅ No coupling between worker pool and event system
- ✅ Follows existing pattern (EventJobSpawn published by CrawlerJob, not worker)