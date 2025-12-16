# SSE Log Streaming Refactor - Architect Analysis

## Executive Summary

This document analyzes the current codebase architecture for the SSE log streaming refactor. The goal is to replace the WebSocket signal-then-fetch pattern with Server-Sent Events (SSE) for log streaming.

## Current Architecture

### Server-Side Components

#### 1. WebSocket Handler (`internal/handlers/websocket.go` - ~1500 lines)
**Purpose**: Central WebSocket connection management and event broadcasting

**Key Structures**:
- `WebSocketHandler` - Main handler with client tracking, rate limiters, and aggregators
- `WSMessage` - Generic message envelope with type and payload
- Multiple update structs: `JobStatusUpdate`, `CrawlProgressUpdate`, `QueueStatsUpdate`, etc.

**Key Methods**:
- `HandleWebSocket()` - Connection upgrade and client management
- `BroadcastStatus/JobStatusChange/CrawlProgress/etc.` - Event broadcasting
- `broadcastUnifiedRefreshTrigger()` - Sends refresh_logs triggers (scope: service/job)
- `SubscribeToCrawlerEvents()` - Subscribes to internal event bus

**Dependencies**:
- `gorilla/websocket` - WebSocket library
- `UnifiedLogAggregator` - Batches log events before triggering
- `EventService` - Internal pub/sub for events

#### 2. Event Subscriber (`internal/handlers/websocket_events.go` - ~380 lines)
**Purpose**: Bridges internal events to WebSocket broadcasts

**Key Structures**:
- `EventSubscriber` - Subscribes to job lifecycle events

**Events Handled**:
- `job_created`, `job_started`, `job_completed`, `job_failed`, `job_cancelled`, `job_spawn`

#### 3. Unified Log Aggregator (`internal/services/events/unified_aggregator.go` - ~340 lines)
**Purpose**: Batches log events to reduce WebSocket message frequency

**Key Features**:
- Scaling intervals: 1s, 2s, 3s, 4s -> 10s periodic
- Separate tracking for service logs and step logs
- Immediate trigger on step completion

**Callback**: `broadcastUnifiedRefreshTrigger()` sends WebSocket message to UI

#### 4. Unified Logs Handler (`internal/handlers/unified_logs_handler.go` - ~600 lines)
**Purpose**: REST API for fetching logs

**Endpoints**:
- `GET /api/logs?scope=service` - Service logs from memory
- `GET /api/logs?scope=job&job_id=X` - Job logs from storage
- `GET /api/logs?scope=job&job_id=X&step=Y` - Step-grouped logs

**Can be Extended**: This handler already has the log retrieval logic needed for SSE.

#### 5. Log Service (`internal/logs/service.go`)
**Purpose**: Log storage and retrieval

**Key Methods**:
- `GetLogs()`, `GetLogsByLevel()` - Retrieve logs
- `CountLogs()`, `CountLogsByLevel()` - Count logs
- `GetAggregatedLogs()` - Logs with descendant traversal

**Needs Extension**: Add `Subscribe()` method for pub/sub pattern

### Client-Side Components

#### 1. WebSocket Manager (`pages/static/websocket-manager.js` - ~210 lines)
**Purpose**: Singleton WebSocket connection with reconnection

**Key Features**:
- Exponential backoff reconnection
- Subscribe/unsubscribe pattern
- Server restart detection

**Decision**: KEEP for non-log events (job status, queue stats, etc.)

#### 2. Service Logs Component (`pages/static/common.js` - ~400 lines)
**Purpose**: Alpine.js component for service logs display

**Key Features**:
- Subscribes to 'log' and 'refresh_logs' WebSocket events
- Fetches from `/api/logs?scope=service`
- Auto-scroll, max 200 logs

**To Replace**: With SSE-based component

#### 3. Queue Page (`pages/queue.html` - ~3000+ lines)
**Purpose**: Job queue management with step logs

**Key Features**:
- Job list with expandable steps
- Step logs panel with level filtering
- WebSocket subscriptions for: refresh_logs, step_progress, job_update

**To Modify**: Replace log-related WebSocket handling with SSE

### Routes (`internal/server/routes.go`)

Current log-related routes:
- `GET /api/logs` - UnifiedLogsHandler.GetLogsHandler
- `GET /api/logs/recent` - Legacy endpoint (WSHandler.GetRecentLogsHandler)

## Code to REMOVE

### Server-Side

1. **WebSocket log broadcasting** (in `websocket.go`):
   - `broadcastUnifiedRefreshTrigger()` method
   - `log_event` subscription in `SubscribeToCrawlerEvents()`
   - `EventJobLog` subscription
   - `EventStepProgress` log-related handling

2. **Unified Log Aggregator** (`unified_aggregator.go`):
   - Entire file - SSE replaces the signal-then-fetch pattern

3. **Legacy endpoint**:
   - `GetRecentLogsHandler` in websocket.go (line 543-641)
   - Route `/api/logs/recent` in routes.go

### Client-Side

1. **WebSocket log subscriptions** (in `queue.html`):
   - `refresh_logs` subscription (line 1414-1422)
   - `refresh_step_events` subscription (line 1425-1431)

