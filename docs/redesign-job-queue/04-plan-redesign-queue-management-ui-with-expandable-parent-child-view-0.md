I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Architecture Issues:**

1. **Dual Progress Calculation:** Backend calculates child statistics (child_count, completed_children, failed_children) but UI recalculates progress text client-side in `getParentProgressText()` (lines 1640-1712) and progress bar styles in `getParentProgressBarStyle()` (lines 1714-1735)

2. **Complex Child Job Management:** UI maintains multiple caches:
   - `childJobsCache` - Full child job objects
   - `childJobsList` - Child job metadata
   - `expandedParents` - Expansion state
   - `loadingParents` - Loading state
   This creates synchronization issues and complexity

3. **Progress Bars:** Lines 284-287 render progress bars using client-side calculations, which are inaccurate when child counts are unavailable

4. **WebSocket Update Logic:** `updateJobInList()` (lines 2018-2099) updates individual job fields but doesn't refresh parent job status when children update. The `handleChildJobStatus()` method (lines 1426-1447) manually increments counters which can drift from backend state

5. **Job Type Discrimination:** UI has special handling for "workflow" vs "task" jobs (lines 187, 209-230, 556-558, 698-708) which should be removed per user requirements

6. **Backend Enrichment:** `JobHandler.ListJobsHandler` (lines 162-184) enriches jobs with child statistics but doesn't use `GetStatusReport()` method that was added in phase 1

**Key Findings:**

- `CrawlJob.GetStatusReport()` already exists (crawler_job.go:315-372) and returns `JobStatusReport` with ProgressText, Errors, Warnings
- Backend enrichment happens in `convertJobToMap()` and manual field addition (job_handler.go:169-181)
- WebSocket events use `JobStatusUpdate` struct (websocket.go:181-197) which doesn't include status_report
- UI Alpine component has 20+ methods, many for client-side calculations that should be backend-driven
- Child job display is inline within parent cards (lines 290-323) but uses complex caching logic

### Approach

Simplify the Queue Management UI by eliminating client-side progress calculations and hierarchical rendering complexity. Use the backend's `GetStatusReport()` method to provide standardized status information including progress text, errors, and warnings. Remove progress bars and replace with text-based status. Implement inline child job display when parent cards are clicked (expand/collapse). Update WebSocket handlers to refresh parent jobs when child jobs update. Make the UI job-agnostic by removing special handling for workflow vs task jobs.

### Reasoning

I explored the codebase by reading queue.html (1982+ lines with complex Alpine.js components), job_handler.go (API enrichment logic), websocket_events.go (event broadcasting), websocket.go (WebSocket handler), crawler_job.go (GetStatusReport implementation), and jobtypes.go (JobStatusReport structure). I traced how jobs are fetched, enriched with child statistics, rendered in the UI, and updated via WebSocket events. I identified the client-side progress calculation methods (getParentProgressText, getParentProgressBarStyle), child job caching logic (childJobsCache, childJobsList), and hierarchical rendering complexity that need to be simplified.

## Mermaid Diagram

sequenceDiagram
    participant UI as Queue UI (Alpine.js)
    participant API as JobHandler
    participant WS as WebSocket
    participant Job as CrawlJob Model
    participant Storage as JobStorage

    Note over UI,Storage: Initial Page Load
    UI->>API: GET /api/jobs?parent_id=root
    API->>Storage: ListJobs(opts)
    Storage-->>API: []*CrawlJob
    loop For each parent job
        API->>Storage: GetJobChildStats([parentIDs])
        Storage-->>API: map[parentID]*JobChildStats
        API->>Job: job.GetStatusReport(childStats)
        Job-->>API: *JobStatusReport
        Note over API: Enrich jobMap with status_report:<br/>progress_text, errors, warnings
    end
    API-->>UI: {jobs: [...], status_report: {...}}
    Note over UI: Display jobs using<br/>status_report.progress_text<br/>No client-side calculations

    Note over UI,Storage: Child Job Update via WebSocket
    WS->>UI: job_status_change (child job)
    Note over UI: Detect job has parent_id
    UI->>API: GET /api/jobs/{parent_id}
    API->>Storage: GetJob(parentID)
    Storage-->>API: *CrawlJob
    API->>Storage: GetJobChildStats([parentID])
    Storage-->>API: map[parentID]*JobChildStats
    API->>Job: parent.GetStatusReport(childStats)
    Job-->>API: *JobStatusReport (updated)
    API-->>UI: {job: {...}, status_report: {...}}
    Note over UI: Update parent job in allJobs<br/>Display updated progress_text<br/>Show errors/warnings if any
    UI->>UI: renderJobs()

    Note over UI,Storage: User Expands Parent Job
    UI->>API: GET /api/jobs?parent_id={parentID}
    API->>Storage: ListJobs(opts with parent_id filter)
    Storage-->>API: []*CrawlJob (children)
    API-->>UI: {jobs: [...]}
    Note over UI: Display children inline<br/>Show status, URL, depth

