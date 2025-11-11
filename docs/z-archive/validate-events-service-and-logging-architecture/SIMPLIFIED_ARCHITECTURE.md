# Simplified Architecture: LogService Publishes Events

## User's Solution (CORRECT ✅)

**Principle:** Clean separation of concerns

1. **Services:** Only use logger (no direct event publishing)
2. **LogService:** Consumes logs from channel, publishes events at configured level
3. **WebSocket:** Only subscribes to events (no direct log broadcasting)

## Benefits

### 1. Services Stay Simple
```go
// Services only do this:
jobLogger := logger.WithCorrelationId(job.ID)
jobLogger.Info().Msg("Processing started")
jobLogger.Warn().Msg("Retrying connection")
jobLogger.Error().Msg("Failed to process")

// NO MORE: eventService.Publish(...)
```

### 2. LogService Controls Event Publishing
```go
// LogService consumer decides what becomes events
func (s *Service) consumer() {
    for batch := range s.logBatchChannel {
        for _, event := range batch {
            jobID := event.CorrelationID

            // 1. Always save to database
            logEntry := s.transformEvent(event)
            s.storage.AppendLog(ctx, jobID, logEntry)

            // 2. Publish as event ONLY if level >= configured threshold
            if s.shouldPublishEvent(event.Level) {
                s.eventService.Publish(ctx, interfaces.Event{
                    Type: "log_event",
                    Payload: map[string]interface{}{
                        "job_id":   jobID,
                        "level":    event.Level,
                        "message":  event.Message,
                        "timestamp": event.Timestamp,
                    },
                })
            }
        }
    }
}

func (s *Service) shouldPublishEvent(level arbor.Level) bool {
    // Configuration determines threshold
    // Example: minLevel = Info → publish Info, Warn, Error (not Debug)
    return level >= s.config.MinEventLevel
}
```

### 3. WebSocket Stays Clean
```go
// WebSocket ONLY subscribes to events
func (h *WebSocketHandler) SubscribeToEvents() {
    h.eventService.Subscribe("log_event", func(ctx context.Context, event interfaces.Event) error {
        payload := event.Payload.(map[string]interface{})

        // Broadcast to UI clients
        h.broadcast(WSMessage{
            Type: "log",
            Payload: payload,
        })
        return nil
    })
}
```

## Configuration

Add to `quaero.toml`:

```toml
[logging]
# Minimum log level to publish as events to UI (debug, info, warn, error)
# Logs below this level go to database only, not to UI
min_event_level = "info"

# Optional: Per-job-type overrides
[logging.event_levels]
crawler_url = "debug"     # Crawler jobs show all logs in UI
corpus_summary = "info"    # Summary jobs show info and above
```

## Flow Diagram

### Before (Inconsistent)
```
Service A
  ├─> logger.Info()           → DB ✅
  └─> eventService.Publish()  → UI ✅

Service B
  └─> logger.Info()           → DB ✅
                                 UI ❌ (missing!)

Service C
  └─> eventService.Publish()  → UI ✅
                                 DB ❌ (missing!)
```

### After (Consistent)
```
All Services
  └─> logger.Info()
        └─> Arbor Channel
              └─> LogService Consumer
                    ├─> Database (always)
                    └─> EventService.Publish() (if level >= threshold)
                          └─> WebSocket → UI

Result: ALL services get consistent behavior
```

## Implementation Plan

### Step 1: Add Configuration

**File:** `internal/common/config.go`

```go
type LoggingConfig struct {
    MinEventLevel string            `toml:"min_event_level"` // "debug", "info", "warn", "error"
    EventLevels   map[string]string `toml:"event_levels"`    // Per-job-type overrides
}

type Config struct {
    // ... existing fields ...
    Logging LoggingConfig `toml:"logging"`
}
```

### Step 2: Modify LogService Consumer

**File:** `internal/logs/service.go`

