# Complete: Fix Service Logs Duplicate Polling
Type: fix | Tasks: 4 | Files: 4

## User Request
"The service logs are doubling up on requests even though nothing is happening."

## Result
Fixed infinite loop caused by circular logging: `refresh_logs` broadcast → Debug log → `log_event` → `hasPendingLogs=true` → another `refresh_logs`. Removed all logging from refresh trigger functions to break the cycle. Also added in-flight tracking to prevent duplicate child job fetches.

## Root Cause
The Debug log statement inside `broadcastLogsRefreshTrigger()` was being captured by the LogConsumer, which published it as a `log_event`, which set `hasPendingLogs=true`, which triggered another refresh 1 second later - creating an infinite loop.

## Skills Used
- go
- frontend

## Validation: MATCHES
Fixed the circular logging loop that caused continuous requests.

## Review: N/A
No critical triggers.

## Verify
Build: Pass | Tests: Skipped

## Files Changed
- `internal/handlers/websocket.go` - Removed logging from broadcast functions
- `internal/services/events/log_aggregator.go` - Removed logging from flushPending
- `internal/logs/consumer.go` - Updated comment
- `pages/queue.html` - Added in-flight tracking for child fetches
