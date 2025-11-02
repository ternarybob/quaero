I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Event Infrastructure (Complete)**
- 5 job lifecycle events defined in `event_service.go`: EventJobCreated, EventJobStarted, EventJobCompleted, EventJobFailed, EventJobCancelled
- Events published from `crawler.go` (started, completed) and `service.go` (created, failed, cancelled)
- EventService uses goroutines for async publishing, handlers are thread-safe

**WebSocket Infrastructure (Existing Pattern)**
- `websocket.go` has `SubscribeToCrawlerEvents()` method (lines 594-697) demonstrating the subscription pattern
- Existing broadcast methods: BroadcastCrawlProgress, BroadcastJobSpawn, BroadcastQueueStats
- Helper functions for type conversion: getString, getInt, getFloat64 (lines 700-730)
- WSMessage struct with Type and Payload fields for all WebSocket messages

**UI Current State**
- `queue.html` uses polling (lines 956-969) to refresh job list every 5 seconds when running jobs exist
- WebSocket already used for queue_stats updates (lines 1046-1094)
- No job_status_change handler exists yet - this is what we're adding

**Missing Components**
1. **JobStatusUpdate struct** - Need to define payload structure for job lifecycle events
2. **BroadcastJobStatusChange method** - Need to add to websocket.go
3. **EventSubscriber** - Need to create in websocket_events.go with transformers
4. **Initialization** - Need to wire up EventSubscriber in app.go

**Design Decisions**
- Follow existing pattern from SubscribeToCrawlerEvents (subscribe in constructor, transform payload, broadcast)
- Use separate transformers for each event type (cleaner than monolithic switch)
- JobStatusUpdate struct should include all fields needed by UI (job_id, status, counts, timestamps)
- EventSubscriber doesn't need to be stored on App struct (subscriptions persist until EventService closes)

### Approach

**Three-Component Architecture:**

1. **WebSocket Message Structure** - Add JobStatusUpdate struct to websocket.go with fields for all job lifecycle states
2. **Event Subscriber** - Create websocket_events.go with EventSubscriber that subscribes to 5 job lifecycle events and transforms them
3. **Initialization** - Wire up EventSubscriber in app.go after WebSocket handler creation

**Key Design Principles:**
- Reuse existing helper functions (getString, getInt) for type-safe payload extraction
- Follow established broadcast pattern (lock management, error handling)
- Keep transformers simple and focused (one per event type)
- No storage on App struct needed (subscriptions are fire-and-forget)

### Reasoning

I explored the codebase by reading websocket.go (existing patterns), event_service.go (pub/sub mechanics), crawler_job.go (job model), queue.html (UI expectations), and app.go (initialization flow). I searched for existing update structs, broadcast methods, and WebSocket message handlers. I confirmed that the 5 job lifecycle events are already defined and published, and identified the exact pattern to follow from SubscribeToCrawlerEvents. I verified that helper functions exist for type conversion and that the WebSocket handler is created in initHandlers with EventService available.

## Mermaid Diagram

