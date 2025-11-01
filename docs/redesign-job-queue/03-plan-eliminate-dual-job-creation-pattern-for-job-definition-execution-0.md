I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Architecture Analysis:**

The codebase has a dual-job system that creates confusion:

1. **Orchestration Job Flow:**
   - `ExecuteJobDefinitionHandler` creates a "parent" type message using `NewJobDefinitionMessage()`
   - Creates a job record with `job.ID = parentMsg.ID` (lines 399-409 in job_definition_handler.go)
   - Enqueues the parent message to the queue
   - Worker pool receives it and routes to `parentJobHandler` (app.go:660-730)
   - `parentJobHandler` loads the job definition and calls `JobExecutor.Execute()`
   - This job completes immediately after steps finish

2. **Crawler Job Flow:**
   - `JobExecutor.Execute()` iterates through job definition steps
   - For "crawl" action, calls `crawlAction()` in crawler_actions.go
   - `crawlAction()` calls `StartCrawlJob()` helper (line 129-138)
   - `StartCrawlJob()` calls `CrawlerService.StartCrawl()` (line 320-329 in job_helper.go)
   - `StartCrawl()` creates a NEW crawler parent job (line 314-331 in service.go)
   - This is the actual job that spawns child URL jobs

**Key Findings:**

- The "parent" message type is ONLY used for job definition orchestration (no other use cases)
- `JobExecutor.Execute()` already handles multi-step workflows, error strategies, retry logic, and post-jobs
- The orchestration job provides no value to users - it's an internal implementation detail
- `StartCrawl()` already accepts 8 parameters including source config and auth snapshots
- Job metadata is stored as JSON in the `metadata` column of `crawl_jobs` table
- The `parentJobHandler` in app.go (lines 660-730) is the only handler for "parent" type messages

**Design Decision:**

The cleanest solution is to eliminate the orchestration wrapper entirely rather than trying to hide it in the UI. This simplifies the architecture and removes the confusion at the source.

### Approach

Eliminate the dual-job creation pattern by removing the orchestration wrapper job. When a Job Definition is executed, directly invoke the JobExecutor asynchronously without creating a parent message or job record. The JobExecutor will call StartCrawl() which creates the crawler parent job - this becomes the ONLY job visible in the UI. Pass JobDefinitionID through the execution chain and store it in the crawler job's metadata for traceability. Remove the now-unused "parent" message handler registration.

### Reasoning

I explored the repository structure, read the four files mentioned by the user (job_definition_handler.go, executor.go, manager.go, service.go), examined the queue message types and worker pool architecture, traced the current dual-job creation flow from handler → parent message → worker → JobExecutor → crawlAction → StartCrawlJob → StartCrawl, and identified that the "parent" message type is exclusively used for job definition orchestration and can be eliminated.

## Mermaid Diagram

sequenceDiagram
    participant UI as User/UI
    participant Handler as JobDefinitionHandler
    participant Executor as JobExecutor
    participant Action as crawlAction
    participant Helper as StartCrawlJob
    participant Crawler as CrawlerService
    participant Storage as JobStorage
    participant Queue as QueueManager
    participant Worker as WorkerPool

    Note over UI,Worker: BEFORE: Dual Job Creation (Current)
    UI->>Handler: POST /api/job-definitions/{id}/execute
    Handler->>Handler: Create parent message
    Handler->>Storage: SaveJob(orchestration job)
    Handler->>Queue: Enqueue(parent message)
    Handler-->>UI: 202 Accepted (orchestration job_id)
    Worker->>Queue: Receive parent message
    Worker->>Executor: Execute(jobDef)
    Executor->>Action: crawlAction(step)
    Action->>Helper: StartCrawlJob(source)
    Helper->>Crawler: StartCrawl(...)
    Crawler->>Storage: SaveJob(crawler parent job)
    Crawler->>Queue: Enqueue(child URL jobs)
    Note over UI,Worker: Result: TWO jobs in UI ❌

    Note over UI,Worker: AFTER: Single Job Creation (New)
    UI->>Handler: POST /api/job-definitions/{id}/execute
    Handler->>Handler: Launch goroutine
    Handler-->>UI: 202 Accepted (execution_id)
    Note over Handler: Async goroutine starts
    Handler->>Executor: Execute(jobDef, nil callbacks)
    Note over Executor: Store jobDef.ID in step.Config
    Executor->>Action: crawlAction(step)
    Note over Action: Extract job_definition_id from config
    Action->>Helper: StartCrawlJob(source, jobDefID)
    Helper->>Crawler: StartCrawl(..., jobDefID)
    Note over Crawler: Store jobDefID in job.Metadata
    Crawler->>Storage: SaveJob(crawler parent job)
    Crawler->>Queue: Enqueue(child URL jobs)
    Note over UI,Worker: Result: ONE job in UI ✅

