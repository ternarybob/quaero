I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Problem Analysis

**Issue 1: Inaccurate Document Count**
- `ResultCount` and `FailedCount` fields in `CrawlJob` model are never updated during job execution
- These fields remain at their initial value (0) throughout the job lifecycle
- The storage layer validates the mismatch (lines 164-187 in `job_storage.go`) but doesn't fix it
- UI displays `Progress.CompletedURLs` correctly, but internal systems rely on `ResultCount` for validation and transformer logic

**Issue 2: Root Cause**
- In `crawler.go` (lines 445-520): Progress counters are updated but `ResultCount`/`FailedCount` are not synced
- In `service.go` (`CancelJob` and `FailJob` methods): Terminal status is set but counts are not synced
- The fields exist for historical tracking but are never populated

**Impact**
- Validation warnings in logs (storage layer detects mismatch)
- Transformer logic may skip processing (checks `ResultCount > 0`)
- Inconsistent job statistics for reporting/analytics
- User confusion when viewing job details

**Solution Approach**
Sync `ResultCount` and `FailedCount` with their corresponding Progress counters at job completion/termination points. This is a simple field assignment that maintains data consistency without changing business logic.

### Approach

**Three-Point Synchronization Strategy**

1. **Primary Sync Point** (`crawler.go`): Update counts when marking job complete during URL processing
2. **Cancellation Sync** (`service.go`): Update counts when user cancels a running job
3. **Failure Sync** (`service.go`): Update counts when system marks job as failed

This ensures counts are accurate regardless of how the job terminates (normal completion, cancellation, or failure).

### Reasoning

I explored the repository structure, read the four key files mentioned by the user (`crawler.go`, `service.go`, `job_storage.go`, `crawler_job.go`), searched for all references to `ResultCount` and `FailedCount` across the codebase, examined the UI display logic in `job.html`, and analyzed the transformer logic that depends on these fields. This revealed that the fields are validated but never updated, and identified three specific locations where synchronization is needed.

## Mermaid Diagram

sequenceDiagram
    participant Worker as CrawlerJob Worker
    participant Job as CrawlJob Model
    participant Storage as JobStorage
    participant UI as Job Detail UI

    Note over Worker,UI: Current Behavior (Broken)
    Worker->>Job: Process URL
    Worker->>Job: Update Progress.CompletedURLs++
    Worker->>Job: Check PendingURLs == 0
    Worker->>Job: Set Status = Completed
    Note over Job: ResultCount = 0 (never updated!)
    Worker->>Storage: SaveJob()
    Storage->>Storage: Validate: ResultCount != CompletedURLs
    Storage-->>Storage: ⚠️ Log Warning (mismatch detected)
    UI->>Storage: GET /api/jobs/{id}
    Storage-->>UI: ResultCount=0, Progress.CompletedURLs=341
    UI->>UI: Display Progress.CompletedURLs (correct)
    Note over UI: Shows 341 documents (from Progress)

    Note over Worker,UI: Fixed Behavior
    Worker->>Job: Process URL
    Worker->>Job: Update Progress.CompletedURLs++
    Worker->>Job: Check PendingURLs == 0
    Worker->>Job: Sync: ResultCount = Progress.CompletedURLs
    Worker->>Job: Sync: FailedCount = Progress.FailedURLs
    Worker->>Job: Set Status = Completed
    Worker->>Storage: SaveJob()
    Storage->>Storage: Validate: ResultCount == CompletedURLs
    Storage-->>Storage: ✅ No Warning (counts match)
    UI->>Storage: GET /api/jobs/{id}
    Storage-->>UI: ResultCount=341, Progress.CompletedURLs=341
    UI->>UI: Display Progress.CompletedURLs (correct)
    Note over UI: Shows 341 documents (consistent)

## Proposed File Changes

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\models\crawler_job.go(MODIFY)
- internal\storage\sqlite\job_storage.go(MODIFY)

**Location: Lines 472-491 (Job Completion Logic)**

Add field synchronization immediately before marking job as complete:

1. After line 474 (`isComplete := job.Progress.PendingURLs == 0 && job.Progress.TotalURLs > 0`), add synchronization logic
2. Before line 477 (`job.Status = crawler.JobStatusCompleted`), sync the counts:
   - Set `job.ResultCount = job.Progress.CompletedURLs`
   - Set `job.FailedCount = job.Progress.FailedURLs`
3. This ensures counts are accurate when the job transitions to completed status

**Rationale**: This is the primary completion path where all URLs have been processed. Syncing here ensures the final counts reflect actual processing results.

**Note**: The sync must happen BEFORE setting status to completed, so the SaveJob call (line 494) persists both the status change and the count updates atomically.

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\models\crawler_job.go(MODIFY)
- internal\storage\sqlite\job_storage.go(MODIFY)

**Location 1: Lines 588-637 (CancelJob Method)**

Add field synchronization when cancelling a job:

1. After line 605 (`job.Status = JobStatusCancelled`), add synchronization:
   - Set `job.ResultCount = job.Progress.CompletedURLs`
   - Set `job.FailedCount = job.Progress.FailedURLs`
2. This ensures cancelled jobs have accurate counts reflecting work completed before cancellation
3. The subsequent SaveJob call (line 611) will persist these values

**Location 2: Lines 639-687 (FailJob Method)**

Add field synchronization when marking job as failed:

1. After line 655 (`job.Error = reason`), add synchronization:
   - Set `job.ResultCount = job.Progress.CompletedURLs`
   - Set `job.FailedCount = job.Progress.FailedURLs`
2. This ensures failed jobs have accurate counts reflecting work completed before failure
3. The subsequent SaveJob call (line 660) will persist these values

**Rationale**: Jobs can terminate via cancellation (user action) or failure (system detection of stale jobs). Both paths need count synchronization to maintain consistency.

**Note**: The `RerunJob` method (lines 745-829) already correctly initializes counts to 0 for new jobs, so no changes needed there.

### internal\storage\sqlite\job_storage.go(MODIFY)

References: 

- internal\models\crawler_job.go(MODIFY)

**Location: Lines 163-187 (SaveJob Validation Logic)**

**Optional Enhancement** (not required for fix, but improves observability):

1. Keep the existing validation warnings (lines 168-186) as they provide useful debugging information
2. Consider upgrading the log level from `Warn` to `Info` after the fix is deployed, since mismatches should no longer occur
3. Alternatively, add a comment explaining that these warnings indicate a bug if they appear after the fix

**Rationale**: The validation logic serves as a canary to detect if the synchronization is working correctly. Keeping it helps catch regressions.

**No Code Changes Required**: The validation is already correct and will automatically stop warning once the sync logic is in place.

### internal\models\crawler_job.go(MODIFY)

**Location: Lines 47-48 (Field Documentation)**

**Optional Enhancement** (improves code maintainability):

Add documentation comments to clarify the relationship between these fields and Progress counters:

1. Above line 47 (`ResultCount int`), add comment:
   - Explain that this field is a snapshot of `Progress.CompletedURLs` at job completion
   - Note that it's synced when job reaches terminal status (completed/failed/cancelled)
   - Mention it's used for historical tracking and validation

2. Above line 48 (`FailedCount int`), add similar comment:
   - Explain that this field is a snapshot of `Progress.FailedURLs` at job completion
   - Note the same synchronization behavior

**Rationale**: Clear documentation prevents future developers from making the same mistake of forgetting to sync these fields.

**No Functional Changes**: This is purely documentation to improve code clarity.