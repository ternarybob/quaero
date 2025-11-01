I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase has a well-structured job execution system with:

1. **Job Definition Model** - Stores job configuration in `models.JobDefinition` with JSON serialization for database storage
2. **Queue-Based Execution** - Jobs are executed via `JobMessage` in the queue, with `JobDefinitionID` field already available for linking
3. **Child Job Tracking** - `JobStorage.GetJobChildStats()` provides aggregate child statistics (total, completed, failed counts)
4. **Event System** - `EventJobFailed` already exists and is wired through WebSocket for UI updates
5. **Completion Detection** - `ExecuteCompletionProbe()` handles job completion verification with grace period and stale job detection
6. **Job Cancellation** - Jobs can be marked as cancelled via `UpdateJobStatus()`, and `DeleteJob()` supports recursive cascade deletion

**Key Findings:**
- `JobDefinitionID` is already propagated through `JobMessage` to all child jobs (lines 790, 1005, 1124, 1193 in crawler.go)
- Child failure counts are available via `GetJobChildStats()` which returns `FailedChildren` count
- The completion probe is the ideal place to check failure thresholds since it already loads job state and checks completion conditions
- No direct queue message deletion API exists - cancellation works by marking jobs as cancelled in storage

### Approach

Add error tolerance configuration to `JobDefinition` model and implement failure threshold checking in the crawler job completion probe. When threshold is exceeded, mark parent as failed, cancel all running children, and broadcast failure event. The error tolerance config will be stored in the job definition and accessed via `JobDefinitionID` during job execution.

**Design Decisions:**
1. Store `ErrorTolerance` as a nested struct in `JobDefinition.Config` map (no schema change needed)
2. Check failure threshold in `ExecuteCompletionProbe()` after loading job state
3. Create `StopAllChildJobs()` method in `JobManager` to cancel running children
4. Reuse existing `EventJobFailed` for parent failure broadcast
5. Add integration tests to verify failure threshold behavior

### Reasoning

I explored the job execution flow by reading `crawler.go` (Execute and ExecuteCompletionProbe methods), `executor.go` (job definition execution), `crawler_actions.go` (crawl action), `types.go` (JobMessage structure), `manager.go` (job management), `job_storage.go` (child statistics), and `schema.go` (database structure). I traced how `JobDefinitionID` flows through the system and identified that the completion probe is the optimal place to check failure thresholds since it already loads job state and determines terminal status.

## Mermaid Diagram

sequenceDiagram
    participant UI as User/UI
    participant JD as JobDefinition
    participant Exec as JobExecutor
    participant Queue as QueueManager
    participant Worker as WorkerPool
    participant Crawler as CrawlerJob
    participant Probe as CompletionProbe
    participant Storage as JobStorage
    participant Manager as JobManager
    participant Events as EventService
    participant WS as WebSocket

    Note over UI,WS: Job Definition with Error Tolerance
    UI->>JD: Create JobDefinition<br/>ErrorTolerance{MaxChildFailures: 3, FailureAction: "stop_all"}
    JD->>Storage: SaveJobDefinition (with error_tolerance JSON)

    Note over UI,WS: Job Execution Flow
    UI->>Exec: Execute JobDefinition
    Exec->>Queue: Enqueue parent job (with JobDefinitionID)
    Worker->>Queue: Receive parent job message
    Worker->>Crawler: Execute(msg)
    Crawler->>Queue: Enqueue child URL jobs (inherit JobDefinitionID)

    Note over UI,WS: Child Jobs Processing
    loop For each child URL
        Worker->>Queue: Receive child job
        Worker->>Crawler: Execute(childMsg)
        alt URL fails
            Crawler->>Storage: UpdateJobStatus(child, "failed")
            Crawler->>Storage: UpdateProgressCountersAtomic(failedDelta: +1)
        else URL succeeds
            Crawler->>Storage: UpdateJobStatus(child, "completed")
            Crawler->>Storage: UpdateProgressCountersAtomic(completedDelta: +1)
        end
        Crawler->>Queue: Enqueue completion probe (delayed 5s)
    end

    Note over UI,WS: Failure Threshold Check
    Worker->>Queue: Receive completion probe
    Worker->>Probe: ExecuteCompletionProbe(msg)
    Probe->>Storage: GetJob(parentID)
    Probe->>Storage: GetJobDefinition(JobDefinitionID)
    Probe->>Storage: GetJobChildStats([parentID])
    
    alt FailedChildren >= MaxChildFailures
        Note over Probe: Threshold Exceeded!
        Probe->>Storage: UpdateJobStatus(parent, "failed", error)
        Probe->>Manager: StopAllChildJobs(parentID)
        Manager->>Storage: GetChildJobs(parentID)
        loop For each running child
            Manager->>Storage: UpdateJobStatus(child, "cancelled")
        end
        Manager-->>Probe: cancelledCount
        Probe->>Events: Publish(EventJobFailed)
        Events->>WS: Broadcast job_failed event
        WS->>UI: Update UI (parent failed, children cancelled)
    else FailedChildren < MaxChildFailures
        Note over Probe: Continue Normal Completion
        Probe->>Storage: UpdateJobStatus(parent, "completed")
        Probe->>Events: Publish(EventJobCompleted)
        Events->>WS: Broadcast job_completed event
        WS->>UI: Update UI (job completed)
    end

