# Event Publishers and Subscribers - Complete Inventory

**Validation Date:** 2025-01-08
**Agent:** Agent 2 (Implementer)
**Task:** Step 1 - Validate EventService usage

---

## Event Publishers (Services that call EventService.Publish)

### 1. EnhancedCrawlerExecutor
**File:** `internal/jobs/processor/enhanced_crawler_executor.go`
**Initialization:** Created per job execution by ParentJobExecutor

| Event Type | Method | Line | Frequency | Payload Fields |
|------------|--------|------|-----------|----------------|
| `crawler_job_log` | `publishCrawlerJobLog()` | 655 | ~10-20 per URL | `job_id`, `level`, `message`, `timestamp`, `metadata` |
| `crawler_job_progress` | `publishCrawlerProgressUpdate()` | 685 | ~5-8 per URL | `job_id`, `parent_id`, `status`, `job_type`, `current_url`, `current_activity`, `timestamp`, `depth` |
| `EventJobSpawn` | `publishJobSpawnEvent()` | 754 | Once per discovered link | `parent_job_id`, `discovered_by`, `child_job_id`, `job_type`, `url`, `depth`, `timestamp` |

**Usage Pattern:**
```go
// All event publications are asynchronous (non-blocking)
go func() {
    if err := e.eventService.Publish(ctx, event); err != nil {
        e.logger.Warn().Err(err).Msg("Failed to publish event")
    }
}()
```

**Context:**
- Used during enhanced web crawling with ChromeDP
- Publishes real-time progress updates as jobs crawl URLs
- Spawns child jobs for discovered links
- All logs use parent job correlation ID for aggregation

---

### 2. SchedulerService
**File:** `internal/services/scheduler/scheduler_service.go`
**Initialization:** Created in `app.go:178` with database persistence

| Event Type | Method | Line | Frequency | Payload Fields |
|------------|--------|------|-----------|----------------|
| `EventCollectionTriggered` | `runScheduledTask()` | 669-673 | Cron schedule (legacy) | `nil` |
| `EventCollectionTriggered` | `TriggerCollectionNow()` | 231-236 | Manual trigger | `nil` |
| `EventCrawlProgress` | `DetectStaleJobs()` | 982-989 | Every 5 minutes | `job_id`, `status: "failed"`, `error: "Job stale..."` |

**Usage Pattern:**
```go
// Synchronous publication for collection events (blocks until all subscribers complete)
if err := s.eventService.PublishSync(ctx, collectionEvent); err != nil {
    s.logger.Error().Err(err).Msg("Collection event publish error")
    return
}

// Asynchronous publication for progress events
_ = s.eventService.Publish(ctx, event)
```

**Context:**
- Orchestrates scheduled data collection from Jira/Confluence
- Detects and marks stale jobs (no heartbeat for 10+ minutes)
- Publishes synchronously for collection to ensure handlers complete before continuing

---

### 3. StatusService
**File:** `internal/services/status/service.go`
**Initialization:** Created in `app.go:134`

| Event Type | Method | Line | Frequency | Payload Fields |
|------------|--------|------|-----------|----------------|
| `EventStatusChanged` | `SetState()` | 64-74 | On state change | `state`, `metadata`, `timestamp` |

**Usage Pattern:**
```go
// Asynchronous publication with background context
s.eventService.Publish(context.Background(), event)
```

**Context:**
- Tracks application state transitions (idle ↔ crawling ↔ offline)
- Broadcasts state changes for UI status indicators
- Subscribed by StatusService itself to auto-update on crawler events

---

### 4. ParentJobExecutor (Likely)
**File:** `internal/jobs/processor/parent_job_executor.go`
**Note:** Not fully traced, but likely publishes `EventJobCreated`, `EventJobStarted`, `EventJobCompleted`, `EventJobFailed`, `EventJobCancelled`

**Expected Events:**
- Job lifecycle events (created, started, completed, failed, cancelled)
- Parent job progress updates
- Child job monitoring events

---

## Event Subscribers (Services that call EventService.Subscribe)

### 1. EventSubscriber
**File:** `internal/handlers/websocket_events.go`
**Initialization:** Created in `app.go:282`
**Purpose:** Bridges EventService to WebSocketHandler for real-time UI updates

| Subscribed Event | Handler Method | Calls WebSocket Method | Line |
|------------------|----------------|------------------------|------|
| `EventJobCreated` | `handleJobCreated()` | `BroadcastJobStatusChange()` | 85 |
| `EventJobStarted` | `handleJobStarted()` | `BroadcastJobStatusChange()` | 88 |
| `EventJobCompleted` | `handleJobCompleted()` | `BroadcastJobStatusChange()` | 91 |
| `EventJobFailed` | `handleJobFailed()` | `BroadcastJobStatusChange()` | 94 |
| `EventJobCancelled` | `handleJobCancelled()` | `BroadcastJobStatusChange()` | 97 |
| `EventJobSpawn` | `handleJobSpawn()` | `BroadcastJobSpawn()` | 100 |

