I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Configuration Infrastructure (Mostly Complete)**
- `config.go` lines 195-199: WebSocketConfig struct exists with MinLevel and ExcludePatterns
- `config.go` lines 312-321: Default config sets sensible defaults (info level, 5 exclude patterns)
- `config.go` lines 571-587: Environment variable overrides work for WebSocket config
- `quaero.toml` lines 158-177: Commented-out [websocket] section documents the config

**WebSocket Writer (Needs Config Wiring)**
- `websocket_writer.go` lines 31-39: Hardcoded minLevel and excludePatterns
- `app.go` lines 109-177: Creates WebSocketWriter with empty WriterConfiguration, doesn't pass actual config values
- **Gap**: Config values exist but aren't passed to WebSocketWriter constructor

**Event Subscriber (Needs Filtering & Throttling)**
- `websocket_events.go` lines 40-63: SubscribeAll() subscribes to all 5 job lifecycle events unconditionally
- `websocket.go` lines 644-747: SubscribeToCrawlerEvents() subscribes to EventCrawlProgress and EventJobSpawn (high-frequency)
- **Gap**: No event filtering (allowed_events whitelist)
- **Gap**: No throttling for high-frequency events (crawl_progress, job_spawn)

**Event Types Identified**
From `event_service.go`:
- **High-frequency**: EventCrawlProgress (line 25), EventJobSpawn (line 93) - published on every URL processed
- **Low-frequency**: EventJobCreated (107), EventJobStarted (120), EventJobCompleted (135), EventJobFailed (149), EventJobCancelled (162) - published once per job lifecycle transition
- **Other**: EventStatusChanged, EventJobProgress, EventSourceCreated/Updated/Deleted

## Design Decisions

**1. Event Filtering Approach**
- **Whitelist pattern**: Only broadcast events in allowed_events list
- **Default**: Allow all job lifecycle events + crawl_progress + job_spawn (backward compatible)
- **Rationale**: Whitelist is safer than blacklist (explicit opt-in for new events)

**2. Throttling Strategy**
- **Per-event-type rate limiters**: Separate limiter for each high-frequency event
- **Token bucket algorithm**: Use golang.org/x/time/rate.Limiter (standard library extension)
- **Default intervals**: crawl_progress=1s, job_spawn=500ms (reasonable for UI updates)
- **Rationale**: Prevents WebSocket flooding during large crawls (1000s of URLs)

**3. Configuration Structure**
Add to WebSocketConfig:
- `AllowedEvents []string` - whitelist of event types to broadcast
- `ThrottleIntervals map[string]string` - event type → duration (e.g., "crawl_progress": "1s")

**4. Backward Compatibility**
- Empty allowed_events → allow all events (current behavior)
- Empty throttle_intervals → no throttling (current behavior)
- Existing log filtering continues to work unchanged

## Implementation Complexity

**Low Complexity** - This is configuration plumbing, not architectural change:
- Config struct extension: 2 new fields
- TOML documentation: 10 lines
- WebSocketWriter: Pass config instead of hardcoded values (5 lines)
- EventSubscriber: Add filtering logic (10 lines) + rate limiters (20 lines)
- App initialization: Pass config to constructors (3 lines)

**No Breaking Changes** - All changes are additive with sensible defaults.

### Approach

**Configuration-Driven WebSocket Filtering & Throttling**

Extend the existing WebSocket configuration system to support:
1. **Log filtering** - Load min_level and exclude_patterns from config (already structured, just needs wiring)
2. **Event filtering** - Add allowed_events whitelist to control which events are broadcast
3. **Event throttling** - Add rate limiters for high-frequency events (crawl_progress, job_spawn) using golang.org/x/time/rate

All filtering/throttling will be configurable via TOML file, environment variables, and use sensible defaults. This maintains the clean architecture where services publish events normally, and the WebSocket layer handles filtering/throttling transparently.

### Reasoning

I explored the codebase by reading the four files mentioned by the user (quaero.toml, config.go, app.go, websocket_events.go), then examined websocket_writer.go and websocket.go to understand current implementation patterns. I also reviewed event_service.go to identify all event types. I discovered that WebSocketConfig struct exists with MinLevel and ExcludePatterns fields, but the WebSocketWriter uses hardcoded values instead of reading from config. EventSubscriber subscribes to all 5 job lifecycle events with no filtering or throttling. The infrastructure is 90% complete - just needs configuration wiring and rate limiting logic.

