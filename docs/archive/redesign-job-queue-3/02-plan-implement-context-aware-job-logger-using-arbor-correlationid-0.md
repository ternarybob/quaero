I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

The codebase has a **LogService** (`internal/logs/service.go`) that consumes log batches from Arbor's context channel and extracts `jobID` from `event.CorrelationID` (line 94). However, **BaseJob** currently uses raw `arbor.ILogger` directly (line 25 in `base.go`), which means logs are not automatically associated with job IDs.

**Key Findings:**

1. **LogService is Ready**: The service already extracts jobID from CorrelationID and dispatches to database and WebSocket
2. **Missing Context**: BaseJob and all job types (CrawlerJob, SummarizerJob, etc.) use raw logger without correlation context
3. **Manual Logging**: BaseJob has a `LogJobEvent` method that manually creates JobLogEntry and writes to storage - this bypasses the Arbor context channel
4. **Arbor API**: Web search confirms Arbor supports `WithCorrelationId(string)` method for setting correlation context
5. **Integration Points**: BaseJob is instantiated in 6 places in `app.go` (lines 408, 417, 430, 443, 455, 469, 483) with raw logger

**Architecture Gap**: There's no wrapper that bridges Arbor's correlation mechanism with job execution context. All job logs currently lack jobID association unless manually written via `LogJobEvent`.

## Solution Architecture

Create a **JobLogger** wrapper that:
- Wraps `arbor.ILogger` and adds jobID as CorrelationID
- Provides structured logging helpers for job lifecycle events
- Ensures all logs flow through Arbor's context channel (already configured)
- Supports parent-child log association via inherited jobID

**Design Decision**: Use composition over inheritance - JobLogger embeds `arbor.ILogger` and delegates all standard logging methods while adding job-specific helpers.

## Parent-Child Log Association Strategy

**Approach**: Child jobs inherit parent's jobID as their CorrelationID, creating a flat log hierarchy:
- Parent job: `jobID = "parent-123"`, logs with `CorrelationID = "parent-123"`
- Child job: `jobID = "child-456"`, logs with `CorrelationID = "parent-123"` (inherited)
- All logs for parent and children share the same CorrelationID for aggregation

**Alternative Considered**: Use child's own jobID as CorrelationID - **Rejected** because it breaks log aggregation (parent and child logs would be separate).

## Structured Logging Helpers

The JobLogger will provide lifecycle helpers that emit consistent, structured log messages:
- `LogJobStart(name, sourceType, config)` - Job initialization
- `LogJobProgress(completed, total, message)` - Progress updates
- `LogJobComplete(duration, resultCount)` - Successful completion
- `LogJobError(err, context)` - Error with context

These helpers ensure consistent formatting across all job types and reduce boilerplate.

### Approach

Create `JobLogger` wrapper in `internal/jobs/types/logger.go` that wraps `arbor.ILogger` with correlation context. Update `BaseJob` to create and use `JobLogger` instead of raw logger. Modify all job handler registrations in `app.go` to pass jobID for correlation. Remove manual `LogJobEvent` method from BaseJob since logs will flow through Arbor's context channel automatically.

### Reasoning

I explored the codebase structure, examined the LogService implementation in `internal/logs/service.go`, reviewed BaseJob and CrawlerJob in `internal/jobs/types/`, analyzed the app initialization sequence in `internal/app/app.go`, searched for Arbor's WithCorrelation API documentation, and identified the integration points where BaseJob is instantiated.

## Mermaid Diagram

