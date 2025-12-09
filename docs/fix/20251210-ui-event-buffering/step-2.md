# Step 2: Implement LogEventAggregator for Service Logs

## Implementation Summary
Created a time-based aggregator for service log events that triggers periodic UI refreshes instead of pushing individual log messages over WebSocket.

## Changes Made

### 1. Created `internal/services/events/log_aggregator.go`
- **LogEventAggregator**: Simple aggregator that tracks boolean flag for pending logs
- **Time-based triggering**: Default 1 second interval (configurable)
- **RecordEvent()**: Mark that logs are pending (no individual log tracking)
- **StartPeriodicFlush()**: Background goroutine that triggers every timeThreshold
- **FlushAll()**: Cleanup method for graceful shutdown
- **OnTrigger callback**: Broadcasts `refresh_logs` WebSocket message

Pattern follows `StepEventAggregator` but simplified:
- No per-entity tracking (steps had per-step tracking)
- Single boolean flag for all logs (hasPendingLogs)
- No immediate triggers (logs don't need instant final state like steps do)

### 2. Updated `internal/handlers/websocket.go`

#### Added field to WebSocketHandler:
```go
logEventAggregator *events.LogEventAggregator
```

#### Initialized aggregator in NewWebSocketHandler():
```go
h.logEventAggregator = events.NewLogEventAggregator(
    timeThreshold,
    h.broadcastLogsRefreshTrigger,
    arborLogger,
)
h.logEventAggregator.StartPeriodicFlush(context.Background())
```

#### Added broadcastLogsRefreshTrigger() method:
- Sends `refresh_logs` WebSocket message with timestamp
- Broadcasts to all connected clients
- Logs debug info for monitoring

#### Updated log_event subscriber:
- Calls `h.logEventAggregator.RecordEvent(ctx)` instead of direct broadcast
- Keeps fallback to direct broadcast if aggregator not initialized
- No longer sends individual `log` WebSocket messages during high volume

## WebSocket Message Format

### New Message Type: `refresh_logs`
```json
{
  "type": "refresh_logs",
  "payload": {
    "timestamp": "2025-12-10T15:04:05Z"
  }
}
```

The UI should respond to this message by fetching logs from the API endpoint.

## Behavior

### Before (Direct Broadcast)
1. Log event occurs
2. Immediately broadcast `log` WebSocket message
3. High log volume = WebSocket flooding

### After (Trigger-Based Buffering)
1. Log event occurs
2. Record event in aggregator (set hasPendingLogs = true)
3. Periodic flush (every 1s) checks for pending logs
4. If pending: broadcast `refresh_logs` trigger
5. UI fetches logs from API when it receives trigger

## Benefits
- Reduces WebSocket message count during high log volume
- Batches multiple log events into single trigger
- UI pulls data on demand instead of being pushed
- Same pattern as step events for consistency
- Graceful degradation if aggregator not initialized

## Configuration
Uses same `time_threshold` as step aggregator from WebSocket config:
- Default: 1 second
- Configurable via `WebSocketConfig.TimeThreshold`

## Testing Recommendations
1. Monitor WebSocket traffic during high log volume jobs
2. Verify `refresh_logs` messages sent at ~1s intervals
3. Confirm no individual `log` messages broadcast when aggregator active
4. Test graceful shutdown (FlushAll on context cancellation)
5. Verify UI responds to `refresh_logs` by fetching from API

## Next Steps
- Task 3: Update UI to handle `refresh_logs` trigger and fetch from API
- Task 4: Similar pattern for job_log events if needed
