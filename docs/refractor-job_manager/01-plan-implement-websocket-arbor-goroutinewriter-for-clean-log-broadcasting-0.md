I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**WebSocket Log Streaming (Polling-Based)**
- `websocket.go` uses `StartLogStreamer()` (line 389) with 2-second ticker polling
- `sendLogs()` (line 405) retrieves logs from arbor memory writer
- `parseAndBroadcastLog()` (line 459) manually parses arbor format with hardcoded filtering
- Filtering patterns hardcoded: "WebSocket client", "HTTP request", "Publishing Event" (lines 465-471)
- `BroadcastUILog()` method exists but appears unused (no references found)

**Client-Side Filtering**
- `common.js` serviceLogs component has `filteredLogs` computed property (lines 71-97)
- Filters by log level with localStorage persistence
- Handles level variations (WARN vs WARNING, ERR vs ERROR)
- `service-logs.html` has dropdown for level selection

**Arbor Integration**
- Version: `v1.4.51` (go.mod line 15)
- Logger type: `arbor.ILogger` used throughout codebase
- No existing custom arbor writers registered
- App.Close() (line 997) calls `common.Stop()` to flush context logs

**Configuration**
- No WebSocket-specific config section exists in `Config` struct
- `LoggingConfig` exists (lines 145-150) with level, format, output fields
- Config supports TOML file + env vars + CLI flags priority system

**Key Findings**
1. Arbor v1.4.51 supports GoroutineWriter pattern per README
2. WebSocket handler's `BroadcastLog()` is thread-safe (uses mutexes per connection)
3. No cleanup for WebSocket writer in App.Close() - needs to be added
4. Client expects `LogEntry` format: `{timestamp, level, message}`
5. `/api/logs/recent` endpoint (line 517) still needed for initial page load

### Approach

**Three-Component Clean Architecture:**

1. **WebSocket Arbor Writer** - New `websocket_writer.go` using GoroutineWriter pattern with processor function for filtering/transformation
2. **Configuration Extension** - Add `WebSocketConfig` to support min_level and exclude_patterns (future-proof, start with hardcoded defaults)
3. **Cleanup & Simplification** - Remove polling methods, simplify UI to display server-filtered logs

**Key Design Decisions:**
- Use arbor's `writers.NewGoroutineWriter()` with 1000-entry buffer for non-blocking writes
- Processor function filters logs and calls existing `BroadcastLog()` method
- Keep `/api/logs/recent` endpoint for initial page load (unchanged)
- Remove client-side filtering entirely - server filters before broadcasting
- Register writer in `app.go` initialization, close in `App.Close()`

### Reasoning

I explored the codebase by reading websocket.go (polling implementation), app.go (initialization and shutdown), common.js (client-side filtering), config.go (configuration structure), and the arbor README (GoroutineWriter pattern). I searched for BroadcastLog/SendLog usage, checked arbor version in go.mod (v1.4.51), verified thread-safety of WebSocket handler, and confirmed no existing custom arbor writers. I identified that the polling pattern uses a 2-second ticker, hardcoded filtering exists in parseAndBroadcastLog, and client-side filtering is implemented in the serviceLogs Alpine component.

## Mermaid Diagram

sequenceDiagram
    participant App as Application
    participant Logger as Arbor Logger
    participant WSWriter as WebSocket Writer<br/>(GoroutineWriter)
    participant Processor as Processor Function
    participant WSHandler as WebSocket Handler
    participant Client as Browser UI

    Note over App,Client: Initialization Phase
    App->>Logger: Create logger instance
    App->>WSHandler: NewWebSocketHandler()
    App->>WSWriter: NewWebSocketWriter(handler, config)
    WSWriter->>WSWriter: Initialize excludePatterns<br/>Set minLevel = InfoLevel
    WSWriter->>Processor: Define processor function
    WSWriter->>WSWriter: writers.NewGoroutineWriter(config, 1000, processor)
    WSWriter->>WSWriter: writer.Start() (background goroutine)
    App->>Logger: logger.AddWriter(wsWriter)
    Note over Logger,WSWriter: Writer registered - logs flow automatically

    Note over App,Client: Runtime - Log Streaming
    App->>Logger: logger.Info().Msg("Job started")
    Logger->>WSWriter: Write(logEntry) [non-blocking ~100μs]
    WSWriter->>WSWriter: Buffer entry (1000 capacity)
    WSWriter->>Processor: Process entry (background goroutine)
    Processor->>Processor: Check: entry.Level >= InfoLevel?
    Processor->>Processor: Check: message contains excludePatterns?
    alt Passes Filters
        Processor->>Processor: Transform to LogEntry format
        Processor->>WSHandler: BroadcastLog(entry)
        WSHandler->>Client: WebSocket message (type: "log")
        Client->>Client: Display log in UI
    else Filtered Out
        Processor->>Processor: Drop entry (no broadcast)
    end

    Note over App,Client: Shutdown Phase
    App->>WSWriter: Close() (graceful shutdown)
    WSWriter->>WSWriter: Stop accepting new entries
    WSWriter->>WSWriter: Drain buffer (process remaining)
    WSWriter->>Processor: Process final entries
    Processor->>WSHandler: BroadcastLog(final entries)
    WSWriter->>App: Shutdown complete (no log loss)