```go
type Service struct {
    storage         interfaces.JobLogStorage
    jobStorage      interfaces.JobStorage
    wsHandler       interfaces.WebSocketHandler  // REMOVE THIS
    eventService    interfaces.EventService      // ADD THIS
    logger          arbor.ILogger
    logBatchChannel chan []arbormodels.LogEvent
    ctx             context.Context
    cancel          context.CancelFunc
    wg              sync.WaitGroup
    minEventLevel   arbor.Level                  // ADD THIS
}

func NewService(storage interfaces.JobLogStorage, jobStorage interfaces.JobStorage, eventService interfaces.EventService, logger arbor.ILogger, config *common.LoggingConfig) interfaces.LogService {
    // Parse min event level from config
    minLevel := arbor.InfoLevel  // default
    if config != nil {
        minLevel = parseLogLevel(config.MinEventLevel)
    }

    return &Service{
        storage:       storage,
        jobStorage:    jobStorage,
        eventService:  eventService,  // Use EventService instead of WebSocket
        logger:        logger,
        minEventLevel: minLevel,
    }
}

func (s *Service) consumer() {
    defer s.wg.Done()

    for {
        select {
        case batch, ok := <-s.logBatchChannel:
            if !ok {
                return
            }

            // Group by jobID for batch writes
            entriesByJob := make(map[string][]models.JobLogEntry)

            for _, event := range batch {
                jobID := event.CorrelationID
                if jobID == "" {
                    continue
                }

                // Transform to log entry
                logEntry := s.transformEvent(event)
                entriesByJob[jobID] = append(entriesByJob[jobID], logEntry)

                // Publish as event if level >= threshold
                if event.Level >= s.minEventLevel {
                    s.publishLogEvent(event, logEntry)
                }
            }

            // Batch write to database
            var wg sync.WaitGroup
            for jobID, entries := range entriesByJob {
                wg.Add(1)
                go func(jid string, logs []models.JobLogEntry) {
                    defer wg.Done()
                    s.storage.AppendLogs(s.ctx, jid, logs)
                }(jobID, entries)
            }
            wg.Wait()

        case <-s.ctx.Done():
            return
        }
    }
}

func (s *Service) publishLogEvent(event arbormodels.LogEvent, logEntry models.JobLogEntry) {
    // Publish to EventService (not directly to WebSocket)
    s.eventService.Publish(s.ctx, interfaces.Event{
        Type: "log_event",  // New event type
        Payload: map[string]interface{}{
            "job_id":    event.CorrelationID,
            "level":     logEntry.Level,
            "message":   logEntry.Message,
            "timestamp": logEntry.Timestamp,
        },
    })
}

func parseLogLevel(level string) arbor.Level {
    switch strings.ToLower(level) {
    case "debug":
        return arbor.DebugLevel
    case "info":
        return arbor.InfoLevel
    case "warn":
        return arbor.WarnLevel
    case "error":
        return arbor.ErrorLevel
    default:
        return arbor.InfoLevel
    }
}
```

### Step 3: Update WebSocketHandler

**File:** `internal/handlers/websocket.go`

```go
func (h *WebSocketHandler) SubscribeToCrawlerEvents() {
    if h.eventService == nil {
        return
    }

    // Subscribe to log events from LogService
    h.eventService.Subscribe("log_event", func(ctx context.Context, event interfaces.Event) error {
        payload, ok := event.Payload.(map[string]interface{})
        if !ok {
            h.logger.Warn().Msg("Invalid log_event payload type")
            return nil
        }

        // Convert to LogEntry for WebSocket broadcast
        entry := interfaces.LogEntry{
            Timestamp: getString(payload, "timestamp"),
            Level:     getString(payload, "level"),
            Message:   getString(payload, "message"),
        }

        // Broadcast to all clients
        h.broadcastToClients(WSMessage{
            Type:    "log",
            Payload: entry,
        })

        return nil
    })

    // Keep existing subscriptions for other events
    h.eventService.Subscribe(interfaces.EventCrawlProgress, ...)
    h.eventService.Subscribe(interfaces.EventJobSpawn, ...)
    // etc.
}

// REMOVE: BroadcastLog() method no longer needed
// WebSocket only broadcasts events, never called directly
```

