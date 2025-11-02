I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Problem Analysis

**Root Cause:**
The job completion check in `crawler.go` (line 474) happens immediately after updating progress counters for a single URL. This creates a race condition where:
1. Worker A processes URL #20 and updates: `CompletedURLs++`, `PendingURLs--`
2. Worker A checks: `PendingURLs == 0` → TRUE (temporarily)
3. Worker A marks job as `completed` and saves to database
4. **Meanwhile:** Worker B is still discovering child URLs from URL #15
5. Worker B enqueues 50 new child URLs → `PendingURLs = 50`
6. **Result:** Job marked complete but still has 50 pending URLs in queue

**Key Findings:**

1. **Heartbeat Infrastructure Exists:**
   - `last_heartbeat` column in `crawl_jobs` table (schema.go:124)
   - `UpdateJobHeartbeat(ctx, jobID)` method in JobStorage (job_storage.go:379)
   - Currently only used at job start (service.go:509), not during URL processing

2. **Queue Statistics Available:**
   - `GetQueueStats()` returns global queue stats (pending, in-flight, total)
   - goqite stores messages in `body` blob containing serialized `JobMessage` with `ParentID`
   - **Limitation:** No built-in per-job filtering - would need custom SQL query

3. **Current Completion Logic:**
   - Single-phase check: `PendingURLs == 0 && TotalURLs > 0` → immediate completion
   - No grace period or delayed verification
   - No consideration for concurrent child URL discovery

4. **Testing Infrastructure:**
   - Integration tests exist in `test/api/crawl_transform_test.go`
   - Tests poll job status until completion (lines 102-131)
   - Mock server available for testing (test/mock_server.go)

## Solution Strategy

**Two-Phase Completion Detection with Grace Period:**

1. **Phase 1: Completion Candidate** - When `PendingURLs == 0`, update heartbeat and wait
2. **Phase 2: Verification** - After grace period (5 seconds), verify:
   - `PendingURLs` still 0
   - Heartbeat hasn't been updated (no new URLs processed)
   - Queue has no pending messages for this job (optional validation)
3. **Only then:** Mark job as completed

**Why This Works:**
- Grace period allows in-flight child URL discovery to complete
- Heartbeat tracking detects if any worker processed a URL during grace period
- If heartbeat updated → reset grace period (more work happening)
- If heartbeat unchanged for 5 seconds → truly idle, safe to complete

**Trade-offs:**
- **Pros:** Simple, uses existing infrastructure, no new database fields needed
- **Cons:** 5-second delay before completion (acceptable for async jobs)
- **Alternative considered:** Per-job queue statistics - rejected due to complexity (requires custom SQL, JSON parsing of body column)

### Approach

**Three-Point Implementation:**

1. **Heartbeat Tracking** - Update heartbeat on every URL completion to track activity
2. **Delayed Completion Logic** - Add grace period and verification before marking complete
3. **Integration Test** - Verify completion only happens after all child URLs processed

This ensures jobs are marked complete only when truly idle (no activity for 5+ seconds).

### Reasoning

I explored the codebase by reading the four key files mentioned by the user (`crawler.go`, `worker.go`, `manager.go`, `job_storage.go`), then investigated the CrawlJob model structure, queue interfaces, and existing heartbeat mechanisms. I searched for completion detection patterns, examined the database schema to confirm `last_heartbeat` column exists, reviewed queue statistics APIs, and studied the test infrastructure. I also researched goqite's table structure to understand message storage and discovered that per-job queue filtering would require complex JSON parsing. This revealed that the existing heartbeat mechanism is the simplest solution for delayed completion detection.

## Mermaid Diagram