## Proposed File Changes

### internal\handlers\websocket_writer.go(NEW)

References: 

- internal\handlers\websocket.go(MODIFY)

**Create WebSocket Arbor Writer using GoroutineWriter pattern**

**Package and Imports:**
- Package: `handlers`
- Import: `github.com/ternarybob/arbor`, `github.com/ternarybob/arbor/writers`, `github.com/ternarybob/arbor/models`
- Import: `time`, `strings`

**Type Definition:**
- `WebSocketWriter` struct with fields:
  - `handler *WebSocketHandler` - reference to WebSocket handler for broadcasting
  - `writer writers.IGoroutineWriter` - arbor goroutine writer instance
  - `config models.WriterConfiguration` - arbor writer config
  - `minLevel arbor.Level` - minimum log level to broadcast (default: InfoLevel)
  - `excludePatterns []string` - patterns to filter out

**Constructor: `NewWebSocketWriter(handler *WebSocketHandler, config models.WriterConfiguration) (*WebSocketWriter, error)`**
- Initialize excludePatterns with defaults:
  - "WebSocket client connected"
  - "WebSocket client disconnected"
  - "HTTP request"
  - "HTTP response"
  - "Publishing Event"
  - "DEBUG: Memory writer entry"
- Set minLevel to `arbor.InfoLevel` (hardcoded for now, config support in future phase)
- Define processor function: `func(entry models.LogEvent) error`
  - Check if `entry.Level >= w.minLevel` (filter by level)
  - Check if message contains any excludePatterns (filter by pattern)
  - If passes filters: transform to `LogEntry` format and call `w.handler.BroadcastLog()`
  - Transform: `LogEntry{Timestamp: entry.Timestamp.Format("15:04:05"), Level: mapLevel(entry.Level), Message: entry.Message}`
- Create GoroutineWriter: `writers.NewGoroutineWriter(config, 1000, processor)`
- Start the writer: `writer.Start()`
- Return `&WebSocketWriter{handler, writer, config, minLevel, excludePatterns}`

**Helper: `mapLevel(level arbor.Level) string`**
- Map arbor levels to UI strings: ErrorLevel→"error", WarnLevel→"warn", InfoLevel→"info", DebugLevel→"debug"

**IWriter Interface Implementation:**
- `Write(data []byte) (int, error)` - delegate to `w.writer.Write(data)`
- `WithLevel(level arbor.Level) writers.IWriter` - update minLevel and return self
- `GetFilePath() string` - return empty string (not file-based)
- `Close() error` - call `w.writer.Close()` for graceful shutdown with buffer draining

**Rationale:** Uses arbor's GoroutineWriter for non-blocking writes (~100μs latency), automatic buffer management (1000 entries), and graceful shutdown with buffer draining. Processor function centralizes filtering logic (previously scattered in parseAndBroadcastLog). Reuses existing BroadcastLog method for thread-safe WebSocket broadcasting.

### internal\app\app.go(MODIFY)

References: 

- internal\handlers\websocket_writer.go(NEW)
- internal\handlers\websocket.go(MODIFY)

**Location 1: Lines 45-106 (App struct definition)**

Add new field to App struct:
- After line 102 (`WSHandler *handlers.WebSocketHandler`), add:
  - `WSWriter *handlers.WebSocketWriter` - WebSocket arbor writer for log streaming

**Location 2: Lines 139-142 (WebSocket initialization in New())**

Replace WebSocket background task initialization:
- Remove line 141: `app.WSHandler.StartLogStreamer()` (polling-based log streaming)
- After line 140 (`app.WSHandler.StartStatusBroadcaster()`), add:
  - Create arbor writer config: `writerConfig := models.WriterConfiguration{Type: models.LogWriterTypeConsole, TimeFormat: "15:04:05"}`
  - Create WebSocket writer: `wsWriter, err := handlers.NewWebSocketWriter(app.WSHandler, writerConfig)`
  - Handle error: `if err != nil { return nil, fmt.Errorf("failed to create WebSocket writer: %w", err) }`
  - Register with arbor logger: `logger.AddWriter(wsWriter)`
  - Store reference: `app.WSWriter = wsWriter`
  - Log success: `logger.Info().Msg("WebSocket arbor writer registered for real-time log streaming")`
