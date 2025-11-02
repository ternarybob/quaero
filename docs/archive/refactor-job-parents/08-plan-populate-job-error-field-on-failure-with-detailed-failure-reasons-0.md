I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**What Already Exists:**
1. **Error Field**: `CrawlJob.Error` field exists (line 51 in `crawler_job.go`) and is persisted by `JobStorage.SaveJob()` (line 160 in `job_storage.go`)
2. **Failure Detection**: The code already detects failures in multiple places:
   - Validation errors (lines 49-52 in `crawler.go`)
   - Scraping failures (lines 231-285)
   - HTTP errors (lines 287-349)
   - Storage errors (lines 399-402)
3. **Event Publishing**: `EventJobFailed` events are published with error messages in payloads
4. **Existing Pattern**: `crawler/service.go` has a `FailJob()` method (lines 737-789) that shows the correct pattern: set `job.Error`, update status, save job, publish event

**What's Missing:**
1. The `job.Error` field is **never populated** in `CrawlerJob.Execute()` when failures occur
2. Child job failures don't update the parent job's Error field
3. `ExecuteCompletionProbe()` doesn't detect or mark stale jobs as failed
4. Error messages in events are verbose (full Go error strings) rather than concise/actionable

**Key Insights:**
- The infrastructure is already in place (Error field, SaveJob, events)
- We just need to populate `job.Error` at each failure point
- The pattern from `crawler/service.go:FailJob()` should be followed
- Error messages should be concise: "HTTP 404: Not Found", "Scraping timeout after 30s", "Storage error: database locked"

### Approach

## Implementation Strategy

**Phase 1: Add Error Helper Function**
Create a helper function `formatJobError()` in `crawler.go` to generate concise, actionable error messages from Go errors. This ensures consistency across all failure points.

**Phase 2: Populate Error in Execute() Failure Paths**
Update each failure scenario in `CrawlerJob.Execute()` to:
1. Load parent job from storage
2. Set `job.Error` with concise message
3. Update `job.Status` to `JobStatusFailed` (for critical failures)
4. Save job back to storage

**Phase 3: Update Completion Probe for Stale Jobs**
Enhance `ExecuteCompletionProbe()` to detect stale jobs (heartbeat older than threshold) and mark them as failed with a timeout error message.

**Phase 4: Ensure Error Persistence**
Verify that all `SaveJob()` calls properly persist the Error field (already implemented, just needs verification).

**Key Design Decisions:**
- **Child failures**: Update parent job's Error field with the first/most recent child error
- **Multiple failures**: Store only the most recent error (single string field)
- **Error format**: "Category: Brief description" (e.g., "HTTP 404: Not Found", "Timeout: No activity for 10m")
- **Status transitions**: Only set status to `failed` for critical errors; non-critical errors (like individual URL failures) just populate Error without changing status

### Reasoning

I explored the codebase by:
1. Reading the three main files mentioned by the user (`crawler.go`, `crawler_job.go`, `job_storage.go`)
2. Searching for existing uses of `job.Error` to understand current patterns
3. Examining `crawler/service.go:FailJob()` to see the correct error handling pattern
4. Reviewing other job types (`summarizer.go`, `cleanup.go`) to understand error handling conventions
5. Checking the storage interface to confirm SaveJob() persists the Error field
6. Analyzing the completion probe logic to understand stale job detection

## Mermaid Diagram

