# Logging Architecture Analysis

**Date:** 2025-11-08
**Purpose:** Investigate debug log filtering behavior in the logging architecture
**Context:** Step 2 of homepage test logging implementation

## Executive Summary

The logging architecture uses a **level-based filtering system** where logs are filtered at the `LogService` layer before being published to the UI via `EventService`. The test configuration sets `min_event_level="debug"`, which **should** allow debug logs to reach the UI, but the filtering happens in the `LogService.shouldPublishEvent()` method.

**Key Finding:** The architecture is correctly implemented. Debug logs **will** appear in the UI when `min_event_level="debug"` is configured, as the LogService filters based on this threshold before publishing events.

## Logging Flow Architecture

### Complete Data Flow

```
1. Service Code (with correlation ID)
   logger.WithCorrelationId(jobID).Debug().Msg("message")
   ↓
2. Arbor Logger
   - Writes to configured channels (stdout, file, memory, context)
   - Context channel sends to LogService
   ↓
3. LogService.consumer() (internal/logs/service.go)
   - Receives batches from Arbor context channel
   - Groups by jobID for database writes
   - Filters by min_event_level threshold (LINE 417)
   - Publishes filtered events to EventService
   ↓
4. EventService (pub/sub hub)
   - Broadcasts "log_event" to all subscribers
   ↓
5. WebSocketHandler.SubscribeToCrawlerEvents()
   - Subscribes to "log_event" (LINE 762 in websocket.go)
   - Transforms to LogEntry format
   - Broadcasts to all WebSocket clients
   ↓
6. Client (Alpine.js serviceLogs component)
   - Receives via WebSocket
   - Displays in UI (pages/static/common.js)
```

## Configuration Loading Flow

### 1. Test Configuration (test/config/test-config.toml)

```toml
[logging]
min_event_level = "debug"  # Minimum log level to publish as real-time events to UI
```

**Location:** Line 12 in `test/config/test-config.toml`

### 2. Config Structure (internal/common/config.go)

```go
type LoggingConfig struct {
    Level         string   `toml:"level"`           // "debug", "info", "warn", "error"
    Format        string   `toml:"format"`          // "json" or "text"
    Output        []string `toml:"output"`          // "stdout", "file"
    ClientDebug   bool     `toml:"client_debug"`    // Enable client-side debug logging
    MinEventLevel string   `toml:"min_event_level"` // Minimum log level to publish as events to UI
}
```

**Location:** Lines 148-154 in `internal/common/config.go`

**Default Value:** `"info"` (Line 289 in `NewDefaultConfig()`)

**Environment Override:** `QUAERO_LOG_MIN_EVENT_LEVEL` (Line 505-507)

### 3. LogService Initialization (internal/app/app.go)

```go
// Line 129-135 in app.go
logService := logs.NewService(
    app.StorageManager.JobLogStorage(),
    app.StorageManager.JobStorage(),
    app.EventService,
    app.Logger,
    app.Config.Logging.MinEventLevel, // ← Config value passed here
)
```

**Key Point:** The `MinEventLevel` from config is passed to `LogService` constructor.

### 4. LogService Configuration (internal/logs/service.go)

```go
// Line 298-308: NewService constructor
func NewService(storage interfaces.JobLogStorage, jobStorage interfaces.JobStorage,
                eventService interfaces.EventService, logger arbor.ILogger,
                minEventLevel string) interfaces.LogService {
    level := parseLogLevel(minEventLevel) // ← Parses string to arbor.LogLevel

    return &Service{
        storage:       storage,
        jobStorage:    jobStorage,
        eventService:  eventService,
        logger:        logger,
        minEventLevel: level, // ← Stored as arbor.LogLevel enum
    }
}
```

**Level Parsing (Lines 312-325):**

```go
func parseLogLevel(levelStr string) arbor.LogLevel {
    switch strings.ToLower(levelStr) {
    case "debug":
        return arbor.DebugLevel    // ← "debug" maps to DebugLevel
    case "info":
        return arbor.InfoLevel
    case "warn", "warning":
        return arbor.WarnLevel
    case "error":
        return arbor.ErrorLevel
    default:
        return arbor.InfoLevel     // ← Default fallback
    }
}
```

## Log Filtering Logic

### Critical Filtering Point

**Location:** `internal/logs/service.go`, Line 327-332

```go
// shouldPublishEvent checks if a log event should be published based on level threshold
func (s *Service) shouldPublishEvent(level log.Level) bool {
    eventLevel := arborlevels.FromLogLevel(level)
    return eventLevel >= s.minEventLevel  // ← Numeric comparison
}
```