## Proposed File Changes

### internal\models\job_definition.go(MODIFY)

Add `ErrorTolerance` struct and field to `JobDefinition` model:

1. **Define ErrorTolerance struct** (after line 54, before JobDefinition struct):
   - `MaxChildFailures` (int) - Maximum number of child job failures before stopping parent job (0 = unlimited)
   - `FailureAction` (string) - Action to take when threshold exceeded: "stop_all" (cancel all children and fail parent), "continue" (log warning but continue), "mark_warning" (complete with warning status)

2. **Add ErrorTolerance field to JobDefinition struct** (after line 97, before CreatedAt):
   - `ErrorTolerance *ErrorTolerance` (pointer to allow nil for no tolerance config)

3. **Add validation in Validate() method** (after line 156, before return nil):
   - If `ErrorTolerance` is not nil, validate `MaxChildFailures >= 0`
   - Validate `FailureAction` is one of: "stop_all", "continue", "mark_warning"

4. **Add JSON marshal/unmarshal methods** (after line 271, after UnmarshalPreJobs):
   - `MarshalErrorTolerance() (string, error)` - Serialize ErrorTolerance to JSON string
   - `UnmarshalErrorTolerance(data string) error` - Deserialize ErrorTolerance from JSON string
   - Handle nil ErrorTolerance gracefully (return "{}" for marshal, set nil for empty unmarshal)

**Design Note:** Using a pointer allows distinguishing between "no error tolerance configured" (nil) and "error tolerance configured with defaults" (non-nil struct). This maintains backward compatibility with existing job definitions.

### internal\storage\sqlite\schema.go(MODIFY)

References: 

- internal\models\job_definition.go(MODIFY)

Add database migration to add `error_tolerance` column to `job_definitions` table:

1. **Add migration method** (after existing migration methods, around line 1745):
   - `migrateAddErrorToleranceColumn() error`
   - Check if column exists: `PRAGMA table_info(job_definitions)` and look for `error_tolerance`
   - If column doesn't exist, add it: `ALTER TABLE job_definitions ADD COLUMN error_tolerance TEXT`
   - Log migration success/skip

2. **Call migration in runMigrations()** (add after last migration call, around line 386):
   - `if err := s.migrateAddErrorToleranceColumn(); err != nil { return err }`

3. **Update schema SQL** (around line 198-213):
   - Add `error_tolerance TEXT` column after `config TEXT` line (around line 209)
   - This ensures new databases have the column from the start

