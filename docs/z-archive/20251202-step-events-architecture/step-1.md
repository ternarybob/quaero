# Step 1: Audit and fix crawler_worker direct publishing

Model: opus | Status: ✅

## Done

- Reviewed crawler_worker.go for direct EventService.Publish calls
- Refactored `publishCrawlerProgressUpdate()` to use `logWithEvent()` via Job Manager
- Refactored `publishJobSpawnEvent()` to use `logWithEvent()` via Job Manager
- Removed unused `common` import
- Both methods now route through the 3-layer architecture: Worker -> Job Manager -> WebSocket/DB

## Files Changed

- `internal/queue/workers/crawler_worker.go` - Replaced direct EventService.Publish with Job Manager's unified logging

## Build Check

Build: ✅ | Tests: ⏭️ (deferred to task 6)

## Notes

The crawler worker no longer has ANY direct EventService.Publish calls. All events now flow through:
1. `logWithEvent()` -> `AddJobLogWithEvent()` -> Job Manager publishes `job_log` event
2. Events include `step_name`, `manager_id` from `buildJobLogOptions()`
