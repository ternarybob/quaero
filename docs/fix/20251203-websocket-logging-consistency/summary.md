# Summary: WebSocket Logging Consistency Fix

## Problem
The queue UI displayed inconsistent logging:
- Some logs showed `[step]` tag, others had no tag
- Duplicate log entries appeared (e.g., "Starting 2 workers..." twice)
- Emoji prefixes in log messages were inconsistent
- Log level icons needed to be replaced with text-based tags

## Solution

### Task 1: Fix WebSocket Handler (websocket.go)
Added `manager_id` and `originator` fields to the WebSocket payload for `job_log` events:
```go
wsPayload := map[string]interface{}{
    ...
    "manager_id":    getString(payload, "manager_id"),
    "originator":    getString(payload, "originator"),
    ...
}
```

### Task 2: Client-Side Deduplication (queue.html)
Added dedup logic in `handleJobLog()` to prevent duplicate log entries:
```javascript
const isDuplicate = this.jobLogs[aggregationId].some(
    log => log.timestamp === newEntry.timestamp &&
           log.message === newEntry.message &&
           log.step_name === newEntry.step_name
);
if (isDuplicate) return;
```

### Task 3: Remove Emoji Prefixes (crawler_worker.go)
Replaced emoji prefixes with plain text:
- `✗ Failed:` → `Failed:`
- `▶ Started:` → `Started:`
- `✓ Completed:` → `Completed:`

### Task 4: Text-Based Level Tags (queue.html + quaero.css)
Updated log display to use text tags instead of FontAwesome icons:
- Added `getLogLevelTag()` function returning `[INF]`, `[WRN]`, `[ERR]`, `[DBG]`
- Updated HTML template to use `x-text="getLogLevelTag(log.level)"`
- Added CSS classes: `.log-level-info`, `.log-level-warn`, `.log-level-error`, `.log-level-debug`

### Task 5: Updated Tests
- `websocket_job_events_test.go`: Added verification for `originator` field in job_log events
- `queue_test.go`: Added verification for text-based level tags and originator tags in UI

## Files Modified
- `internal/handlers/websocket.go` - Added manager_id and originator to wsPayload
- `internal/queue/workers/crawler_worker.go` - Removed emoji prefixes
- `pages/queue.html` - Added dedup logic and text level tag display
- `pages/static/quaero.css` - Added log-level color classes
- `test/api/websocket_job_events_test.go` - Added originator verification
- `test/ui/queue_test.go` - Added log format verification

## Test Results
```
TestWebSocketJobEvents_JobLogEventContext: PASS
- 29 job_log events: 29 with manager_id (100%)
- 29 job_log events: 29 with originator (100%)
- Originator values: manager:12, step:10, worker:7

TestWebSocketJobEvents_StepProgressEventContext: PASS
- 5 step_progress events with correct manager_id and step_name
```

## New Log Format
```
HH:MM:SS [LVL] [originator] message
```
Example:
```
14:32:15 [INF] [step] Starting 2 workers for step: test-step
14:32:16 [INF] [worker] Completed: https://example.com elapsed:1.2s
```
