I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Existing Log Infrastructure:**
- `LogService` interface (`internal/interfaces/queue_service.go`, lines 40-51) provides single-job log fetching via `GetLogs()` and `GetLogsByLevel()`
- `logs.Service` implementation (`internal/logs/service.go`) delegates to `JobLogStorage` for database operations
- `GetJobLogsHandler` (`internal/handlers/job_handler.go`, lines 414-524) handles single-job log requests with level filtering and ordering
- `JobStorage.GetChildJobs()` (`internal/interfaces/storage.go`, line 84) retrieves all child jobs for a parent

**Parent-Child Job Architecture:**
- Jobs use flat hierarchy: child jobs have `ParentID` field pointing to parent job ID
- `CrawlJob` model (`internal/models/crawler_job.go`) contains metadata: Name, Config (with URL), Progress (with depth tracking)
- `JobLogEntry` model (`internal/models/job_log.go`) is simple: Timestamp, Level, Message

**Current Limitations:**
- No aggregated log fetching across parent-child job hierarchy
- UI cannot display unified log stream for a job family
- No job context enrichment in log entries (job name, URL, depth)

## Design Decisions

**1. Aggregation Strategy: Fetch-and-Merge Pattern**
- **Chosen Approach**: Fetch logs for parent + all children separately, then merge in-memory
- **Rationale**: 
  - Leverages existing `GetChildJobs()` and `GetLogs()`/`GetLogsByLevel()` methods
  - No database schema changes required
  - Simple to implement and test
  - Performance acceptable for typical job hierarchies (1 parent + 10-100 children)
- **Alternative Rejected**: Database-level JOIN query
  - Would require new storage method and complex SQL
  - Harder to maintain consistency with existing log fetching logic
  - Premature optimization for current scale

**2. Log Enrichment: Post-Fetch Decoration Pattern**
- **Chosen Approach**: Enrich logs with job context after fetching, before returning to client
- **Rationale**:
  - Keeps storage layer simple (no schema changes)
  - Enrichment logic centralized in handler
  - Flexible - can add/remove context fields without storage migration
- **Context Fields to Add**:
  - `job_id`: Identifies which job produced the log
  - `job_name`: User-friendly job name for display
  - `job_url`: URL being crawled (extracted from Config.SeedURLs or Progress.CurrentURL)
  - `job_depth`: Crawl depth (extracted from Config.MaxDepth)
  - `job_type`: Parent vs child job type

**3. Chronological Ordering: Client-Side Flexibility**
- **Chosen Approach**: Support both `asc` (oldest-first) and `desc` (newest-first) via query parameter
- **Rationale**:
  - Oldest-first (asc) is better for scrolling log displays (natural reading order)
  - Newest-first (desc) is better for monitoring recent activity
  - Existing `GetJobLogsHandler` already supports this pattern (line 433)
- **Default**: `asc` (oldest-first) for scrolling display use case

**4. Pagination: Simple Limit-Based Approach**
- **Chosen Approach**: Single `limit` parameter caps total logs returned (default: 1000)
- **Rationale**:
  - Simple to implement and understand
  - Sufficient for typical job log volumes
  - Matches existing `GetJobLogsHandler` pattern (line 485)
- **Alternative Rejected**: Offset-based pagination
  - Adds complexity without clear benefit for log viewing use case
  - Logs are typically viewed as continuous stream, not paginated

