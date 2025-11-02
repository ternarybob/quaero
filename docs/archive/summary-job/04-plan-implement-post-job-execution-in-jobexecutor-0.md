I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires implementing post-job execution functionality in the JobExecutor. After analyzing the codebase, I've identified the following key observations:

**Current Architecture:**
- JobExecutor orchestrates multi-step job workflows defined by JobDefinitions
- Jobs can complete synchronously (immediate) or asynchronously (with polling)
- The executor already has a sophisticated async polling mechanism for crawl jobs
- Job execution is triggered via queue messages processed by the "parent" handler in `app.go`

**Post-Job Requirements:**
- Trigger after successful job completion (both sync and async paths)
- Query JobDefinitionStorage for each post-job ID
- Execute independently (no parent/child relationship)
- Log execution events
- Continue on errors (graceful degradation)

**Critical Insight:**
Post-jobs must be triggered in TWO locations:
1. **Line 318** - After synchronous completion (when `!asyncPollingLaunched`)
2. **Line 244** - Inside async polling goroutine (after successful polling)

**Dependency Challenge:**
The executor needs to enqueue post-jobs to the queue, but adding QueueManager directly would create tight coupling. The solution is to use a callback/interface pattern similar to the existing `StatusUpdateCallback`.

**Data Model:**
The `JobDefinition.PostJobs` field (array of job IDs) is already implemented and persisted in the database. The storage layer provides `GetJobDefinition(ctx, id)` for fetching job definitions.

### Approach

Add post-job execution capability to JobExecutor using a callback pattern to avoid circular dependencies. Create a `PostJobTriggerCallback` that the parent handler provides, allowing the executor to request post-job execution without knowing about the queue system. Implement a helper method `executePostJobs` that fetches job definitions and invokes the callback for each post-job. Call this method in both sync and async completion paths.

### Reasoning

I explored the repository structure, read the executor implementation, analyzed the job execution flow from HTTP handler through queue to executor, examined the async polling mechanism, reviewed the JobDefinition model with PostJobs field, and studied the dependency injection pattern in app.go. I identified that post-jobs need to be triggered in two distinct code paths (sync and async) and that a callback pattern is the cleanest solution to avoid circular dependencies.

## Mermaid Diagram

sequenceDiagram
    participant Handler as Parent Job Handler
    participant Executor as JobExecutor
    participant Storage as JobDefinitionStorage
    participant Queue as QueueManager
    participant Worker as Worker Pool

    Note over Handler,Worker: Job Execution with Post-Jobs

    Handler->>Executor: Execute(ctx, jobDef, statusCallback, postJobCallback)
    Executor->>Executor: Execute all job steps
    
    alt Synchronous Completion
        Executor->>Executor: All steps complete (no async polling)
        Executor->>Executor: executePostJobs(ctx, jobDef, postJobCallback)
        loop For each post-job ID
            Executor->>Storage: GetJobDefinition(postJobID)
            Storage-->>Executor: postJobDef
            Executor->>Executor: Validate (enabled, valid)
            Executor->>Handler: postJobCallback(ctx, postJobID)
            Handler->>Queue: NewJobDefinitionMessage(postJobDef)
            Handler->>Queue: Enqueue(message)
            Queue-->>Handler: Success
            Handler-->>Executor: nil
        end
        Executor-->>Handler: ExecutionResult{AsyncPollingActive: false}
    else Async Polling (Crawl Jobs)
        Executor->>Executor: Launch polling goroutine
        Executor-->>Handler: ExecutionResult{AsyncPollingActive: true}
        
        Note over Executor: Polling goroutine continues...
        
        Executor->>Executor: Poll crawl jobs until complete
        Executor->>Executor: executePostJobs(pollingCtx, jobDef, postJobCallback)
        loop For each post-job ID
            Executor->>Storage: GetJobDefinition(postJobID)
            Storage-->>Executor: postJobDef
            Executor->>Executor: Validate (enabled, valid)
            Executor->>Handler: postJobCallback(pollingCtx, postJobID)
            Handler->>Queue: NewJobDefinitionMessage(postJobDef)
            Handler->>Queue: Enqueue(message)
            Queue-->>Handler: Success
            Handler-->>Executor: nil
        end
        Executor->>Handler: statusCallback(ctx, "completed", "")
    end

    Note over Queue,Worker: Post-jobs execute independently
    Worker->>Queue: Receive post-job message
    Worker->>Executor: Execute(ctx, postJobDef, ...)
    Note over Worker: No parent/child relationship

