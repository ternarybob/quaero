I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires creating a scheduled job for corpus summary generation that runs hourly and can be triggered post-crawl. Currently, the `SummaryService.GenerateSummaryDocument` method exists and generates document statistics (total docs, Jira docs, Confluence docs), which aligns perfectly with the requirement.

**Current State:**
- `SummaryService` exists with `GenerateSummaryDocument` method that counts documents by category
- Event-based triggers have been removed (previous phase)
- Maintenance actions framework exists with `reindexAction` as a reference pattern
- Default job definitions are created via `CreateDefaultJobDefinitions` method
- Job registry and executor infrastructure is fully operational

**Key Observations:**
1. The `GenerateSummaryDocument` method already performs the exact statistics gathering required (document counts by source type)
2. The maintenance actions pattern provides a clean template to follow
3. The hourly schedule requirement translates to cron expression `0 * * * *`
4. Post-job execution will be handled in subsequent phases (not part of this task)
5. The action should be registered under `JobTypeCustom` (like reindex) since it's a maintenance operation

### Approach

Add a `corpus_summary` action to the maintenance actions module, update dependencies to include `SummaryService`, register the action with the job registry, create a default hourly job definition, and wire the service dependency during app initialization.

### Reasoning

I examined the repository structure, read the relevant service files (`summary_service.go`, `maintenance_actions.go`, `job_definition_storage.go`, `app.go`), and analyzed the existing patterns for action registration and default job creation. I identified that the `GenerateSummaryDocument` method already implements the required functionality and just needs to be exposed as a job action.

## Mermaid Diagram

sequenceDiagram
    participant Scheduler
    participant JobExecutor
    participant MaintenanceActions
    participant SummaryService
    participant DocumentStorage
    participant Documents

    Note over Scheduler: Hourly trigger (0 * * * *)
    Scheduler->>JobExecutor: Execute "Corpus Summary Generation" job
    JobExecutor->>MaintenanceActions: corpusSummaryAction(ctx, step, sources, deps)
    MaintenanceActions->>SummaryService: GenerateSummaryDocument(ctx)
    
    SummaryService->>DocumentStorage: CountDocuments()
    DocumentStorage-->>SummaryService: totalDocs
    
    SummaryService->>DocumentStorage: CountDocumentsBySource("jira")
    DocumentStorage-->>SummaryService: jiraDocs
    
    SummaryService->>DocumentStorage: CountDocumentsBySource("confluence")
    DocumentStorage-->>SummaryService: confluenceDocs
    
    SummaryService->>SummaryService: Generate summary content
    SummaryService->>Documents: SaveDocuments([summaryDoc])
    Documents-->>SummaryService: Success
    
    SummaryService-->>MaintenanceActions: nil (success)
    MaintenanceActions-->>JobExecutor: nil (success)
    JobExecutor-->>Scheduler: Job completed
    
    Note over Documents: Summary document updated<br/>ID: corpus-summary-metadata<br/>Searchable via RAG

## Proposed File Changes

### internal\services\jobs\actions\maintenance_actions.go(MODIFY)

References: 

- internal\services\summary\summary_service.go
- internal\models\job_definition.go

**Add SummaryService to MaintenanceActionDeps (after line 16):**

Add a new field `SummaryService` with type from `internal/services/summary` package. This follows the same pattern as `DocumentStorage` and `Logger` fields.

**Create corpusSummaryAction function (after line 60, before RegisterMaintenanceActions):**

Implement a new action handler function `corpusSummaryAction` that:
- Accepts `ctx context.Context`, `step *models.JobStep`, `sources []*models.SourceConfig`, and `deps *MaintenanceActionDeps` parameters
- Logs the start of corpus summary generation using `deps.Logger.Info()` with action name
- Calls `deps.SummaryService.GenerateSummaryDocument(ctx)` to generate the summary document
- Handles errors by logging with `deps.Logger.Error()` and returning a wrapped error
- Logs successful completion with `deps.Logger.Info()`
- Returns nil on success