## Proposed File Changes

### internal\handlers\job_definition_handler.go(MODIFY)

References: 

- internal\services\jobs\executor.go(MODIFY)
- internal\queue\types.go(MODIFY)

**Refactor ExecuteJobDefinitionHandler to eliminate orchestration job creation:**

1. **Remove parent message and job record creation** (lines 384-415):
   - Delete the `queue.NewJobDefinitionMessage()` call
   - Delete the job record creation (`job := &models.CrawlJob{...}`)
   - Delete the `h.jobStorage.SaveJob()` call
   - Delete the `h.queueManager.Enqueue()` call

2. **Add direct JobExecutor invocation** (replace lines 384-424):
   - Generate a unique execution ID using `uuid.New().String()` for tracking
   - Launch a goroutine that calls `h.jobExecutor.Execute()` directly
   - Pass `jobDef` and `nil` for both callbacks (status updates and post-job triggers)
   - The goroutine should handle errors by logging them (no status updates needed since no orchestration job exists)

3. **Update response payload** (lines 426-431):
   - Change `job_id` to return the execution ID (for logging/tracking purposes)
   - Update `message` to indicate "Job execution started" instead of "queued"
   - Keep `status` as "running" to indicate async execution

4. **Add logging:**
   - Log at start: "Starting job definition execution asynchronously"
   - Log in goroutine: "Job definition execution started" (with execution ID)
   - Log in goroutine on completion: "Job definition execution completed" or "Job definition execution failed"

**Design Rationale:**
- Async execution via goroutine maintains non-blocking behavior for the HTTP handler
- No orchestration job means only the crawler parent job appears in UI
- Execution ID provides traceability in logs without creating a database record
- JobExecutor handles all workflow logic (steps, errors, retries, post-jobs)
- Callbacks are nil because there's no orchestration job to update

### internal\services\jobs\executor.go(MODIFY)

References: 

- internal\services\jobs\actions\crawler_actions.go(MODIFY)
- internal\models\job_definition.go

**Modify JobExecutor to accept and propagate JobDefinitionID:**

1. **Update Execute() method signature** (around line 115):
   - Keep existing parameters: `ctx`, `jobDef`, `statusCallback`, `postJobCallback`
   - No signature change needed - `jobDef.ID` already contains the JobDefinitionID

2. **Store JobDefinitionID in step config for action handlers** (around line 180, in step iteration loop):
   - Before calling `action()`, add JobDefinitionID to step config:
     ```
     if step.Config == nil {
         step.Config = make(map[string]interface{})
     }
     step.Config["job_definition_id"] = jobDef.ID
     ```
   - This makes JobDefinitionID available to all action handlers

3. **Update logging** (throughout Execute method):
   - Add `job_definition_id` field to all log statements
   - Example: `logger.Info().Str("job_definition_id", jobDef.ID).Msg(...)`

4. **Handle nil callbacks gracefully** (lines 709-726):
   - Wrap all `statusCallback()` calls with nil checks: `if statusCallback != nil { ... }`
   - Wrap all `postJobCallback()` calls with nil checks: `if postJobCallback != nil { ... }`
   - This allows ExecuteJobDefinitionHandler to pass nil callbacks

**Design Rationale:**
- Storing JobDefinitionID in step config is the cleanest way to pass it to action handlers
- No interface changes needed - uses existing config map pattern
- Nil callback checks maintain backward compatibility with existing callers
- JobDefinitionID flows naturally through the execution chain

### internal\services\jobs\actions\crawler_actions.go(MODIFY)

References: 