### Step 4: Update app.go Initialization

**File:** `internal/app/app.go`

```go
// Initialize LogService with EventService (not WebSocketHandler)
app.LogService = logs.NewService(
    app.JobLogStorage,
    app.JobStorage,
    app.EventService,  // Pass EventService instead of WSHandler
    app.Logger,
    &app.Config.Logging,  // Pass logging config
)
```

### Step 5: Remove Direct WebSocket Calls

**Search and remove all instances of:**
- `wsHandler.BroadcastLog(entry)` ❌
- `wsHandler.SendLog(level, message)` ❌
- `wsHandler.StreamCrawlerJobLog(...)` ❌

**Services should ONLY:**
- `logger.Info().Msg("...")` ✅
- `logger.WithCorrelationId(jobID).Warn().Msg("...")` ✅

## Migration Checklist

### Phase 1: Configuration
- [ ] Add `LoggingConfig` to `internal/common/config.go`
- [ ] Add `[logging]` section to `quaero.toml`
- [ ] Set default `min_event_level = "info"`

### Phase 2: LogService
- [ ] Change `NewService()` signature (replace `wsHandler` with `eventService`)
- [ ] Add `minEventLevel` field
- [ ] Modify `consumer()` to publish events conditionally
- [ ] Add `publishLogEvent()` method
- [ ] Add `parseLogLevel()` helper

### Phase 3: WebSocketHandler
- [ ] Add subscription to `"log_event"` event type
- [ ] Remove `BroadcastLog()` method (no longer needed)
- [ ] Remove `SendLog()` method (no longer needed)
- [ ] Remove `StreamCrawlerJobLog()` method (no longer needed)

### Phase 4: App Initialization
- [ ] Update `app.go` to pass `EventService` to `LogService` (not `WSHandler`)
- [ ] Pass `Config.Logging` to `LogService`

### Phase 5: Cleanup
- [ ] Search for `wsHandler.BroadcastLog` - remove all calls
- [ ] Search for `wsHandler.SendLog` - remove all calls
- [ ] Search for `wsHandler.StreamCrawlerJobLog` - remove all calls
- [ ] Verify services only use `logger` (never `eventService` directly)

### Phase 6: Testing
- [ ] Start application
- [ ] Create crawler job
- [ ] Verify logs appear in UI (info, warn, error)
- [ ] Set `min_event_level = "error"` in config
- [ ] Restart, verify only errors appear in UI
- [ ] Check database has all logs (debug, info, warn, error)

## Benefits Summary

### Before (Complex)
- Services call logger AND eventService
- WebSocket called directly from multiple places
- Inconsistent implementation across services
- Hard to change event publishing rules

### After (Simple)
- Services ONLY call logger
- LogService controls event publishing
- WebSocket ONLY subscribes to events
- Easy to configure event filtering (one place)

### Code Reduction
- Remove ~100 lines of `eventService.Publish()` calls from services
- Remove ~50 lines of WebSocket broadcast methods
- Add ~30 lines to LogService consumer
- **Net reduction: ~120 lines**

### Maintainability
- ✅ Services stay simple (only logging)
- ✅ Single source of truth (LogService)
- ✅ Easy to add event filtering rules
- ✅ Easy to change UI update behavior (modify LogService, not 20 services)

---

**Date:** 2025-11-08
**Architecture:** Simplified pub/sub with LogService as event source
**Risk:** LOW - Clear migration path, no breaking changes to core services
**Effort:** 2-3 hours for implementation + testing
**Priority:** HIGH - Fixes UI real-time updates and simplifies codebase