**How it works:**
1. Arbor log levels are numeric enums (DebugLevel < InfoLevel < WarnLevel < ErrorLevel)
2. When `min_event_level="debug"`, `s.minEventLevel = arbor.DebugLevel` (lowest value)
3. **Debug logs WILL pass the filter** because `DebugLevel >= DebugLevel` is true
4. Info, Warn, and Error logs also pass (they're >= DebugLevel)

### Consumer Batch Processing

**Location:** `internal/logs/service.go`, Lines 389-444

```go
// Line 417: Filter applied before publishing
if s.eventService != nil && s.shouldPublishEvent(event.Level) {
    s.publishLogEvent(event, logEntry)
}
```

**Flow:**
1. Receive batch of log events from Arbor context channel
2. Group entries by jobID for database writes
3. **For each event:** Check `shouldPublishEvent(event.Level)`
4. If passes threshold → Publish to EventService as "log_event"
5. WebSocket subscribers receive and broadcast to UI clients

## WebSocket Event Publishing

### EventService Publish (Line 447-466)

```go
func (s *Service) publishLogEvent(event arbormodels.LogEvent, logEntry models.JobLogEntry) {
    go func() {
        err := s.eventService.Publish(s.ctx, interfaces.Event{
            Type: "log_event", // ← Event type for WebSocket subscription
            Payload: map[string]interface{}{
                "job_id":    event.CorrelationID,
                "level":     logEntry.Level,      // ← "debug", "info", etc.
                "message":   logEntry.Message,
                "timestamp": logEntry.Timestamp,  // ← HH:MM:SS format
            },
        })
        // Non-blocking publish in goroutine
    }()
}
```

### WebSocket Subscription (websocket.go, Lines 762-805)

```go
h.eventService.Subscribe("log_event", func(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        h.logger.Warn().Msg("Invalid log_event payload type")
        return nil
    }

    // Convert to LogEntry for WebSocket broadcast
    entry := interfaces.LogEntry{
        Timestamp: getString(payload, "timestamp"),
        Level:     getString(payload, "level"),      // ← "debug" preserved
        Message:   getString(payload, "message"),
    }

    // Broadcast log message to all clients
    msg := WSMessage{
        Type:    "log",      // ← WebSocket message type
        Payload: entry,
    }
    // ... marshal and send to all connected clients
})
```

## Client-Side Display

### Alpine.js Component (pages/static/common.js)

**Lines 27-178:** `serviceLogs` component

```javascript
Alpine.data('serviceLogs', () => ({
    logs: [],
    maxLogs: 200,

    subscribeToWebSocket() {
        WebSocketManager.subscribe('log', (data) => {
            this.addLog(data);  // ← Receives all logs that passed server filter
        });
    },

    _parseLogEntry(logData) {
        const level = (logData.level || 'INFO').toUpperCase();  // ← Preserves "DEBUG"
        return {
            timestamp: this._formatLogTime(timestamp),
            level: level,
            levelClass: this._getLevelClass(level),  // ← Maps to CSS class
            message: message
        };
    },

    _getLevelClass(level) {
        const levelMap = {
            'ERROR': 'terminal-error',
            'WARN': 'terminal-warning',
            'WARNING': 'terminal-warning',
            'INFO': 'terminal-info',
            'DEBUG': 'terminal-time'   // ← Debug logs get 'terminal-time' class
        };
        return levelMap[level] || 'terminal-info';
    }
}));
```

**Key Point:** Client does **no filtering** - it displays all logs received from WebSocket.

## Verification of Debug Log Visibility

### Test Scenario: `min_event_level="debug"` in test-config.toml

**Expected Behavior:**

1. ✅ Config loads with `Logging.MinEventLevel = "debug"`
2. ✅ LogService parses to `arbor.DebugLevel` (lowest threshold)
3. ✅ Service emits debug log: `logger.Debug().Msg("test")`
4. ✅ Arbor sends to context channel
5. ✅ LogService.consumer receives event
6. ✅ `shouldPublishEvent(DebugLevel)` returns **true** (DebugLevel >= DebugLevel)
7. ✅ Publishes "log_event" to EventService
8. ✅ WebSocket broadcasts to clients
9. ✅ UI displays debug log with `terminal-time` CSS class

### Potential Issues Found: **NONE**

The architecture is correctly implemented. Debug logs **will** appear in the UI when configured with `min_event_level="debug"`.

## Additional Filtering Mechanisms

### 1. WebSocket Config (NOT applied to log_event)

**Location:** `internal/common/config.go`, Lines 199-209

```go
type WebSocketConfig struct {
    MinLevel        string   `toml:"min_level"`        // DEPRECATED - not used by LogService
    ExcludePatterns []string `toml:"exclude_patterns"` // Pattern-based filtering
    AllowedEvents   []string `toml:"allowed_events"`   // Event type whitelist
    ThrottleIntervals map[string]string `toml:"throttle_intervals"` // Rate limiting
}
```

**Important:** The `WebSocket.MinLevel` field is **deprecated** and not used by LogService. The **only** filter that matters for logs is `Logging.MinEventLevel`.

### 2. Memory Writer Filtering (GetRecentLogsHandler)

**Location:** `internal/handlers/websocket.go`, Lines 458-539

The `/api/logs/recent` endpoint filters logs differently:
- Uses `arbor.GetRegisteredMemoryWriter()` to read from in-memory buffer
- Applies pattern-based exclusions (Line 477-485)
- **Does NOT respect min_event_level** - returns all logs in memory

This is separate from real-time WebSocket streaming.

## Timestamp Handling

### Server-Side Formatting (Line 469-474 in service.go)

```go
func (s *Service) transformEvent(event arbormodels.LogEvent) models.JobLogEntry {
    formattedTime := event.Timestamp.Format("15:04:05")      // ← HH:MM:SS format
    fullTimestamp := event.Timestamp.Format(time.RFC3339)   // ← For sorting

    return models.JobLogEntry{
        Timestamp:       formattedTime,    // ← "14:35:22"
        FullTimestamp:   fullTimestamp,    // ← "2025-11-08T14:35:22Z"
        Level:           strings.ToLower(event.Level.String()),
        Message:         message,
    }
}
```

### Client-Side Preservation (common.js, Lines 129-147)

```javascript
_formatLogTime(timestamp) {
    // If timestamp is already formatted as HH:MM:SS, return as-is
    if (typeof timestamp === 'string' && /^\d{2}:\d{2}:\d{2}$/.test(timestamp)) {
        return timestamp;  // ← Server-provided timestamp preserved
    }

    // Otherwise try to parse as date (fallback)
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { hour12: false, ... });
}
```

**Key Point:** Timestamps are formatted on the **server** and preserved through to the UI. The client does **not** recalculate times.

## Recommendations for Step 3 Test Implementation

### 1. Test Debug Log Visibility

```javascript
// Wait for logs to populate
await sleep(2000);

// Extract logs from Alpine.js component
const debugLogs = await ctx.Evaluate(() => {
    const component = Alpine.$data(document.querySelector('[x-data="serviceLogs"]'));
    return component.logs.filter(log => log.level.toLowerCase() === 'debug');
});

// Assert debug logs exist
if (debugLogs.length === 0) {
    env.LogTest("WARNING: No debug logs found in UI");
} else {
    env.LogTest(`SUCCESS: Found ${debugLogs.length} debug logs`);
}
```

### 2. Verify Log Levels Present

```javascript
const logLevels = await ctx.Evaluate(() => {
    const component = Alpine.$data(document.querySelector('[x-data="serviceLogs"]'));
    const levels = new Set(component.logs.map(log => log.level.toLowerCase()));
    return Array.from(levels);
});

env.LogTest("Log levels present in UI: " + logLevels.join(', '));
// Expected: ["debug", "info", "warn"] or similar
```

### 3. Timestamp Validation

```javascript
const timestamps = await ctx.Evaluate(() => {
    const component = Alpine.$data(document.querySelector('[x-data="serviceLogs"]'));
    return component.logs.slice(0, 5).map(log => log.timestamp);
});

// Verify HH:MM:SS format
const timestampRegex = /^\d{2}:\d{2}:\d{2}$/;
const validTimestamps = timestamps.filter(ts => timestampRegex.test(ts));

env.LogTest(`Timestamp format validation: ${validTimestamps.length}/${timestamps.length} valid`);
```

## Conclusion

**No bugs or misconfigurations found.** The logging architecture is correctly implemented:

1. ✅ Config loading works correctly (`min_event_level` from TOML → LogService)
2. ✅ Level filtering uses proper numeric comparison (arbor.LogLevel enum)
3. ✅ Debug logs **will** pass the filter when `min_event_level="debug"`
4. ✅ EventService pub/sub works correctly
5. ✅ WebSocket broadcasts all filtered events to UI
6. ✅ Client displays all received logs without additional filtering
7. ✅ Timestamps are server-generated and preserved through the entire flow

**Next Steps (Step 3):**
- Implement test to verify debug logs appear in UI
- Check for presence of `DEBUG` level in `serviceLogs.logs` array
- Validate that log count > 0 (may need to trigger debug logs if none exist)
- Screenshot debug logs section for visual verification

**Potential Test Issues:**
- If no debug logs appear, it may be because **no service is emitting debug logs during test startup**. Consider adding a test-specific debug log trigger or checking existing service initialization logs.
- The test should wait at least 2-3 seconds after page load to allow WebSocket connection and log streaming to complete.
