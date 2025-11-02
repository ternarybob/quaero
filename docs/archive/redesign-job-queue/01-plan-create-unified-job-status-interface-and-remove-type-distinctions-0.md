I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase currently has inconsistent job status reporting:

1. **Handler enrichment** (`internal/handlers/job_handler.go` lines 172-174) adds `is_workflow` and `is_task` flags based on `source_type`, but these flags are not used anywhere in the UI or backend
2. **Child statistics** are fetched separately via `JobManager.GetJobChildStats()` and manually enriched into job responses as `child_count`, `completed_children`, `failed_children`
3. **UI calculates progress** client-side using these enriched fields (e.g., `getParentProgressText()` in `queue.html`)
4. **No standardized interface** exists for job status reporting - each component calculates status differently

The `JobChildStats` struct already exists in `internal/interfaces/storage.go` with the necessary fields. The storage layer (`JobStorage.GetJobChildStats()`) efficiently fetches child statistics in batch. The handler already enriches jobs with these stats, so no storage layer changes are needed.

The task is to create a standardized interface and move status calculation logic into the model layer, preparing for the UI redesign in subsequent phases.

### Approach

Create a unified `JobStatusReport` interface to standardize job status reporting across all job types. Implement `GetStatusReport()` method on `CrawlJob` model to encapsulate status calculation logic. Remove deprecated `is_workflow` and `is_task` enrichment flags from the handler. This establishes a consistent foundation for job status reporting that will be used by subsequent phases for UI redesign and error tolerance implementation.

### Reasoning

I explored the repository structure, read the relevant files mentioned by the user (`internal/interfaces/jobs.go`, `internal/models/crawler_job.go`, `internal/handlers/job_handler.go`, `internal/storage/sqlite/job_storage.go`), examined the `JobManager` interface and implementation, reviewed the `JobChildStats` structure, and analyzed the UI implementation in `pages/queue.html` to understand how job status is currently displayed and calculated.

## Mermaid Diagram

sequenceDiagram
    participant Handler as JobHandler
    participant Manager as JobManager
    participant Storage as JobStorage
    participant Model as CrawlJob
    participant UI as Queue UI

    Note over Handler,UI: Current Flow (Before Changes)
    UI->>Handler: GET /api/jobs
    Handler->>Manager: ListJobs(opts)
    Manager->>Storage: ListJobs(opts)
    Storage-->>Manager: []*CrawlJob
    Manager-->>Handler: []*CrawlJob
    Handler->>Manager: GetJobChildStats(parentIDs)
    Manager->>Storage: GetJobChildStats(parentIDs)
    Storage-->>Manager: map[string]*JobChildStats
    Manager-->>Handler: map[string]*JobChildStats
    Note over Handler: Enrich jobs with:<br/>- child_count<br/>- completed_children<br/>- failed_children<br/>- is_workflow ❌<br/>- is_task ❌
    Handler-->>UI: Enriched jobs JSON

    Note over Handler,UI: New Flow (After Changes)
    UI->>Handler: GET /api/jobs
    Handler->>Manager: ListJobs(opts)
    Manager->>Storage: ListJobs(opts)
    Storage-->>Manager: []*CrawlJob
    Manager-->>Handler: []*CrawlJob
    Handler->>Manager: GetJobChildStats(parentIDs)
    Manager->>Storage: GetJobChildStats(parentIDs)
    Storage-->>Manager: map[string]*JobChildStats
    Manager-->>Handler: map[string]*JobChildStats
    Note over Handler: Enrich jobs with:<br/>- child_count<br/>- completed_children<br/>- failed_children<br/>(is_workflow removed ✓)<br/>(is_task removed ✓)
    Handler->>Model: job.GetStatusReport(childStats)
    Model-->>Handler: *JobStatusReport
    Note over Handler: Future: Use JobStatusReport<br/>for consistent status display
    Handler-->>UI: Enriched jobs JSON

## Proposed File Changes

