I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase has a well-established pattern for job types and storage operations. The `ReindexJob` type has been implemented in `c:/development/quaero/internal/jobs/types/reindex.go` and registered with the WorkerPool in `c:/development/quaero/internal/app/app.go` (lines 471-481).

The initialization flow is: `NewManager()` → `NewSQLiteDB()` → `InitSchema()` → `runMigrations()`. The `JobDefinitionStorage` is created in `NewManager()` after the database schema is initialized, making it the ideal location to create default job definitions.

The `SaveJobDefinition()` method (lines 40-122 of `job_definition_storage.go`) already implements `ON CONFLICT` handling (lines 88-105), which means calling it multiple times with the same ID will update the existing record rather than fail. This is perfect for idempotent default data creation.

### Approach

Add a `CreateDefaultJobDefinitions()` method to `JobDefinitionStorage` that creates a "Database Maintenance" job definition with a weekly schedule to run the reindex job. Call this method from `NewManager()` after all storage instances are initialized. Use the existing `SaveJobDefinition()` method which handles conflicts gracefully, ensuring the operation is idempotent across application restarts.

### Reasoning

I explored the codebase structure by reading the relevant files mentioned by the user: `job_definition_storage.go`, `schema.go`, and `job_definition.go`. I examined the initialization flow in `connection.go` and `manager.go` to understand when and where default job definitions should be created. I also reviewed the migration patterns in `schema.go` to understand the codebase conventions for database initialization tasks.

## Mermaid Diagram

sequenceDiagram
    participant App as Application
    participant Mgr as Manager.NewManager()
    participant DB as SQLiteDB
    participant JDS as JobDefinitionStorage
    participant SQLite as SQLite Database

    App->>Mgr: NewManager(logger, config)
    Mgr->>DB: NewSQLiteDB(logger, config)
    DB->>DB: InitSchema()
    DB->>DB: runMigrations()
    Note over DB: Schema & migrations complete
    DB-->>Mgr: SQLiteDB instance
    
    Mgr->>JDS: NewJobDefinitionStorage(db, logger)
    JDS-->>Mgr: JobDefinitionStorage instance
    
    Note over Mgr: All storage instances created
    
    Mgr->>JDS: CreateDefaultJobDefinitions(ctx)
    
    JDS->>JDS: Create JobDefinition struct<br/>ID: "default-database-maintenance"<br/>Schedule: "0 2 * * 0"<br/>Action: "reindex"
    
    JDS->>JDS: SaveJobDefinition(ctx, jobDef)
    JDS->>SQLite: INSERT ... ON CONFLICT DO UPDATE
    
    alt First Run
        SQLite-->>JDS: Row inserted
    else Subsequent Runs
        SQLite-->>JDS: Row updated (idempotent)
    end
    
    JDS->>JDS: Log success
    JDS-->>Mgr: nil (success)
    
    Mgr->>Mgr: Log "Default job definitions initialized"
    Mgr-->>App: Manager instance with defaults
    
    Note over App,SQLite: Default "Database Maintenance" job<br/>now available in job_definitions table

## Proposed File Changes

### internal\storage\sqlite\job_definition_storage.go(MODIFY)

References: 

- internal\models\job_definition.go
- internal\app\app.go

Add a new method `CreateDefaultJobDefinitions()` to the `JobDefinitionStorage` struct following the existing method patterns.

**Method Signature:**
- Name: `CreateDefaultJobDefinitions`
- Receiver: `(s *JobDefinitionStorage)`
- Parameters: `ctx context.Context`
- Returns: `error`

**Implementation Details:**

1. **Method Documentation:** Add a comment explaining that this method creates default job definitions that ship with Quaero, and that it's idempotent (safe to call multiple times).

