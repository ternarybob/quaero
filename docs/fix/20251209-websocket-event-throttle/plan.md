# Plan: WebSocket Event Throttling for Step Events

## Overview
Refactor the WebSocket event system for step events from direct push to trigger-based polling to improve processing speed when 100+ jobs are in queue.

## Architecture Change

### Current Flow
```
Step/Job Status Change → EventService.Publish() → WebSocketHandler → Broadcast to ALL clients → UI updates
```
**Problem:** Each event broadcasts to all WebSocket clients, creating bottleneck with high job volumes.

### New Flow
```
Step Event → EventAggregator (in-memory) → Trigger threshold reached → WebSocket sends "refresh_step_events" trigger → UI fetches events from API
```

## Tasks

### Task 1: Add Event Aggregator Configuration (go)
- Add `EventAggregator` config to `WebSocketConfig` in `internal/common/config.go`
- Settings: `event_count_threshold` (default: 100), `time_threshold` (default: "1s")
- Add env var support: `QUAERO_WEBSOCKET_EVENT_COUNT_THRESHOLD`, `QUAERO_WEBSOCKET_TIME_THRESHOLD`

### Task 2: Create Event Aggregator Service (go)
- Create `internal/services/events/aggregator.go`
- In-memory aggregator that:
  - Accumulates step events per step_id
  - Tracks event count and last trigger time
  - Fires "refresh" trigger when: count >= threshold OR time >= interval
  - Resets counters after trigger
- Thread-safe with mutex

### Task 3: Refactor WebSocket Handler (go)
- Modify `internal/handlers/websocket.go`
- Replace direct `step_progress` broadcast with aggregator integration
- New WebSocket message type: `refresh_step_events` with payload `{ step_ids: string[] }`
- Keep manager_progress and job_step_progress unchanged (lower volume)

### Task 4: Create Step Events API Endpoint (go)
- Add `GET /api/jobs/{job_id}/events` to `internal/server/routes.go`
- Handler in `internal/handlers/jobs_handler.go`
- Parameters:
  - `limit` (default: 100) - max events to return
  - `since` (RFC3339 timestamp) - events after this time
  - `offset` (int) - for pagination
- Returns JSON array of step events

### Task 5: Refactor UI Event Handling (frontend)
- Modify `pages/queue.html`
- Handle new `refresh_step_events` WebSocket message
- On trigger: fetch events from API for affected step_ids
- Update step panels with API response
- Remove direct step_progress payload processing (keep listener for backward compat)

## Files to Modify

| File | Changes |
|------|---------|
| `internal/common/config.go` | Add EventAggregator config fields |
| `internal/services/events/aggregator.go` | New file - event aggregator service |
| `internal/handlers/websocket.go` | Integrate aggregator, new message type |
| `internal/server/routes.go` | Add events API route |
| `internal/handlers/jobs_handler.go` | Add GetStepEvents handler |
| `pages/queue.html` | Trigger-based fetching |
| `bin/quaero.toml` | Add example config (commented) |

## Dependencies
- Task 2 depends on Task 1 (config)
- Task 3 depends on Task 2 (aggregator)
- Task 4 is independent
- Task 5 depends on Task 3 and Task 4

## Execution Order
1. Task 1 (config) + Task 4 (API) - parallel
2. Task 2 (aggregator)
3. Task 3 (websocket handler)
4. Task 5 (UI)
5. Build and verify