- Update line 142 log message to: `"WebSocket handlers started (status broadcaster + arbor writer)"`

**Location 3: Lines 985-1075 (App.Close() method)**

Add WebSocket writer cleanup:
- After line 997 (`common.Stop()`), before closing log batch channel (line 999), add:
  - Close WebSocket writer: `if a.WSWriter != nil { if err := a.WSWriter.Close(); err != nil { a.Logger.Warn().Err(err).Msg("Failed to close WebSocket writer") } else { a.Logger.Info().Msg("WebSocket writer closed (buffer drained)") } }`

**Rationale:** Registers WebSocket writer during app initialization (after logger is created), ensuring all logs are automatically streamed to WebSocket clients. Cleanup in Close() ensures graceful shutdown with buffer draining (prevents log loss). Removes polling-based log streaming in favor of push-based arbor writer.

### internal\handlers\websocket.go(MODIFY)

References: 

- internal\handlers\websocket_writer.go(NEW)

**Location 1: Lines 67-77 (BroadcastUILog method)**

**Remove entire method** - no longer needed:
- Delete lines 67-77 (`BroadcastUILog` method)
- This method was for manual log broadcasting, now handled by arbor writer

**Location 2: Lines 388-454 (StartLogStreamer and sendLogs methods)**

**Remove polling-based log streaming:**
- Delete lines 388-402 (`StartLogStreamer` method with ticker)
- Delete lines 404-454 (`sendLogs` method with memory writer polling)
- These are replaced by WebSocket arbor writer (push-based)

**Location 3: Lines 456-514 (parseAndBroadcastLog method)**

**Remove manual log parsing and filtering:**
- Delete lines 456-514 (`parseAndBroadcastLog` method)
- Filtering logic moved to WebSocket writer processor function
- Parsing logic moved to WebSocket writer transformation

**Location 4: Lines 378-386 (SendLog helper method)**

**Keep this method** - may be used by other components:
- No changes to `SendLog` method (lines 378-386)
- This is a convenience wrapper around `BroadcastLog` for simple log messages
- Used for ad-hoc logging from handlers (if any)

**Location 5: Lines 34-43 (WebSocketHandler struct)**

**Remove unused fields:**
- Delete line 39: `lastLogKeys map[string]bool` - used by polling logic
- Delete line 40: `logKeysMu sync.RWMutex` - used by polling logic
- Update constructor `NewWebSocketHandler` (lines 45-60) to remove initialization of these fields

**Rationale:** Removes all polling-based log streaming infrastructure (ticker, memory writer polling, manual parsing). The WebSocket arbor writer handles all log streaming via push-based architecture. Keeps `BroadcastLog` and `SendLog` methods as they're used by the arbor writer and potentially other components.

### pages\static\common.js(MODIFY)

References: 

- pages\partials\service-logs.html(MODIFY)

**Location: Lines 71-103 (serviceLogs component - filteredLogs and setLogLevel)**

**Remove client-side log filtering:**
- Delete lines 71-97 (`get filteredLogs()` computed property with filtering logic)
- Delete lines 99-103 (`setLogLevel` method with localStorage persistence)
- Delete lines 32-66 (filter initialization logic in `init()` method - savedFilter, aliasMap, normalization)
- Simplify line 32: Change `selectedLogLevel: 'all'` to remove this field entirely (no longer needed)

**Update component to display all logs:**
- Modify template references from `filteredLogs` to `logs` (this change is in service-logs.html, noted below)
- Keep all other functionality: `addLog`, `clearLogs`, `refresh`, `toggleAutoScroll`, `_parseLogEntry`, `_formatLogTime`, `_getLevelClass`

**Rationale:** Server-side filtering via WebSocket arbor writer eliminates need for client-side filtering. Simplifies UI code by removing 30+ lines of filtering logic, localStorage management, and level normalization. Users will see all logs that pass server-side filters (min level: info, excluded patterns).