**Design Note:** Using TEXT column for JSON storage is consistent with other config fields (`config`, `steps`, `sources`). The migration is idempotent and safe to run multiple times.

### internal\storage\sqlite\job_definition_storage.go(MODIFY)

References: 

- internal\models\job_definition.go(MODIFY)

Update `JobDefinitionStorage` to persist and retrieve `ErrorTolerance` field:

1. **Update SaveJobDefinition() method** (around lines 40-136):
   - After `MarshalPreJobs()` call (line 83), add:
     - `errorToleranceJSON, err := jobDef.MarshalErrorTolerance()`
     - Handle error if marshal fails
   - Update INSERT query (line 101-104) to include `error_tolerance` column
   - Update VALUES placeholder list (line 104) to include `?` for error_tolerance
   - Update ON CONFLICT DO UPDATE (line 105-118) to include `error_tolerance = excluded.error_tolerance`
   - Update ExecContext args (line 121-125) to include `errorToleranceJSON`

2. **Update UpdateJobDefinition() method** (around lines 139-239):
   - After `MarshalPreJobs()` call (line 190), add:
     - `errorToleranceJSON, err := jobDef.MarshalErrorTolerance()`
     - Handle error if marshal fails
   - Update UPDATE query (line 206-221) to include `error_tolerance = ?`
   - Update ExecContext args (line 224-227) to include `errorToleranceJSON`

3. **Update GetJobDefinition() query** (around line 244):
   - Add `COALESCE(error_tolerance, '{}') AS error_tolerance` to SELECT list
   - Update scanJobDefinition() call to handle new column

4. **Update ListJobDefinitions() query** (around line 265):
   - Add `COALESCE(error_tolerance, '{}') AS error_tolerance` to SELECT list
   - Update scanJobDefinitions() call to handle new column

5. **Update GetJobDefinitionsByType() query** (around line 330):
   - Add `COALESCE(error_tolerance, '{}') AS error_tolerance` to SELECT list

6. **Update GetEnabledJobDefinitions() query** (around line 349):
   - Add `COALESCE(error_tolerance, '{}') AS error_tolerance` to SELECT list

7. **Update scanJobDefinition() method** (around lines 618-693):
   - Add `errorToleranceJSON` variable to scan list (line 620)
   - Add `&errorToleranceJSON` to Scan() call (line 625-628)
   - After UnmarshalPostJobs() call (line 683-690), add:
     - `if err := jobDef.UnmarshalErrorTolerance(errorToleranceJSON); err != nil { ... }`
     - Log warning and set `jobDef.ErrorTolerance = nil` on error

8. **Update scanJobDefinitions() method** (around lines 696-784):
   - Add `errorToleranceJSON` variable to scan list (line 701)
   - Add `&errorToleranceJSON` to Scan() call (line 706-709)
   - After UnmarshalPostJobs() call (line 767-774), add:
     - `if err := jobDef.UnmarshalErrorTolerance(errorToleranceJSON); err != nil { ... }`
     - Log warning and set `jobDef.ErrorTolerance = nil` on error

**Design Note:** Using COALESCE ensures backward compatibility with existing rows that don't have error_tolerance set. Empty JSON object `{}` will unmarshal to nil ErrorTolerance pointer.

### internal\services\jobs\manager.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\models\crawler_job.go

Add `StopAllChildJobs()` method to cancel all running child jobs of a parent:

1. **Add method after CopyJob()** (after line 286):
   - Method signature: `func (m *Manager) StopAllChildJobs(ctx context.Context, parentID string) (int, error)`
   - Returns count of cancelled jobs and error

2. **Implementation steps:**
   - Get all child jobs: `children, err := m.jobStorage.GetChildJobs(ctx, parentID)`
   - Handle error if GetChildJobs fails
   - Iterate through children and cancel running ones:
     - Check if `child.Status == models.JobStatusRunning || child.Status == models.JobStatusPending`
     - Update status: `err := m.jobStorage.UpdateJobStatus(ctx, child.ID, string(models.JobStatusCancelled), "Parent job failed due to error threshold")`
     - Log each cancellation (success and failure)
     - Track count of successfully cancelled jobs
   - Return total cancelled count and aggregated errors (if any)