## Proposed File Changes

### internal\common\config.go(MODIFY)

**Location 1: Lines 195-199 (WebSocketConfig struct)**

Extend WebSocketConfig struct with event filtering and throttling fields:

1. After line 198 (`ExcludePatterns []string`), add new fields:
   - `AllowedEvents []string \`toml:"allowed_events"\`` - Whitelist of event types to broadcast (empty = allow all)
   - `ThrottleIntervals map[string]string \`toml:"throttle_intervals"\`` - Event type to throttle interval mapping (e.g., {"crawl_progress": "1s"})

2. Add documentation comments:
   - AllowedEvents: "Whitelist of event types to broadcast via WebSocket. Empty list allows all events. Example: [\"job_created\", \"job_completed\", \"crawl_progress\"]"
   - ThrottleIntervals: "Throttle intervals for high-frequency events. Map of event type to duration string. Example: {\"crawl_progress\": \"1s\", \"job_spawn\": \"500ms\"}"

**Location 2: Lines 312-321 (NewDefaultConfig - WebSocket defaults)**

Add default values for new fields:

1. After line 320 (ExcludePatterns array), add:
   - `AllowedEvents: []string{}` - Empty list = allow all events (backward compatible)
   - `ThrottleIntervals: map[string]string{"crawl_progress": "1s", "job_spawn": "500ms"}` - Throttle high-frequency events by default

2. Add comment explaining defaults:
   - "Empty AllowedEvents allows all events (backward compatible)"
   - "Throttle high-frequency events to prevent WebSocket flooding during large crawls"

**Location 3: Lines 571-587 (applyEnvOverrides - WebSocket env vars)**

Add environment variable overrides for new fields:

1. After line 587 (ExcludePatterns override), add:
   - `QUAERO_WEBSOCKET_ALLOWED_EVENTS` override:
     - Split comma-separated event types
     - Trim whitespace from each event type
     - Set config.WebSocket.AllowedEvents if non-empty
   - `QUAERO_WEBSOCKET_THROTTLE_CRAWL_PROGRESS` override:
     - Parse duration string (e.g., "2s", "1500ms")
     - Set config.WebSocket.ThrottleIntervals["crawl_progress"] if valid
   - `QUAERO_WEBSOCKET_THROTTLE_JOB_SPAWN` override:
     - Parse duration string
     - Set config.WebSocket.ThrottleIntervals["job_spawn"] if valid

2. Use existing helper functions:
   - `splitString()` for comma-separated parsing
   - `trimSpace()` for whitespace removal
   - `time.ParseDuration()` for duration validation