## Proposed File Changes

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\models\crawler_job.go
- internal\interfaces\jobtypes\jobtypes.go

**Update ListJobsHandler to include status_report in API responses:**

1. **After enriching jobs with child statistics** (after line 181, before appending to enrichedJobs):
   - Call `job.GetStatusReport(stats)` where stats is from `childStatsMap[masked.ID]`
   - Add the returned `JobStatusReport` to `jobMap["status_report"]`
   - This provides: `progress_text`, `errors`, `warnings`, `child_count`, `completed_children`, `failed_children`, `running_children`

2. **Update GetJobHandler similarly** (after line 332, before encoding response):
   - For parent jobs (empty parent_id), call `masked.GetStatusReport(stats)` and add to `jobMap["status_report"]`
   - For child jobs, call `masked.GetStatusReport(nil)` and add to response

3. **Import required package:**
   - Add import for `github.com/ternarybob/quaero/internal/interfaces/jobtypes` if not already present

**Design Note:** The status_report field will contain all necessary information for the UI to display job status without client-side calculations. The backend becomes the single source of truth for progress text and error/warning lists.

### internal\handlers\websocket_events.go(MODIFY)

References: 

- internal\handlers\websocket.go(MODIFY)

**Add status_report to WebSocket event payloads for job status changes:**

1. **Update handleJobCompleted** (around line 233-263):
   - After creating `JobStatusUpdate`, check if the event payload contains `status_report` data
   - If present, extract and add to the update struct (requires extending `JobStatusUpdate` in websocket.go)
   - Alternatively, include individual fields: `progress_text`, `errors`, `warnings`

2. **Update handleJobFailed** (around line 265-318):
   - Similar to handleJobCompleted, extract status_report fields from payload
   - Ensure `errors` array is populated from `status_report.errors` if available

3. **Update handleJobCancelled** (around line 320-346):
   - Add status_report extraction

4. **Update handleJobStarted** (around line 209-231):
   - Add status_report extraction for consistency

**Design Note:** This ensures WebSocket updates carry the same status_report information as REST API responses, allowing the UI to update without re-fetching from the API.

### internal\handlers\websocket.go(MODIFY)

References: 

- internal\interfaces\jobtypes\jobtypes.go

**Extend JobStatusUpdate struct to include status_report fields:**

1. **Add new fields to JobStatusUpdate struct** (after line 196):
   - `ProgressText string` - Human-readable progress from backend
   - `Errors []string` - List of error messages
   - `Warnings []string` - List of warning messages
   - `RunningChildren int` - Number of running child jobs (for parent status display)

2. **Update JSON tags:**
   - `progress_text` for ProgressText
   - `errors` for Errors (already exists)
   - `warnings` for Warnings
   - `running_children` for RunningChildren

**Design Note:** These fields mirror the `JobStatusReport` structure and allow the UI to receive complete status information via WebSocket without additional API calls.

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\models\crawler_job.go

**Update event publishing to include status_report data:**

1. **In EventJobCompleted publishing** (search for `EventJobCompleted` publish calls):
   - Before publishing, call `job.GetStatusReport(childStats)` where childStats is fetched if job is a parent
   - Add status_report fields to event payload: `progress_text`, `errors`, `warnings`, `running_children`

2. **In EventJobFailed publishing** (search for `EventJobFailed` publish calls):
   - Similar to completed, add status_report fields to payload
   - Ensure `errors` array includes job.Error if present

3. **In EventJobStarted publishing** (if exists):
   - Add status_report fields for consistency

