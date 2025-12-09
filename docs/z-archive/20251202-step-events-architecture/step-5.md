# Step 5: Review UI Event Filtering

## Status: COMPLETE (No Changes Required)

## Review Findings

The WebSocket handler (`internal/handlers/websocket.go`) already passes `step_name` to clients for filtering.

### job_log Event Handler (lines 1206-1271)

The handler already includes `step_name` in the WebSocket payload:

```go
wsPayload := map[string]interface{}{
    "job_id":        getString(payload, "job_id"),
    "parent_job_id": getString(payload, "parent_job_id"),
    "level":         getString(payload, "level"),
    "message":       getString(payload, "message"),
    "step_name":     getString(payload, "step_name"),  // <-- Already included
    "source_type":   getString(payload, "source_type"),
    "timestamp":     getString(payload, "timestamp"),
}
```

### step_progress Event Handler (lines 1133-1179)

The handler forwards the entire payload (including `step_name`) to clients.

## UI Filtering

The frontend should filter events by `step_name` to display logs in the correct step panel. The backend now provides:

1. **All workers** route logs through Job Manager with `StepName` in `JobLogOptions`
2. **StepMonitor** includes `step_name` in all `step_progress` events
3. **JobMonitor** includes `step_name` in step completion and progress events
4. **WebSocket** forwards `step_name` to all clients

## Conclusion

No backend changes required. If events still appear in wrong panels, the issue is in the frontend filtering logic.
