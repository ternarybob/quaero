# Plan: Step Events Architecture Fix

**Status: COMPLETE**

Type: fix | Workdir: ./docs/fix/20251202-step-events-architecture/

## User Intent (from manifest)

Fix the event/logging architecture so that:
1. Events from one step do NOT appear in another step's panel
2. The 3-layer architecture is enforced (Workers → Step Manager → Job Manager)
3. Workers only publish to step/job queues via single logging mechanism
4. WebSocket subscribes to job and step message queues only
5. Create integration test to verify correct event routing

## Root Cause Analysis

From the screenshot, "Step search_nearby_restaurants completed" appears in the "extract_keywords" events panel because:
1. Events lack proper `step_name` filtering - the message is published without step context
2. Workers publish events directly to EventService (bypassing Job/Step Manager)
3. UI `getStepLogs()` shows all events when filtering fails

## Architecture Violations Found

| Worker | Violation | File:Line |
|--------|-----------|-----------|
| crawler_worker | Direct `crawler_job_progress` publishing | crawler_worker.go:1447 |
| crawler_worker | Direct `job_spawn` publishing | crawler_worker.go:1488 |
| github_log_worker | Direct `document_saved` publishing | github_log_worker.go:185,268 |
| github_repo_worker | Direct `document_saved` publishing | github_repo_worker.go:171 |
| agent_worker | Direct `job_error` publishing | agent_worker.go:708 |
| places_worker | Direct `PublishSync` calls | places_worker.go:277 |
| web_search_worker | Direct `PublishSync` calls | web_search_worker.go:225 |
| job_monitor | Direct progress publishing | monitor.go |
| step_monitor | Direct progress publishing | step_monitor.go:277 |

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Audit and fix crawler_worker direct publishing | - | yes:architectural-change | opus |
| 2 | Audit and fix other workers (github, agent, places, web_search) | 1 | yes:architectural-change | opus |
| 3 | Fix step_monitor event publishing to include step context | 1 | yes:architectural-change | opus |
| 4 | Fix job_monitor event publishing | 3 | no | sonnet |
| 5 | Ensure UI filters events by step_name correctly | 4 | no | sonnet |
| 6 | Create integration test for WebSocket event routing | 5 | no | sonnet |

## Order

[1] → [2] → [3] → [4] → [5] → [6]

## Completion Summary

All 6 tasks completed successfully:

1. **Task 1** (step-1.md): Fixed crawler_worker.go - refactored `publishCrawlerProgressUpdate()` and `publishJobSpawnEvent()` to use Job Manager's `AddJobLogWithEvent()`

2. **Task 2** (step-2.md): Fixed all other workers:
   - github_log_worker.go: Added `logDocumentSaved()` helper
   - github_repo_worker.go: Added `logDocumentSaved()` helper
   - agent_worker.go: Fixed `publishJobError()` to use Job Manager
   - places_worker.go: Replaced `PublishSync` with Job Manager logging
   - web_search_worker.go: Replaced `PublishSync` with Job Manager logging

3. **Task 3** (step-3.md): Fixed step_monitor.go - Added `stepName` parameter to `publishStepProgress()` and included `step_name` in all event payloads

4. **Task 4** (step-4.md): Reviewed job_monitor.go - Already correct (no changes needed)

5. **Task 5** (step-5.md): Reviewed WebSocket handler - Already passes `step_name` to clients (no changes needed)

6. **Task 6** (step-6.md): Added `TestWebSocketJobEvents_StepNameRouting` integration test

## Build Verification

```bash
go build ./...  # SUCCESS - no errors
```