sequenceDiagram
    participant Worker as Worker Pool
    participant CrawlerJob as CrawlerJob.Execute()
    participant Helper as formatJobError()
    participant JobStorage as JobStorage
    participant EventSvc as EventService
    participant UI as Queue UI

    Note over Worker,UI: Failure Scenario 1: Validation Error

    Worker->>CrawlerJob: Execute(msg)
    CrawlerJob->>CrawlerJob: Validate(msg)
    CrawlerJob-->>CrawlerJob: ❌ Validation failed
    CrawlerJob->>JobStorage: GetJob(parentID)
    JobStorage-->>CrawlerJob: job
    CrawlerJob->>Helper: formatJobError("Validation", err)
    Helper-->>CrawlerJob: "Validation: URL is required"
    CrawlerJob->>CrawlerJob: job.Error = "Validation: URL is required"
    CrawlerJob->>CrawlerJob: job.Status = "failed"
    CrawlerJob->>JobStorage: SaveJob(job)
    CrawlerJob->>EventSvc: Publish(EventJobFailed)
    EventSvc->>UI: WebSocket: Job failed
    UI->>UI: Display error in job card

    Note over Worker,UI: Failure Scenario 2: HTTP Error

    Worker->>CrawlerJob: Execute(msg)
    CrawlerJob->>CrawlerJob: scraper.ScrapeURL(url)
    CrawlerJob-->>CrawlerJob: ❌ HTTP 404
    CrawlerJob->>JobStorage: GetJob(parentID)
    JobStorage-->>CrawlerJob: job
    CrawlerJob->>CrawlerJob: job.Error = "HTTP 404: Not Found"
    CrawlerJob->>CrawlerJob: Update progress counters
    CrawlerJob->>JobStorage: SaveJob(job)
    CrawlerJob->>EventSvc: Publish(EventJobFailed)
    EventSvc->>UI: WebSocket: Child job failed
    UI->>UI: Display error in job card

    Note over Worker,UI: Failure Scenario 3: Stale Job Timeout

    Worker->>CrawlerJob: ExecuteCompletionProbe(msg)
    CrawlerJob->>JobStorage: GetJob(parentID)
    JobStorage-->>CrawlerJob: job
    CrawlerJob->>CrawlerJob: Check heartbeat age
    CrawlerJob-->>CrawlerJob: ❌ Idle for 10m (stale)
    CrawlerJob->>CrawlerJob: job.Error = "Timeout: No activity for 10m15s"
    CrawlerJob->>CrawlerJob: job.Status = "failed"
    CrawlerJob->>JobStorage: SaveJob(job)
    CrawlerJob->>EventSvc: Publish(EventJobFailed)
    EventSvc->>UI: WebSocket: Job failed
    UI->>UI: Display timeout error

## Proposed File Changes

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\services\crawler\service.go
- internal\models\crawler_job.go(MODIFY)
- internal\storage\sqlite\job_storage.go(MODIFY)
- internal\services\scheduler\scheduler_service.go

**Add formatJobError() helper function (after line 38, before Execute())**

Create a new helper function to format concise, actionable error messages:
- Function signature: `func formatJobError(category string, err error) string`
- Extract the root cause from wrapped errors (unwrap Go error chains)
- Format as: `"Category: Brief description"`
- Handle common error types:
  - HTTP errors: Extract status code → "HTTP 404: Not Found"
  - Timeout errors: "Timeout: Request exceeded 30s"
  - Network errors: "Network: Connection refused"
  - Storage errors: "Storage: Database locked"
  - Generic errors: "Category: error.Error()" (truncate to 200 chars)
- Return concise, user-friendly messages suitable for UI display

**Update Execute() - Validation failure (lines 49-52)**

When validation fails:
- Load parent job: `jobInterface, err := c.deps.JobStorage.GetJob(ctx, msg.ParentID)`
- Type assert to `*models.CrawlJob`
- Set error: `job.Error = formatJobError("Validation", err)`
- Set status: `job.Status = models.JobStatusFailed`
- Set completion time: `job.CompletedAt = time.Now()`
- Save job: `c.deps.JobStorage.SaveJob(ctx, job)`
- Log the error update
- Return the original error (don't swallow it)

**Update Execute() - Scraping failure (lines 231-285)**

When `scraper.ScrapeURL()` fails:
- After publishing EventJobFailed (line 256), load parent job
- Set error: `job.Error = formatJobError("Scraping", err)`
- Check if this is a critical failure (all URLs failed) vs. single URL failure
- For single URL failures: Just update Error, don't change status (job continues)
- For critical failures: Set status to `JobStatusFailed`
- Save job with updated Error field
- The existing progress update code (lines 264-282) already handles counters

**Update Execute() - HTTP error (lines 287-349)**

When `!scrapeResult.Success` (non-2xx status):
- After publishing EventJobFailed (line 345), load parent job
- Set error: `job.Error = fmt.Sprintf("HTTP %d: %s", scrapeResult.StatusCode, scrapeResult.Error)`
- Don't change status (individual URL failures don't fail the entire job)
- Save job with updated Error field
- The existing progress update code (lines 307-325) already handles counters

**Update Execute() - Storage failure (lines 399-402)**