sequenceDiagram
    participant Service as Crawler Service
    participant EventBus as Event Service
    participant Subscriber as EventSubscriber
    participant Handler as WebSocketHandler
    participant Client as Browser UI

    Note over Service,Client: Initialization Phase
    Service->>EventBus: Created during initServices()
    Handler->>EventBus: Created with EventService ref
    Subscriber->>EventBus: NewEventSubscriber(handler, eventService)
    Subscriber->>EventBus: Subscribe(EventJobCreated, handleJobCreated)
    Subscriber->>EventBus: Subscribe(EventJobStarted, handleJobStarted)
    Subscriber->>EventBus: Subscribe(EventJobCompleted, handleJobCompleted)
    Subscriber->>EventBus: Subscribe(EventJobFailed, handleJobFailed)
    Subscriber->>EventBus: Subscribe(EventJobCancelled, handleJobCancelled)
    Note over Subscriber: All subscriptions active

    Note over Service,Client: Job Created Event
    Service->>EventBus: Publish(EventJobCreated, payload)
    EventBus->>Subscriber: handleJobCreated(ctx, event)
    Subscriber->>Subscriber: Extract payload fields
    Subscriber->>Subscriber: Create JobStatusUpdate struct
    Subscriber->>Handler: BroadcastJobStatusChange(update)
    Handler->>Handler: Marshal to WSMessage
    Handler->>Client: WebSocket: {type: "job_status_change", payload: {...}}
    Client->>Client: Update job list UI

    Note over Service,Client: Job Started Event
    Service->>EventBus: Publish(EventJobStarted, payload)
    EventBus->>Subscriber: handleJobStarted(ctx, event)
    Subscriber->>Subscriber: Transform to JobStatusUpdate
    Subscriber->>Handler: BroadcastJobStatusChange(update)
    Handler->>Client: WebSocket: {type: "job_status_change", payload: {...}}
    Client->>Client: Show job as "running"

    Note over Service,Client: Job Completed Event
    Service->>EventBus: Publish(EventJobCompleted, payload)
    EventBus->>Subscriber: handleJobCompleted(ctx, event)
    Subscriber->>Subscriber: Transform with result counts
    Subscriber->>Handler: BroadcastJobStatusChange(update)
    Handler->>Client: WebSocket: {type: "job_status_change", payload: {...}}
    Client->>Client: Show completion with counts

    Note over Service,Client: Job Failed Event
    Service->>EventBus: Publish(EventJobFailed, payload)
    EventBus->>Subscriber: handleJobFailed(ctx, event)
    Subscriber->>Subscriber: Transform with error message
    Subscriber->>Handler: BroadcastJobStatusChange(update)
    Handler->>Client: WebSocket: {type: "job_status_change", payload: {...}}
    Client->>Client: Show error state

    Note over Service,Client: Job Cancelled Event
    Service->>EventBus: Publish(EventJobCancelled, payload)
    EventBus->>Subscriber: handleJobCancelled(ctx, event)
    Subscriber->>Subscriber: Transform with partial counts
    Subscriber->>Handler: BroadcastJobStatusChange(update)
    Handler->>Client: WebSocket: {type: "job_status_change", payload: {...}}
    Client->>Client: Show cancelled state

## Proposed File Changes

### internal\handlers\websocket.go(MODIFY)

**Location 1: After JobSpawnUpdate struct (line 126)**

Add new JobStatusUpdate struct for job lifecycle events:

```go
type JobStatusUpdate struct {
    JobID          string    `json:"job_id"`
    Status         string    `json:"status"`           // "pending", "running", "completed", "failed", "cancelled"
    SourceType     string    `json:"source_type"`      // "jira", "confluence", "github"
    EntityType     string    `json:"entity_type"`      // "project", "issue", "space", "page"
    ResultCount    int       `json:"result_count"`     // Documents successfully processed
    FailedCount    int       `json:"failed_count"`     // Documents that failed
    TotalURLs      int       `json:"total_urls"`       // Total URLs discovered
    CompletedURLs  int       `json:"completed_urls"`   // URLs completed
    PendingURLs    int       `json:"pending_urls"`     // URLs still in queue
    Error          string    `json:"error,omitempty"`  // Error message for failed jobs
    Duration       float64   `json:"duration,omitempty"` // Duration in seconds for completed jobs
    Timestamp      time.Time `json:"timestamp"`        // Event timestamp
}
```

**Rationale:** Comprehensive struct covering all job lifecycle states. Includes progress metrics (counts, URLs) for UI display, error details for failed jobs, and duration for completed jobs. Follows naming convention of existing update structs (CrawlProgressUpdate, JobSpawnUpdate).

**Location 2: After BroadcastJobSpawn method (line 591)**

Add BroadcastJobStatusChange method:

