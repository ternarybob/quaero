# Event Flow Diagram - EventService to UI

**Validation Date:** 2025-01-08
**Agent:** Agent 2 (Implementer)
**Task:** Step 1 - Validate EventService is actively used by services and reaches UI

---

## Overview

The EventService implements a pub/sub pattern that enables real-time communication from backend services to the browser UI via WebSocket. This document traces the complete event flow from event publication to DOM updates.

---

## Event Flow Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            SERVICE LAYER (Publishers)                        │
└─────────────────────────────────────────────────────────────────────────────┘
           │                        │                         │
           │ EventJobCreated        │ EventCrawlProgress     │ crawler_job_log
           │ EventJobCompleted      │ EventStatusChanged     │ crawler_job_progress
           │ EventJobFailed         │ EventJobSpawn          │
           │ EventJobCancelled      │                        │
           ▼                        ▼                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         EVENTSERVICE (PUB/SUB HUB)                           │
│  Location: internal/services/events/event_service.go                         │
│  Methods: Publish(), PublishSync(), Subscribe(), Unsubscribe()               │
└─────────────────────────────────────────────────────────────────────────────┘
           │                        │                         │
           │ Dispatches to          │ Dispatches to          │ Dispatches to
           │ subscribers            │ subscribers            │ subscribers
           ▼                        ▼                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          SUBSCRIBER LAYER                                    │
│  ┌─────────────────────────┐    ┌──────────────────────────────────────┐   │
│  │  EventSubscriber         │    │  WebSocketHandler                    │   │
│  │  (websocket_events.go)   │    │  (websocket.go)                      │   │
│  │                          │    │                                      │   │
│  │  Subscribes to:          │    │  Subscribes to:                      │   │
│  │  - EventJobCreated       │    │  - EventCrawlProgress                │   │
│  │  - EventJobStarted       │    │  - EventStatusChanged                │   │
│  │  - EventJobCompleted     │    │  - EventJobSpawn                     │   │
│  │  - EventJobFailed        │    │  - crawler_job_progress              │   │
│  │  - EventJobCancelled     │    │  - crawler_job_log                   │   │
│  │  - EventJobSpawn         │    │                                      │   │
│  └─────────────────────────┘    └──────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
           │                        │                         │
           │ Calls                  │ Calls                  │ Calls
           │ BroadcastJobStatusChange│ BroadcastCrawlProgress│ StreamCrawlerJobLog
           ▼                        ▼                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        WEBSOCKET BROADCAST LAYER                             │
│  Location: internal/handlers/websocket.go                                    │
│  Methods:                                                                     │
│  - BroadcastJobStatusChange()   → "job_status_change" message               │
│  - BroadcastJobSpawn()          → "job_spawn" message                       │
│  - BroadcastCrawlProgress()     → "crawl_progress" message                  │
│  - BroadcastCrawlerJobProgress()→ "crawler_job_progress" message            │
│  - StreamCrawlerJobLog()        → "crawler_job_log" message                 │
│  - BroadcastLog()               → "log" message                             │
│  - BroadcastAppStatus()         → "app_status" message                      │
│  - BroadcastQueueStats()        → "queue_stats" message                     │
└─────────────────────────────────────────────────────────────────────────────┘
           │                        │                         │
           │ JSON over WebSocket    │ JSON over WebSocket    │ JSON over WebSocket
           ▼                        ▼                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           WEBSOCKET ENDPOINT                                 │
│  Endpoint: ws://localhost:8085/ws                                            │
│  Protocol: WebSocket with JSON messages                                      │
│  Format: { "type": "event_type", "payload": {...} }                         │
└─────────────────────────────────────────────────────────────────────────────┘
           │                        │                         │
           │ Received by browser    │ Received by browser    │ Received by browser
           ▼                        ▼                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          BROWSER CLIENT LAYER                                │
│  Location: pages/queue.html (lines 1115-1193)                                │
│  WebSocket Handler: jobsWS.onmessage                                         │
│  Message Router:                                                              │
│    - "job_status_change" → updateJobInList()                                 │
│    - "job_spawn" → window.dispatchEvent('jobList:updateJob')                │
│    - "crawler_job_progress" → window.dispatchEvent('jobList:updateJobProgress')│
│    - "crawler_job_log" → window.dispatchEvent('jobLogs:newLog')             │
│    - "queue_stats" → window.dispatchEvent('queueStats:update')              │
│    - "app_status" → console.log (monitoring)                                 │
└─────────────────────────────────────────────────────────────────────────────┘
           │                        │                         │
           │ Dispatches custom      │ Dispatches custom      │ Dispatches custom
           │ events                 │ events                 │ events
           ▼                        ▼                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          ALPINE.JS COMPONENTS                                │