## Proposed File Changes

### internal\services\jobs\executor.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\models\job_definition.go

**Add PostJobTriggerCallback type definition (after line 48):**

Define a new callback type `PostJobTriggerCallback` that accepts `ctx context.Context` and `postJobID string` parameters and returns `error`. This callback will be invoked by the executor to request post-job execution. The callback signature should match: `func(ctx context.Context, postJobID string) error`.

Add documentation explaining that this callback is invoked for each post-job after successful job completion, and that the callback is responsible for enqueueing the post-job to the queue.

**Add JobDefinitionStorage field to JobExecutor struct (after line 56):**

Add a new field `jobDefStorage interfaces.JobDefinitionStorage` to the `JobExecutor` struct. This will be used to fetch job definitions for post-jobs.

**Update NewJobExecutor constructor signature and validation (lines 65-99):**

Add `jobDefStorage interfaces.JobDefinitionStorage` as a new parameter to the `NewJobExecutor` function (after `crawlerService`, before `logger`).

Add validation for the new parameter:
- After line 77 (crawlerService validation), add nil check for `jobDefStorage` with error message "jobDefStorage cannot be nil"

Assign the parameter to the struct field:
- After line 90 (crawlerService assignment), add `jobDefStorage: jobDefStorage,`

Update the success log message to reflect the new dependency.

**Update Execute method signature (line 110):**

Add a new parameter `postJobCallback PostJobTriggerCallback` to the `Execute` method signature (after `statusCallback`, before the return type). This callback will be invoked to trigger post-jobs.

Update the method documentation (lines 107-109) to explain the new parameter: "If postJobCallback is provided, it will be invoked for each post-job after successful completion. If nil, post-jobs are skipped."

**Create executePostJobs helper method (after line 326, before shouldStopOnError):**

Implement a new private method `executePostJobs` with signature:
- `func (e *JobExecutor) executePostJobs(ctx context.Context, definition *models.JobDefinition, postJobCallback PostJobTriggerCallback) error`