**5. Level Filtering: Apply Before Merge**
- **Chosen Approach**: Filter logs by level for each job before merging
- **Rationale**:
  - Reduces memory usage (don't fetch logs that will be filtered out)
  - Consistent with existing `GetLogsByLevel()` behavior
  - More efficient than fetch-all-then-filter

**6. include_children Parameter: Optional Child Inclusion**
- **Chosen Approach**: Boolean query parameter (default: true)
- **Rationale**:
  - Allows clients to fetch only parent logs if needed
  - Reduces response size for large job hierarchies
  - Provides flexibility for different UI views

## Architecture Implications

**Service Layer Responsibility:**
- `LogService` handles log aggregation logic (fetching parent + children, merging, sorting)
- Keeps handler thin - handler only does HTTP concerns (parsing params, formatting response)

**Handler Layer Responsibility:**
- Parse query parameters and validate
- Fetch job metadata for enrichment (job name, config, progress)
- Enrich logs with job context
- Format response with metadata (total count, order, level filter applied)

**No Breaking Changes:**
- Existing `GetJobLogsHandler` remains unchanged
- New endpoint is additive: `/api/jobs/{id}/logs/aggregated`
- Backward compatible with existing clients

### Approach

Add aggregated log fetching capability to the job management system by extending the LogService interface with a new `GetAggregatedLogs()` method, implementing it in `logs.Service`, and exposing it via a new HTTP endpoint `GET /api/jobs/{id}/logs/aggregated`. The implementation follows the fetch-and-merge pattern: fetch logs for parent and all children separately, merge in-memory, sort chronologically, and enrich with job context before returning to the client.

### Reasoning

I explored the codebase structure, read the LogService interface and implementation, examined the existing GetJobLogsHandler pattern, reviewed the JobStorage interface for child job fetching, analyzed the CrawlJob and JobLogEntry models, and studied the routes.go file to understand endpoint registration patterns. This gave me a complete picture of the current log infrastructure and how to extend it for aggregated log fetching.

## Mermaid Diagram

sequenceDiagram
    participant Client as UI Client
    participant Handler as JobHandler
    participant LogService as LogService
    participant JobStorage as JobStorage
    participant LogStorage as JobLogStorage

    Note over Client,LogStorage: Aggregated Log Fetching Flow

    Client->>Handler: GET /api/jobs/{id}/logs/aggregated?level=error&limit=500&include_children=true
    Handler->>Handler: Parse query params (level, limit, include_children, order)
    Handler->>Handler: Validate parameters
    
    Handler->>LogService: GetAggregatedLogs(ctx, jobID, includeChildren, level, limit)
    
    Note over LogService,LogStorage: Fetch Parent Logs
    LogService->>LogStorage: GetLogsByLevel(ctx, parentJobID, level, limit)
    LogStorage-->>LogService: Parent logs
    
    alt includeChildren = true
        Note over LogService,JobStorage: Fetch Child Jobs
        LogService->>JobStorage: GetChildJobs(ctx, parentJobID)
        JobStorage-->>LogService: Child jobs list
        
        Note over LogService,LogStorage: Fetch Child Logs (Concurrent)
        par For each child job
            LogService->>LogStorage: GetLogsByLevel(ctx, childJobID, level, limit)
            LogStorage-->>LogService: Child logs
        end
    end
    
    Note over LogService: Merge & Sort Logs
    LogService->>LogService: Combine parent + child logs
    LogService->>LogService: Sort by timestamp (oldest-first)
    LogService->>LogService: Apply limit (truncate if needed)
    
    Note over LogService,JobStorage: Build Metadata Map
    LogService->>JobStorage: GetJob(ctx, parentJobID)
    JobStorage-->>LogService: Parent job metadata
    par For each child job
        LogService->>JobStorage: GetJob(ctx, childJobID)
        JobStorage-->>LogService: Child job metadata
    end
    LogService->>LogService: Extract job context (name, URL, depth, type)
    
    LogService-->>Handler: logs, metadata, nil
    
    Note over Handler: Enrich Logs with Context
    Handler->>Handler: For each log, add job_id, job_name, job_url, job_depth, job_type
    
    alt order = desc
        Handler->>Handler: Reverse log slice (newest-first)
    end
    
    Handler->>Handler: Build JSON response with enriched logs + metadata
    Handler-->>Client: 200 OK + JSON response
    
    Note over Client: Display logs with job context in UI

## Proposed File Changes

### internal\interfaces\queue_service.go(MODIFY)

References: 

- internal\models\job_log.go
- internal\logs\service.go(MODIFY)

Add a new method to the `LogService` interface for aggregated log fetching:

**New Method Signature (add after line 51):**
- `GetAggregatedLogs(ctx context.Context, parentJobID string, includeChildren bool, level string, limit int) ([]models.JobLogEntry, map[string]interface{}, error)`

**Method Documentation:**
- Add doc comment explaining:
  - Fetches logs for parent job and optionally all child jobs
  - Merges logs from all jobs and sorts chronologically (oldest-first)
  - Returns logs slice and metadata map containing job context for enrichment
  - Metadata map structure: `map[jobID]map[string]interface{}` with keys: `job_name`, `job_url`, `job_depth`, `job_type`, `parent_id`
  - If `includeChildren` is false, only parent logs are returned
  - If `level` is non-empty, filters logs by level before merging
  - `limit` caps total logs returned across all jobs (default: 1000)

**Design Rationale:**
- Returns metadata map separately to avoid modifying JobLogEntry model (keeps storage layer clean)
- Handler will use metadata to enrich logs before sending to client
- Metadata includes job context needed for UI display (name, URL, depth, type)

**Error Handling:**
- Returns error if parent job not found
- Logs warning but continues if child job fetch fails (partial results acceptable)
- Returns error if log fetching fails for parent job

### internal\logs\service.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\interfaces\queue_service.go(MODIFY)
- internal\models\crawler_job.go

Implement the `GetAggregatedLogs()` method in the `Service` struct:

**Method Implementation (add after line 206):**

**Step 1: Fetch Parent Job**
- Use `s.storage.GetLogs()` or `s.storage.GetLogsByLevel()` to fetch parent logs based on `level` parameter
- If parent job has no logs, check if job exists in storage (return 404 if not found)
- Build metadata entry for parent job with keys: `job_name`, `job_url`, `job_depth`, `job_type`, `parent_id`

**Step 2: Fetch Child Jobs (if includeChildren=true)**
- Call `jobStorage.GetChildJobs(ctx, parentJobID)` to get all child jobs
- For each child job:
  - Fetch logs using `s.storage.GetLogs()` or `s.storage.GetLogsByLevel()` based on `level` parameter
  - Build metadata entry for child job
  - If fetch fails, log warning and continue (partial results acceptable)

**Step 3: Merge and Sort Logs**
- Combine parent logs and all child logs into single slice
- Sort by timestamp (oldest-first for scrolling display)
- Timestamp format is "HH:MM:SS" (e.g., "14:23:45") - use string comparison for sorting
- Handle edge case: logs with same timestamp maintain original order (stable sort)

**Step 4: Apply Limit**
- If merged logs exceed `limit`, truncate to first `limit` entries (oldest logs)
- This ensures chronological display starts from beginning of job execution

**Step 5: Build Metadata Map**
- Create map structure: `map[jobID]map[string]interface{}`
- For each job (parent + children), add metadata entry:
  - `job_name`: From `job.Name` field
  - `job_url`: Extract from `job.Config.SeedURLs[0]` or `job.Progress.CurrentURL` (first available)
  - `job_depth`: From `job.Config.MaxDepth`
  - `job_type`: From `job.JobType` (parent, crawler_url, etc.)
  - `parent_id`: From `job.ParentID` (empty for parent jobs)

**Dependencies:**
- Requires access to `JobStorage` interface to fetch job metadata
- Add `jobStorage interfaces.JobStorage` field to `Service` struct
- Update `NewService()` constructor to accept `jobStorage` parameter

**Error Handling:**
- Return error if parent job not found in storage
- Log warning (not error) if child job fetch fails - continue with partial results
- Return error if parent log fetch fails
- Use `s.logger.Warn()` for non-critical failures (child job fetch, metadata extraction)

**Performance Considerations:**
- Fetch logs concurrently for parent and children using goroutines and sync.WaitGroup
- Use buffered channels to collect results
- Limit concurrency to avoid overwhelming database (max 10 concurrent fetches)
- Total execution time should be O(max child fetch time) not O(sum of all fetch times)

### internal\app\app.go(MODIFY)

References: 

- internal\logs\service.go(MODIFY)

Update LogService initialization to pass JobStorage dependency:

**Locate LogService Initialization (around line 240):**
- Find the line: `logService := logs.NewService(a.StorageManager.JobLogStorage(), a.WSHandler, a.Logger)`

**Update Constructor Call:**
- Change to: `logService := logs.NewService(a.StorageManager.JobLogStorage(), a.StorageManager.JobStorage(), a.WSHandler, a.Logger)`
- This adds `JobStorage` as the second parameter (after `JobLogStorage`, before `WSHandler`)

**Rationale:**
- LogService needs JobStorage to fetch job metadata for log enrichment
- JobStorage is already initialized before LogService (correct dependency order)
- No other changes needed - LogService initialization happens at the right point in the sequence

**Verification:**
- Ensure JobStorage is initialized before LogService (should already be correct)
- Ensure no circular dependencies introduced (JobStorage doesn't depend on LogService)

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\interfaces\queue_service.go(MODIFY)
- internal\models\job_log.go
- internal\models\crawler_job.go

Add new HTTP handler for aggregated job logs:

**New Handler Method (add after GetJobLogsHandler, around line 525):**

**Method Signature:**
- `func (h *JobHandler) GetAggregatedJobLogsHandler(w http.ResponseWriter, r *http.Request)`

**Step 1: Extract Job ID from Path**
- Parse path: `/api/jobs/{id}/logs/aggregated`
- Extract jobID from path parts (same pattern as GetJobLogsHandler, line 420-425)
- Return 400 Bad Request if jobID is empty

**Step 2: Parse Query Parameters**
- `level` (string): Log level filter (error, warn, info, debug, all) - default: "all"
- `limit` (int): Max logs to return - default: 1000
- `include_children` (bool): Include child job logs - default: true
- `order` (string): Sort order (asc=oldest-first, desc=newest-first) - default: "asc"
- Validate level parameter (same validation as GetJobLogsHandler, lines 442-468)
- Normalize level aliases: "warning" → "warn", "err" → "error"

**Step 3: Fetch Aggregated Logs**
- Call `h.logService.GetAggregatedLogs(ctx, jobID, includeChildren, level, limit)`
- Returns: `logs []models.JobLogEntry`, `metadata map[string]interface{}`, `error`
- Handle errors:
  - Job not found: Return 404 with message "Job not found"
  - Other errors: Return 500 with message "Failed to get aggregated logs"

**Step 4: Enrich Logs with Job Context**
- For each log entry, create enriched log object:
  - Extract jobID from log's correlation context (stored in metadata map)
  - Add fields from metadata: `job_id`, `job_name`, `job_url`, `job_depth`, `job_type`, `parent_id`
  - Preserve original fields: `timestamp`, `level`, `message`
- Result structure: `[]map[string]interface{}` with all fields combined

**Step 5: Apply Ordering**
- Logs come from LogService in oldest-first order (asc)
- If `order=desc` requested, reverse the slice (same logic as GetJobLogsHandler, lines 509-514)
- This allows UI to choose display order

**Step 6: Build Response**
- Response structure:
  ```
  {
    "job_id": string,
    "logs": []map[string]interface{},  // Enriched logs with job context
    "count": int,                       // Number of logs returned
    "order": string,                    // Applied order (asc/desc)
    "level": string,                    // Applied level filter (or "all")
    "include_children": bool,           // Whether children were included
    "metadata": map[string]interface{}  // Job metadata for client reference
  }
  ```
- Set Content-Type: application/json
- Encode response with json.NewEncoder(w).Encode()

**Error Handling:**
- Log all errors with structured logging: `h.logger.Error().Err(err).Str("job_id", jobID).Msg(...)`
- Return appropriate HTTP status codes: 400 (bad request), 404 (not found), 500 (internal error)
- Include error context in log messages for debugging

**Performance Considerations:**
- Enrichment happens in-memory (fast)
- No additional database queries beyond LogService.GetAggregatedLogs()
- Response size capped by `limit` parameter (default 1000 logs)

### internal\server\routes.go(MODIFY)

References: 

- internal\handlers\job_handler.go(MODIFY)

Register the new aggregated logs endpoint in the routing logic:

**Locate Job Routes Handler (line 100-174):**
- Find the `handleJobRoutes()` function

**Add Route for Aggregated Logs (after line 148, before line 150):**
- Add new condition in the GET request handling section:
  ```
  // Check if it's /api/jobs/{id}/logs/aggregated
  if len(pathSuffix) > 0 && strings.HasSuffix(pathSuffix, "/logs/aggregated") {
      s.app.JobHandler.GetAggregatedJobLogsHandler(w, r)
      return
  }
  ```

**Placement Rationale:**
- Must come BEFORE the `/logs` check (line 146-149) to avoid false match
- Route matching order matters: more specific routes must be checked first
- Pattern: `/logs/aggregated` is more specific than `/logs`

**Route Pattern:**
- Full path: `GET /api/jobs/{id}/logs/aggregated`
- Query parameters: `?level=error&limit=500&include_children=true&order=asc`
- Example: `GET /api/jobs/abc123/logs/aggregated?level=error&limit=100&include_children=true`

**Backward Compatibility:**
- Existing `/api/jobs/{id}/logs` endpoint remains unchanged
- No breaking changes to existing clients
- New endpoint is purely additive

**Testing Considerations:**
- Verify route matching order: `/logs/aggregated` should match before `/logs`
- Test with various path formats to ensure correct handler is called
- Verify query parameters are correctly parsed and passed to handler