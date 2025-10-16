I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Existing Infrastructure:**
- \`CrawlJob\` model has basic fields but lacks snapshot capabilities
- \`JobHandler\` is initialized with only \`CrawlerService\` and \`JobStorage\` (line 328 in \`app.go\`)
- \`CrawlerService\` is initialized with \`AuthService\`, \`EventService\`, and \`JobStorage\` (line 241 in \`app.go\`)
- Database schema has \`crawl_jobs\` table but missing snapshot columns
- Routes exist for job management but missing \`/api/jobs/create\` and \`/api/jobs/queue\`
- UI (\`jobs.html\`) already has queue visibility and management features
- Test infrastructure uses \`HTTPTestHelper\` pattern from \`test/helpers.go\`

**Key Dependencies:**
- \`SourceService\` manages source configurations with validation
- \`AuthStorage\` provides auth credential retrieval by ID
- \`SourceStorage\` provides source configuration retrieval
- \`models.SourceConfig\` has \`Validate()\` method for validation
- \`models.AuthCredentials\` structure includes cookies as \`[]byte\` and tokens as \`map[string]string\`

**Current Job Flow:**
- \`StartCrawl()\` accepts \`sourceType\`, \`entityType\`, \`seedURLs\`, and \`CrawlConfig\`
- Jobs are created with UUID, stored in database, and workers are started
- Progress is tracked and emitted via \`EventCrawlProgress\` events
- Queue uses priority heap with deduplication in \`queue.go\`

### Approach

## Implementation Strategy

**Phase 1: Data Model & Schema**
Extend \`CrawlJob\` to include snapshot fields and update database schema to persist them.

**Phase 2: Service Layer Enhancement**
Modify \`CrawlerService\` to accept source/auth services, capture snapshots, validate configurations, and implement refresh logic.

**Phase 3: Handler & API Layer**
Add new job creation endpoint and queue visibility endpoint to \`JobHandler\`, update routing.

**Phase 4: Testing**
Create comprehensive API tests for job lifecycle with snapshots, validation, and refresh scenarios.

**Key Design Decisions:**
1. Store snapshots as JSON strings in database for flexibility and auditability
2. Add \`SourceService\` and \`AuthStorage\` dependencies to \`CrawlerService\` via constructor
3. Inject dependencies through \`app.go\` initialization
4. Refresh logic fetches latest config/auth and re-validates before updating snapshots
5. Queue endpoint returns jobs filtered by \`pending\` and \`running\` status
6. Seed URLs will be derived from source configuration or passed explicitly in create request

### Reasoning

Explored the codebase structure by reading key files including the crawler service, job storage, handlers, models, and application initialization. Examined existing test patterns and routing configuration. Analyzed how services are wired together in \`app.go\` and identified the dependency injection points needed for the new functionality. Reviewed the database schema and existing job management UI to understand integration points.

## Mermaid Diagram

sequenceDiagram
    participant UI as UI (jobs.html)
    participant Handler as JobHandler
    participant CrawlerSvc as CrawlerService
    participant SourceSvc as SourceService
    participant AuthStore as AuthStorage
    participant JobStore as JobStorage
    participant DB as SQLite Database

    Note over UI,DB: Job Creation with Snapshots
    UI->>Handler: POST /api/jobs/create<br/>{source_id, refresh_source}
    Handler->>SourceSvc: GetSource(source_id)
    SourceSvc->>DB: SELECT from sources
    DB-->>SourceSvc: SourceConfig
    SourceSvc-->>Handler: SourceConfig
    
    alt Source has auth_id
        Handler->>AuthStore: GetCredentialsByID(auth_id)
        AuthStore->>DB: SELECT from auth_credentials
        DB-->>AuthStore: AuthCredentials
        AuthStore-->>Handler: AuthCredentials
    end
    
    Handler->>Handler: Validate SourceConfig
    Handler->>Handler: Derive seed URLs
    Handler->>CrawlerSvc: StartCrawl(sourceID, refreshSource,<br/>sourceSnapshot, authSnapshot)
    
    alt refresh_source == true
        CrawlerSvc->>SourceSvc: GetSource(source_id)
        SourceSvc-->>CrawlerSvc: Latest SourceConfig
        CrawlerSvc->>CrawlerSvc: Validate latest config
        CrawlerSvc->>CrawlerSvc: Update snapshots
    end
    
    CrawlerSvc->>CrawlerSvc: Create CrawlJob with snapshots
    CrawlerSvc->>JobStore: SaveJob(job)
    JobStore->>DB: INSERT into crawl_jobs<br/>(with snapshot columns)
    DB-->>JobStore: Success
    JobStore-->>CrawlerSvc: Success
    CrawlerSvc->>CrawlerSvc: Start workers
    CrawlerSvc-->>Handler: job_id
    Handler-->>UI: 201 Created {job_id}
    
    Note over UI,DB: Queue Visibility
    UI->>Handler: GET /api/jobs/queue
    Handler->>JobStore: GetJobsByStatus(\"pending\")
    JobStore->>DB: SELECT WHERE status='pending'
    DB-->>JobStore: Pending jobs
    Handler->>JobStore: GetJobsByStatus(\"running\")
    JobStore->>DB: SELECT WHERE status='running'
    DB-->>JobStore: Running jobs
    Handler-->>UI: {pending: [...], running: [...]}

## Proposed File Changes

### internal\\services\\crawler\\types.go(MODIFY)

References: 

- internal\\models\\source.go
- internal\\models\\atlassian.go

Add three new fields to the \`CrawlJob\` struct:
- \`SourceConfigSnapshot\` (string): JSON-serialized snapshot of \`models.SourceConfig\` at job creation time
- \`AuthSnapshot\` (string): JSON-serialized snapshot of \`models.AuthCredentials\` at job creation time
- \`RefreshSource\` (bool): Flag indicating whether to refresh config/auth before execution

Add helper methods:
- \`SetSourceConfigSnapshot(*models.SourceConfig) error\`: Marshals and stores source config as JSON
- \`GetSourceConfigSnapshot() (*models.SourceConfig, error)\`: Unmarshals source config from JSON
- \`SetAuthSnapshot(*models.AuthCredentials) error\`: Marshals and stores auth credentials as JSON
- \`GetAuthSnapshot() (*models.AuthCredentials, error)\`: Unmarshals auth credentials from JSON

These methods handle JSON serialization/deserialization for the snapshot fields, similar to existing \`ToJSON()\` and \`FromJSON()\` methods for \`CrawlConfig\` and \`CrawlProgress\`.

### internal\\storage\\sqlite\\schema.go(MODIFY)

Update the \`crawl_jobs\` table definition in \`schemaSQL\` constant to add three new columns:
- \`source_config_snapshot TEXT\`: Stores JSON snapshot of source configuration
- \`auth_snapshot TEXT\`: Stores JSON snapshot of authentication credentials
- \`refresh_source INTEGER DEFAULT 0\`: Boolean flag (0/1) for refresh behavior

Add these columns after the \`config_json\` column and before \`status\` to maintain logical grouping of configuration-related fields. These columns are nullable to support backward compatibility with existing jobs.

### internal\\storage\\sqlite\\job_storage.go(MODIFY)

References: 

- internal\\services\\crawler\\types.go(MODIFY)

Update \`SaveJob()\` method to persist the new snapshot fields:
- Extract \`SourceConfigSnapshot\`, \`AuthSnapshot\`, and \`RefreshSource\` from \`CrawlJob\`
- Convert \`RefreshSource\` bool to integer (0/1) for SQLite
- Add these fields to the INSERT/UPDATE query
- Handle nullable snapshots (use \`sql.NullString\` for optional fields)

Update \`scanJob()\` and \`scanJobs()\` methods to read the new fields:
- Add variables for \`sourceConfigSnapshot\`, \`authSnapshot\`, and \`refreshSource\` in Scan()
- Convert integer back to bool for \`RefreshSource\`
- Populate the new fields in the returned \`CrawlJob\` struct
- Handle NULL values gracefully (empty strings for snapshots, false for refresh flag)

Ensure backward compatibility by checking if snapshot fields are empty before attempting deserialization.

### internal\\services\\crawler\\service.go(MODIFY)

References: 

- internal\\services\\sources\\service.go
- internal\\storage\\sqlite\\auth_storage.go
- internal\\models\\source.go

Add new dependencies to \`Service\` struct:
- \`sourceService *sources.Service\`: For fetching source configurations
- \`authStorage interfaces.AuthStorage\`: For fetching auth credentials

Update \`NewService()\` constructor signature to accept these new dependencies and assign them to the struct.

Modify \`StartCrawl()\` method to accept additional parameters:
- Add \`sourceID string\` parameter (can be empty if not using source-based config)
- Add \`refreshSource bool\` parameter
- Add \`sourceConfigSnapshot *models.SourceConfig\` parameter (optional, can be nil)
- Add \`authSnapshot *models.AuthCredentials\` parameter (optional, can be nil)

Implement snapshot and validation logic in \`StartCrawl()\`:
1. **If \`sourceID\` is provided and snapshots are nil**: Fetch source config using \`sourceService.GetSource()\`, fetch auth using \`authStorage.GetCredentialsByID()\` if \`source.AuthID\` is set
2. **If \`refreshSource\` is true**: Re-fetch latest source config and auth credentials, validate using \`source.Validate()\`, update snapshots
3. **Validation**: Call \`sourceConfig.Validate()\` to ensure configuration is valid before job execution
4. **Snapshot storage**: Use the new helper methods (\`SetSourceConfigSnapshot()\`, \`SetAuthSnapshot()\`) to store snapshots in the job
5. **Set \`RefreshSource\` field** in the job based on the parameter

Update the \`CrawlJob\` creation to include the new fields before persisting to database.

Add error handling for:
- Source not found
- Auth credentials not found
- Validation failures
- JSON serialization errors

### internal\\app\\app.go(MODIFY)

References: 

- internal\\services\\crawler\\service.go(MODIFY)
- internal\\handlers\\job_handler.go(MODIFY)

Update the \`CrawlerService\` initialization in \`initServices()\` method (around line 241):

Change the \`crawler.NewService()\` call to pass the additional dependencies:
- Add \`a.SourceService\` as a parameter
- Add \`a.StorageManager.AuthStorage()\` as a parameter

This requires updating the call from:
\`\`\`
a.CrawlerService = crawler.NewService(a.AuthService, a.EventService, a.StorageManager.JobStorage(), a.Logger, crawlerConfig)
\`\`\`

To include the new parameters in the correct order as defined by the updated \`NewService()\` signature in \`c:/development/quaero/internal/services/crawler/service.go\`.

No other changes needed in this file as \`SourceService\` and \`AuthStorage\` are already initialized earlier in the method.
Update the \`JobHandler\` initialization in \`initHandlers()\` method (around line 328):

Change the \`handlers.NewJobHandler()\` call to pass the additional dependencies:
- Add \`a.SourceService\` as a parameter
- Add \`a.StorageManager.AuthStorage()\` as a parameter

This requires updating the call from:
\`\`\`
a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.JobStorage(), a.Logger)
\`\`\`

To include the new parameters in the correct order as defined by the updated \`NewJobHandler()\` signature in \`c:/development/quaero/internal/handlers/job_handler.go\`.

### internal\\handlers\\job_handler.go(MODIFY)

References: 

- internal\\services\\sources\\service.go
- internal\\storage\\sqlite\\auth_storage.go
- internal\\models\\source.go
- internal\\services\\crawler\\service.go(MODIFY)

Add new dependencies to \`JobHandler\` struct:
- \`sourceService *sources.Service\`: For fetching source configurations
- \`authStorage interfaces.AuthStorage\`: For fetching auth credentials

Update \`NewJobHandler()\` constructor to accept and assign these dependencies.

Add new handler method \`CreateJobHandler()\` for \`POST /api/jobs/create\`:
- Accept JSON request body with fields: \`source_id\` (string, required), \`refresh_source\` (bool, optional, default false), \`seed_urls\` ([]string, optional), \`config_overrides\` (CrawlConfig, optional)
- Fetch source configuration using \`sourceService.GetSource()\`
- Fetch auth credentials if \`source.AuthID\` is set using \`authStorage.GetCredentialsByID()\`
- Validate source configuration using \`source.Validate()\`
- Derive seed URLs from source configuration if not provided in request (e.g., construct base API URLs from \`source.BaseURL\` and \`source.Type\`)
- Merge config overrides with source's \`CrawlConfig\` if provided
- Call \`crawlerService.StartCrawl()\` with source ID, refresh flag, and snapshots
- Return 201 Created with job details on success
- Return appropriate error codes: 400 for validation errors, 404 for source not found, 500 for internal errors

Add new handler method \`GetJobQueueHandler()\` for \`GET /api/jobs/queue\`:
- Use \`jobStorage.GetJobsByStatus()\` to fetch jobs with status \`pending\` and \`running\`
- Combine and sort by \`created_at\` DESC
- Return JSON response with structure: \`{\"pending\": [...], \"running\": [...], \"total\": count}\`
- Include job details: ID, source type, entity type, status, progress, created timestamp

Add helper method \`deriveSeeedURLs(source *models.SourceConfig) []string\`:
- Based on \`source.Type\` (jira, confluence, github), construct appropriate API endpoint URLs
- For Jira: \`{baseURL}/rest/api/3/project\`
- For Confluence: \`{baseURL}/rest/api/space\`
- For GitHub: construct from base URL and filters
- Return empty slice if unable to derive

### internal\\server\\routes.go(MODIFY)

References: 

- internal\\handlers\\job_handler.go(MODIFY)

Update the \`handleJobRoutes()\` function to add routing for the new endpoints:

Add handling for \`POST /api/jobs/create\`:
- Check if \`r.Method == \"POST\"\` and \`path == \"/api/jobs/create\"\`
- Call \`s.app.JobHandler.CreateJobHandler(w, r)\`
- Place this check before the existing POST handlers for rerun/cancel

Add handling for \`GET /api/jobs/queue\`:
- In the existing GET method check, add condition for \`pathSuffix == \"queue\"\`
- Call \`s.app.JobHandler.GetJobQueueHandler(w, r)\`
- Place this check alongside the existing \`stats\` check (around line 94-97)

Ensure proper ordering: check for specific paths (\`/create\`, \`/queue\`, \`/stats\`) before falling through to generic \`/{id}\` handling.

### test\\api\\job_api_test.go(NEW)

References: 

- test\\api\\sources_api_test.go
- test\\helpers.go

Create comprehensive API tests for job management with snapshots:

**Test: \`TestCreateJobWithSource\`**
- Create test authentication using pattern from \`c:/development/quaero/test/api/sources_api_test.go\`
- Create test source configuration with auth_id
- Call \`POST /api/jobs/create\` with \`{\"source_id\": \"...\", \"refresh_source\": false}\`
- Assert 201 Created status
- Verify response contains job_id
- Fetch job details using \`GET /api/jobs/{id}\`
- Assert job has \`source_config_snapshot\` and \`auth_snapshot\` fields populated
- Assert \`refresh_source\` is false
- Cleanup: delete job, source, and auth

**Test: \`TestCreateJobWithRefresh\`**
- Create source and auth
- Create job with \`refresh_source: true\`
- Update source configuration (change name or config)
- Verify job was created with original snapshot (not refreshed yet)
- Test that refresh logic would fetch latest on execution

**Test: \`TestCreateJobValidationFailure\`**
- Create source with invalid configuration (e.g., negative concurrency)
- Attempt to create job
- Assert 400 Bad Request status
- Verify error message indicates validation failure

**Test: \`TestCreateJobSourceNotFound\`**
- Call \`POST /api/jobs/create\` with non-existent source_id
- Assert 404 Not Found status

**Test: \`TestCreateJobWithoutAuth\`**
- Create source without auth_id
- Create job successfully
- Verify \`auth_snapshot\` is empty or null

**Test: \`TestGetJobQueue\`**
- Create multiple jobs with different statuses (pending, running, completed)
- Call \`GET /api/jobs/queue\`
- Assert response contains only pending and running jobs
- Verify jobs are sorted by created_at DESC
- Verify response structure has \`pending\` and \`running\` arrays

**Test: \`TestJobSnapshotImmutability\`**
- Create source and job
- Modify source configuration
- Fetch job details
- Verify job's snapshot still contains original configuration
- Demonstrates snapshot isolation

Use \`HTTPTestHelper\` from \`c:/development/quaero/test/helpers.go\` for all HTTP operations. Follow cleanup patterns from \`c:/development/quaero/test/api/sources_api_test.go\` using defer statements.

### pages\\jobs.html(MODIFY)

References: 

- pages\\sources.html

Add UI functionality for creating jobs from sources:

**Add \"Create Job\" button** in the Job Statistics card header (around line 25):
- Button with text \"Create Job\" and icon \`fa-plus\`
- Onclick handler \`showCreateJobModal()\`

**Add modal dialog** for job creation (before closing \`</body>\` tag):
- Modal with form containing:
  - Dropdown to select source (populated from \`/api/sources\`)
  - Checkbox for \"Refresh source before execution\"
  - Optional text area for seed URLs (one per line)
  - Submit button \"Create Job\"
  - Cancel button to close modal
- Use Alpine.js for modal state management
- Style using existing Quaero CSS classes

**Add JavaScript functions**:
- \`showCreateJobModal()\`: Opens modal, loads sources list
- \`loadSourcesForJobCreation()\`: Fetches sources from \`/api/sources\` and populates dropdown
- \`createJobFromSource()\`: Submits form data to \`POST /api/jobs/create\`, handles response, refreshes job list on success
- \`closeCreateJobModal()\`: Closes modal and resets form

**Update existing \`loadJobs()\` function**:
- Add call to load queue data from \`/api/jobs/queue\` for real-time queue visibility
- Display queue count in statistics section

**Add queue indicator** in Job Statistics card:
- Show count of pending jobs in queue
- Show count of currently running jobs
- Update these counts when jobs are created or status changes

Follow existing patterns from \`c:/development/quaero/pages/sources.html\` for modal implementation and form handling.