Method logic:
1. Check if `postJobCallback` is nil - if so, return nil immediately (no-op)
2. Check if `definition.PostJobs` is empty - if so, return nil (nothing to do)
3. Log the start of post-job execution with job_id, post_job_count
4. Iterate through `definition.PostJobs` slice:
   - For each `postJobID`:
     a. Log: "Fetching post-job definition" with post_job_id
     b. Call `e.jobDefStorage.GetJobDefinition(ctx, postJobID)`
     c. If error (not found or other):
        - Log warning with error, post_job_id, parent_job_id
        - Continue to next post-job (don't fail entire process)
     d. Validate the fetched job definition:
        - Check if `postJobDef.Enabled` is false - if so, log warning and skip
        - Call `postJobDef.Validate()` - if error, log warning and skip
     e. Log: "Triggering post-job execution" with post_job_id, post_job_name
     f. Invoke `postJobCallback(ctx, postJobID)`
     g. If callback returns error:
        - Log error with post_job_id, parent_job_id
        - Continue to next post-job (graceful degradation)
     h. If successful, log info: "Post-job triggered successfully"
5. Log completion: "Post-job execution completed" with parent_job_id, triggered_count, skipped_count
6. Return nil (always succeeds, errors are logged but don't fail the parent job)

Use structured logging with fields: parent_job_id (definition.ID), post_job_id, post_job_name, post_job_count, triggered_count, skipped_count.

**Trigger post-jobs after synchronous completion (after line 318):**

After the existing completion event publication (line 318), add a call to `executePostJobs`:
- Call `e.executePostJobs(ctx, definition, postJobCallback)`
- Ignore the return value (method always returns nil, errors are logged internally)
- Add a comment: "Trigger post-jobs after successful synchronous completion"

This ensures post-jobs are triggered when the job completes without async polling.

**Trigger post-jobs after async polling completion (inside polling goroutine, after line 244):**

Inside the async polling success path (after line 244, before the status callback invocation on line 251), add a call to `executePostJobs`:
- Call `e.executePostJobs(pollingCtx, definition, postJobCallback)`
- Ignore the return value
- Add a comment: "Trigger post-jobs after successful async polling completion"

This ensures post-jobs are triggered when async crawl jobs complete successfully.

**Important:** Do NOT trigger post-jobs in the failure paths (lines 222-239) - post-jobs should only run after successful completion.

**Update all test files that call NewJobExecutor:**

Search for all test files that instantiate `NewJobExecutor` and update them to pass a mock `JobDefinitionStorage` as the new parameter. The mock can be a simple in-memory implementation or nil (with validation disabled for tests).

Update test files in `internal/services/jobs/executor_test.go` - all calls to `NewJobExecutor` need the new parameter.

### internal\app\app.go(MODIFY)

References: 

- internal\services\jobs\executor.go(MODIFY)
- internal\queue\types.go
- internal\handlers\job_definition_handler.go

**Update JobExecutor initialization (lines 597-603):**

Modify the `NewJobExecutor` call to include the new `JobDefinitionStorage` parameter:
- Change line 599 from:
  `a.JobExecutor, err = jobs.NewJobExecutor(a.JobRegistry, a.SourceService, a.EventService, a.CrawlerService, a.Logger)`
- To:
  `a.JobExecutor, err = jobs.NewJobExecutor(a.JobRegistry, a.SourceService, a.EventService, a.CrawlerService, a.StorageManager.JobDefinitionStorage(), a.Logger)`

This wires the JobDefinitionStorage dependency so the executor can fetch post-job definitions.

**Create PostJobTriggerCallback in parent handler (lines 520-566):**

Before the `statusCallback` definition (line 520), create a new `postJobCallback` function:

Implement `postJobCallback` with signature matching `PostJobTriggerCallback`:
- Parameters: `callbackCtx context.Context`, `postJobID string`
- Returns: `error`

Callback logic:
1. Log: "Post-job trigger requested" with parent_job_id (targetID), post_job_id
2. Fetch the post-job definition:
   - Call `a.StorageManager.JobDefinitionStorage().GetJobDefinition(callbackCtx, postJobID)`
   - If error, log error and return it
3. Create a new parent job message for the post-job:
   - Use `queue.NewJobDefinitionMessage(postJobDef.ID, config)` where config contains job_definition_id, job_name, job_type, sources, steps, timeout
   - Follow the same pattern as `ExecuteJobDefinitionHandler` (lines 384-394 in `job_definition_handler.go`)
4. Create a job record in database:
   - Create `models.CrawlJob` with message ID as job ID
   - Set Status to `models.JobStatusPending`
   - Call `a.StorageManager.JobStorage().SaveJob(callbackCtx, job)`
   - If error, log error and return it
5. Enqueue the message:
   - Call `a.QueueManager.Enqueue(callbackCtx, parentMsg)`
   - If error, log error and return it
6. Log success: "Post-job enqueued successfully" with post_job_id, message_id, parent_job_id
7. Return nil

This callback allows the executor to trigger post-jobs without knowing about the queue system.

**Update Execute call to pass postJobCallback (line 541):**

Modify the `a.JobExecutor.Execute` call to include the new `postJobCallback` parameter:
- Change line 541 from:
  `result, err := a.JobExecutor.Execute(ctx, jobDef, statusCallback)`
- To:
  `result, err := a.JobExecutor.Execute(ctx, jobDef, statusCallback, postJobCallback)`

This wires the post-job trigger callback so the executor can request post-job execution.

**Add import for queue package (if not already present):**

Ensure the import section includes:
- `"github.com/ternarybob/quaero/internal/queue"` (for `queue.NewJobDefinitionMessage`)

This is needed for creating post-job messages in the callback.