sequenceDiagram
    participant App as app.go
    participant Handler as Job Handler
    participant BaseJob as BaseJob
    participant JobLogger as JobLogger
    participant Arbor as Arbor Logger
    participant Channel as Context Channel
    participant LogService as LogService
    participant DB as Database
    participant WS as WebSocket

    Note over App,Handler: Job Handler Registration
    App->>Handler: Register handler with baseLogger
    
    Note over Handler,JobLogger: Job Execution Start
    Handler->>Handler: Extract jobID and parentID from msg
    Handler->>BaseJob: NewBaseJob(msgID, defID, jobID, parentID, baseLogger, ...)
    BaseJob->>JobLogger: NewJobLogger(baseLogger, jobID, parentID)
    alt Child Job (parentID not empty)
        JobLogger->>Arbor: baseLogger.WithCorrelationId(parentID)
        Note over JobLogger: Use parent's jobID for log aggregation
    else Parent Job (parentID empty)
        JobLogger->>Arbor: baseLogger.WithCorrelationId(jobID)
        Note over JobLogger: Use own jobID
    end
    JobLogger-->>BaseJob: Return correlated logger
    BaseJob-->>Handler: Return BaseJob with JobLogger
    
    Note over Handler,LogService: Job Execution with Logging
    Handler->>BaseJob: Execute job logic
    BaseJob->>JobLogger: LogJobStart(name, sourceType, config)
    JobLogger->>Arbor: Info().Str("job_id", jobID).Msg("Job started")
    Arbor->>Channel: Send LogEvent with CorrelationID
    
    BaseJob->>JobLogger: LogJobProgress(completed, total, msg)
    JobLogger->>Arbor: Info().Int("completed", n).Msg(msg)
    Arbor->>Channel: Send LogEvent with CorrelationID
    
    alt Job Success
        BaseJob->>JobLogger: LogJobComplete(duration, resultCount)
        JobLogger->>Arbor: Info().Float64("duration_sec", d).Msg("Job completed")
    else Job Failure
        BaseJob->>JobLogger: LogJobError(err, context)
        JobLogger->>Arbor: Error().Str("error", err).Msg("Job failed")
    end
    Arbor->>Channel: Send LogEvent with CorrelationID
    
    Note over Channel,WS: Log Dispatch (Async)
    Channel->>LogService: Batch of LogEvents
    LogService->>LogService: Extract jobID from CorrelationID
    LogService->>LogService: Transform to JobLogEntry
    par Dispatch to Database
        LogService->>DB: AppendLog(jobID, entry)
    and Dispatch to WebSocket
        LogService->>WS: BroadcastLog(entry)
    end
    
    Note over DB,WS: Result
    DB-->>LogService: Log persisted
    WS-->>LogService: Log broadcasted to UI

## Proposed File Changes

### internal\jobs\types\logger.go(NEW)

References: 

- internal\jobs\types\base.go(MODIFY)
- internal\logs\service.go
- internal\models\job_log.go

Create the JobLogger wrapper with the following structure:

**Package and Imports:**
- Package `types`
- Import: `fmt`, `time`
- Import: `github.com/ternarybob/arbor`
- Import: `internal/models`

**JobLogger Struct:**
- Define `JobLogger` struct with fields:
  - `logger arbor.ILogger` - the underlying Arbor logger with correlation context
  - `jobID string` - the job ID for this logger instance
  - `parentID string` - optional parent job ID for child jobs

**Constructor:**
- Implement `NewJobLogger(baseLogger arbor.ILogger, jobID string, parentID string) *JobLogger`
- Use `baseLogger.WithCorrelationId(jobID)` to create a correlated logger (note: method name is `WithCorrelationId` not `WithCorrelation` based on web search)
- If parentID is not empty, use parentID as the CorrelationID instead of jobID (for parent-child log aggregation)
- Store the correlated logger in the struct
- Return `&JobLogger` instance

**Delegation Methods:**
- Implement delegation methods that forward to the underlying logger:
  - `Info() arbor.IEvent` - returns `logger.Info()`
  - `Warn() arbor.IEvent` - returns `logger.Warn()`
  - `Error() arbor.IEvent` - returns `logger.Error()`
  - `Debug() arbor.IEvent` - returns `logger.Debug()`
- These allow callers to use standard Arbor logging: `jobLogger.Info().Msg("message")`

**Structured Logging Helpers:**

**LogJobStart Method:**
- Signature: `LogJobStart(name string, sourceType string, config interface{})`
- Log at Info level with structured fields:
  - `job_id` - the job ID
  - `name` - job name
  - `source_type` - source type (jira, confluence, etc.)
  - `config` - formatted config string (use `fmt.Sprintf("%+v", config)`)
- Message: "Job started"
- This creates a consistent start event for all jobs

**LogJobProgress Method:**
- Signature: `LogJobProgress(completed int, total int, message string)`
- Log at Info level with structured fields:
  - `job_id` - the job ID
  - `completed` - number of completed items
  - `total` - total items
  - `progress_pct` - calculated percentage (completed/total * 100)
- Message: provided message parameter
- Use for periodic progress updates during job execution