- internal\services\jobs\job_helper.go(MODIFY)

**Update crawlAction to extract and pass JobDefinitionID:**

1. **Extract JobDefinitionID from step config** (after line 33, before source validation):
   - Add: `jobDefinitionID := extractString(step.Config, "job_definition_id", "")`
   - Log if present: `if jobDefinitionID != "" { logger.Debug().Str("job_definition_id", jobDefinitionID).Msg("Job definition ID found in step config") }`

2. **Pass JobDefinitionID to StartCrawlJob()** (line 129-138):
   - Add `jobDefinitionID` as the last parameter to `startCrawlJobFunc()` call
   - Update the function signature expectation in the call

3. **Update startCrawlJobFunc variable declaration** (line 26):
   - Change signature to include `jobDefinitionID string` as last parameter
   - This maintains testability while adding the new parameter

4. **Add logging** (around line 163):
   - Log JobDefinitionID when crawl job starts: `Str("job_definition_id", jobDefinitionID)`

**Design Rationale:**
- Uses existing `extractString()` helper for consistency
- Empty string default means JobDefinitionID is optional (backward compatible)
- Maintains testability via function variable pattern
- Minimal changes to existing logic

### internal\services\jobs\job_helper.go(MODIFY)

References: 

- internal\services\crawler\service.go(MODIFY)
- internal\services\jobs\crawl_collect_job.go

**Update StartCrawlJob to accept and pass JobDefinitionID:**

1. **Add jobDefinitionID parameter** (line 19):
   - Add `jobDefinitionID string` as the last parameter to `StartCrawlJob()` function signature
   - Update function documentation to describe the parameter

2. **Pass JobDefinitionID to StartCrawl()** (line 320-329):
   - Add `jobDefinitionID` as the last parameter to `crawlerService.StartCrawl()` call
   - This requires updating the call to include the new parameter

3. **Add logging** (around line 334):
   - If JobDefinitionID is not empty, log it: `if jobDefinitionID != "" { logger.Info().Str("job_definition_id", jobDefinitionID).Str("job_id", jobID).Msg("Crawl job linked to job definition") }`