**Features:**
- Config-driven event filtering (whitelist pattern via `allowed_events`)
- Rate limiting for high-frequency events (throttle intervals configurable)
- Graceful payload field extraction with snake_case/camelCase fallbacks
- Non-blocking event handling (returns `nil` error for invalid payloads)

**Configuration:**
```toml
[websocket]
allowed_events = ["job_created", "job_started", "job_completed", "job_failed", "job_spawn"]
throttle_intervals = { "job_spawn" = "100ms", "job_progress" = "500ms" }
```

**Code Example:**
```go
// Line 131-149
func (s *EventSubscriber) shouldBroadcastEvent(eventType string) bool {
    // Check whitelist (empty allowedEvents = allow all)
    if len(s.allowedEvents) > 0 && !s.allowedEvents[eventType] {
        return false
    }

    // Check throttling
    if limiter, ok := s.throttlers[eventType]; ok {
        if !limiter.Allow() {
            s.logger.Debug().
                Str("event_type", eventType).
                Msg("Event throttled - rate limit exceeded")
            return false
        }
    }

    return true
}
```

---

### 2. WebSocketHandler
**File:** `internal/handlers/websocket.go`
**Initialization:** Created in `app.go:274`
**Purpose:** Direct subscription to crawler-specific events for legacy compatibility

| Subscribed Event | Handler Method | WebSocket Message Type | Line |
|------------------|----------------|------------------------|------|
| `EventCrawlProgress` | Anonymous function | `"crawl_progress"` | 841 |
| `EventStatusChanged` | Anonymous function | `"app_status"` | 899 |
| `EventJobSpawn` | Anonymous function | `"job_spawn"` | 931 |
| `crawler_job_progress` | Anonymous function | `"crawler_job_progress"` | 963 |
| `crawler_job_log` | Anonymous function | `"crawler_job_log"` | 1025 |

**Subscription Pattern:**
```go
// Line 841-897 (EventCrawlProgress handler)
h.eventService.Subscribe(interfaces.EventCrawlProgress, func(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        h.logger.Warn().Msg("Invalid crawl progress event payload type")
        return nil
    }

    // Check whitelist (empty allowedEvents = allow all)
    if len(h.allowedEvents) > 0 && !h.allowedEvents["crawl_progress"] {
        return nil
    }

    // Throttle crawl progress events to prevent WebSocket flooding
    if h.crawlProgressThrottler != nil && !h.crawlProgressThrottler.Allow() {
        // Event throttled, skip broadcasting
        return nil
    }

    // Convert to CrawlProgressUpdate struct
    progress := CrawlProgressUpdate{
        JobID:         getString(payload, "job_id"),
        SourceType:    getString(payload, "source_type"),
        // ... more fields
    }

    // Broadcast to all clients
    h.BroadcastCrawlProgress(progress)
    return nil
})
```