```go
// BroadcastJobStatusChange sends job status change events to all connected clients
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

**Rationale:** Follows exact pattern of existing broadcast methods (BroadcastCrawlProgress, BroadcastJobSpawn). Uses same locking strategy (RLock for reading clients, per-connection mutex for writing). Message type "job_status_change" will be handled by UI in subsequent phase.

### internal\handlers\websocket_events.go(NEW)

References: 

- internal\handlers\websocket.go(MODIFY)
- internal\interfaces\event_service.go

**Create new file for event subscription and transformation**

**Package and Imports:**
```go
package handlers

import (
    "context"
    "time"
    "github.com/ternarybob/arbor"
    "github.com/ternarybob/quaero/internal/interfaces"
)
```

**EventSubscriber struct:**
```go
// EventSubscriber manages subscriptions to job lifecycle events and broadcasts them via WebSocket
type EventSubscriber struct {
    handler      *WebSocketHandler
    eventService interfaces.EventService
    logger       arbor.ILogger
}
```

**Constructor:**
```go
// NewEventSubscriber creates and initializes an event subscriber
// Automatically subscribes to all job lifecycle events
func NewEventSubscriber(handler *WebSocketHandler, eventService interfaces.EventService, logger arbor.ILogger) *EventSubscriber {
    s := &EventSubscriber{
        handler:      handler,
        eventService: eventService,
        logger:       logger,
    }
    
    // Subscribe to all job lifecycle events
    s.SubscribeAll()
    
    return s
}
```

**SubscribeAll method:**
```go
// SubscribeAll registers subscriptions for all job lifecycle events
func (s *EventSubscriber) SubscribeAll() {
    // Subscribe to job creation events
    s.eventService.Subscribe(interfaces.EventJobCreated, s.handleJobCreated)
    
    // Subscribe to job start events
    s.eventService.Subscribe(interfaces.EventJobStarted, s.handleJobStarted)
    
    // Subscribe to job completion events
    s.eventService.Subscribe(interfaces.EventJobCompleted, s.handleJobCompleted)
    
    // Subscribe to job failure events
    s.eventService.Subscribe(interfaces.EventJobFailed, s.handleJobFailed)
    
    // Subscribe to job cancellation events
    s.eventService.Subscribe(interfaces.EventJobCancelled, s.handleJobCancelled)
    
    s.logger.Info().Msg("EventSubscriber registered for all job lifecycle events")
}
```

**Event Handlers (5 transformers):**

1. **handleJobCreated:**
```go
func (s *EventSubscriber) handleJobCreated(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        s.logger.Warn().Msg("Invalid job created event payload type")
        return nil
    }
    
    update := JobStatusUpdate{
        JobID:      getString(payload, "job_id"),
        Status:     getString(payload, "status"),
        SourceType: getString(payload, "source_type"),
        EntityType: getString(payload, "entity_type"),
        Timestamp:  time.Now(),
    }
    
    s.handler.BroadcastJobStatusChange(update)
    return nil
}
```

2. **handleJobStarted:**
```go
func (s *EventSubscriber) handleJobStarted(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        s.logger.Warn().Msg("Invalid job started event payload type")
        return nil
    }
    
    update := JobStatusUpdate{
        JobID:      getString(payload, "job_id"),
        Status:     getString(payload, "status"),
        SourceType: getString(payload, "source_type"),
        EntityType: getString(payload, "entity_type"),
        Timestamp:  time.Now(),
    }
    
    s.handler.BroadcastJobStatusChange(update)
    return nil
}
```

3. **handleJobCompleted:**
```go
func (s *EventSubscriber) handleJobCompleted(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        s.logger.Warn().Msg("Invalid job completed event payload type")
        return nil
    }
    
    update := JobStatusUpdate{
        JobID:         getString(payload, "job_id"),
        Status:        getString(payload, "status"),
        SourceType:    getString(payload, "source_type"),
        EntityType:    getString(payload, "entity_type"),
        ResultCount:   getInt(payload, "result_count"),
        FailedCount:   getInt(payload, "failed_count"),
        TotalURLs:     getInt(payload, "total_urls"),
        CompletedURLs: getInt(payload, "total_urls"), // All URLs completed
        PendingURLs:   0,                              // No pending URLs
        Duration:      getFloat64(payload, "duration_seconds"),
        Timestamp:     time.Now(),
    }
    
    s.handler.BroadcastJobStatusChange(update)
    return nil
}
```

4. **handleJobFailed:**
```go
func (s *EventSubscriber) handleJobFailed(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        s.logger.Warn().Msg("Invalid job failed event payload type")
        return nil
    }
    
    update := JobStatusUpdate{
        JobID:         getString(payload, "job_id"),
        Status:        getString(payload, "status"),
        SourceType:    getString(payload, "source_type"),
        EntityType:    getString(payload, "entity_type"),
        ResultCount:   getInt(payload, "result_count"),
        FailedCount:   getInt(payload, "failed_count"),
        CompletedURLs: getInt(payload, "completed_urls"),
        PendingURLs:   getInt(payload, "pending_urls"),
        Error:         getString(payload, "error"),
        Timestamp:     time.Now(),
    }
    
    s.handler.BroadcastJobStatusChange(update)
    return nil
}
```

5. **handleJobCancelled:**
```go
func (s *EventSubscriber) handleJobCancelled(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        s.logger.Warn().Msg("Invalid job cancelled event payload type")
        return nil
    }
    
    update := JobStatusUpdate{
        JobID:         getString(payload, "job_id"),
        Status:        getString(payload, "status"),
        SourceType:    getString(payload, "source_type"),
        EntityType:    getString(payload, "entity_type"),
        ResultCount:   getInt(payload, "result_count"),
        FailedCount:   getInt(payload, "failed_count"),
        CompletedURLs: getInt(payload, "completed_urls"),
        PendingURLs:   getInt(payload, "pending_urls"),
        Timestamp:     time.Now(),
    }
    
    s.handler.BroadcastJobStatusChange(update)
    return nil
}
```

**Rationale:** 
- Follows exact pattern from SubscribeToCrawlerEvents in websocket.go (lines 594-697)
- Each handler is focused and simple (single responsibility)
- Reuses existing helper functions (getString, getInt, getFloat64) from websocket.go
- Handlers return nil on invalid payload (non-fatal, just log warning)
- Timestamp set to time.Now() for consistency (event may have been queued)
- No need for Close() method - subscriptions persist until EventService closes

### internal\app\app.go(MODIFY)

References: 

- internal\handlers\websocket_events.go(NEW)
- internal\handlers\websocket.go(MODIFY)
- internal\services\events\event_service.go

**Location: After WebSocket handler creation in initHandlers (line 808)**

Add EventSubscriber initialization immediately after WebSocketHandler creation:

1. After line 808 (`a.WSHandler = handlers.NewWebSocketHandler(a.EventService, a.Logger)`)
2. Before line 809 (`a.AuthHandler = handlers.NewAuthHandler(...)`)
3. Add the following code:

```go
// Initialize EventSubscriber for job lifecycle events
// Subscribes to EventJobCreated, EventJobStarted, EventJobCompleted, EventJobFailed, EventJobCancelled
// Transforms events and broadcasts to WebSocket clients via BroadcastJobStatusChange
_ = handlers.NewEventSubscriber(a.WSHandler, a.EventService, a.Logger)
a.Logger.Info().Msg("EventSubscriber initialized for job lifecycle events")
```

**Rationale:**
- Initialize immediately after WebSocketHandler creation (handler must exist before subscriber)
- EventService is already available (created in initServices at line 322)
- Use blank identifier `_` since we don't need to store the subscriber (subscriptions persist)
- NewEventSubscriber constructor calls SubscribeAll() internally, so subscriptions are active immediately
- Log message confirms initialization for debugging
- No cleanup needed in App.Close() - EventService.Close() unsubscribes all handlers automatically

**Alternative considered:** Store subscriber on App struct for explicit cleanup
- **Rejected:** EventService.Close() already clears all subscriptions (line 227 in event_service.go)
- Storing subscriber would add unnecessary complexity with no benefit
- Following pattern from SubscribeToCrawlerEvents which also doesn't store reference