4. **Fetch child statistics before publishing parent job events:**
   - Use `c.deps.JobStorage.GetJobChildStats(ctx, []string{job.ID})` to get stats
   - Pass stats to `GetStatusReport()`

**Design Note:** This ensures events published from the crawler job include complete status information that flows through to WebSocket clients.

### pages\queue.html(MODIFY)

**Simplify Alpine.js jobList component - remove client-side progress calculations:**

1. **Remove state variables** (lines 1365-1369):
   - Delete `showChildJobs` - no longer needed (always show parent jobs by default)
   - Keep `expandedParents` - still needed for expand/collapse
   - Delete `childJobsCache` - no longer needed
   - Delete `loadingParents` - simplify loading state
   - Keep `childJobsList` - but simplify to only track expanded state

2. **Remove client-side progress calculation methods** (lines 1640-1735):
   - Delete `getParentProgressText()` - use `job.status_report.progress_text` from backend
   - Delete `getParentProgressBarStyle()` - remove progress bars entirely
   - Delete `getParentProgressStyle()` - no longer needed

3. **Simplify renderJobs()** (lines 1607-1638):
   - Remove hierarchical vs flat view logic
   - Always render parent jobs with expand/collapse capability
   - Remove `showChildJobs` conditional
   - Simplify `itemsToRender` to just parent jobs from `filteredJobs.filter(job => !job.parent_id)`

4. **Update toggleShowChildJobs()** (lines 1599-1605):
   - Delete this method entirely - no longer needed

5. **Simplify loadJobs()** (lines 1488-1540):
   - Remove `showChildJobs` conditional (lines 1499-1501)
   - Always fetch parent jobs only: `params.append('parent_id', 'root')`
   - Remove `expandedParents.clear()` logic

6. **Update updateJobInList()** (lines 2018-2099):
   - When a child job updates, fetch the parent job's updated status_report
   - Use `job.status_report` fields instead of manually updating counters
   - Remove manual progress field updates (lines 2067-2087)
   - Add logic: if `update.job_id` is a child job, fetch parent and update its status_report

7. **Simplify handleChildJobStatus()** (lines 1426-1447):
   - Remove manual counter increments
   - Instead, trigger parent job refresh from API to get updated status_report

8. **Update handleChildSpawned()** (lines 1399-1424):
   - Keep basic child metadata tracking for display
   - Remove counter increments - rely on backend status_report

**Design Note:** The component becomes much simpler by delegating all status calculations to the backend. The UI only needs to display `job.status_report.progress_text` and `job.status_report.errors/warnings`.
**Update HTML template to use backend status_report and remove progress bars:**

1. **Remove progress bar rendering** (lines 278-288):
   - Delete the entire `<template x-if="item.type === 'parent'">` block that renders progress bars
   - Replace with simple text display: `<div x-text="item.job.status_report?.progress_text || 'No progress data'"></div>`

2. **Add error/warning display** (after line 276, after failure reason display):
   - Add new section for errors: `<template x-if="item.job.status_report?.errors?.length > 0">`
   - Render errors as a list with red styling (similar to existing error alert)
   - Add new section for warnings: `<template x-if="item.job.status_report?.warnings?.length > 0">`
   - Render warnings as a list with yellow/warning styling

3. **Update child jobs list display** (lines 290-323):
   - Keep the inline child jobs list (spawned URLs)
   - Simplify to use `childJobsList.get(item.job.id)` without complex caching
   - Display remains the same but data source is simplified

4. **Remove "Show All Jobs" toggle button** (lines 154-156):
   - Delete the toggle button that switches between parent-only and all-jobs view
   - Always show parent jobs by default

5. **Remove workflow/task job type badges** (lines 217-230):
   - Delete the "ORCHESTRATION" and "WORKFLOW" warning badges
   - Remove special styling for workflow jobs (line 187)
   - Make UI job-agnostic

6. **Update status badge logic** (lines 236-240):
   - Simplify to use `item.job.status` directly
   - Remove calls to `getStatusBadgeClass()` and `getStatusBadgeText()` which derive status from children
   - Use backend `status_report.status` if available

7. **Remove CSS for progress bars** (lines 1-53):
   - Delete `.parent-progress-bar` and related CSS classes
   - Keep basic card styling

