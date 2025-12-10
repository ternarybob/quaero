# Complete: Step Events Flow + Timestamp Ordering
Type: fix | Tasks: 3 | Files: 3

## User Request
"UI not flowing/updating correctly. Step should load events via API either triggered by websocket (if running) or last 100 (if complete). Events not able to be ordered - need millisecond timestamps."

## Result
Fixed two key issues with step event display:

1. **Timestamp Precision**: Changed event timestamps from second-only (RFC3339) to nanosecond precision (RFC3339Nano). Display format updated from "15:04:05" to "15:04:05.000" with milliseconds. This enables proper ordering of events for fast jobs that complete under 1 second.

2. **Completed Steps Loading**: Added `loadCompletedStepEvents()` function that runs on page load to fetch events for completed/failed/cancelled steps. Previously, steps that completed before WebSocket triggers fired showed "0 events" on page reload.

## Files Changed
1. `internal/logs/consumer.go` - Changed timestamp format to RFC3339Nano with millisecond display
2. `internal/models/job_log.go` - Updated documentation to reflect new timestamp formats
3. `pages/queue.html` - Added loadCompletedStepEvents() and fetchStepEventsById() functions

## Skills Used
- go (timestamp format handling)
- frontend (Alpine.js event loading)

## Validation: ✅ MATCHES
All success criteria met:
- Completed steps auto-load events on page reload
- Timestamps have millisecond precision for ordering
- Build passes

## Review: N/A
No critical triggers

## Verify
Build: ✅ | Tests: N/A
