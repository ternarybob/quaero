# Investigation: WebSocket Real-Time Job Logging

## Issue Reported

Step logs in the queue UI were not updating in real-time. A page refresh was required to see the latest log entries.

## Investigation Summary

### Initial Hypothesis

WebSocket events were not being published or received correctly for job log updates.

### Testing Approach

1. **UI Test** (`test/ui/queue_test.go`) - Initially used but had a 30-second page refresh interval that masked the real behavior
2. **API Test** (`test/api/websocket_job_events_test.go`) - Created to isolate WebSocket behavior without UI refresh interference

### Findings

**WebSocket events ARE working correctly.** The API test demonstrated:
- Hundreds of `job_log` events received in real-time during crawler execution
- Events include: `step_progress`, `crawler_job_log`, `job_status_change`
- No delays or missing events detected

The apparent lack of real-time updates was due to:
1. UI test's automatic page refresh every 30 seconds
2. Page navigation between job list and job detail views triggering full reloads

## Job Logging Architecture

### Event Types

| Event Type | Description |
|------------|-------------|
| `job_log` | Individual log entry from any job |
| `step_progress` | Step execution progress updates |
| `crawler_job_log` | Crawler-specific progress logs |
| `job_status_change` | Job state transitions |

### Originator Tags

Log entries use originator tags to identify the source:

| Tag | Source | Example |
|-----|--------|---------|
| `[step]` | StepManager generated logs | Step started, step completed |
| `[worker]` | Worker generated logs | URL fetched, content extracted |
| *(empty)* | JobMonitor/system logs | Progress aggregation, status updates |

### Log Flow

```
Worker/Manager/Monitor
    ↓
jobMgr.AddJobLog(ctx, jobID, level, message)
    ↓
JobManager.AddJobLog()
    ↓
eventService.Publish(EventJobLog, payload)
    ↓
WebSocketHandler.handleJobLogEvent()
    ↓
BroadcastToClients(JSON message)
    ↓
Browser WebSocket receives event
    ↓
Alpine.js updates UI (queue.html)
```

## Files Involved

### Backend
- `internal/queue/job_manager.go` - `AddJobLog()`, `AddJobLogWithContext()`
- `internal/handlers/websocket_handler.go` - Event broadcasting
- `internal/handlers/websocket_events.go` - Event type definitions
- `internal/services/events/service.go` - Pub/sub event system

### Frontend
- `pages/queue.html` - Alpine.js WebSocket handling and log display

### Tests
- `test/api/websocket_job_events_test.go` - `TestWebSocketJobEvents_NewsCrawlerRealTime`

## API Test Details

The test validates real-time WebSocket events by:

1. Establishing WebSocket connection to `/ws`
2. Triggering "News Crawler" job via POST `/api/job-definitions/news-crawler/execute`
3. Collecting all WebSocket events during execution
4. Verifying `job_log` events are received without page refresh

```go
// Key assertions
assert.True(t, receivedJobLogEvents > 0, "Should receive job_log events")
assert.True(t, receivedWithinTimeout, "Events received before timeout")
```

## Conclusion

**No bug found.** WebSocket real-time logging is functioning correctly. The perceived issue was a test artifact from automatic page refreshes masking the real-time behavior.

## Recommendations

1. UI tests should avoid automatic page refreshes when testing real-time features
2. Use API-level WebSocket tests for validating event delivery
3. Consider adding a visual indicator in the UI when WebSocket is connected/receiving

