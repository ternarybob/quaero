# Complete: WebSocket Event Throttling for Step Events

Type: fix | Tasks: 5 | Files: 5

## User Request
"WebSocket event pushing for step events slows processing when 100+ jobs in queue. Refactor to use trigger-based polling instead of direct event push."

## Result
Implemented event aggregator that batches step events and sends refresh triggers to UI instead of individual event pushes. UI now fetches events from API when triggered. Steps finishing trigger an immediate "finish" refresh so UI receives final events.

## Architecture Change

### Before
```
Step Event → EventService → WebSocket Broadcast (per event) → UI Update
```

### After
```
Step Event → EventAggregator → (1s interval OR step finished) → WebSocket Trigger → UI Fetches from API
```

## Key Changes

1. **Config** (`internal/common/config.go`)
   - `TimeThreshold`: Trigger interval (default: "1s")
   - `EventCountThreshold`: Deprecated (no longer used)

2. **Event Aggregator** (`internal/services/events/aggregator.go`)
   - Time-based triggering only (every 1 second by default)
   - Immediate trigger when step finishes (completed/failed/cancelled) with `finished=true`
   - UI always shows last 100 events (fetched from API)

3. **WebSocket Handler** (`internal/handlers/websocket.go`)
   - New message type: `refresh_step_events`
   - Message payload includes: `step_ids`, `timestamp`, `finished`
   - Aggregator records events instead of broadcasting
   - Fallback to direct broadcast if aggregator not initialized

4. **Events API** (`internal/handlers/job_handler.go`)
   - Added `limit` query parameter (default: 1000, max: 5000)

5. **UI** (`pages/queue.html`)
   - Handles `refresh_step_events` trigger with `finished` flag
   - Fetches events from `/api/jobs/{id}/logs?limit=100`
   - Client-side 500ms throttle per step (skipped for finished steps)
   - On-demand event loading when step panel is expanded
   - Uses `full_timestamp` for datetime display
   - Events stored in `jobLogs[managerId]` keyed by step_name

## Configuration

```toml
[websocket]
time_threshold = "1s"        # Trigger interval (default)
```

Environment variable:
- `QUAERO_WEBSOCKET_TIME_THRESHOLD`

## Skills Used
go, frontend

## Validation: ✅ COMPLETE

## Verify
Build: ✅ | Tests: ⏭️

## Files Changed
- `internal/common/config.go` - WebSocket config fields
- `internal/services/events/aggregator.go` - NEW: Event aggregator service
- `internal/handlers/websocket.go` - Aggregator integration with finished flag
- `internal/handlers/job_handler.go` - API limit parameter
- `pages/queue.html` - Trigger-based event refresh with finished handling