2. **Database Maintenance Job Definition:**
   - **ID:** `"default-database-maintenance"` - Use a prefix to distinguish default jobs from user-created ones
   - **Name:** `"Database Maintenance"`
   - **Type:** `models.JobTypeCustom` - This is a custom job type as defined in `c:/development/quaero/internal/models/job_definition.go` (line 24)
   - **Description:** `"Rebuilds the FTS5 full-text search index to ensure optimal search performance. Runs weekly to keep the search index synchronized with document changes."`
   - **Sources:** Empty array `[]string{}` - This job doesn't operate on specific sources
   - **Schedule:** `"0 2 * * 0"` - Cron expression for Sunday at 2:00 AM (weekly)
   - **Timeout:** `"30m"` - 30 minutes should be sufficient for index rebuilding
   - **Enabled:** `true` - Enable by default
   - **AutoStart:** `false` - Don't auto-start on scheduler initialization, only run on schedule
   - **Config:** Empty map `make(map[string]interface{})` - No job-level config needed
   - **Steps:** Single step array with one `models.JobStep`:
     - **Name:** `"reindex"`
     - **Action:** `"reindex"` - Matches the job type registered in `c:/development/quaero/internal/app/app.go` (line 479)
     - **Config:** Map with `dry_run` set to `false`: `map[string]interface{}{"dry_run": false}`
     - **OnError:** `models.ErrorStrategyFail` - Stop execution if reindexing fails (defined in `c:/development/quaero/internal/models/job_definition.go` line 43)

3. **Timestamps:** Set `CreatedAt` and `UpdatedAt` to `time.Now()` - The `SaveJobDefinition()` method will handle these appropriately (lines 50-53 of `job_definition_storage.go`)

4. **Save Operation:** Call `s.SaveJobDefinition(ctx, jobDef)` to persist the job definition. This method uses `ON CONFLICT DO UPDATE` (lines 88-105), making it safe to call multiple times.

5. **Error Handling:**
   - If `SaveJobDefinition()` returns an error, wrap it with context: `fmt.Errorf("failed to create default database maintenance job: %w", err)`
   - Log success using `s.logger.Info().Str("job_def_id", jobDef.ID).Msg("Default job definition created/updated")`
   - Log any errors using `s.logger.Error().Err(err).Msg("Failed to create default job definitions")`

6. **Return:** Return `nil` on success, or the wrapped error on failure

**Location:** Add this method after the `CountJobDefinitions()` method (after line 374) and before the `scanJobDefinition()` method (before line 376).

**Imports:** Ensure `time` package is imported (already present at line 14).

### internal\storage\sqlite\manager.go(MODIFY)

References: 

- internal\storage\sqlite\job_definition_storage.go(MODIFY)
- internal\storage\sqlite\connection.go

Call the new `CreateDefaultJobDefinitions()` method after all storage instances are initialized in the `NewManager()` function.

**Location:** After line 39 (after the log message "Job definition storage initialized") and before the return statement (before line 41).

**Implementation:**

1. **Create Context:** Create a background context for the operation: `ctx := context.Background()`

2. **Call Method:** Call the `CreateDefaultJobDefinitions()` method on the `jobDefinition` storage instance:
   - Use the manager's `jobDefinition` field (which is of type `interfaces.JobDefinitionStorage`)
   - Cast it to `*JobDefinitionStorage` to access the concrete method: `if jds, ok := manager.jobDefinition.(*JobDefinitionStorage); ok { ... }`
   - Inside the type assertion block, call `jds.CreateDefaultJobDefinitions(ctx)`

3. **Error Handling:**
   - If the method returns an error, log it as a warning (not fatal): `logger.Warn().Err(err).Msg("Failed to create default job definitions")`
   - Do NOT return the error - this should not prevent the application from starting
   - The rationale: Default job definitions are a convenience feature, not critical for application startup

4. **Success Logging:** If successful, log an info message: `logger.Info().Msg("Default job definitions initialized")`

**Imports:** Add `context` to the imports at the top of the file (currently imports are at lines 3-7).

**Rationale for Placement:**
- This must happen AFTER `NewJobDefinitionStorage()` is called (line 35)
- This must happen AFTER the database schema is initialized (which happens in `NewSQLiteDB()` at line 23)
- This should happen BEFORE the manager is returned to ensure defaults are available immediately
- Placing it in `NewManager()` ensures it runs once per application startup, not on every schema initialization