Follow the exact same structure and logging pattern as `reindexAction` for consistency.

**Update RegisterMaintenanceActions function (lines 63-94):**

Add validation for the new `SummaryService` dependency:
- After line 76 (Logger validation), add a nil check for `deps.SummaryService` with appropriate error message

Create a closure handler for the corpus summary action:
- After line 81 (reindexActionHandler closure), create `corpusSummaryActionHandler` closure that captures dependencies and calls `corpusSummaryAction`

Register the corpus_summary action:
- After line 86 (reindex action registration), call `registry.RegisterAction(models.JobTypeCustom, "corpus_summary", corpusSummaryActionHandler)` with error handling

Update the success log message:
- Change `Int("action_count", 1)` to `Int("action_count", 2)` on line 90 to reflect both registered actions

**Import statement:**

Add import for `github.com/ternarybob/quaero/internal/services/summary` at the top of the file (line 8 area) to reference the SummaryService type.

### internal\storage\sqlite\job_definition_storage.go(MODIFY)

References: 

- internal\models\job_definition.go

**Add Corpus Summary default job definition in CreateDefaultJobDefinitions (after line 473, before return statement):**

Create a new `JobDefinition` struct for the corpus summary job:
- `ID`: "default-corpus-summary" (follows naming convention of "default-database-maintenance")
- `Name`: "Corpus Summary Generation"
- `Type`: `models.JobTypeCustom` (maintenance operation)
- `Description`: "Generates a summary document containing statistics about the document corpus (total documents, documents by source type). This summary is searchable and enables queries like 'how many documents are in the system'. Runs hourly to keep statistics current."
- `Sources`: Empty slice `[]string{}` (operates on all documents, not specific sources)
- `Steps`: Single step array with:
  - `Name`: "corpus_summary"
  - `Action`: "corpus_summary"
  - `Config`: Empty map `map[string]interface{}{}`
  - `OnError`: `models.ErrorStrategyFail`
- `Schedule`: "0 * * * *" (hourly at minute 0)
- `Timeout`: "5m" (5 minutes should be sufficient for counting)
- `Enabled`: `true` (enable by default)
- `AutoStart`: `false` (only run on schedule, not on startup)
- `Config`: `make(map[string]interface{})`
- `CreatedAt`: `time.Now()`
- `UpdatedAt`: `time.Now()`

**Serialize and insert the job definition:**

Follow the exact same pattern as the database maintenance job (lines 406-471):
- Call `MarshalSources()`, `MarshalSteps()`, `MarshalConfig()` on the job definition
- Convert bools to integers (enabled, autoStart)
- Convert timestamps to Unix integers
- Execute INSERT query with `ON CONFLICT(id) DO NOTHING` to preserve user customizations
- Check `RowsAffected()` and log appropriately:
  - `Logger.Info()` if row inserted (new default created)
  - `Logger.Debug()` if no rows affected (already exists)
  - `Logger.Error()` on failure

**Error handling:**

If any marshaling or database operation fails, log the error with job_def_id context and return the wrapped error. This ensures the method fails fast if the corpus summary job cannot be created.

### internal\app\app.go(MODIFY)

References: 

- internal\services\summary\summary_service.go
- internal\services\jobs\actions\maintenance_actions.go(MODIFY)

**Update maintenance actions registration (lines 629-637):**

Modify the `MaintenanceActionDeps` struct initialization to include the `SummaryService` field:
- After line 631 (`DocumentStorage: a.StorageManager.DocumentStorage(),`), add a new line:
  - `SummaryService: a.SummaryService,`

This wires the `SummaryService` dependency so that the `corpus_summary` action can call `GenerateSummaryDocument` when executed.

**Verify initialization order:**

Ensure that `a.SummaryService` is initialized before the maintenance actions registration. Based on the current code:
- `SummaryService` is initialized at lines 591-595
- Maintenance actions are registered at lines 629-637

The order is correct, so no changes needed to initialization sequence.

**No import changes needed:**

The `summary` package is already imported at line 41, so no additional imports are required.