**Design Note:** The template becomes much simpler by directly displaying backend-provided status_report fields. No client-side calculations or complex conditional rendering needed.
**Remove workflow vs task job filtering and special handling:**

1. **Remove jobType filter from defaultFilters** (lines 553-558):
   - Delete `jobType: new Set(['workflow', 'task'])` from defaultFilters
   - Remove jobType from activeFilters initialization

2. **Remove jobType filter UI** (lines 843-851 in filter chips, and filter modal):
   - Delete jobType filter checkboxes from filter modal (search for `filter-job-type`)
   - Remove jobType chip rendering
   - Remove jobType from filter persistence (lines 613-625)

3. **Remove isValidSourceType checks for filtering** (lines 698-708):
   - Delete the workflow vs task job type discrimination logic in `matchesActiveFilters()`
   - Remove `isWorkflowJob` and `isTaskJob` variables
   - Simplify to only check status, source, and entity filters

4. **Remove getSourceTypeDisplay special handling** (lines 673-688):
   - Simplify to just return the source type display name
   - Remove "Unknown Source" warning for invalid source types

5. **Remove isValidSourceType function** (lines 690-694):
   - Delete this function entirely
   - Remove all calls to it in the template

6. **Update deriveParentStatus()** (lines 1775-1802):
   - Simplify or remove this method entirely
   - Use backend `status_report.status` instead of deriving from children

7. **Simplify getStatusBadgeClass() and getStatusBadgeText()** (lines 1804-1872):
   - Remove parent vs child distinction logic
   - Use simple status-to-class mapping
   - Remove "Orchestrating" vs "Processing" text differences

**Design Note:** Making the UI job-agnostic means treating all jobs uniformly regardless of source type. The backend already provides the necessary status information via status_report.
**Update WebSocket event handlers to refresh parent jobs when children update:**

1. **Update job_status_change handler** (lines 902-911):
   - After calling `updateJobInList(update)`, check if the updated job is a child (has parent_id)
   - If yes, fetch the parent job from API: `GET /api/jobs/{parent_id}`
   - Update the parent job in `allJobs` array with the fresh status_report
   - This ensures parent status reflects child changes immediately

2. **Add debouncing for parent refreshes:**
   - Use `parentRefreshTimeouts` map (already exists, line 1379) to debounce parent refreshes
   - Only refresh parent once per second even if multiple children update
   - Clear timeout and set new one: `clearTimeout(parentRefreshTimeouts.get(parentId)); parentRefreshTimeouts.set(parentId, setTimeout(() => fetchParent(parentId), 1000))`

3. **Update job_spawn handler** (lines 930-944):
   - After handling child spawn, trigger parent refresh
   - Use same debouncing logic

4. **Create helper function fetchParentJob():**
   - Add new method to Alpine component: `async fetchParentJob(parentId)`
   - Fetch from `/api/jobs/${parentId}`
   - Update parent job in `allJobs` array
   - Call `renderJobs()` to update UI

**Design Note:** This ensures the UI always shows current parent status by fetching updated status_report from the backend when children change. Debouncing prevents excessive API calls when many children update simultaneously.
**Simplify child job expansion/collapse logic:**

1. **Update toggleParentExpansion()** (lines 1580-1597):
   - Keep basic expand/collapse logic
   - Simplify child loading: just call `loadChildJobs(parentId)` without complex loading state
   - Remove `loadingParents` set management
   - Keep `expandedParents` set for tracking expansion state

2. **Simplify loadChildJobs()** (lines 1542-1578):
   - Keep the API call to fetch children
   - Simplify metadata extraction (lines 1560-1568)
   - Remove `childJobsCache` - only keep `childJobsList` with minimal metadata
   - Return children for inline display

3. **Update child job display in template:**
   - Keep inline child list rendering (lines 290-323)
   - Use `childJobsList.get(item.job.id)` directly
   - Display child job status, URL, depth as before
   - No changes to visual presentation, just simplified data flow

4. **Remove loading indicator complexity** (lines 1616-1627):
   - Simplify or remove loading indicators
   - Use simple "Loading..." text if needed
   - Remove `loadingParents` set checks

**Design Note:** Child job expansion remains functional but with simpler state management. The UI only needs to track which parents are expanded and fetch children on demand.