### internal\interfaces\jobs.go(MODIFY)

Add a new `JobStatusReport` struct (not an interface) to standardize job status reporting across all job types. Include the following fields:

- `Status` (string) - Current job status (pending, running, completed, failed, cancelled)
- `ChildCount` (int) - Total number of child jobs spawned
- `CompletedChildren` (int) - Number of completed child jobs
- `FailedChildren` (int) - Number of failed child jobs
- `RunningChildren` (int) - Number of running child jobs (calculated as ChildCount - CompletedChildren - FailedChildren)
- `ProgressText` (string) - Human-readable progress description (e.g., "44 URLs (11 completed, 2 failed, 31 running)")
- `Errors` ([]string) - List of error messages from the job (extracted from job.Error field if present)
- `Warnings` ([]string) - List of warning messages (reserved for future use, initially empty)

Place this struct after the existing `JobListOptions` struct definition. This struct will be returned by the `GetStatusReport()` method on `CrawlJob` and can be used by handlers to provide consistent status information to the UI.

**Design Note:** Using a struct instead of an interface allows for direct instantiation and JSON serialization without type assertions. The struct can be embedded in API responses or used standalone.

### internal\models\crawler_job.go(MODIFY)

References: 

- internal\interfaces\jobs.go(MODIFY)

Add a `GetStatusReport()` method to the `CrawlJob` struct that returns a `*interfaces.JobStatusReport`. This method should:

1. **Calculate child job statistics:**
   - Accept `childStats *interfaces.JobChildStats` as a parameter (can be nil for jobs without children)
   - If `childStats` is nil, set all child counts to 0
   - Extract `ChildCount`, `CompletedChildren`, `FailedChildren` from the provided stats
   - Calculate `RunningChildren` as `ChildCount - CompletedChildren - FailedChildren` (ensure non-negative)

2. **Generate progress text:**
   - For parent jobs (ParentID is empty) with children:
     - If `ChildCount == 0`: return "No child jobs spawned yet"
     - Otherwise: format as "Completed: X | Failed: Y | Running: Z | Total: N" where X, Y, Z, N are the respective counts
   - For child jobs (ParentID is not empty) or jobs without children:
     - Use the job's own progress: "X URLs (Y completed, Z failed, W running)" where values come from `job.Progress.CompletedURLs`, `job.Progress.FailedURLs`, `job.Progress.PendingURLs`
     - If progress is not available, return "Status: {job.Status}"

3. **Extract errors:**
   - If `job.Error` is not empty, add it to the `Errors` slice
   - Otherwise, return an empty slice

4. **Set warnings:**
   - Return an empty slice (reserved for future use)

5. **Return the populated JobStatusReport struct**

Add the import for `github.com/ternarybob/quaero/internal/interfaces` at the top of the file.

**Method Signature:** `func (j *CrawlJob) GetStatusReport(childStats *interfaces.JobChildStats) *interfaces.JobStatusReport`

**Design Note:** Accepting `childStats` as a parameter avoids coupling the model to the storage layer. The handler will fetch child stats and pass them to this method.

### internal\handlers\job_handler.go(MODIFY)

Remove the `is_workflow` and `is_task` enrichment logic from the `ListJobsHandler` method (lines 172-174). These lines add job type flags based on `source_type`, but they are not used anywhere in the codebase and introduce unnecessary complexity.

**Specific changes:**
1. Delete lines 172-174 which add `is_workflow` and `is_task` to the `jobMap`
2. Keep the existing child statistics enrichment (lines 177-185) as it is still needed by the UI
3. No other changes to the handler logic are required

**Rationale:** The `is_workflow` and `is_task` flags were an attempt to distinguish between different job types, but the user has clarified that "a job is a job" and all jobs should be treated uniformly. The job type information is already available in the `job.JobType` field (parent, pre_validation, crawler_url, post_summary), so these derived flags are redundant.

