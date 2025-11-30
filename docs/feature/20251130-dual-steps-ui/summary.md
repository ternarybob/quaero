# Summary: Dual Steps UI

## Completed Changes

### 1. Fixed Job Definition Bug
**File:** `test/config/job-definitions/nearby-resturants-keywords.toml`

Changed `filter_source_type` from `"crawler"` to `"places"` in the `extract_keywords` step. The agent worker was filtering for crawler documents but the places search creates documents with `source_type = "places"`, causing no documents to be processed.

### 2. Fixed Job Monitoring for Zero Child Jobs
**File:** `internal/queue/state/monitor.go`

Added a 30-second grace period for jobs that report `ReturnsChildJobs() = true` but don't actually spawn any child jobs. Previously, such jobs would hang indefinitely waiting for child jobs that never arrived.

Changes:
- Added `noChildrenGracePeriod` (30 seconds)
- Added `hasSeenChildren` flag to track if any children were ever seen
- Created `checkChildJobProgressWithCount()` function that returns both completion status and child count
- Job completes gracefully with log message "Job completed (no child jobs were spawned)"

### 3. Added Step Progress Events
**File:** `internal/queue/manager.go`

Added WebSocket events for step progress in `ExecuteJobDefinition()`:
- Emits `job_progress` event when step starts (status: "running")
- Emits `job_progress` event when step completes (status: "completed")
- Payload includes: `job_id`, `job_name`, `step_index`, `step_name`, `step_type`, `current_step`, `total_steps`, `step_status`, `timestamp`

### 4. Added WebSocket Handler for Step Progress
**File:** `internal/handlers/websocket.go`

Added subscription for `EventJobProgress` events that broadcasts `job_step_progress` messages to all connected WebSocket clients.

### 5. Updated Queue UI to Display Step Progress
**File:** `pages/queue.html`

- Added handler for `job_step_progress` WebSocket messages
- Progress text now shows: "Step 1/2: search_nearby_restaurants (completed)"
- Step progress data is stored in `job.status_report` for display
- Fields added: `current_step`, `total_steps`, `step_name`, `step_type`, `step_status`

### 6. Created Multi-Step Job Test
**File:** `test/ui/queue_test.go`

Added `TestNearbyRestaurantsKeywordsMultiStep` test that:
- Triggers the multi-step job definition
- Monitors for successful completion (not hung in running state)
- Verifies documents are created
- Takes screenshots at key stages

## Files Modified

| File | Change |
|------|--------|
| `test/config/job-definitions/nearby-resturants-keywords.toml` | Fixed `filter_source_type` bug |
| `internal/queue/state/monitor.go` | Added grace period for zero child jobs |
| `internal/queue/manager.go` | Added step progress event publishing |
| `internal/handlers/websocket.go` | Added WebSocket handler for step events |
| `pages/queue.html` | Added step progress display in UI |
| `test/ui/queue_test.go` | Added `TestNearbyRestaurantsKeywordsMultiStep` |

## Technical Details

### Step Progress Event Flow
1. `ExecuteJobDefinition` starts a step → publishes `job_progress` (status: running)
2. Step completes → publishes `job_progress` (status: completed)
3. `websocket.go` receives event → broadcasts `job_step_progress` to clients
4. `queue.html` receives message → updates `job.status_report.progress_text`
5. UI re-renders showing "Step X/Y: step_name (status)"

### Grace Period Logic
```
monitorStartTime = now()
for {
    if !hasSeenChildren && time.Since(monitorStartTime) > 30s {
        // Complete job - no children were ever spawned
        return completed
    }
    // normal monitoring logic
}
```

## Validation

- Build: `go build ./...` - PASSED
- Test compile: `go test -c ./test/ui/...` - PASSED
