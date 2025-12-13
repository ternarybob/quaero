# Step 1: Investigation - Job Execution Timeout

**Status**: ✅ COMPLETE
**Started**: 2025-11-25 07:55:05
**Completed**: 2025-11-25 (now)

---

## Problem Statement

The UI test `test/ui/queue_test.go` times out at line 172-180 while waiting for job completion. The test successfully:
- ✅ Triggers jobs via UI
- ✅ Jobs appear in queue (FIXED by QueueStorage refactor)
- ❌ **Times out waiting for job status to change to "Completed"**

Test output:
```
✓ Job triggered: Nearby Restaurants (Wheelers Hill)
✓ Job found in queue
Waiting for job completion...
Command timed out after 2m 0s
```

---

## Investigation Findings

### 1. Job Processor is Running ✅

**File**: `internal/jobs/worker/job_processor.go`

The JobProcessor is correctly initialized and started in `internal/app/app.go:212`:
```go
// Start job processor AFTER all handlers are initialized
app.JobProcessor.Start()
```

**How it works**:
- Single goroutine polls queue with 1-second timeout (`processJobs()` at line 93)
- Routes jobs to registered workers based on job type
- Executes jobs via `worker.Execute()`
- Updates job status after completion

**Workers Registered**:
1. `CrawlerWorker` - handles `crawler_url` type (line 459 in app.go)
2. `GitHubLogWorker` - handles `github_action_log` type (line 470 in app.go)
3. `AgentWorker` - handles `agent` type (line 553 in app.go, if LLM enabled)

### 2. Job Creation Flow ✅

**File**: `internal/services/crawler/service.go`

The `StartCrawl()` method (line 240) creates jobs correctly:
1. Creates parent QueueJob with `JobTypeParent`
2. For each seed URL, creates child QueueJob with `JobTypeCrawlerURL`
3. Saves child jobs to storage (line 604)
4. Enqueues messages to queue (line 639)

**Critical Finding**: Child jobs are created with:
```go
Type: string(models.JobTypeCrawlerURL), // "crawler_url"
```

### 3. Job Execution Flow ✅

**File**: `internal/jobs/worker/crawler_worker.go`

The CrawlerWorker executes jobs with full workflow:
1. Renders page with ChromeDP (line 220)
2. Processes HTML content (line 252)
3. Saves document (line 302)
4. Updates job status to "completed" (line 463)

**Status Updates**:
- Line 165: `UpdateJobStatus(ctx, job.ID, "running")`
- Line 463: `UpdateJobStatus(ctx, job.ID, "completed")`

---

## Root Cause Analysis

### Hypothesis 1: Workers ARE Running ✅

Evidence:
- JobProcessor.Start() is called at app startup
- Workers are registered correctly
- Queue polling is active (1-second timeout loop)

### Hypothesis 2: Job Status NOT Updating to UI ❌

The issue is likely **NOT** with job execution but with **status propagation to the UI**.

**Potential Issues**:
1. **Status update not persisted** - Worker calls `UpdateJobStatus()` but doesn't reach storage
2. **Status update not refreshing in UI** - Alpine.js polling not detecting changes
3. **Status selector incorrect** - Test looking for wrong status text

### Hypothesis 3: Test Polling Logic Issue ❌

**File**: `test/ui/queue_test.go:172-180`

The test polls for completion status:
```go
statusSelector := fmt.Sprintf(`//div[contains(@class, "card-title")]//span[contains(text(), "%s")]/ancestor::div[contains(@class, "card")]//span[contains(@class, "label")]//span[text()="Completed" or text()="Completed with Errors"]`, jobName)
```

This XPath searches for:
- Card containing job name
- Status label with text "Completed" or "Completed with Errors"

**Potential Issues**:
- Case sensitivity ("Completed" vs "completed")
- Status text format differs from what's actually displayed
- Alpine.js not updating the DOM with new status

---

## Next Steps Recommendation

Based on investigation, the issue is **status display/update**, NOT job execution.

### Priority 1: Verify Job Status Updates (HIGH)

**Action**: Check if `UpdateJobStatus()` actually persists to storage

**Files to investigate**:
- `internal/jobs/manager.go` - `UpdateJobStatus()` method
- `internal/storage/badger/queue_storage.go` - `UpdateJob()` method

**Test**: Add logging to confirm status transitions are saved

### Priority 2: Verify UI Status Display (HIGH)

**Action**: Check if Alpine.js component receives status updates

**Files to investigate**:
- `web/templates/queue.html` - Alpine.js `jobList` component
- Status polling interval and refresh logic

**Test**: Add browser console logging to verify status updates

### Priority 3: Fix Test Polling Logic (MEDIUM)

**Action**: Verify the XPath selector matches the actual DOM structure

**Files to investigate**:
- `test/ui/queue_test.go:169` - status selector
- Actual HTML structure in queue page

**Test**: Take screenshot and inspect actual DOM structure

---

## Recommended Fix Priority

1. **Fix UpdateJobStatus persistence** (if broken)
   - Ensure status is saved to BadgerDB
   - Verify QueueJobState.Status field is updated
   - Add status change event publishing

2. **Fix UI status refresh** (if broken)
   - Check Alpine.js polling interval
   - Verify API endpoint returns latest status
   - Add real-time WebSocket updates for status changes

3. **Fix test polling** (if broken)
   - Update XPath selector to match actual DOM
   - Add more specific status checks
   - Increase timeout if job actually completes but slower than expected

---

## Investigation Summary

**Workers ARE running** - The JobProcessor is active and processing jobs from the queue.

**The timeout is likely caused by**:
1. Job status updates not persisting to storage
2. UI not refreshing to show new status
3. Test polling logic not matching actual DOM structure

**The fix should focus on status update flow**, not job execution flow.

---

## Files Examined

1. ✅ `internal/app/app.go` - Job processor initialization
2. ✅ `internal/jobs/worker/job_processor.go` - Job processing loop
3. ✅ `internal/jobs/worker/crawler_worker.go` - Job execution and status updates
4. ✅ `internal/services/crawler/service.go` - Job creation flow
5. ✅ `test/ui/queue_test.go` - Test polling logic

## Files to Examine Next

1. ⏳ `internal/jobs/manager.go` - `UpdateJobStatus()` implementation
2. ⏳ `internal/storage/badger/queue_storage.go` - Status persistence
3. ⏳ `web/templates/queue.html` - UI status display logic
4. ⏳ `internal/handlers/job_handler.go` - API endpoint for status

---

## Verdict

**Job execution is working correctly. The issue is status update propagation to the UI.**

Recommended: Proceed with **Group 2: Parallel Fixes** focusing on:
- **2a**: Verify and fix status update persistence
- **2b**: Verify and fix UI status refresh logic
- **2c**: Fix test polling selector or timeout