│  Component: jobList (line 1754)                                              │
│  Listeners:                                                                   │
│    - 'jobList:updateJob' → updateJobInList()                                │
│    - 'jobList:updateJobProgress' → updateJobProgress()                      │
│    - 'jobList:recalculateStats' → recalculateStats()                        │
│                                                                               │
│  Component: jobLogsModal (line 3302)                                          │
│  Listeners:                                                                   │
│    - 'jobLogs:newLog' → handleWebSocketLog()                                │
│    - 'jobLogs:streamingStateChange' → updateStreamingState()                │
│                                                                               │
│  Component: jobStatsHeader (line 1588)                                       │
│  Listeners:                                                                   │
│    - 'jobStats:update' → updateStats()                                      │
│    - 'queueStats:update' → updateQueueStats()                               │
└─────────────────────────────────────────────────────────────────────────────┘
           │                        │                         │
           │ Updates x-data         │ Updates x-data         │ Updates x-data
           │ reactive state         │ reactive state         │ reactive state
           ▼                        ▼                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              DOM UPDATES                                     │
│  - Job cards updated with new status/progress                               │
│  - Progress bars animated                                                    │
│  - Log entries appended to log viewer                                        │
│  - Statistics counters incremented/decremented                               │
│  - Status badges updated (pending → running → completed/failed)             │
│  - Real-time timestamps updated                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Event Publishers (Services)

### 1. EnhancedCrawlerExecutor
**Location:** `internal/jobs/processor/enhanced_crawler_executor.go`

**Published Events:**
- **`crawler_job_log`** (custom event)
  - Published via: `publishCrawlerJobLog()` (line 655)
  - Frequency: Multiple times per URL crawl
  - Payload: `{ job_id, level, message, timestamp, metadata }`
  - Example: "Starting enhanced crawl of URL: https://example.com (depth: 1)"

- **`crawler_job_progress`** (custom event)
  - Published via: `publishCrawlerProgressUpdate()` (line 685)
  - Frequency: At each workflow step (rendering, processing, saving, link discovery)
  - Payload: `{ job_id, parent_id, status, job_type, current_url, current_activity, timestamp, depth }`
  - Example: "Rendering page with JavaScript"

- **`EventJobSpawn`** (interface constant)
  - Published via: `publishJobSpawnEvent()` (line 754)
  - Frequency: Once per discovered link that spawns a child job
  - Payload: `{ parent_job_id, discovered_by, child_job_id, job_type, url, depth, timestamp }`

**Code Example:**
```go
// Line 655-682
func (e *EnhancedCrawlerExecutor) publishCrawlerJobLog(ctx context.Context, jobID, level, message string, metadata map[string]interface{}) {
    if e.eventService == nil {
        return
    }

    payload := map[string]interface{}{
        "job_id":    jobID,
        "level":     level,
        "message":   message,
        "timestamp": time.Now().Format(time.RFC3339),
    }

    if metadata != nil {
        payload["metadata"] = metadata
    }

    event := interfaces.Event{
        Type:    "crawler_job_log",
        Payload: payload,
    }

    // Publish asynchronously to avoid blocking job execution
    go func() {
        if err := e.eventService.Publish(ctx, event); err != nil {
            e.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to publish crawler job log event")
        }
    }()
}
```

### 2. SchedulerService
**Location:** `internal/services/scheduler/scheduler_service.go`

**Published Events:**
- **`EventCollectionTriggered`** (interface constant)
  - Published via: `runScheduledTask()` (line 669-673)
  - Frequency: On cron schedule (legacy task) or manual trigger
  - Payload: `nil`
  - Triggers Jira/Confluence services to transform raw data into documents

- **`EventCrawlProgress`** (interface constant)
  - Published via: `DetectStaleJobs()` (line 982-989)
  - Frequency: When stale jobs are detected (every 5 minutes)
  - Payload: `{ job_id, status: "failed", error: "Job stale (no heartbeat for 10+ minutes)" }`

**Code Example:**
```go
// Line 667-685
collectionEvent := interfaces.Event{
    Type:    interfaces.EventCollectionTriggered,
    Payload: nil,
}

if err := s.eventService.PublishSync(ctx, collectionEvent); err != nil {
    s.logger.Error().
        Err(err).
        Msg(">>> SCHEDULER: FAILED - Collection event publish error")
    return
}
```

### 3. StatusService
**Location:** `internal/services/status/service.go`