**LogJobComplete Method:**
- Signature: `LogJobComplete(duration time.Duration, resultCount int)`
- Log at Info level with structured fields:
  - `job_id` - the job ID
  - `duration_sec` - duration in seconds (use `duration.Seconds()`)
  - `result_count` - number of results/documents processed
- Message: "Job completed successfully"
- Marks successful job completion

**LogJobError Method:**
- Signature: `LogJobError(err error, context string)`
- Log at Error level with structured fields:
  - `job_id` - the job ID
  - `error` - error message (use `err.Error()`)
  - `context` - additional context string
- Message: "Job failed"
- Use for job failures with detailed error information

**LogJobCancelled Method:**
- Signature: `LogJobCancelled(reason string)`
- Log at Warn level with structured fields:
  - `job_id` - the job ID
  - `reason` - cancellation reason
- Message: "Job cancelled"
- Use when jobs are explicitly cancelled

**GetJobID Method:**
- Signature: `GetJobID() string`
- Return the jobID field
- Allows callers to retrieve the job ID if needed

**GetParentID Method:**
- Signature: `GetParentID() string`
- Return the parentID field
- Allows callers to check if this is a child job

**Design Notes:**
- All logs automatically flow through Arbor's context channel (configured in app.go)
- LogService extracts jobID from CorrelationID and dispatches to database and WebSocket
- Parent-child association: child jobs use parent's jobID as CorrelationID for log aggregation
- Structured helpers ensure consistent log formatting across all job types

### internal\jobs\types\base.go(MODIFY)

References: 

- internal\jobs\types\logger.go(NEW)
- internal\jobs\types\crawler.go
- internal\app\app.go(MODIFY)

Update BaseJob to use JobLogger instead of raw arbor.ILogger:

**Update BaseJob Struct (line 22):**
- Change `logger arbor.ILogger` to `logger *JobLogger`
- Keep all other fields unchanged (messageID, jobDefinitionID, jobManager, queueManager, jobLogStorage)

**Update NewBaseJob Constructor (line 32):**
- Change signature to accept `jobID string` and `parentID string` parameters
- Signature: `NewBaseJob(messageID, jobDefinitionID, jobID, parentID string, baseLogger arbor.ILogger, jobManager interfaces.JobManager, queueManager interfaces.QueueManager, jobLogStorage interfaces.JobLogStorage) *BaseJob`
- Create JobLogger: `logger := NewJobLogger(baseLogger, jobID, parentID)`
- Store the JobLogger in the struct
- Return `&BaseJob` with the JobLogger

**Update EnqueueChildJob Method (line 44):**
- No changes needed - method already uses `b.logger.Debug()` which will work with JobLogger delegation
- The debug log will automatically include the job's CorrelationID

**Remove LogJobEvent Method (lines 59-76):**
- Delete the entire `LogJobEvent` method
- This method is now obsolete because:
  - JobLogger's structured helpers provide better logging
  - All logs flow through Arbor's context channel automatically
  - LogService handles database persistence and WebSocket broadcasting
- Callers should use JobLogger's structured helpers instead: `LogJobStart`, `LogJobProgress`, `LogJobComplete`, `LogJobError`

**Update CreateChildJobRecord Method (line 80):**
- No changes to logic needed
- The existing `b.logger.Debug()` calls will work with JobLogger delegation
- Logs will automatically include the parent job's CorrelationID

**Add GetLogger Method:**
- Add new method: `GetLogger() *JobLogger`
- Return `b.logger`
- Allows job implementations to access the logger for custom logging
- Signature: `func (b *BaseJob) GetLogger() *JobLogger { return b.logger }`

**Migration Notes:**
- All job types (CrawlerJob, SummarizerJob, etc.) that call `b.logger` will continue to work because JobLogger delegates standard methods
- Jobs using `LogJobEvent` must be updated to use JobLogger's structured helpers (handled in subsequent phase)
- The jobID and parentID parameters enable proper correlation context for all logs

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\types\base.go(MODIFY)
- internal\jobs\types\logger.go(NEW)
- internal\queue\types.go(MODIFY)

Update all job handler registrations to pass jobID and parentID for JobLogger correlation:

**Crawler Job Handler (lines 407-413):**
- Extract jobID from `msg.JobID` (the parent job ID from the message)
- Extract parentID from `msg.ParentID` (empty for parent jobs, set for child jobs)
- Update BaseJob creation: `baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, msg.JobID, msg.ParentID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())`
- Note: For child jobs, use `msg.ParentID` as the CorrelationID to aggregate logs with parent
- The JobLogger will be created inside NewBaseJob with proper correlation context