**Rationale**: Extends existing configuration pattern (TOML → defaults → env vars → CLI). Uses map[string]string for throttle intervals to support TOML serialization (map[string]time.Duration doesn't serialize well). Environment variables provide runtime configuration without file changes.

### deployments\local\quaero.toml(MODIFY)

**Location: Lines 158-177 (WebSocket configuration section)**

Extend the commented-out [websocket] section with event filtering and throttling documentation:

1. Keep existing lines 158-177 (min_level and exclude_patterns documentation)

2. After line 176 (exclude_patterns array closing), add new configuration options:

```toml
# Event Filtering - Control which events are broadcast to WebSocket clients
# allowed_events = []  # Empty list allows all events (default)
# allowed_events = [   # Whitelist specific events (example)
#     "job_created",
#     "job_started",
#     "job_completed",
#     "job_failed",
#     "job_cancelled",
#     "crawl_progress",
#     "job_spawn",
# ]

# Event Throttling - Rate limit high-frequency events to prevent WebSocket flooding
# Throttling reduces network traffic and client load during large crawls (1000s of URLs)
# [websocket.throttle_intervals]
# crawl_progress = "1s"   # Max 1 crawl progress update per second per job (default)
# job_spawn = "500ms"     # Max 2 job spawn events per second (default)
```

3. Update section header comment (line 160) to mention event filtering:
   - Change: "Configure server-side filtering for real-time log streaming to browser UI."
   - To: "Configure server-side filtering for real-time log streaming and event broadcasting to browser UI."

4. Update defaults comment (line 165) to include event defaults:
   - Change: "Defaults: min_level=\"info\", exclude_patterns=[\"WebSocket client\", \"HTTP request\", ...]"
   - To: "Defaults: min_level=\"info\", exclude_patterns=[...], allowed_events=[] (all), throttle_intervals={crawl_progress:1s, job_spawn:500ms}"

5. Update env vars comment (line 166) to include new variables:
   - Change: "Env vars: QUAERO_WEBSOCKET_MIN_LEVEL, QUAERO_WEBSOCKET_EXCLUDE_PATTERNS"
   - To: "Env vars: QUAERO_WEBSOCKET_MIN_LEVEL, QUAERO_WEBSOCKET_EXCLUDE_PATTERNS, QUAERO_WEBSOCKET_ALLOWED_EVENTS, QUAERO_WEBSOCKET_THROTTLE_CRAWL_PROGRESS, QUAERO_WEBSOCKET_THROTTLE_JOB_SPAWN"

**Rationale**: Documents new configuration options following existing pattern (commented out with sensible defaults). Explains the purpose of throttling (prevent flooding during large crawls). Provides example whitelist for users who want to reduce WebSocket traffic.

### internal\handlers\websocket_writer.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)

**Location 1: Lines 26-40 (NewWebSocketWriter constructor)**

Update constructor to accept and use WebSocketConfig instead of hardcoded values:

1. Change function signature (line 27):
   - Add parameter: `wsConfig *common.WebSocketConfig` (import `github.com/ternarybob/quaero/internal/common`)
   - Signature becomes: `func NewWebSocketWriter(handler *WebSocketHandler, config models.WriterConfiguration, wsConfig *common.WebSocketConfig) (*WebSocketWriter, error)`

2. Replace hardcoded minLevel (line 31):
   - Remove: `minLevel: levels.InfoLevel,`
   - Add: Parse wsConfig.MinLevel string to levels.LogLevel:
     - "debug" → levels.DebugLevel
     - "info" → levels.InfoLevel
     - "warn" → levels.WarnLevel
     - "error" → levels.ErrorLevel
     - Default to levels.InfoLevel if invalid
   - Use helper function: `parseLogLevel(wsConfig.MinLevel)` (add below)

3. Replace hardcoded excludePatterns (lines 32-39):
   - Remove hardcoded array
   - Add: `excludePatterns: wsConfig.ExcludePatterns,`
   - If wsConfig.ExcludePatterns is nil or empty, use default patterns (same as current hardcoded list)

4. Add helper function after constructor:
```go
// parseLogLevel converts config string to arbor log level
func parseLogLevel(level string) levels.LogLevel {
    switch strings.ToLower(strings.TrimSpace(level)) {
    case "debug":
        return levels.DebugLevel
    case "info":
        return levels.InfoLevel
    case "warn", "warning":
        return levels.WarnLevel
    case "error":
        return levels.ErrorLevel
    default:
        return levels.InfoLevel
    }
}
```

**Location 2: Lines 42-68 (Processor function)**

No changes needed - processor function already uses w.minLevel and w.excludePatterns from struct fields, which are now populated from config.

**Rationale**: Removes hardcoded configuration in favor of config-driven values. Maintains backward compatibility by using defaults if config is empty. The processor function doesn't need changes because it references struct fields that are now populated from config. This completes the configuration wiring for log filtering.

### internal\app\app.go(MODIFY)

References: 

- internal\handlers\websocket_writer.go(MODIFY)
- internal\common\config.go(MODIFY)
- internal\handlers\websocket_events.go(MODIFY)

**Location: Lines 109-177 (New function - WebSocket writer initialization)**

Update WebSocketWriter creation to pass actual config values:

1. Find the line where WebSocketWriter is created (around line 140-150 based on summary)
   - Current: `wsWriter, err := handlers.NewWebSocketWriter(app.WSHandler, writerConfig)`
   - Change to: `wsWriter, err := handlers.NewWebSocketWriter(app.WSHandler, writerConfig, &config.WebSocket)`

2. Add logging to show loaded configuration:
   - After successful wsWriter creation, before `logger.AddWriter(wsWriter)`
   - Log: `logger.Info().Str("min_level", config.WebSocket.MinLevel).Int("exclude_patterns", len(config.WebSocket.ExcludePatterns)).Int("allowed_events", len(config.WebSocket.AllowedEvents)).Msg("WebSocket writer configured")`

3. Handle nil config gracefully:
   - If config.WebSocket is nil, create default WebSocketConfig from common.NewDefaultConfig().WebSocket
   - This ensures WebSocketWriter always receives valid config

**Rationale**: Completes the configuration wiring by passing actual config values to WebSocketWriter. The config object is already available in the New() function (parameter), so this is just passing it through. Logging helps with debugging configuration issues. Nil check ensures robustness if config loading fails.
**Location: Lines 804-967 (initHandlers method - EventSubscriber initialization)**

Update EventSubscriber creation to pass config:

1. Find the line where EventSubscriber is created (around line 810-820 based on summary)
   - Current: `_ = handlers.NewEventSubscriber(a.WSHandler, a.EventService, a.Logger)`
   - Change to: `_ = handlers.NewEventSubscriber(a.WSHandler, a.EventService, a.Logger, &config.WebSocket)`

2. Add logging to show event filtering configuration:
   - After EventSubscriber creation
   - Log: `a.Logger.Info().Int("allowed_events", len(config.WebSocket.AllowedEvents)).Int("throttle_intervals", len(config.WebSocket.ThrottleIntervals)).Msg("EventSubscriber configured with filtering and throttling")`

3. Handle nil config gracefully:
   - If config.WebSocket is nil, create default WebSocketConfig from common.NewDefaultConfig().WebSocket
   - This ensures EventSubscriber always receives valid config

**Rationale**: Completes the configuration wiring for EventSubscriber. The config object is available in initHandlers (passed from New() function). Logging helps verify that configuration is loaded correctly. Nil check ensures robustness.

### internal\handlers\websocket_events.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\handlers\websocket.go(MODIFY)

**Location 1: Lines 11-16 (EventSubscriber struct)**

Extend EventSubscriber struct with filtering and throttling fields:

1. After line 15 (`logger arbor.ILogger`), add new fields:
   - `allowedEvents map[string]bool` - Fast lookup map for event filtering (converted from config slice)
   - `throttlers map[string]*rate.Limiter` - Per-event-type rate limiters for throttling
   - `config *common.WebSocketConfig` - Reference to WebSocket config for accessing settings

2. Add import: `"golang.org/x/time/rate"` and `"github.com/ternarybob/quaero/internal/common"`

**Location 2: Lines 18-37 (NewEventSubscriber constructor)**

Update constructor to accept config and initialize filtering/throttling:

1. Change function signature (line 20):
   - Add parameter: `config *common.WebSocketConfig`
   - Signature becomes: `func NewEventSubscriber(handler *WebSocketHandler, eventService interfaces.EventService, logger arbor.ILogger, config *common.WebSocketConfig) *EventSubscriber`

2. After line 25 (logger assignment), add initialization:
   - Initialize allowedEvents map:
     - If config.AllowedEvents is empty, set allowedEvents to nil (allow all)
     - If config.AllowedEvents has values, convert slice to map for O(1) lookup
     - Example: `allowedEvents := make(map[string]bool); for _, evt := range config.AllowedEvents { allowedEvents[evt] = true }`
   - Initialize throttlers map:
     - Parse config.ThrottleIntervals (map[string]string) to create rate.Limiters
     - For each event type in ThrottleIntervals:
       - Parse duration string using time.ParseDuration()
       - Create rate.Limiter: `rate.NewLimiter(rate.Every(duration), 1)` (1 token per interval)
       - Store in throttlers map: `throttlers[eventType] = limiter`
     - Log warning if duration parsing fails for any event type
   - Store config reference: `config: config`

3. Update struct initialization (line 21-25) to include new fields

**Location 3: Lines 39-63 (SubscribeAll method)**

Add event filtering logic to subscription:

1. After line 45 (nil check), add filtering check for each Subscribe call:
   - Before subscribing to each event (lines 48, 51, 54, 57, 60), add:
     - `if !s.shouldBroadcastEvent("event_type_name") { return }` (skip subscription if filtered)
   - Event type names: "job_created", "job_started", "job_completed", "job_failed", "job_cancelled"

2. Add helper method after SubscribeAll:
```go
// shouldBroadcastEvent checks if an event type should be broadcast based on allowed_events filter
func (s *EventSubscriber) shouldBroadcastEvent(eventType string) bool {
    // If allowedEvents is nil (empty config), allow all events
    if s.allowedEvents == nil {
        return true
    }
    // Check if event is in whitelist
    return s.allowedEvents[eventType]
}
```

**Location 4: Lines 99-209 (Event handler methods)**

Add throttling logic to each event handler:

1. At the start of each handler (handleJobCreated, handleJobStarted, handleJobCompleted, handleJobFailed, handleJobCancelled):
   - After payload type check, before creating JobStatusUpdate struct
   - Add throttling check:
```go
// Check throttle limiter for this event type
if limiter, ok := s.throttlers["event_type_name"]; ok {
    if !limiter.Allow() {
        // Event throttled, skip broadcasting
        return nil
    }
}
```
   - Event type names match config keys: "job_created", "job_started", "job_completed", "job_failed", "job_cancelled"

2. Note: Job lifecycle events are low-frequency (once per job), so throttling is optional
   - Throttling is more important for high-frequency events (crawl_progress, job_spawn)
   - These are handled in websocket.go SubscribeToCrawlerEvents() method

**Location 5: Add note about high-frequency events**

Add comment at end of file explaining that high-frequency event throttling (crawl_progress, job_spawn) is handled in websocket.go SubscribeToCrawlerEvents() method, not here. This EventSubscriber only handles job lifecycle events which are naturally low-frequency.

**Rationale**: 
- Event filtering uses whitelist pattern (explicit opt-in) for security
- Throttling uses token bucket algorithm (rate.Limiter) for smooth rate limiting
- Fast lookup with map[string]bool for O(1) event filtering
- Per-event-type throttlers allow different rates for different events
- Graceful degradation: empty config = allow all events, no throttling (backward compatible)
- Job lifecycle events are naturally low-frequency, so throttling is less critical here

### internal\handlers\websocket.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\app\app.go(MODIFY)

**Location: Lines 643-747 (SubscribeToCrawlerEvents method)**

Add throttling for high-frequency events (crawl_progress, job_spawn):

1. Add throttling fields to WebSocketHandler struct (lines 35-42):
   - After line 41 (`eventService interfaces.EventService`), add:
     - `crawlProgressThrottler *rate.Limiter` - Rate limiter for crawl_progress events
     - `jobSpawnThrottler *rate.Limiter` - Rate limiter for job_spawn events
   - Add import: `"golang.org/x/time/rate"`

2. Update NewWebSocketHandler constructor (lines 44-58) to accept config:
   - Add parameter: `config *common.WebSocketConfig`
   - Signature becomes: `func NewWebSocketHandler(eventService interfaces.EventService, logger arbor.ILogger, config *common.WebSocketConfig) *WebSocketHandler`
   - Initialize throttlers from config.ThrottleIntervals:
     - Parse "crawl_progress" interval, create rate.Limiter, assign to h.crawlProgressThrottler
     - Parse "job_spawn" interval, create rate.Limiter, assign to h.jobSpawnThrottler
     - Use default intervals (1s, 500ms) if parsing fails or config is nil
     - Log warning if duration parsing fails

3. Add throttling check in EventCrawlProgress handler (lines 649-694):
   - After line 655 (payload type check), before line 658 (creating CrawlProgressUpdate)
   - Add:
```go
// Throttle crawl progress events to prevent WebSocket flooding
if h.crawlProgressThrottler != nil && !h.crawlProgressThrottler.Allow() {
    // Event throttled, skip broadcasting
    return nil
}
```

4. Add throttling check in EventJobSpawn handler (lines 728-746):
   - After line 730 (payload type check), before line 735 (creating JobSpawnUpdate)
   - Add same throttling check using h.jobSpawnThrottler

5. Update all NewWebSocketHandler calls in app.go:
   - Pass &config.WebSocket as third parameter
   - This is in initHandlers method (around line 808)

**Rationale**: 
- High-frequency events (crawl_progress, job_spawn) are published on every URL processed
- During large crawls (1000s of URLs), this can flood WebSocket with 100+ events/second
- Throttling reduces network traffic and client load without losing critical information
- UI doesn't need sub-second updates for progress bars
- Token bucket algorithm (rate.Limiter) provides smooth rate limiting
- Throttlers are stored on WebSocketHandler (not EventSubscriber) because these events are subscribed in SubscribeToCrawlerEvents, not EventSubscriber