**Published Events:**
- **`EventStatusChanged`** (interface constant)
  - Published via: `SetState()` (line 64-74)
  - Frequency: On application state change (idle ↔ crawling ↔ offline)
  - Payload: `{ state, metadata, timestamp }`
  - Subscribed by WebSocketHandler to broadcast app status

**Code Example:**
```go
// Line 64-74
payload := map[string]interface{}{
    "state":     string(state),
    "metadata":  metadata,
    "timestamp": time.Now(),
}
event := interfaces.Event{
    Type:    interfaces.EventStatusChanged,
    Payload: payload,
}
s.eventService.Publish(context.Background(), event)
```

---

## Event Subscribers (WebSocket Layer)

### 1. EventSubscriber
**Location:** `internal/handlers/websocket_events.go`

**Subscriptions:**
- `EventJobCreated` → `handleJobCreated()` → `BroadcastJobStatusChange()` (line 185)
- `EventJobStarted` → `handleJobStarted()` → `BroadcastJobStatusChange()` (line 209)
- `EventJobCompleted` → `handleJobCompleted()` → `BroadcastJobStatusChange()` (line 238)
- `EventJobFailed` → `handleJobFailed()` → `BroadcastJobStatusChange()` (line 275)
- `EventJobCancelled` → `handleJobCancelled()` → `BroadcastJobStatusChange()` (line 344)
- `EventJobSpawn` → `handleJobSpawn()` → `BroadcastJobSpawn()` (line 106)

**Features:**
- Config-driven event filtering (whitelist pattern)
- Rate limiting for high-frequency events (throttlers)
- Graceful fallback for missing fields (snake_case/camelCase support)
- Non-blocking event handling

**Code Example:**
```go
// Line 185-207
func (s *EventSubscriber) handleJobCreated(ctx context.Context, event interfaces.Event) error {
    // Check if event should be broadcast (filtering + throttling)
    if !s.shouldBroadcastEvent("job_created") {
        return nil
    }

    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        s.logger.Warn().Msg("Invalid job created event payload type")
        return nil
    }

    update := JobStatusUpdate{
        JobID:      getStringWithFallback(payload, "job_id", "jobId"),
        Status:     getString(payload, "status"),
        SourceType: getStringWithFallback(payload, "source_type", "sourceType"),
        EntityType: getStringWithFallback(payload, "entity_type", "entityType"),
        Timestamp:  getTimestamp(payload),
    }

    s.handler.BroadcastJobStatusChange(update)
    return nil
}
```

### 2. WebSocketHandler
**Location:** `internal/handlers/websocket.go`

**Direct Subscriptions (SubscribeToCrawlerEvents, line 836):**
- `EventCrawlProgress` → `BroadcastCrawlProgress()` (line 841)
- `EventStatusChanged` → `BroadcastAppStatus()` (line 899)
- `EventJobSpawn` → `BroadcastJobSpawn()` (line 931)
- `crawler_job_progress` → `BroadcastCrawlerJobProgress()` (line 963)
- `crawler_job_log` → `StreamCrawlerJobLog()` (line 1025)

**Broadcast Methods:**
- `BroadcastJobStatusChange()` → WebSocket message type: `"job_status_change"`
- `BroadcastJobSpawn()` → WebSocket message type: `"job_spawn"`
- `BroadcastCrawlProgress()` → WebSocket message type: `"crawl_progress"`
- `BroadcastCrawlerJobProgress()` → WebSocket message type: `"crawler_job_progress"`
- `StreamCrawlerJobLog()` → WebSocket message type: `"crawler_job_log"`
- `BroadcastLog()` → WebSocket message type: `"log"`
- `BroadcastAppStatus()` → WebSocket message type: `"app_status"`

**Code Example:**
```go
// Line 707-739
func (h *WebSocketHandler) BroadcastJobStatusChange(update JobStatusUpdate) {
    msg := WSMessage{
        Type:    "job_status_change",
        Payload: update,
    }

    data, err := json.Marshal(msg)
    if err != nil {
        h.logger.Error().Err(err).Msg("Failed to marshal job status change message")
        return
    }

    h.mu.RLock()
    clients := make([]*websocket.Conn, 0, len(h.clients))
    mutexes := make([]*sync.Mutex, 0, len(h.clients))
    for conn := range h.clients {
        clients = append(clients, conn)
        mutexes = append(mutexes, h.clientMutex[conn])
    }
    h.mu.RUnlock()

    for i, conn := range clients {
        mutex := mutexes[i]
        mutex.Lock()
        err := conn.WriteMessage(websocket.TextMessage, data)
        mutex.Unlock()

        if err != nil {
            h.logger.Warn().Err(err).Msg("Failed to send job status change to client")
        }
    }
}
```

