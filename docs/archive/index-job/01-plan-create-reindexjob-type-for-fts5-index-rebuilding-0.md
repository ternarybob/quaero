I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires creating a new `ReindexJob` type to rebuild the FTS5 full-text search index. This follows an established pattern in the codebase where job types extend `BaseJob` and implement the `Job` interface. The `RebuildFTS5Index()` method already exists in `DocumentStorage` interface (line 66 of `internal/interfaces/storage.go`) and is implemented in `internal/storage/sqlite/document_storage.go` (lines 524-544).

The `CleanupJob` in `internal/jobs/types/cleanup.go` provides an excellent template, as it has similar characteristics: simple operation, config-based parameters (including `dry_run`), and straightforward execution flow. The job will need minimal dependencies (only `DocumentStorage`) and will follow the same validation and logging patterns.


### Approach

Create a new `ReindexJob` type in `internal/jobs/types/reindex.go` that extends `BaseJob` and implements the `Job` interface. The job will call `DocumentStorage.RebuildFTS5Index()` to rebuild the FTS5 full-text search index, with support for a `dry_run` configuration option for testing. Follow the exact pattern established by `CleanupJob` for structure, validation, logging, and error handling.


### Reasoning

I explored the codebase structure and read the relevant files mentioned by the user: `cleanup.go`, `base.go`, `storage.go`, and `document_storage.go`. I also examined `summarizer.go` for additional patterns, `types.go` for the `JobMessage` structure, and `job_log.go` for logging conventions. This provided a complete understanding of the job type pattern and the available `RebuildFTS5Index()` method.


## Mermaid Diagram

sequenceDiagram
    participant WP as WorkerPool
    participant RJ as ReindexJob
    participant BJ as BaseJob
    participant DS as DocumentStorage
    participant DB as SQLite Database
    participant JLS as JobLogStorage

    WP->>RJ: Execute(ctx, JobMessage)
    RJ->>RJ: Validate(msg)
    Note over RJ: Extract dry_run config<br/>(default: false)
    
    RJ->>BJ: LogJobEvent("Starting reindex")
    BJ->>JLS: AppendLog(jobID, logEntry)
    
    alt dry_run = true
        RJ->>RJ: Log "Dry run mode - skipping rebuild"
    else dry_run = false
        RJ->>DS: RebuildFTS5Index()
        DS->>DB: INSERT INTO documents_fts(documents_fts) VALUES('rebuild')
        DB-->>DS: Success/Error
        DS-->>RJ: Success/Error
    end
    
    RJ->>BJ: LogJobEvent("Reindex completed")
    BJ->>JLS: AppendLog(jobID, logEntry)
    
    RJ-->>WP: nil (success) or error

## Proposed File Changes

### internal\jobs\types\reindex.go(NEW)

References: 

- internal\jobs\types\cleanup.go
- internal\jobs\types\base.go
- internal\interfaces\storage.go
- internal\storage\sqlite\document_storage.go
- internal\queue\types.go
- internal\models\job_log.go

Create a new file implementing the `ReindexJob` type following the pattern from `c:/development/quaero/internal/jobs/types/cleanup.go`.

**Package and Imports:**
- Package: `types`
- Import: `context`, `fmt`, `github.com/ternarybob/quaero/internal/interfaces`, `github.com/ternarybob/quaero/internal/queue`

**ReindexJobDeps Structure:**
Create a dependencies struct similar to `CleanupJobDeps` (lines 12-16 of `cleanup.go`):
- Field: `DocumentStorage interfaces.DocumentStorage` - Required to call `RebuildFTS5Index()`

**ReindexJob Structure:**
Create the main job struct similar to `CleanupJob` (lines 18-22 of `cleanup.go`):
- Embed: `*BaseJob` - Provides common functionality (logging, job events)
- Field: `deps *ReindexJobDeps` - Holds dependencies

**NewReindexJob Constructor:**
Create a constructor function similar to `NewCleanupJob` (lines 24-30 of `cleanup.go`):
- Parameters: `base *BaseJob`, `deps *ReindexJobDeps`
- Returns: `*ReindexJob`
- Initialize the struct with provided base and deps

**Execute Method:**
Implement the `Execute(ctx context.Context, msg *queue.JobMessage) error` method following the pattern from `CleanupJob.Execute` (lines 32-213 of `cleanup.go`):

1. **Initial Logging:** Log the message ID using `logger.Info()` with structured fields
2. **Validation:** Call `Validate(msg)` and return error if validation fails (lines 38-41)
3. **Config Extraction:** Extract `dry_run` boolean from `msg.Config["dry_run"]` with type assertion to `bool` (similar to lines 64-67 of `cleanup.go`). Default to `false` if not present.
4. **Job Start Logging:** Call `LogJobEvent(ctx, msg.ParentID, "info", ...)` to log the start with dry_run status (similar to lines 70-74)
5. **Structured Logging:** Log the dry_run flag using `logger.Info().Bool("dry_run", dryRun).Msg(...)` (similar to lines 76-80)
6. **Dry Run Check:** If `dry_run` is true, log that no actual rebuild will occur and skip the rebuild operation (similar to lines 191-195)
7. **Index Rebuild:** If not dry run, call `deps.DocumentStorage.RebuildFTS5Index()` and handle any errors. Log success or failure appropriately.
8. **Completion Logging:** Call `LogJobEvent(ctx, msg.ParentID, "info", ...)` with summary message including dry_run status (similar to lines 198-203)
9. **Final Logging:** Log completion with structured fields using `logger.Info()` (similar to lines 205-211)
10. **Return:** Return `nil` on success

**Validate Method:**
Implement the `Validate(msg *queue.JobMessage) error` method following the pattern from `CleanupJob.Validate` (lines 215-252 of `cleanup.go`):

1. **Config Check:** Verify `msg.Config` is not nil (lines 217-219)
2. **ParentID Check:** Verify `msg.ParentID` is not empty, as it's required for logging job events (lines 221-224)
3. **Dry Run Validation:** If `dry_run` is present in config, verify it's a boolean type (optional but recommended for type safety)
4. **Return:** Return `nil` if all validations pass

**GetType Method:**
Implement the `GetType() string` method following the pattern from `CleanupJob.GetType` (lines 254-257 of `cleanup.go`):
- Return the string `"reindex"` to identify this job type

**Error Handling:**
- Wrap all errors with context using `fmt.Errorf("...: %w", err)` pattern
- Log errors before returning them using `logger.Error().Err(err).Msg(...)`
- Use `logger.Warn()` for non-critical issues (e.g., failed to log job event)

**Logging Conventions:**
- Use structured logging with `logger.Info()`, `logger.Warn()`, `logger.Error()`
- Include relevant fields: `message_id`, `parent_id`, `dry_run`
- Follow timestamp format from `JobLogEntry`: "15:04:05" (HH:MM:SS)
- Use log levels: "info" for normal operations, "warn" for non-critical issues, "error" for failures