**Note:** If future requirement emerges for client-side filtering, it can be re-added as an optional UI-only feature (doesn't affect server filtering).

### pages\partials\service-logs.html(MODIFY)

References: 

- pages\static\common.js(MODIFY)

**Location 1: Lines 8-15 (Log level filter dropdown)**

**Remove log level filter UI:**
- Delete lines 8-15 (entire `<select>` element for log level filtering)
- This dropdown is no longer functional since client-side filtering is removed

**Location 2: Line 31 (Template condition)**

**Update template to use logs array:**
- Change line 31: `x-if="filteredLogs.length === 0"` to `x-if="logs.length === 0"`

**Location 3: Line 34 (Template loop)**

**Update template to iterate logs array:**
- Change line 34: `x-for="log in filteredLogs"` to `x-for="log in logs"`

**Rationale:** Removes UI controls for client-side filtering since server now handles all filtering. Simplifies template by directly displaying the logs array. Users see all logs that pass server-side filters (info level and above, excluding internal patterns).

**Future Enhancement:** If user-configurable filtering is needed, add server-side config endpoint to adjust min_level and exclude_patterns (not in this phase).

### internal\common\config.go(MODIFY)

**Location 1: Lines 14-29 (Config struct definition)**

**Add WebSocket configuration section:**
- After line 28 (`Search SearchConfig`), add new field:
  - `WebSocket WebSocketConfig \`toml:"websocket"\`` - WebSocket log streaming configuration

**Location 2: After line 191 (after SearchConfig struct)**

**Add WebSocketConfig struct definition:**
```go
// WebSocketConfig contains configuration for WebSocket log streaming
type WebSocketConfig struct {
	MinLevel        string   \`toml:"min_level"\`        // Minimum log level to broadcast ("debug", "info", "warn", "error")
	ExcludePatterns []string \`toml:"exclude_patterns"\` // Log message patterns to exclude from broadcasting
}
```

**Location 3: Lines 196-303 (NewDefaultConfig function)**

**Add WebSocket defaults:**
- After line 302 (Search config), before closing brace (line 303), add:
```go
WebSocket: WebSocketConfig{
	MinLevel: "info", // Default: info level and above
	ExcludePatterns: []string{
		"WebSocket client connected",
		"WebSocket client disconnected",
		"HTTP request",
		"HTTP response",
		"Publishing Event",
	},
},
```

**Location 4: Lines 332-548 (applyEnvOverrides function)**

**Add environment variable overrides:**
- After line 547 (Search config overrides), before closing brace (line 548), add:
```go
// WebSocket configuration
if minLevel := os.Getenv("QUAERO_WEBSOCKET_MIN_LEVEL"); minLevel != "" {
	config.WebSocket.MinLevel = minLevel
}
if excludePatterns := os.Getenv("QUAERO_WEBSOCKET_EXCLUDE_PATTERNS"); excludePatterns != "" {
	// Split comma-separated patterns
	patterns := []string{}
	for _, p := range splitString(excludePatterns, ",") {
		trimmed := trimSpace(p)
		if trimmed != "" {
			patterns = append(patterns, trimmed)
		}
	}
	if len(patterns) > 0 {
		config.WebSocket.ExcludePatterns = patterns
	}
}
```

**Rationale:** Adds configuration support for WebSocket log filtering (future-proof). Defaults match current hardcoded values in websocket_writer.go. Environment variables allow runtime configuration without code changes. This phase uses hardcoded defaults in websocket_writer.go; future phase will read from config.

**Note:** The websocket_writer.go implementation in this phase uses hardcoded defaults. A future phase ("Add configuration-driven filtering") will update websocket_writer.go to read from config.WebSocket.

### deployments\local\quaero.toml(MODIFY)

**Location: After line 156 (after Crawler configuration section)**

**Add WebSocket configuration section:**

Add new commented-out section with documentation:

```toml
# =============================================================================
# WebSocket Log Streaming Configuration
# =============================================================================
# Configure server-side filtering for real-time log streaming to browser UI.
# Logs are filtered before broadcasting to reduce client load and network traffic.
# Technical parameters have sensible defaults hardcoded in the application.
# Only uncomment settings you want to override from defaults.
#
# Defaults: min_level="info", exclude_patterns=["WebSocket client", "HTTP request", ...]
# Env vars: QUAERO_WEBSOCKET_MIN_LEVEL, QUAERO_WEBSOCKET_EXCLUDE_PATTERNS

# [websocket]
# min_level = "info"  # Minimum log level to broadcast ("debug", "info", "warn", "error")
# exclude_patterns = [  # Log message patterns to exclude from broadcasting
#     "WebSocket client connected",
#     "WebSocket client disconnected",
#     "HTTP request",
#     "HTTP response",
#     "Publishing Event",
# ]
```

**Rationale:** Documents WebSocket configuration options for users. Commented out by default since hardcoded defaults are sensible. Users can uncomment and customize if needed. Follows existing pattern in quaero.toml (most sections are commented with defaults documented).

**Note:** This configuration is not used in the current phase (websocket_writer.go uses hardcoded defaults). A future phase will update websocket_writer.go to read from config.