3. **Add logging:**
   - Log at start: "Stopping all child jobs for parent"
   - Log each child cancellation: "Child job cancelled"
   - Log at end: "Stopped X child jobs for parent"

**Design Note:** This method only updates job status in storage. It does not delete queue messages because:
1. Workers check job status before processing and will skip cancelled jobs
2. Queue messages have visibility timeout and will eventually be deleted
3. Deleting messages requires message ID which we don't have from job records

The method is idempotent - calling it multiple times is safe.

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\models\job_definition.go(MODIFY)
- internal\services\jobs\manager.go(MODIFY)
- internal\interfaces\storage.go
- internal\interfaces\event_service.go

Add failure threshold checking in `ExecuteCompletionProbe()` method:

1. **Add failure threshold check** (after loading job state, around line 1043, before completion condition checks):
   - Check if `msg.JobDefinitionID` is not empty (job was started from job definition)
   - If yes, load job definition: `jobDef, err := c.deps.JobDefinitionStorage.GetJobDefinition(ctx, msg.JobDefinitionID)`
   - Handle error if job definition not found (log warning and continue without threshold check)
   - Check if `jobDef.ErrorTolerance != nil && jobDef.ErrorTolerance.MaxChildFailures > 0`
   - If yes, get child failure count:
     - `childStats, err := c.deps.JobStorage.GetJobChildStats(ctx, []string{msg.ParentID})`
     - Extract `failedCount := childStats[msg.ParentID].FailedChildren`
   - Compare `failedCount >= jobDef.ErrorTolerance.MaxChildFailures`
   - If threshold exceeded, handle based on `FailureAction`:

2. **Handle "stop_all" action:**
   - Log error: "Job failed: child failure threshold exceeded (X/Y failures)"
   - Set job error: `job.Error = fmt.Sprintf("Child failure threshold exceeded: %d/%d failures", failedCount, jobDef.ErrorTolerance.MaxChildFailures)`
   - Mark job as failed: `job.Status = models.JobStatusFailed`
   - Set completion time: `job.CompletedAt = time.Now()`
   - Sync result counts: `job.ResultCount = job.Progress.CompletedURLs`, `job.FailedCount = job.Progress.FailedURLs`
   - Save job: `c.deps.JobStorage.SaveJob(ctx, job)`
   - Cancel all child jobs: `cancelledCount, err := c.deps.JobManager.StopAllChildJobs(ctx, msg.ParentID)`
   - Log cancellation result
   - Publish `EventJobFailed` event with payload:
     - `job_id`, `status: "failed"`, `source_type`, `entity_type`, `error`, `result_count`, `failed_count`, `child_failure_count: failedCount`, `threshold: jobDef.ErrorTolerance.MaxChildFailures`, `timestamp`
   - Return nil (job processing complete)