sequenceDiagram
    participant W1 as Worker 1
    participant W2 as Worker 2
    participant Job as CrawlJob
    participant DB as JobStorage
    participant Queue as QueueManager

    Note over W1,Queue: Current Behavior (Broken)
    W1->>Job: Process URL #20
    W1->>Job: CompletedURLs++, PendingURLs--
    W1->>Job: Check: PendingURLs == 0?
    Job-->>W1: TRUE
    W1->>Job: Set Status = Completed ❌
    W1->>DB: SaveJob()
    
    Note over W2: Meanwhile...
    W2->>W2: Discover 50 child URLs from URL #15
    W2->>Queue: Enqueue 50 child messages
    W2->>Job: PendingURLs += 50
    
    Note over Job: Job marked complete but has 50 pending URLs!

    Note over W1,Queue: Fixed Behavior (Two-Phase)
    W1->>Job: Process URL #20
    W1->>DB: UpdateJobHeartbeat(jobID)
    W1->>Job: CompletedURLs++, PendingURLs--
    W1->>Job: Check: PendingURLs == 0?
    Job-->>W1: TRUE
    W1->>Job: Set CompletionCandidateAt = Now()
    W1->>DB: SaveJob() (still running)
    
    Note over W2: Meanwhile...
    W2->>W2: Discover 50 child URLs
    W2->>Queue: Enqueue 50 child messages
    W2->>Job: PendingURLs += 50
    W2->>DB: UpdateJobHeartbeat(jobID)
    
    Note over W1: 5 seconds later...
    W1->>Job: Process URL #45
    W1->>DB: UpdateJobHeartbeat(jobID)
    W1->>Job: CompletedURLs++, PendingURLs--
    W1->>Job: Check: PendingURLs == 0?
    Job-->>W1: TRUE (again)
    W1->>Job: Check: CompletionCandidateAt set?
    Job-->>W1: YES (but heartbeat changed)
    W1->>Job: Reset CompletionCandidateAt = Now()
    
    Note over W1: Another 5 seconds later (all work done)...
    W1->>Job: Process last URL
    W1->>DB: UpdateJobHeartbeat(jobID)
    W1->>Job: CompletedURLs++, PendingURLs--
    W1->>Job: Check: PendingURLs == 0?
    Job-->>W1: TRUE
    W1->>Job: Check: Elapsed >= 5s?
    Job-->>W1: YES
    W1->>Job: Check: Heartbeat unchanged?
    Job-->>W1: YES (no activity for 5s)
    W1->>Job: Set Status = Completed ✅
    W1->>DB: SaveJob()
    
    Note over Job: Job correctly completed after grace period

## Proposed File Changes

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\models\crawler_job.go(MODIFY)
- internal\storage\sqlite\job_storage.go(MODIFY)
- internal\interfaces\storage.go

**Location 1: Lines 445-524 (Progress Update Section)**

Add heartbeat update immediately after successful URL processing:

1. After line 443 (`childrenToSpawn = enqueuedCount`), before the progress update section (line 445)
2. Add heartbeat update call:
   - `if err := c.deps.JobStorage.UpdateJobHeartbeat(ctx, msg.ParentID); err != nil`
   - Log warning if heartbeat update fails (non-fatal)
3. This tracks "last URL processed" timestamp for every completed URL

**Location 2: Lines 472-495 (Completion Detection Logic)**

Replace immediate completion check with two-phase delayed detection:

1. **Remove existing logic** (lines 474-495): Single-phase `isComplete` check and immediate status update