**Note:** The `GetJobHandler` method does not have this enrichment logic, so no changes are needed there. The child statistics enrichment in both handlers should remain unchanged as it provides necessary data for the UI (this will be refactored in subsequent phases to use the new `GetStatusReport()` method).

### internal\storage\sqlite\job_storage.go(MODIFY)

References: 

- internal\interfaces\storage.go

**No changes required to this file.** The storage layer already provides consistent status reporting through the `GetJobChildStats()` method (lines 386-424), which returns a map of parent job IDs to `JobChildStats` structs. This method efficiently fetches child statistics in a single batch query and is already used by the handler.

The `ListJobs()` method returns `CrawlJob` structs with all necessary fields populated, including `Status`, `Progress`, `Error`, `ParentID`, etc. The new `GetStatusReport()` method on `CrawlJob` will use these fields to generate standardized status reports.

**Verification:** Confirm that `GetJobChildStats()` correctly aggregates child job counts by status (completed, failed) and that the returned `JobChildStats` struct matches the interface definition in `internal/interfaces/storage.go` (lines 69-74).

### internal\models\crawler_job_test.go(NEW)

References: 

- internal\models\crawler_job.go(MODIFY)
- internal\interfaces\jobs.go(MODIFY)

Create a new test file for the `GetStatusReport()` method with comprehensive test coverage. Use table-driven tests to cover the following scenarios:

**Test Cases:**

1. **Parent job with no children:**
   - Input: `CrawlJob` with `ParentID = ""`, `childStats = nil`
   - Expected: `ProgressText = "No child jobs spawned yet"`, all child counts = 0

2. **Parent job with children (all completed):**
   - Input: `CrawlJob` with `ParentID = ""`, `childStats = &JobChildStats{ChildCount: 10, CompletedChildren: 10, FailedChildren: 0}`
   - Expected: `ProgressText = "Completed: 10 | Failed: 0 | Running: 0 | Total: 10"`, `RunningChildren = 0`

3. **Parent job with children (mixed status):**
   - Input: `CrawlJob` with `ParentID = ""`, `childStats = &JobChildStats{ChildCount: 44, CompletedChildren: 11, FailedChildren: 2}`
   - Expected: `ProgressText = "Completed: 11 | Failed: 2 | Running: 31 | Total: 44"`, `RunningChildren = 31`

4. **Parent job with children (all failed):**
   - Input: `CrawlJob` with `ParentID = ""`, `childStats = &JobChildStats{ChildCount: 5, CompletedChildren: 0, FailedChildren: 5}`
   - Expected: `ProgressText = "Completed: 0 | Failed: 5 | Running: 0 | Total: 5"`, `RunningChildren = 0`

5. **Child job with progress:**
   - Input: `CrawlJob` with `ParentID = "parent-123"`, `Progress.CompletedURLs = 15`, `Progress.FailedURLs = 3`, `Progress.PendingURLs = 7`, `childStats = nil`
   - Expected: `ProgressText = "15 URLs (15 completed, 3 failed, 7 running)"` (format may vary based on implementation)

6. **Child job without progress:**
   - Input: `CrawlJob` with `ParentID = "parent-123"`, `Progress = CrawlProgress{}` (empty), `Status = "running"`, `childStats = nil`
   - Expected: `ProgressText = "Status: running"`

7. **Job with error:**
   - Input: `CrawlJob` with `Error = "HTTP 404: Not Found"`
   - Expected: `Errors = []string{"HTTP 404: Not Found"}`

8. **Job without error:**
   - Input: `CrawlJob` with `Error = ""`
   - Expected: `Errors = []string{}` (empty slice)

**Test Structure:**
- Use `testing.T` and table-driven tests with a slice of test cases
- Each test case should have: name, input `CrawlJob`, input `childStats`, expected `JobStatusReport`
- Assert all fields of the returned `JobStatusReport` match expected values
- Use `t.Run()` for subtests to improve test output readability

**Imports:** `testing`, `github.com/ternarybob/quaero/internal/interfaces`, `github.com/ternarybob/quaero/internal/models`