3. **Handle "continue" action:**
   - Log warning: "Child failure threshold exceeded but continuing (X/Y failures)"
   - Continue with normal completion logic (don't fail the job)

4. **Handle "mark_warning" action:**
   - Log warning: "Child failure threshold exceeded, marking as completed with warning"
   - Add warning to job metadata: `job.Metadata["warning"] = "Child failure threshold exceeded"`
   - Continue with normal completion logic

5. **Add JobDefinitionStorage dependency:**
   - Update `CrawlerJobDeps` struct (around line 19) to include `JobDefinitionStorage interfaces.JobDefinitionStorage`
   - Update `NewCrawlerJob()` constructor to accept and store the dependency
   - Update all call sites in `internal/app/app.go` to pass `a.StorageManager.JobDefinitionStorage()`

**Design Note:** Checking threshold in completion probe ensures we only evaluate failure after job activity has settled. This avoids premature cancellation during active crawling. The probe already has all necessary context (job state, child stats) and handles terminal status transitions.

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\types\crawler.go(MODIFY)

Update `CrawlerJobDeps` initialization to include `JobDefinitionStorage`:

1. **Update CrawlerJobDeps struct initialization** (around line 438-445):
   - Add `JobDefinitionStorage: a.StorageManager.JobDefinitionStorage()` to the struct literal
   - Ensure it's added after `EventService` field

**Design Note:** This is a simple dependency injection update to provide the crawler job with access to job definition storage for reading error tolerance configuration.

### internal\handlers\websocket_events.go(MODIFY)

**No changes required.** The existing `handleJobFailed()` method (lines 265-292) already handles `EventJobFailed` events and broadcasts them via WebSocket. The method extracts all necessary fields from the event payload including `error`, `result_count`, `failed_count`, and broadcasts a `JobStatusUpdate` to all connected clients.

When the crawler job publishes `EventJobFailed` with the new `child_failure_count` and `threshold` fields in the payload, they will be included in the WebSocket broadcast automatically (the payload is passed through as-is).

**Verification:** Confirm that the UI can handle additional fields in the `JobStatusUpdate` payload without breaking. The UI should gracefully ignore unknown fields.

### test\api\job_error_tolerance_test.go(NEW)

References: 

- test\helpers.go
- internal\models\job_definition.go(MODIFY)
- internal\services\jobs\manager.go(MODIFY)

Create integration tests for error tolerance functionality:

1. **Test setup:**
   - Use existing test helpers from `test/helpers.go` for database setup
   - Create test job definition with error tolerance configuration
   - Use mock crawler service or test URLs that can be controlled to fail

2. **Test cases:**

   **TestErrorTolerance_StopAll:**
   - Create job definition with `ErrorTolerance{MaxChildFailures: 3, FailureAction: "stop_all"}`
   - Start crawl job with 10 seed URLs
   - Simulate 3 child job failures (use invalid URLs or mock HTTP errors)
   - Wait for completion probe to run
   - Assert parent job status is "failed"
   - Assert parent job error contains "Child failure threshold exceeded: 3/3"
   - Assert remaining child jobs are cancelled
   - Assert `EventJobFailed` was published with correct payload

   **TestErrorTolerance_Continue:**
   - Create job definition with `ErrorTolerance{MaxChildFailures: 2, FailureAction: "continue"}`
   - Start crawl job with 10 seed URLs
   - Simulate 5 child job failures
   - Wait for completion
   - Assert parent job status is "completed" (not failed)
   - Assert all child jobs were processed (not cancelled)

   **TestErrorTolerance_MarkWarning:**
   - Create job definition with `ErrorTolerance{MaxChildFailures: 2, FailureAction: "mark_warning"}`
   - Start crawl job with 10 seed URLs
   - Simulate 3 child job failures
   - Wait for completion
   - Assert parent job status is "completed"
   - Assert parent job metadata contains warning

   **TestErrorTolerance_NoConfig:**
   - Create job definition without error tolerance (nil)
   - Start crawl job with 10 seed URLs
   - Simulate 5 child job failures
   - Wait for completion
   - Assert parent job completes normally (no threshold check)

   **TestErrorTolerance_ZeroThreshold:**
   - Create job definition with `ErrorTolerance{MaxChildFailures: 0, FailureAction: "stop_all"}`
   - Start crawl job with 10 seed URLs
   - Simulate 5 child job failures
   - Wait for completion
   - Assert parent job completes normally (0 = unlimited failures)

3. **Test utilities:**
   - Helper function to create job definition with error tolerance
   - Helper function to simulate child job failures
   - Helper function to wait for job completion with timeout
   - Helper function to verify event publication

**Design Note:** These tests verify the end-to-end behavior of error tolerance, including database persistence, threshold checking, job cancellation, and event broadcasting. Use table-driven tests where appropriate to reduce duplication.