2. **Service logs WebSocket handling** (in `common.js`):
   - 'log' subscription
   - 'refresh_logs' subscription for scope=service

## Code to EXTEND

### Server-Side

1. **LogService** (`internal/logs/service.go`):
   - Add `Subscribe(ctx, jobID, opts)` method returning a channel
   - Add `PublishLog(jobID, entry)` method for real-time streaming
   - Add `PublishStatus(jobID, status)` for status changes

2. **Routes** (`internal/server/routes.go`):
   - Add `GET /api/jobs/{id}/logs/stream` - SSE for job logs
   - Add `GET /api/service/logs/stream` - SSE for service logs

3. **UnifiedLogsHandler** (`internal/handlers/unified_logs_handler.go`):
   - Add `StreamJobLogs()` method for SSE
   - Add `StreamServiceLogs()` method for SSE

### Client-Side

1. **Create new files** in `pages/static/js/`:
   - `log-stream.js` - QuaeroLogs global library
   - `log-components.js` - Alpine.js components

2. **Create CSS** in `pages/static/css/`:
   - `log-stream.css` - Log display styling

3. **Modify `head.html`**:
   - Add references to new JS/CSS files

4. **Modify `queue.html`**:
   - Replace log WebSocket handling with SSE calls

## Code to KEEP (No Changes)

### Server-Side

1. **WebSocket for non-log events**:
   - `job_status_change`, `job_created`, `job_spawn`
   - `crawler_job_progress`, `parent_job_progress`
   - `queue_stats`, `status`
   - `job_update` (for status updates, not logs)

2. **Event Service** (`internal/services/events/`):
   - Keep for internal pub/sub
   - SSE handlers will subscribe to log events

### Client-Side

1. **WebSocketManager**:
   - Keep for non-log real-time updates
   - Job status, queue stats, etc.

## Implementation Plan

### Phase 1: Server SSE Handlers (EXTEND)

1. Create `internal/handlers/sse_logs_handler.go`:
   - `StreamJobLogs()` - SSE handler for job/step logs
   - `StreamServiceLogs()` - SSE handler for service logs
   - Batching: 150ms intervals
   - Heartbeat: 5s ping
   - Status events on job/step changes

2. Extend LogService with pub/sub:
   - `logSubscribers map[string][]*LogStream`
   - Hook into existing `WriteLog()` to publish

3. Add routes:
   - `GET /api/jobs/{id}/logs/stream`
   - `GET /api/service/logs/stream`

### Phase 2: Client Library (CREATE)

1. Create `pages/static/js/log-stream.js`:
   ```javascript
   window.QuaeroLogs = {
       streamJob(jobId, options) { ... },
       streamService(options) { ... }
   }
   ```

2. Create `pages/static/js/log-components.js`:
   - Alpine.js `jobLogs` component
   - Alpine.js `serviceLogs` component (SSE-based)

3. Create `pages/static/css/log-stream.css`:
   - Terminal styling
   - Level colors

### Phase 3: Cleanup (REMOVE)

1. Remove from `websocket.go`:
   - `unifiedLogAggregator` field
   - `broadcastUnifiedRefreshTrigger()` method
   - Log-related event subscriptions

2. Delete `unified_aggregator.go`

3. Remove legacy endpoint `/api/logs/recent`

4. Update `queue.html`:
   - Remove log WebSocket subscriptions
   - Use new SSE components

5. Update `common.js`:
   - Remove WebSocket log subscriptions
   - Or replace with SSE-based approach

## File Changes Summary

| File | Action | Reason |
|------|--------|--------|
| `internal/handlers/sse_logs_handler.go` | CREATE | New SSE handlers |
| `internal/logs/service.go` | EXTEND | Add pub/sub methods |
| `internal/server/routes.go` | EXTEND | Add SSE routes |
| `internal/services/events/unified_aggregator.go` | DELETE | Replaced by SSE |
| `internal/handlers/websocket.go` | MODIFY | Remove log broadcasting |
| `pages/static/js/log-stream.js` | CREATE | Client library |
| `pages/static/js/log-components.js` | CREATE | Alpine components |
| `pages/static/css/log-stream.css` | CREATE | Styling |
| `pages/partials/head.html` | MODIFY | Add new assets |
| `pages/queue.html` | MODIFY | Use SSE for logs |
| `pages/static/common.js` | MODIFY | SSE-based service logs |

## Risks and Mitigations

1. **Risk**: Breaking existing job status updates
   - **Mitigation**: Keep WebSocket for non-log events, only replace log streaming

2. **Risk**: SSE connection limits
   - **Mitigation**: Each tab opens one connection per active view; same as WebSocket

3. **Risk**: Browser compatibility
   - **Mitigation**: SSE is well-supported (IE11 excluded, but not a target)

## Questions Resolved

1. **Should WebSocket be completely removed?**
   - NO - Keep for job status, queue stats, crawler progress
   - Only replace log-specific functionality

2. **Where should SSE handlers live?**
   - New file: `internal/handlers/sse_logs_handler.go`
   - Clean separation from existing handlers

3. **How to handle log filtering?**
   - Server-side filtering in SSE handler
   - Query params: `?step=X&level=info&limit=100`