**Features:**
- Direct event-to-WebSocket routing (no intermediary)
- Throttling for high-frequency events (`crawl_progress`, `job_spawn`)
- Payload transformation and validation
- Graceful error handling (logs warnings, doesn't crash)

---

### 3. StatusService (Self-Subscription)
**File:** `internal/services/status/service.go`
**Initialization:** Called after StatusService creation in `app.go`

| Subscribed Event | Purpose | Line |
|------------------|---------|------|
| `EventCrawlProgress` | Auto-update app state based on crawler events | 98 |

**Purpose:**
- Automatically transitions app state from idle → crawling when jobs start
- Transitions back to idle when jobs complete/fail/cancel
- Provides centralized state management for the application

**Code Example:**
```go
// Line 95-133
func (s *Service) SubscribeToCrawlerEvents() {
    s.eventService.Subscribe(interfaces.EventCrawlProgress, func(ctx context.Context, event interfaces.Event) error {
        payload, ok := event.Payload.(map[string]interface{})
        if !ok {
            return nil
        }

        status, ok := payload["status"].(string)
        if !ok {
            return nil
        }

        switch status {
        case "started", "running":
            // Extract job information
            metadata := map[string]interface{}{}
            if jobID, ok := payload["job_id"].(string); ok {
                metadata["active_job_id"] = jobID
            }
            s.SetState(StateCrawling, metadata)

        case "completed", "failed", "cancelled":
            // Clear metadata and return to idle
            s.SetState(StateIdle, nil)
        }

        return nil
    })
}
```

---

## WebSocket Message Types Sent to Browser

### Message Format
```json
{
  "type": "message_type",
  "payload": { /* event-specific data */ }
}
```

### Message Type Catalog

| Message Type | Source | Broadcast Method | Purpose |
|--------------|--------|------------------|---------|
| `"job_status_change"` | EventSubscriber | `BroadcastJobStatusChange()` | Job lifecycle updates (created, started, completed, failed, cancelled) |
| `"job_spawn"` | EventSubscriber / WebSocketHandler | `BroadcastJobSpawn()` | Child job creation notifications |
| `"crawl_progress"` | WebSocketHandler | `BroadcastCrawlProgress()` | Legacy crawler progress (deprecated in favor of crawler_job_progress) |
| `"crawler_job_progress"` | WebSocketHandler | `BroadcastCrawlerJobProgress()` | Comprehensive crawler job progress with parent-child stats and link metrics |
| `"crawler_job_log"` | WebSocketHandler | `StreamCrawlerJobLog()` | Real-time job log streaming with correlation ID |
| `"log"` | LogService (via BroadcastLog) | `BroadcastLog()` | General application logs |
| `"app_status"` | WebSocketHandler | `BroadcastAppStatus()` | Application state changes (idle, crawling, offline) |
| `"queue_stats"` | Queue Manager (disabled) | `BroadcastQueueStats()` | Queue statistics (currently commented out) |
| `"status"` | WebSocketHandler | `BroadcastStatus()` | Server heartbeat and connection status |
| `"auth"` | AuthHandler | `BroadcastAuth()` | Authentication updates from Chrome extension |

---

## Event Type Constants

**File:** `internal/interfaces/events.go`

| Constant | Value | Usage |
|----------|-------|-------|
| `EventCollectionTriggered` | `"collection_triggered"` | Scheduled data collection from Jira/Confluence |
| `EventEmbeddingTriggered` | `"embedding_triggered"` | Scheduled embedding generation for documents |
| `EventDocumentForceSync` | `"document_force_sync"` | Manual document re-sync request |
| `EventCrawlProgress` | `"crawl_progress"` | Legacy crawler progress (deprecated) |
| `EventStatusChanged` | `"status_changed"` | Application state transitions |
| `EventSourceCreated` | `"source_created"` | New data source added |
| `EventSourceUpdated` | `"source_updated"` | Data source configuration updated |
| `EventSourceDeleted` | `"source_deleted"` | Data source removed |
| `EventJobProgress` | `"job_progress"` | Generic job progress (unused - replaced by crawler_job_progress) |
| `EventJobSpawn` | `"job_spawn"` | Child job creation |
| `EventJobCreated` | `"job_created"` | Job lifecycle: created |
| `EventJobStarted` | `"job_started"` | Job lifecycle: started |
| `EventJobCompleted` | `"job_completed"` | Job lifecycle: completed |
| `EventJobFailed` | `"job_failed"` | Job lifecycle: failed |
| `EventJobCancelled` | `"job_cancelled"` | Job lifecycle: cancelled |

**Custom Event Types (not in constants):**
- `"crawler_job_log"` - Real-time job log streaming
- `"crawler_job_progress"` - Enhanced crawler progress with child stats

---

## Subscriber Count per Event Type

| Event Type | Subscriber Count | Subscribers |
|------------|------------------|-------------|
| `EventJobCreated` | 1 | EventSubscriber |
| `EventJobStarted` | 1 | EventSubscriber |
| `EventJobCompleted` | 1 | EventSubscriber |
| `EventJobFailed` | 1 | EventSubscriber |
| `EventJobCancelled` | 1 | EventSubscriber |
| `EventJobSpawn` | 2 | EventSubscriber, WebSocketHandler |
| `EventCrawlProgress` | 2 | WebSocketHandler, StatusService |
| `EventStatusChanged` | 1 | WebSocketHandler |
| `crawler_job_progress` | 1 | WebSocketHandler |
| `crawler_job_log` | 1 | WebSocketHandler |

---

## Publisher Count per Event Type

| Event Type | Publisher Count | Publishers |
|------------|-----------------|------------|
| `crawler_job_log` | 1 | EnhancedCrawlerExecutor |
| `crawler_job_progress` | 1 | EnhancedCrawlerExecutor |
| `EventJobSpawn` | 1 | EnhancedCrawlerExecutor |
| `EventCollectionTriggered` | 1 | SchedulerService |
| `EventCrawlProgress` | 1 | SchedulerService (stale job detection) |
| `EventStatusChanged` | 1 | StatusService |
| `EventJobCreated` | 1+ | ParentJobExecutor (not fully traced) |
| `EventJobStarted` | 1+ | ParentJobExecutor (not fully traced) |
| `EventJobCompleted` | 1+ | ParentJobExecutor (not fully traced) |
| `EventJobFailed` | 1+ | ParentJobExecutor (not fully traced) |
| `EventJobCancelled` | 1+ | ParentJobExecutor (not fully traced) |

---

## Initialization Sequence (from app.go)

```
1. EventService created (line 121)
   ↓
2. WebSocketHandler created with EventService (line 274)
   ↓
3. WebSocketHandler.SubscribeToCrawlerEvents() called (line 836)
   ↓
4. EventSubscriber created with WebSocketHandler and EventService (line 282)
   ↓
5. EventSubscriber.SubscribeAll() called automatically in constructor (line 71)
   ↓
6. StatusService created with EventService (line 134)
   ↓
7. StatusService.SubscribeToCrawlerEvents() called (line 95)
   ↓
8. Services start publishing events during normal operation
```

**Critical Ordering:**
- EventService MUST be created before any subscribers
- WebSocketHandler MUST be created before EventSubscriber (dependency)
- Subscribers MUST be initialized before any events are published

---

## Configuration and Throttling

### EventSubscriber Configuration
```go
// From websocket_events.go:25-63
type EventSubscriber struct {
    handler       *WebSocketHandler
    eventService  interfaces.EventService
    logger        arbor.ILogger
    allowedEvents map[string]bool          // Whitelist of events to broadcast (empty = allow all)
    throttlers    map[string]*rate.Limiter // Rate limiters for high-frequency events
    config        *common.WebSocketConfig
}
```

### WebSocketHandler Configuration
```go
// From websocket.go:37-47
type WebSocketHandler struct {
    logger                 arbor.ILogger
    clients                map[*websocket.Conn]bool
    clientMutex            map[*websocket.Conn]*sync.Mutex
    mu                     sync.RWMutex
    authLoader             AuthLoader
    eventService           interfaces.EventService
    crawlProgressThrottler *rate.Limiter   // Rate limiter for crawl_progress events
    jobSpawnThrottler      *rate.Limiter   // Rate limiter for job_spawn events
    allowedEvents          map[string]bool // Whitelist of events to broadcast (empty = allow all)
}
```

### Throttle Configuration Example
```toml
[websocket]
allowed_events = [
    "job_created",
    "job_started",
    "job_completed",
    "job_failed",
    "job_cancelled",
    "job_spawn",
    "crawler_job_progress",
    "crawler_job_log"
]

[websocket.throttle_intervals]
crawl_progress = "500ms"      # Max 2 events per second
job_spawn = "100ms"           # Max 10 events per second
crawler_job_progress = "200ms" # Max 5 events per second
```

---

## Summary Statistics

- **Total Publishers:** 3+ services (4+ if ParentJobExecutor is included)
- **Total Subscribers:** 3 components (EventSubscriber, WebSocketHandler, StatusService)
- **Total Event Types:** 15 defined constants + 2 custom types = 17 total
- **Total WebSocket Message Types:** 10 distinct message types
- **Most Active Publisher:** EnhancedCrawlerExecutor (3 event types, ~20-30 events per URL crawl)
- **Most Subscribed Event:** `EventJobSpawn` (2 subscribers)
- **Throttled Events:** 3 event types (crawl_progress, job_spawn, crawler_job_progress - configurable)

---

## Redundancy Analysis

### Duplicate Event Handling
**Issue:** Both EventSubscriber and WebSocketHandler subscribe to `EventJobSpawn` and `EventCrawlProgress`

**Reason:**
- EventSubscriber: Clean, unified pattern for all job lifecycle events
- WebSocketHandler: Legacy direct subscription for crawler events

**Impact:** Low - Both paths work correctly, but adds complexity

**Recommendation:** Consolidate into EventSubscriber pattern (see Step 4 of plan)

### Multiple Log Broadcast Methods
**Issue:** Three different log broadcasting methods:
- `BroadcastLog()` - General logs
- `StreamCrawlerJobLog()` - Crawler-specific logs with metadata
- `SendLog()` - Helper method

**Reason:** Different use cases evolved over time

**Impact:** Low - Each serves a slightly different purpose

**Recommendation:** Consider unified method with optional metadata (see Step 4 of plan)

---

## Conclusion

The EventService pub/sub architecture is **extensively used and actively maintained**. The inventory shows:

✅ **3+ distinct service publishers** with varied event types
✅ **3 dedicated subscribers** with clear responsibilities
✅ **17 event types** covering job lifecycle, progress, and application state
✅ **10 WebSocket message types** reaching the browser UI
✅ **Comprehensive configuration** for filtering and throttling
✅ **Proper initialization sequence** preventing race conditions

**EventService is NOT redundant** - it is the backbone of real-time UI updates and inter-service communication.
