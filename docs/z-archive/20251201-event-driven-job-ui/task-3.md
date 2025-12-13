# Task 3: Add WebSocket job log subscription
Depends: 2 | Critical: no | Model: sonnet

## Context
WebSocket handler broadcasts events but client can't subscribe to specific job's logs. Need subscription mechanism for real-time log streaming per job.

## Do
- Add `subscribeJobLogs` map to WebSocketHandler (job_id -> []*websocket.Conn)
- Handle "subscribe_job_logs" message type from client
- Handle "unsubscribe_job_logs" message type
- When log events are published, send only to subscribed clients
- Clean up subscriptions when client disconnects
- Broadcast logs for job AND all child jobs to parent subscribers

## Message Protocol
Client → Server:
```json
{"type": "subscribe_job_logs", "job_id": "xxx"}
{"type": "unsubscribe_job_logs", "job_id": "xxx"}
```

Server → Client:
```json
{
  "type": "job_log",
  "job_id": "parent-xxx",
  "entry": {
    "timestamp": "10:30:01",
    "full_timestamp": "2024-12-01T10:30:01Z",
    "level": "info",
    "message": "Processing document doc_123",
    "source_job_id": "child-yyy",
    "step_name": "extract_keywords"
  }
}
```

## Accept
- [ ] Client can subscribe to specific job logs
- [ ] Only subscribed clients receive logs for that job
- [ ] Child job logs broadcast to parent subscribers
- [ ] Subscriptions cleaned up on disconnect
- [ ] Build compiles without errors