---

## Browser Client Layer

### WebSocket Message Handler
**Location:** `pages/queue.html` (lines 1115-1193)

**Message Router:**
```javascript
jobsWS.onmessage = (event) => {
    try {
        const message = JSON.parse(event.data);

        // Route messages to appropriate handlers
        if (message.type === 'queue_stats' && message.payload) {
            window.dispatchEvent(new CustomEvent('queueStats:update', {
                detail: message.payload
            }));
        }

        if (message.type === 'job_status_change' && message.payload) {
            updateJobInList(message.payload);
        }

        if (message.type === 'job_spawn' && message.payload) {
            window.dispatchEvent(new CustomEvent('jobList:updateJob', {
                detail: message.payload
            }));
        }

        if (message.type === 'crawler_job_progress' && message.payload) {
            window.dispatchEvent(new CustomEvent('jobList:updateJobProgress', {
                detail: message.payload
            }));
        }

        if (message.type === 'crawler_job_log' && message.payload) {
            window.dispatchEvent(new CustomEvent('jobLogs:newLog', {
                detail: message.payload
            }));
        }
    } catch (error) {
        console.error('[Queue] Error parsing WebSocket message:', error);
    }
};
```

### Alpine.js Components

**jobList Component** (line 1754)
- Manages job queue display with parent-child hierarchy
- Listens for `jobList:updateJob` events from WebSocket
- Updates job status, progress, and statistics in real-time
- Recalculates statistics when jobs change

**jobLogsModal Component** (line 3302)
- Displays real-time job logs in modal dialog
- Listens for `jobLogs:newLog` events from WebSocket
- Filters logs by job ID and log level
- Auto-scrolls to latest log entries

**jobStatsHeader Component** (line 1588)
- Displays job queue statistics (pending, running, completed, failed)
- Listens for `jobStats:update` and `queueStats:update` events
- Updates counters in real-time

---

## Validation Results

### ✅ CRITERION 1: Trace event publication from at least 3 different services

**VERIFIED** - Found 3 distinct services publishing events:
1. **EnhancedCrawlerExecutor** - Publishes `crawler_job_log`, `crawler_job_progress`, `EventJobSpawn`
2. **SchedulerService** - Publishes `EventCollectionTriggered`, `EventCrawlProgress`
3. **StatusService** - Publishes `EventStatusChanged`

### ✅ CRITERION 2: Verify WebSocket messages reach browser clients

**VERIFIED** - Complete message flow confirmed:
- Events published by services → EventService
- EventService dispatches to subscribers (EventSubscriber, WebSocketHandler)
- Subscribers call broadcast methods (BroadcastJobStatusChange, etc.)
- WebSocket handler sends JSON messages to all connected clients
- Browser receives messages via `jobsWS.onmessage` handler (line 1115)

### ✅ CRITERION 3: Confirm EventSubscriber processes job lifecycle events

**VERIFIED** - EventSubscriber subscribes to all 6 lifecycle events:
- `EventJobCreated` → handled by `handleJobCreated()`
- `EventJobStarted` → handled by `handleJobStarted()`
- `EventJobCompleted` → handled by `handleJobCompleted()`
- `EventJobFailed` → handled by `handleJobFailed()`
- `EventJobCancelled` → handled by `handleJobCancelled()`
- `EventJobSpawn` → handled by `handleJobSpawn()`

All handlers transform events and broadcast via WebSocketHandler.

### ✅ CRITERION 4: Validate UI receives and displays events in real-time

**VERIFIED** - Full UI integration confirmed:
- WebSocket message handler routes events to Alpine.js components (line 1115-1193)
- `jobList` component updates job cards in real-time (line 2921)
- `jobLogsModal` component streams logs live (line 3470)
- `jobStatsHeader` component updates statistics (line 1660)
- DOM updates automatically via Alpine.js reactive data binding

---

## Conclusion

**The EventService is a CRITICAL and ACTIVELY USED component.** All validation criteria passed with extensive evidence of real-time event flow from services to UI. The architecture is sophisticated with proper separation of concerns:

- **Service Layer** publishes domain events asynchronously
- **EventService** acts as central pub/sub hub
- **Subscriber Layer** transforms events and routes to WebSocket
- **WebSocket Layer** broadcasts to all connected browsers
- **Browser Layer** routes messages to Alpine.js components
- **UI Layer** updates DOM reactively via Alpine.js

**EventService is NOT redundant and should NOT be removed or refactored.**