4. **Update all call sites:**
   - `crawl_collect_job.go` line 95: Pass empty string `""` for JobDefinitionID (scheduled jobs don't have job definitions)
   - Any other call sites in the codebase should pass empty string for backward compatibility

**Design Rationale:**
- Adding parameter at the end maintains backward compatibility for existing callers
- Empty string indicates no job definition (scheduled jobs, manual jobs)
- Simple pass-through function - no complex logic needed

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\models\crawler_job.go

**Update StartCrawl to accept JobDefinitionID and store in metadata:**

1. **Add jobDefinitionID parameter** (line 263):
   - Add `jobDefinitionID string` as the last parameter to `StartCrawl()` method signature
   - Update method documentation to describe the parameter

2. **Store JobDefinitionID in job metadata** (after line 331, after job creation):
   - Initialize metadata map if nil: `if job.Metadata == nil { job.Metadata = make(map[string]interface{}) }`
   - Store JobDefinitionID: `if jobDefinitionID != "" { job.Metadata["job_definition_id"] = jobDefinitionID }`
   - Log storage: `if jobDefinitionID != "" { contextLogger.Debug().Str("job_definition_id", jobDefinitionID).Msg("Job definition ID stored in job metadata") }`

3. **Update all call sites in the file:**
   - Search for all `StartCrawl()` calls within service.go (if any)
   - Add empty string `""` as the last parameter for backward compatibility

4. **Update RerunJob() method** (around line 1050):
   - When calling `StartCrawl()`, extract JobDefinitionID from original job metadata
   - Pass it to the new job: `jobDefID, _ := originalJob.Metadata["job_definition_id"].(string)`
   - This preserves the job definition link when rerunning jobs

**Design Rationale:**
- Metadata storage is the standard pattern for optional job attributes
- Empty string check prevents storing unnecessary metadata
- Preserving JobDefinitionID on rerun maintains traceability
- No schema changes needed - metadata is already a JSON column

### internal\app\app.go(MODIFY)

References: 

- internal\queue\worker.go

**Remove parent job handler registration:**

1. **Delete parentJobHandler function** (lines 660-730):
   - Remove the entire `parentJobHandler` closure function
   - This handler is no longer needed since ExecuteJobDefinitionHandler invokes JobExecutor directly

2. **Delete handler registration** (line 731):
   - Remove: `a.WorkerPool.RegisterHandler("parent", parentJobHandler)`
   - Remove the associated log statement (line 732)

3. **Update comments** (if any reference parent handler):
   - Search for comments mentioning "parent handler" or "orchestration job"
   - Update or remove as appropriate

**Design Rationale:**
- The "parent" message type is no longer used anywhere in the codebase
- Removing dead code simplifies maintenance
- Worker pool will log warnings if "parent" messages are encountered (which shouldn't happen)
- This completes the elimination of the orchestration wrapper pattern

### internal\services\jobs\manager.go(MODIFY)

**Verify CreateJob method - no changes needed:**

The comment on lines 64-66 already states:
```
// NOTE: Parent message enqueuing removed - seed URLs are enqueued directly
// by CrawlerService.StartCrawl() which creates individual crawler_url messages.
// Job tracking is handled via JobStorage, not via queue messages.
```

This confirms that parent message enqueuing was already removed in a previous refactoring. The CreateJob method currently:
1. Creates a job record in the database
2. Does NOT enqueue any parent messages
3. Returns the job ID

**No modifications required** - the method already follows the desired pattern of not creating parent messages. The user's task description mentioned this file, but the work was already completed.

**Verification steps:**
- Confirm lines 64-66 contain the NOTE comment
- Confirm no `queueManager.Enqueue()` calls exist in CreateJob method
- Confirm the method only calls `jobStorage.SaveJob()`

### internal\storage\sqlite\schema.go(MODIFY)

References: 

- internal\models\crawler_job.go

**Add migration to clean up orphaned orchestration jobs:**

1. **Create migration method** (after existing migration methods, around line 1745):
   - Method name: `migrateCleanupOrphanedOrchestrationJobs() error`
   - Purpose: Delete jobs that were created as orchestration wrappers (no children, specific characteristics)

2. **Migration logic:**
   - Identify orphaned orchestration jobs with criteria:
     - `parent_id IS NULL OR parent_id = ''` (top-level jobs)
     - `job_type = 'parent'` (marked as parent type)
     - `entity_type = 'job_definition'` (created from job definitions)
     - `(SELECT COUNT(*) FROM crawl_jobs c WHERE c.parent_id = crawl_jobs.id) = 0` (no children)
   - Delete matching jobs: `DELETE FROM crawl_jobs WHERE ...`
   - Log count of deleted jobs
   - Return error if deletion fails

3. **Call migration in runMigrations()** (around line 386):
   - Add after last migration call: `if err := s.migrateCleanupOrphanedOrchestrationJobs(); err != nil { return err }`

4. **Add logging:**
   - Log at start: "Checking for orphaned orchestration jobs"
   - Log count: "Deleted X orphaned orchestration jobs"
   - Log if none found: "No orphaned orchestration jobs found"

**Design Rationale:**
- Migration runs once on startup to clean up existing orphaned jobs
- Criteria are specific enough to avoid deleting legitimate jobs
- Jobs with children are preserved (they might be legitimate parent jobs)
- Idempotent - safe to run multiple times
- No schema changes needed - just data cleanup

### internal\queue\types.go(MODIFY)

**Deprecate parent message constructors (optional cleanup):**

1. **Add deprecation comments** (lines 68-75 and 115-131):
   - Add comment above `NewParentJobMessage()`: `// Deprecated: Parent messages are no longer used. Job definitions execute directly via JobExecutor.`
   - Add comment above `NewJobDefinitionMessage()`: `// Deprecated: Job definitions execute directly via JobExecutor without creating parent messages.`

2. **Keep functions for backward compatibility:**
   - Do NOT delete the functions (might be used in tests or other code)
   - Deprecation comments warn developers not to use them
   - Can be removed in a future cleanup phase

**Design Rationale:**
- Deprecation is safer than deletion (avoids breaking existing code)
- Clear documentation prevents future use
- Maintains backward compatibility during transition period
- Can be fully removed after verifying no usage in tests