**Crawler Completion Probe Handler (lines 416-422):**
- Same changes as crawler job handler
- Extract jobID and parentID from message
- Update BaseJob creation with jobID and parentID parameters
- Completion probe logs will be associated with the parent job

**Summarizer Job Handler (lines 429-435):**
- Extract jobID from `msg.JobID`
- Extract parentID from `msg.ParentID`
- Update BaseJob creation: `baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, msg.JobID, msg.ParentID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())`
- Summarizer logs will be correlated with the job that triggered summarization

**Cleanup Job Handler (lines 442-448):**
- Extract jobID from `msg.JobID`
- Extract parentID from `msg.ParentID` (typically empty for cleanup jobs)
- Update BaseJob creation with jobID and parentID parameters
- Cleanup job logs will be properly correlated

**Reindex Job Handler (lines 454-460):**
- Extract jobID from `msg.JobID`
- Extract parentID from `msg.ParentID`
- Update BaseJob creation with jobID and parentID parameters
- Reindex job logs will be properly correlated

**Pre-validation Job Handler (lines 468-474):**
- Extract jobID from `msg.JobID`
- Extract parentID from `msg.ParentID`
- Update BaseJob creation with jobID and parentID parameters
- Pre-validation logs will be correlated with the job definition execution

**Post-summarization Job Handler (lines 482-488):**
- Extract jobID from `msg.JobID`
- Extract parentID from `msg.ParentID`
- Update BaseJob creation with jobID and parentID parameters
- Post-summarization logs will be correlated with the parent crawl job

**Correlation Strategy:**
- **Parent jobs**: Use their own jobID as CorrelationID (parentID is empty)
- **Child jobs**: Use parent's jobID as CorrelationID (parentID is set) for log aggregation
- This creates a flat log hierarchy where all logs for a job family share the same CorrelationID
- LogService extracts this CorrelationID and stores logs in the database with the correct jobID

**Verification:**
- Ensure `queue.JobMessage` struct has `JobID` and `ParentID` fields (should already exist based on queue architecture)
- If fields are named differently, adjust the extraction logic accordingly
- The message ID (`msg.ID`) is different from jobID - it's the queue message identifier

**No Other Changes Needed:**
- LogService initialization (lines 239-251) remains unchanged - it already handles CorrelationID extraction
- Arbor context channel configuration (line 249) remains unchanged
- All other service initializations remain unchanged

### internal\queue\types.go(MODIFY)

References: 

- internal\app\app.go(MODIFY)
- internal\jobs\types\logger.go(NEW)

Verify and document the JobMessage structure for correlation context:

**Review JobMessage Struct:**
- Verify that `JobMessage` has the following fields:
  - `ID string` - the queue message ID (unique per message)
  - `JobID string` - the job ID from crawl_jobs table (for correlation)
  - `ParentID string` - the parent job ID (empty for parent jobs, set for child jobs)
  - `JobDefinitionID string` - optional job definition ID for traceability
- If these fields don't exist with these exact names, document the actual field names

**Add Documentation Comment:**
- Add a comment above the JobMessage struct explaining the correlation strategy:
  ```
  // JobMessage represents a job message in the queue.
  // 
  // Correlation Strategy:
  // - ID: Unique queue message identifier (not used for log correlation)
  // - JobID: The job ID from crawl_jobs table (used as CorrelationID for parent jobs)
  // - ParentID: The parent job ID (used as CorrelationID for child jobs to aggregate logs)
  // - JobDefinitionID: Optional job definition ID for traceability
  //
  // Log Aggregation:
  // - Parent jobs: logs use JobID as CorrelationID
  // - Child jobs: logs use ParentID as CorrelationID (inherits parent's context)
  // - This creates a flat log hierarchy where all logs for a job family share the same CorrelationID
  ```

**Field Naming Verification:**
- If the actual field names differ from the expected names, update the documentation in `app.go` accordingly
- Common variations: `JobId` vs `JobID`, `ParentId` vs `ParentID`
- Ensure consistency with the extraction logic in `app.go` job handlers

**No Code Changes:**
- This is a documentation-only change to clarify the correlation strategy
- The struct should already have the necessary fields based on the queue architecture
- If fields are missing, they need to be added (but this is unlikely given the existing job hierarchy support)