When `DocumentStorage.SaveDocument()` fails:
- Load parent job from storage
- Set error: `job.Error = formatJobError("Storage", err)`
- Don't change status (storage errors might be transient)
- Save job with updated Error field
- Log the error update
- Return the original error

**Reference files:**
- `internal/services/crawler/service.go` (lines 737-789) - Shows the correct pattern for FailJob()
- `internal/models/crawler_job.go` (line 51) - Error field definition
- `internal/storage/sqlite/job_storage.go` (line 160) - SaveJob() persists Error field
**Update ExecuteCompletionProbe() - Detect stale jobs (lines 727-843)**

Add stale job detection logic before marking job as completed:

**After line 788 (after checking heartbeat age), add stale job detection:**

If the job has been idle for too long (e.g., heartbeat older than 10 minutes) AND still has pending URLs:
- This indicates a stuck/stale job (workers crashed, queue issues, etc.)
- Calculate idle duration: `idleDuration := time.Since(job.LastHeartbeat)`
- Define stale threshold: `const staleThreshold = 10 * time.Minute`
- If `idleDuration > staleThreshold && job.Progress.PendingURLs > 0`:
  - Set error: `job.Error = fmt.Sprintf("Timeout: No activity for %s (pending: %d URLs)", idleDuration.Round(time.Second), job.Progress.PendingURLs)`
  - Set status: `job.Status = models.JobStatusFailed`
  - Set completion time: `job.CompletedAt = time.Now()`
  - Sync result counts: `job.ResultCount = job.Progress.CompletedURLs`, `job.FailedCount = job.Progress.FailedURLs`
  - Save job: `c.deps.JobStorage.SaveJob(ctx, job)`
  - Publish EventJobFailed with timeout error
  - Log the stale job detection
  - Return nil (job marked as failed successfully)

**Update existing completion logic (lines 790-843):**

The existing completion logic should only run if the job is NOT stale:
- Wrap the existing completion code in a check: `if job.Status != models.JobStatusFailed`
- This ensures stale jobs are marked as failed before attempting normal completion

**Add logging for stale job detection:**

Log when a job is detected as stale:
- `c.logger.Warn().Str("parent_id", msg.ParentID).Dur("idle_duration", idleDuration).Int("pending_urls", job.Progress.PendingURLs).Msg("Job marked as failed due to inactivity")`

**Reference:**
- The stale threshold should match the scheduler's stale job detection threshold (currently 10 minutes in `scheduler_service.go`)
- The error message format should be concise and actionable: "Timeout: No activity for 10m15s (pending: 5 URLs)"

### internal\models\crawler_job.go(MODIFY)

**Verify Error field serialization (line 51)**

No changes needed - just verify that:
- The `Error` field has the correct JSON tag: `json:"error,omitempty"`
- The field is exported (capitalized) so it's serialized
- The `omitempty` tag ensures empty errors aren't included in JSON responses

**Add documentation comment (above line 51):**

Add a comment explaining the Error field usage:
```
// Error contains a concise, user-friendly description of why the job failed.
// Format: "Category: Brief description" (e.g., "HTTP 404: Not Found", "Timeout: No activity for 10m").
// Only populated when job status is 'failed' or when individual operations fail.
// This field is displayed in the UI and should be actionable for users.
```

This documentation helps future developers understand the expected format and usage.

### internal\storage\sqlite\job_storage.go(MODIFY)

**Verify Error field persistence (lines 54-199)**

No changes needed - just verify that:
- Line 160 includes `crawlJob.Error` in the INSERT statement parameters
- Line 135 includes `error = excluded.error` in the UPDATE clause
- The Error field is properly read from the database in `scanJob()` (lines 662, 757)

**Add logging for error persistence (around line 197):**

When saving a job with an error, log it for debugging:
- After line 197 (`s.logger.Debug().Str("job_id", crawlJob.ID)...`)
- Add conditional logging: `if crawlJob.Error != "" { s.logger.Info().Str("job_id", crawlJob.ID).Str("error", crawlJob.Error).Msg("Job saved with error") }`

This helps track when errors are persisted and aids in debugging failure scenarios.

**Reference:**
- The SaveJob() method already handles the Error field correctly (line 160)
- The scanJob() methods already read the Error field from the database (lines 662, 757)