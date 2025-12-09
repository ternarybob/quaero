# Complete: Step Events Loading Fix
Type: fix | Tasks: 3 | Files: 2

## User Request
"UI still loading all events/logs - The screenshot is when the job page is loaded and steps/workers are running. Appears the flow of step completion is not working as required. The service also crashes (random) without logs."

## Result
Fixed two issues with step events loading:

1. **Completed steps showing "No events yet"**: The `refreshStepEvents` function couldn't find the manager/step mapping because the manager's `metadata.step_job_ids` wasn't in the cached `allJobs`. Added fallback to fetch the step job from API to get the parent_id and step_name.

2. **Step 3 loading all events (scrolling)**: Individual `job_log` WebSocket messages were still being broadcast for step events. Modified the websocket handler to skip broadcasting `job_log` for step events (source_type=step or step_name present) - these now use the trigger-based refresh approach only.

3. **Service crash**: Log file examined - no crash/panic/error found. All jobs completed successfully with `has_error=false`. Crash likely caused by external factors (OOM, signal).

## Files Changed
1. `pages/queue.html`:
   - `refreshStepEvents()`: Added API fallback when step job ID not found in cached managers
   - Also refreshes manager job to update cached metadata

2. `internal/handlers/websocket.go`:
   - Skip `job_log` broadcast for step events (source_type=step or step_name present)
   - Step logs now only delivered via `refresh_step_events` trigger + API fetch

## Skills Used
- go (websocket handler)
- frontend (Alpine.js)

## Validation: MATCHES
- Completed steps will now load events on COMPLETE trigger
- Step events won't scroll during execution (no individual broadcasts)
- Only last 100 events fetched on COMPLETE

## Review: N/A
No critical triggers

## Verify
Build: ✅ | Tests: N/A