2. **Add new two-phase logic:**
   - **Phase 1: Detect Completion Candidate**
     - When `PendingURLs == 0 && TotalURLs > 0`, check if job has `CompletionCandidateAt` timestamp
     - If not set: Set `CompletionCandidateAt = time.Now()` and save job (don't mark complete yet)
     - If already set: Calculate elapsed time since candidate timestamp
   
   - **Phase 2: Verify Completion After Grace Period**
     - If elapsed time >= 5 seconds:
       - Verify `PendingURLs` still 0 (no new URLs enqueued)
       - Verify `last_heartbeat` hasn't changed (no activity during grace period)
       - If both conditions met: Mark job as completed, sync counts, log completion
       - If conditions not met: Reset `CompletionCandidateAt` to nil (more work detected)
     - If elapsed time < 5 seconds: Do nothing, wait for next URL completion

3. **Add logging:**
   - Log when job becomes completion candidate
   - Log when grace period verification passes/fails
   - Log final completion with elapsed time since candidate

**Rationale:** This prevents premature completion by waiting 5 seconds after `PendingURLs` reaches 0, allowing concurrent workers to finish discovering and enqueueing child URLs. The heartbeat check ensures no activity occurred during the grace period.

**Note:** The completion check happens on every URL completion, so the grace period is automatically re-evaluated as workers finish processing.

### internal\models\crawler_job.go(MODIFY)

**Location: Lines 31-58 (CrawlJob struct definition)**

Add new field for tracking completion candidate timestamp:

1. After line 45 (`CompletedAt time.Time`), add new field:
   - `CompletionCandidateAt time.Time` with JSON tag `json:"completion_candidate_at,omitempty"`
   - Add comment: "Timestamp when job first became a completion candidate (PendingURLs == 0). Used for grace period verification before marking complete."

2. This field tracks when the job first reached `PendingURLs == 0`
3. Reset to zero value if new URLs are enqueued during grace period
4. Used to calculate elapsed time for grace period verification

**Rationale:** Separate field is cleaner than reusing `last_heartbeat` or `CompletedAt`. Makes the completion candidate state explicit and easy to query/debug.

**Note:** This is an in-memory field only (not persisted to database) since it's transient state during job execution. If persistence is needed later, a database migration would be required.

### internal\storage\sqlite\job_storage.go(MODIFY)

References: 

- internal\models\crawler_job.go(MODIFY)

**Location: Lines 379-394 (UpdateJobHeartbeat method)**

**No changes required** - method already exists and works correctly:
- Updates `last_heartbeat` to current Unix timestamp
- Thread-safe with mutex lock
- Returns error if update fails

**Verification:** Confirm this method is being called from `crawler.go` after the changes in that file.

**Optional Enhancement** (not required for fix):
Add debug logging to track heartbeat updates:
- Log job_id and timestamp when heartbeat is updated
- Helps with debugging completion timing issues
- Use `Debug` level to avoid log spam

**Rationale:** The existing heartbeat mechanism is sufficient for tracking "last activity" timestamp. No modifications needed to the storage layer.

### test\api\job_completion_test.go(NEW)

References: 

- test\api\crawl_transform_test.go
- test\helpers.go
- test\mock_server.go

**Create new integration test file for delayed completion detection:**

**Test Structure:**

1. **TestJobCompletionWithChildURLs** - Main test verifying delayed completion:
   - Create test source with `follow_links: true` and `max_depth: 2`
   - Point to mock server endpoint that returns HTML with multiple links
   - Create and start crawl job
   - Poll job status every 500ms
   - **Assertions:**
     - Job should NOT complete immediately when first URL finishes
     - Job should remain in "running" status while child URLs are being discovered
     - Job should only complete after ALL child URLs are processed
     - Final `ResultCount` should match total URLs processed (parent + children)
     - Completion should happen within reasonable time (< 30 seconds)

2. **TestJobCompletionGracePeriod** - Verify 5-second grace period:
   - Create job with single URL (no child links)
   - Monitor job status with high-frequency polling (100ms)
   - **Assertions:**
     - Job should NOT complete immediately after URL finishes
     - Job should complete approximately 5 seconds after `PendingURLs` reaches 0
     - Verify grace period is respected (completion time >= 5 seconds)

3. **TestJobCompletionWithConcurrentWorkers** - Stress test with multiple workers:
   - Create job with many seed URLs (10+) and `concurrency: 4`
   - Each URL returns HTML with 5 child links
   - **Assertions:**
     - Job should handle concurrent URL processing correctly
     - No premature completion despite multiple workers
     - All discovered URLs should be processed
     - Final counts should be accurate

**Test Utilities:**
- Use existing `test.NewHTTPTestHelper` for API calls
- Use existing mock server on port 3333 for test endpoints
- Add helper function `waitForJobCompletion(jobID, maxWait)` for polling
- Add helper function `getJobProgress(jobID)` to extract progress counters

**Mock Server Endpoints:**
- `/test/parent` - Returns HTML with 5 child links
- `/test/child/{id}` - Returns HTML with no links (leaf nodes)
- Configure mock server to simulate realistic crawl scenarios

**Cleanup:**
- Delete test jobs and sources in defer statements
- Ensure tests are idempotent and don't interfere with each other

**Rationale:** Integration tests are essential to verify the race condition is fixed. Unit tests alone cannot reproduce the concurrent worker scenario that causes premature completion.

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\models\crawler_job.go(MODIFY)

**Location 1: Lines 608-610 (CancelJob method)**

**Already implemented** - counts are synced before cancellation (lines 609-610):
- `job.ResultCount = job.Progress.CompletedURLs`
- `job.FailedCount = job.Progress.FailedURLs`

**No changes needed** - this was fixed in the previous task.

**Location 2: Lines 662-664 (FailJob method)**

**Already implemented** - counts are synced before marking failed (lines 663-664):
- `job.ResultCount = job.Progress.CompletedURLs`
- `job.FailedCount = job.Progress.FailedURLs`

**No changes needed** - this was fixed in the previous task.

**Verification Only:**
Confirm that both methods properly sync counts when job reaches terminal status. This ensures consistency regardless of how the job terminates (normal completion, cancellation, or failure).

**Rationale:** The count synchronization was already implemented in the previous task. This file only needs verification, no modifications required for the delayed completion feature.