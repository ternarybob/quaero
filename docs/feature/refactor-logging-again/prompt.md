# Quaero SSE Log Streaming Refactor

## Objective

Refactor the Quaero log streaming system from WebSocket signal-then-fetch to Server-Sent Events (SSE) with server-side batching. This simplifies the architecture while maintaining all current functionality.

## Important Constraints

- **Breaking changes are acceptable** - no requirement for backward compatibility
- **Remove redundant code** - investigate and delete all unused WebSocket, polling, and signal-related code
- **Service logs on all pages** - the SSE log streaming must integrate with the "service logs" component that appears on all pages
- **Global JS library** - create reusable JavaScript functions in `./pages/static/` for use across all pages

## Code Cleanup Tasks

Before implementing SSE, investigate and remove:

1. **WebSocket handlers** - all WS upgrade, connection management, and message handling code
2. **Signal/refresh endpoints** - any endpoints that just signal clients to fetch
3. **Polling logic** - client-side setInterval/setTimeout polling for logs
4. **Redundant log fetch endpoints** - consolidate to single endpoint if multiple exist
5. **Unused event types** - any custom events no longer needed
6. **Dead code paths** - handlers, services, or utilities no longer referenced

Run through the codebase and identify:
```bash
# Find WebSocket references
grep -r "websocket\|WebSocket\|gorilla/websocket" --include="*.go"

# Find polling patterns in JS
grep -r "setInterval\|setTimeout.*fetch\|polling" --include="*.js" --include="*.html"

# Find signal/refresh endpoints
grep -r "refresh\|signal\|notify" --include="*.go" ./handlers ./routes
```

## Current Behaviour to Preserve

- Client displays logs for a specific job and optionally a specific step
- Total log count and currently displayed log count are shown to the user
- Job and step status changes are reflected in the UI
- Logs are ordered chronologically
- Client can filter logs (by level, step, search term)

## Architecture Overview

```
┌─────────────┐     SSE (text/event-stream)      ┌─────────────┐
│   Client    │◄────────────────────────────────│   Server    │
│  Alpine.js  │                                  │     Go      │
└─────────────┘                                  └─────────────┘
      │                                                │
      │ Initial: GET /api/jobs/{id}/logs/stream       │
      │ Params: ?step={stepId}&limit=100&level=info   │
      │                                                │
      │◄──────── event: logs ─────────────────────────│
      │◄──────── event: status ───────────────────────│
      │◄──────── event: ping ─────────────────────────│
```

## SSE Event Types

### 1. `logs` - Batched log entries

Sent when new logs are available or on flush triggers.

```json
{
  "logs": [
    {
      "id": "uuid",
      "timestamp": "2024-01-15T10:30:00Z",
      "level": "info",
      "message": "Processing item 42",
      "step_id": "step-uuid",
      "step_name": "Extract Data"
    }
  ],
  "meta": {
    "total_count": 1542,
    "displayed_count": 100,
    "has_more": true,
    "oldest_id": "uuid",
    "newest_id": "uuid"
  }
}
```

### 2. `status` - Job/step status change

Sent immediately when job or step status changes, triggers log flush.

```json
{
  "job": {
    "id": "uuid",
    "status": "running",
    "progress": 45
  },
  "steps": [
    {
      "id": "uuid",
      "name": "Extract Data",
      "status": "completed"
    },
    {
      "id": "uuid",
      "name": "Transform",
      "status": "running"
    }
  ]
}
```

### 3. `ping` - Heartbeat

Sent every 5 seconds if no other events, keeps connection alive.

```json
{
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Server Implementation

### SSE Handler

```go
// GET /api/jobs/{id}/logs/stream
func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
    jobID := chi.URLParam(r, "id")
    
    // Parse query params
    opts := StreamOptions{
        StepID:   r.URL.Query().Get("step"),
        Limit:    parseIntOrDefault(r.URL.Query().Get("limit"), 100),
        Level:    r.URL.Query().Get("level"),
        SinceID:  r.URL.Query().Get("since"),
    }
    
    // SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
    
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }
    
    // Create stream for this job
    stream := h.logService.Subscribe(r.Context(), jobID, opts)
    defer stream.Close()
    
    // Send initial state
    h.sendInitialState(w, flusher, jobID, opts)
    
    // Batching configuration
    batchInterval := 150 * time.Millisecond
    pingInterval := 5 * time.Second
    
    batchTicker := time.NewTicker(batchInterval)
    pingTicker := time.NewTicker(pingInterval)
    defer batchTicker.Stop()
    defer pingTicker.Stop()
    
    var logBatch []LogEntry
    lastSend := time.Now()
    
    for {
        select {
        case <-r.Context().Done():
            return
            
        case log, ok := <-stream.Logs:
            if !ok {
                return
            }
            logBatch = append(logBatch, log)
            
        case status := <-stream.Status:
            // Status change: flush logs immediately, then send status
            if len(logBatch) > 0 {
                h.sendLogBatch(w, flusher, logBatch, jobID, opts)
                logBatch = logBatch[:0]
            }
            h.sendStatus(w, flusher, status)
            lastSend = time.Now()
            pingTicker.Reset(pingInterval)
            
        case <-batchTicker.C:
            if len(logBatch) > 0 {
                h.sendLogBatch(w, flusher, logBatch, jobID, opts)
                logBatch = logBatch[:0]
                lastSend = time.Now()
                pingTicker.Reset(pingInterval)
            }
            
        case <-pingTicker.C:
            h.sendPing(w, flusher)
        }
    }
}
```

### SSE Write Helpers

```go
func (h *Handler) sendEvent(w http.ResponseWriter, flusher http.Flusher, event string, data any) error {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }
    
    fmt.Fprintf(w, "event: %s\n", event)
    fmt.Fprintf(w, "data: %s\n\n", jsonData)
    flusher.Flush()
    return nil
}

func (h *Handler) sendLogBatch(w http.ResponseWriter, flusher http.Flusher, logs []LogEntry, jobID string, opts StreamOptions) {
    // Get counts from database (server-side filtering applied)
    totalCount := h.logService.CountLogs(jobID, opts.StepID, "") // No level filter for total
    displayedCount := h.logService.CountLogs(jobID, opts.StepID, opts.Level)
    
    payload := LogBatchPayload{
        Logs: logs,
        Meta: LogMeta{
            TotalCount:     totalCount,
            DisplayedCount: displayedCount,
            HasMore:        displayedCount > opts.Limit,
            OldestID:       logs[0].ID,
            NewestID:       logs[len(logs)-1].ID,
        },
    }
    
    h.sendEvent(w, flusher, "logs", payload)
}
```

### Log Subscription Service

```go
type StreamOptions struct {
    StepID  string
    Limit   int
    Level   string // "debug", "info", "warn", "error"
    SinceID string
}

type LogStream struct {
    Logs   chan LogEntry
    Status chan JobStatus
    done   chan struct{}
}

func (s *LogService) Subscribe(ctx context.Context, jobID string, opts StreamOptions) *LogStream {
    stream := &LogStream{
        Logs:   make(chan LogEntry, 100),
        Status: make(chan JobStatus, 10),
        done:   make(chan struct{}),
    }
    
    s.mu.Lock()
    s.subscribers[jobID] = append(s.subscribers[jobID], stream)
    s.mu.Unlock()
    
    go func() {
        select {
        case <-ctx.Done():
        case <-stream.done:
        }
        s.unsubscribe(jobID, stream)
    }()
    
    return stream
}

// Called when a new log is written
func (s *LogService) Publish(jobID string, log LogEntry) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    for _, stream := range s.subscribers[jobID] {
        select {
        case stream.Logs <- log:
        default:
            // Buffer full, drop oldest or handle backpressure
        }
    }
}

// Called when job/step status changes
func (s *LogService) PublishStatus(jobID string, status JobStatus) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    for _, stream := range s.subscribers[jobID] {
        select {
        case stream.Status <- status:
        default:
        }
    }
}
```

### Server-Side Filtering

All filtering happens server-side. The client sends filter params, server applies them:

```go
func (s *LogService) CountLogs(jobID, stepID, level string) int {
    query := s.db.Model(&LogEntry{}).Where("job_id = ?", jobID)
    
    if stepID != "" {
        query = query.Where("step_id = ?", stepID)
    }
    if level != "" {
        query = query.Where("level = ?", level)
    }
    
    var count int64
    query.Count(&count)
    return int(count)
}

func (s *LogService) GetLogs(jobID string, opts StreamOptions) []LogEntry {
    query := s.db.Where("job_id = ?", jobID).Order("timestamp ASC")
    
    if opts.StepID != "" {
        query = query.Where("step_id = ?", opts.StepID)
    }
    if opts.Level != "" {
        query = query.Where("level IN ?", levelsAtOrAbove(opts.Level))
    }
    if opts.SinceID != "" {
        query = query.Where("id > ?", opts.SinceID)
    }
    
    query = query.Limit(opts.Limit)
    
    var logs []LogEntry
    query.Find(&logs)
    return logs
}

func levelsAtOrAbove(level string) []string {
    levels := []string{"debug", "info", "warn", "error"}
    for i, l := range levels {
        if l == level {
            return levels[i:]
        }
    }
    return levels
}
```

## Client Implementation

### Global JavaScript Library

Create a reusable SSE log streaming library at `./pages/static/js/log-stream.js`:

```javascript
/**
 * Quaero SSE Log Streaming Library
 * Global utilities for log streaming across all pages
 */

const QuaeroLogs = (function() {
    'use strict';
    
    // Default configuration
    const defaults = {
        limit: 100,
        fallbackTimeoutMs: 15000,
        maxBufferSize: 1000,
        reconnectDelayMs: 1000
    };
    
    /**
     * Create a new log stream connection
     * @param {string} endpoint - SSE endpoint URL (e.g., '/api/jobs/{id}/logs/stream')
     * @param {Object} options - Stream options
     * @returns {LogStream}
     */
    function createStream(endpoint, options = {}) {
        return new LogStream(endpoint, { ...defaults, ...options });
    }
    
    /**
     * Create a service log stream (for global service logs on all pages)
     * @param {Object} options - Stream options
     * @returns {LogStream}
     */
    function createServiceStream(options = {}) {
        return new LogStream('/api/service/logs/stream', { 
            ...defaults, 
            ...options,
            isServiceLog: true 
        });
    }
    
    /**
     * Build query string from filter options
     * @param {Object} filters
     * @returns {string}
     */
    function buildQueryParams(filters) {
        const params = new URLSearchParams();
        
        if (filters.limit) params.set('limit', filters.limit);
        if (filters.stepId) params.set('step', filters.stepId);
        if (filters.level) params.set('level', filters.level);
        if (filters.since) params.set('since', filters.since);
        if (filters.jobId) params.set('job', filters.jobId);
        
        return params.toString();
    }
    
    /**
     * Format timestamp for display
     * @param {string} isoTimestamp
     * @param {boolean} includeDate
     * @returns {string}
     */
    function formatTimestamp(isoTimestamp, includeDate = false) {
        const date = new Date(isoTimestamp);
        if (includeDate) {
            return date.toLocaleString();
        }
        return date.toLocaleTimeString();
    }
    
    /**
     * Get CSS class for log level
     * @param {string} level
     * @returns {string}
     */
    function levelClass(level) {
        const classes = {
            'debug': 'log-level-debug',
            'info': 'log-level-info',
            'warn': 'log-level-warn',
            'warning': 'log-level-warn',
            'error': 'log-level-error',
            'fatal': 'log-level-fatal'
        };
        return classes[level?.toLowerCase()] || 'log-level-info';
    }
    
    /**
     * LogStream class - manages SSE connection and log state
     */
    class LogStream {
        constructor(endpoint, options) {
            this.endpoint = endpoint;
            this.options = options;
            this.eventSource = null;
            this.logs = [];
            this.totalCount = 0;
            this.displayedCount = 0;
            this.status = null;
            this.steps = [];
            this.lastEventTime = Date.now();
            this.fallbackTimer = null;
            this.connected = false;
            
            // Callbacks
            this.onLogs = null;
            this.onStatus = null;
            this.onError = null;
            this.onConnect = null;
            this.onDisconnect = null;
        }
        
        /**
         * Connect to SSE stream
         * @param {Object} filters - Optional filter overrides
         */
        connect(filters = {}) {
            if (this.eventSource) {
                this.disconnect();
            }
            
            const queryString = buildQueryParams({ 
                limit: this.options.limit,
                ...filters 
            });
            
            const url = queryString ? `${this.endpoint}?${queryString}` : this.endpoint;
            
            this.eventSource = new EventSource(url);
            
            this.eventSource.addEventListener('logs', (e) => {
                this.handleLogs(JSON.parse(e.data));
            });
            
            this.eventSource.addEventListener('status', (e) => {
                this.handleStatus(JSON.parse(e.data));
            });
            
            this.eventSource.addEventListener('ping', () => {
                this.lastEventTime = Date.now();
            });
            
            this.eventSource.onopen = () => {
                this.connected = true;
                this.lastEventTime = Date.now();
                if (this.onConnect) this.onConnect();
            };
            
            this.eventSource.onerror = (err) => {
                this.connected = false;
                if (this.onError) this.onError(err);
                if (this.onDisconnect) this.onDisconnect();
            };
            
            // Setup fallback timer
            this.startFallbackTimer(filters);
        }
        
        /**
         * Disconnect from SSE stream
         */
        disconnect() {
            if (this.eventSource) {
                this.eventSource.close();
                this.eventSource = null;
            }
            this.connected = false;
            this.stopFallbackTimer();
            if (this.onDisconnect) this.onDisconnect();
        }
        
        /**
         * Reconnect with new filters
         * @param {Object} filters
         */
        reconnect(filters = {}) {
            this.logs = [];
            this.connect(filters);
        }
        
        /**
         * Handle incoming log batch
         * @param {Object} data
         */
        handleLogs(data) {
            this.lastEventTime = Date.now();
            
            // Append new logs
            this.logs.push(...data.logs);
            
            // Trim buffer if needed
            const maxBuffer = this.options.maxBufferSize || 1000;
            if (this.logs.length > maxBuffer) {
                this.logs = this.logs.slice(-Math.floor(maxBuffer / 2));
            }
            
            this.totalCount = data.meta.total_count;
            this.displayedCount = data.meta.displayed_count;
            
            if (this.onLogs) this.onLogs(data);
        }
        
        /**
         * Handle status update
         * @param {Object} data
         */
        handleStatus(data) {
            this.lastEventTime = Date.now();
            this.status = data.job || data.service;
            this.steps = data.steps || [];
            
            if (this.onStatus) this.onStatus(data);
        }
        
        /**
         * Start fallback API polling timer
         * @param {Object} filters
         */
        startFallbackTimer(filters) {
            this.stopFallbackTimer();
            
            this.fallbackTimer = setInterval(() => {
                if (Date.now() - this.lastEventTime > this.options.fallbackTimeoutMs) {
                    this.fetchViaApi(filters);
                }
            }, this.options.fallbackTimeoutMs);
        }
        
        /**
         * Stop fallback timer
         */
        stopFallbackTimer() {
            if (this.fallbackTimer) {
                clearInterval(this.fallbackTimer);
                this.fallbackTimer = null;
            }
        }
        
        /**
         * Fallback API fetch
         * @param {Object} filters
         */
        async fetchViaApi(filters = {}) {
            try {
                const lastId = this.logs.length > 0 
                    ? this.logs[this.logs.length - 1].id 
                    : '';
                
                const apiEndpoint = this.endpoint.replace('/stream', '');
                const queryString = buildQueryParams({
                    limit: this.options.limit,
                    since: lastId,
                    ...filters
                });
                
                const res = await fetch(`${apiEndpoint}?${queryString}`);
                if (!res.ok) throw new Error(`HTTP ${res.status}`);
                
                const data = await res.json();
                this.handleLogs(data);
            } catch (err) {
                console.error('Log API fallback failed:', err);
                if (this.onError) this.onError(err);
            }
        }
        
        /**
         * Clear log buffer
         */
        clear() {
            this.logs = [];
            this.totalCount = 0;
            this.displayedCount = 0;
        }
        
        /**
         * Get current state for Alpine.js reactivity
         * @returns {Object}
         */
        getState() {
            return {
                logs: this.logs,
                totalCount: this.totalCount,
                displayedCount: this.displayedCount,
                status: this.status,
                steps: this.steps,
                connected: this.connected
            };
        }
    }
    
    // Public API
    return {
        createStream,
        createServiceStream,
        buildQueryParams,
        formatTimestamp,
        levelClass,
        LogStream
    };
})();

// Export for module systems if available
if (typeof module !== 'undefined' && module.exports) {
    module.exports = QuaeroLogs;
}
```

### Alpine.js Integration Mixin

Create `./pages/static/js/log-components.js`:

```javascript
/**
 * Alpine.js components for log streaming
 * Requires: log-stream.js
 */

/**
 * Job log viewer component
 * Usage: <div x-data="jobLogViewer('job-uuid')">
 */
function jobLogViewer(jobId) {
    return {
        stream: null,
        logs: [],
        totalCount: 0,
        displayedCount: 0,
        jobStatus: null,
        steps: [],
        connected: false,
        
        filters: {
            stepId: '',
            level: '',
            limit: 100
        },
        
        init() {
            this.stream = QuaeroLogs.createStream(`/api/jobs/${jobId}/logs/stream`);
            this.bindStreamCallbacks();
            this.stream.connect(this.filters);
            
            this.$watch('filters', () => this.stream.reconnect(this.filters), { deep: true });
        },
        
        destroy() {
            this.stream?.disconnect();
        },
        
        bindStreamCallbacks() {
            this.stream.onLogs = (data) => {
                this.logs = this.stream.logs;
                this.totalCount = this.stream.totalCount;
                this.displayedCount = this.stream.displayedCount;
                this.$nextTick(() => this.scrollToBottomIfNeeded());
            };
            
            this.stream.onStatus = (data) => {
                this.jobStatus = data.job;
                this.steps = data.steps || [];
            };
            
            this.stream.onConnect = () => { this.connected = true; };
            this.stream.onDisconnect = () => { this.connected = false; };
        },
        
        scrollToBottomIfNeeded() {
            const container = this.$refs.logContainer;
            if (!container) return;
            const isAtBottom = container.scrollHeight - container.scrollTop <= container.clientHeight + 50;
            if (isAtBottom) {
                container.scrollTop = container.scrollHeight;
            }
        },
        
        formatTime(ts) { return QuaeroLogs.formatTimestamp(ts); },
        levelClass(level) { return QuaeroLogs.levelClass(level); }
    };
}

/**
 * Service log viewer component (global, all pages)
 * Usage: <div x-data="serviceLogViewer()">
 */
function serviceLogViewer() {
    return {
        stream: null,
        logs: [],
        totalCount: 0,
        displayedCount: 0,
        serviceStatus: null,
        connected: false,
        expanded: false,
        unreadCount: 0,
        
        filters: {
            level: 'info',
            limit: 50
        },
        
        init() {
            this.stream = QuaeroLogs.createServiceStream({ limit: this.filters.limit });
            this.bindStreamCallbacks();
            this.stream.connect(this.filters);
            
            this.$watch('filters', () => this.stream.reconnect(this.filters), { deep: true });
        },
        
        destroy() {
            this.stream?.disconnect();
        },
        
        bindStreamCallbacks() {
            this.stream.onLogs = (data) => {
                const newLogCount = data.logs.length;
                this.logs = this.stream.logs;
                this.totalCount = this.stream.totalCount;
                this.displayedCount = this.stream.displayedCount;
                
                // Track unread if panel is collapsed
                if (!this.expanded) {
                    this.unreadCount += newLogCount;
                }
                
                if (this.expanded) {
                    this.$nextTick(() => this.scrollToBottom());
                }
            };
            
            this.stream.onStatus = (data) => {
                this.serviceStatus = data.service;
            };
            
            this.stream.onConnect = () => { this.connected = true; };
            this.stream.onDisconnect = () => { this.connected = false; };
        },
        
        toggle() {
            this.expanded = !this.expanded;
            if (this.expanded) {
                this.unreadCount = 0;
                this.$nextTick(() => this.scrollToBottom());
            }
        },
        
        scrollToBottom() {
            const container = this.$refs.serviceLogContainer;
            if (container) {
                container.scrollTop = container.scrollHeight;
            }
        },
        
        clear() {
            this.stream.clear();
            this.logs = [];
            this.unreadCount = 0;
        },
        
        formatTime(ts) { return QuaeroLogs.formatTimestamp(ts); },
        levelClass(level) { return QuaeroLogs.levelClass(level); },
        
        hasErrors() {
            return this.logs.some(l => l.level === 'error' || l.level === 'fatal');
        }
    };
}
```

### Service Logs Component (All Pages)

Add to base template or layout that appears on all pages:

```html
<!-- Service Logs Panel - Include in base layout -->
<div x-data="serviceLogViewer()" 
     x-init="init()" 
     @beforeunload.window="destroy()"
     class="service-logs-panel"
     :class="{ 'expanded': expanded }">
    
    <!-- Toggle Button -->
    <button @click="toggle()" class="service-logs-toggle">
        <span class="toggle-icon" :class="{ 'connected': connected }">●</span>
        <span>Service Logs</span>
        <span x-show="unreadCount > 0" class="unread-badge" x-text="unreadCount"></span>
        <span x-show="hasErrors()" class="error-indicator">!</span>
    </button>
    
    <!-- Expanded Panel -->
    <div x-show="expanded" x-transition class="service-logs-content">
        <!-- Header -->
        <div class="service-logs-header">
            <span>
                <strong x-text="logs.length"></strong> / <strong x-text="displayedCount"></strong>
                (<strong x-text="totalCount"></strong> total)
            </span>
            
            <div class="service-logs-controls">
                <select x-model="filters.level" class="level-filter">
                    <option value="">All</option>
                    <option value="debug">Debug+</option>
                    <option value="info">Info+</option>
                    <option value="warn">Warn+</option>
                    <option value="error">Error</option>
                </select>
                <button @click="clear()" class="clear-btn">Clear</button>
            </div>
        </div>
        
        <!-- Log Entries -->
        <div x-ref="serviceLogContainer" class="service-logs-entries">
            <template x-for="log in logs" :key="log.id">
                <div class="service-log-entry" :class="levelClass(log.level)">
                    <span class="log-time" x-text="formatTime(log.timestamp)"></span>
                    <span class="log-level" x-text="log.level"></span>
                    <span class="log-source" x-text="log.source" x-show="log.source"></span>
                    <span class="log-message" x-text="log.message"></span>
                </div>
            </template>
        </div>
    </div>
</div>

<!-- Include global JS libraries -->
<script src="/static/js/log-stream.js"></script>
<script src="/static/js/log-components.js"></script>
```

### CSS for Service Logs

Create or add to `./pages/static/css/log-stream.css`:

```css
/* Log level colors */
.log-level-debug { color: #6b7280; }
.log-level-info { color: #2563eb; }
.log-level-warn { color: #d97706; }
.log-level-error { color: #dc2626; }
.log-level-fatal { color: #7f1d1d; background: #fecaca; }

/* Service logs panel - fixed position bottom right */
.service-logs-panel {
    position: fixed;
    bottom: 0;
    right: 20px;
    width: 500px;
    max-width: calc(100vw - 40px);
    z-index: 1000;
    font-family: ui-monospace, monospace;
    font-size: 12px;
}

.service-logs-toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    background: #1f2937;
    color: #f3f4f6;
    border: none;
    border-radius: 8px 8px 0 0;
    cursor: pointer;
}

.toggle-icon {
    color: #ef4444;
}
.toggle-icon.connected {
    color: #22c55e;
}

.unread-badge {
    background: #3b82f6;
    color: white;
    padding: 2px 6px;
    border-radius: 10px;
    font-size: 10px;
}

.error-indicator {
    background: #dc2626;
    color: white;
    padding: 2px 6px;
    border-radius: 10px;
    font-weight: bold;
}

.service-logs-content {
    background: #111827;
    border: 1px solid #374151;
    border-top: none;
    max-height: 300px;
    display: flex;
    flex-direction: column;
}

.service-logs-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    background: #1f2937;
    color: #9ca3af;
    border-bottom: 1px solid #374151;
}

.service-logs-controls {
    display: flex;
    gap: 8px;
}

.service-logs-entries {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
}

.service-log-entry {
    display: flex;
    gap: 8px;
    padding: 2px 0;
    color: #d1d5db;
}

.log-time {
    color: #6b7280;
    flex-shrink: 0;
}

.log-level {
    flex-shrink: 0;
    width: 40px;
    text-transform: uppercase;
    font-size: 10px;
}

.log-source {
    color: #8b5cf6;
    flex-shrink: 0;
}

.log-message {
    flex: 1;
    word-break: break-word;
}
```

## Server Implementation - Service Logs Endpoint

Add a service-level log stream endpoint for the global service logs:

```go
// GET /api/service/logs/stream
func (h *Handler) StreamServiceLogs(w http.ResponseWriter, r *http.Request) {
    opts := StreamOptions{
        Level:  r.URL.Query().Get("level"),
        Limit:  parseIntOrDefault(r.URL.Query().Get("limit"), 50),
    }
    
    // SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")
    
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }
    
    // Subscribe to service-level logs (all jobs, system events)
    stream := h.logService.SubscribeService(r.Context(), opts)
    defer stream.Close()
    
    // Send initial service status
    h.sendServiceStatus(w, flusher)
    
    batchTicker := time.NewTicker(150 * time.Millisecond)
    pingTicker := time.NewTicker(5 * time.Second)
    defer batchTicker.Stop()
    defer pingTicker.Stop()
    
    var logBatch []LogEntry
    
    for {
        select {
        case <-r.Context().Done():
            return
            
        case log, ok := <-stream.Logs:
            if !ok {
                return
            }
            logBatch = append(logBatch, log)
            
        case status := <-stream.ServiceStatus:
            if len(logBatch) > 0 {
                h.sendServiceLogBatch(w, flusher, logBatch, opts)
                logBatch = logBatch[:0]
            }
            h.sendEvent(w, flusher, "status", map[string]any{"service": status})
            pingTicker.Reset(5 * time.Second)
            
        case <-batchTicker.C:
            if len(logBatch) > 0 {
                h.sendServiceLogBatch(w, flusher, logBatch, opts)
                logBatch = logBatch[:0]
                pingTicker.Reset(5 * time.Second)
            }
            
        case <-pingTicker.C:
            h.sendPing(w, flusher)
        }
    }
}
```

### Alpine.js Component

```javascript
function jobLogViewer(jobId) {
    return {
        logs: [],
        totalCount: 0,
        displayedCount: 0,
        jobStatus: null,
        steps: [],
        eventSource: null,
        
        // Filter state (triggers reconnect when changed)
        filters: {
            stepId: '',
            level: '',
            limit: 100
        },
        
        lastEventTime: Date.now(),
        fallbackTimer: null,
        
        init() {
            this.connect();
            this.$watch('filters', () => this.reconnect(), { deep: true });
        },
        
        connect() {
            const params = new URLSearchParams({
                limit: this.filters.limit,
                ...(this.filters.stepId && { step: this.filters.stepId }),
                ...(this.filters.level && { level: this.filters.level })
            });
            
            const url = `/api/jobs/${jobId}/logs/stream?${params}`;
            this.eventSource = new EventSource(url);
            
            this.eventSource.addEventListener('logs', (e) => {
                const data = JSON.parse(e.data);
                this.handleLogs(data);
                this.lastEventTime = Date.now();
            });
            
            this.eventSource.addEventListener('status', (e) => {
                const data = JSON.parse(e.data);
                this.handleStatus(data);
                this.lastEventTime = Date.now();
            });
            
            this.eventSource.addEventListener('ping', () => {
                this.lastEventTime = Date.now();
            });
            
            this.eventSource.onerror = () => {
                console.warn('SSE connection error, will auto-reconnect');
            };
            
            // Fallback API check every 15 seconds
            this.fallbackTimer = setInterval(() => {
                if (Date.now() - this.lastEventTime > 15000) {
                    console.warn('No SSE events for 15s, fetching via API');
                    this.fetchViaApi();
                }
            }, 15000);
        },
        
        disconnect() {
            this.eventSource?.close();
            this.eventSource = null;
            clearInterval(this.fallbackTimer);
        },
        
        reconnect() {
            this.disconnect();
            this.logs = [];
            this.connect();
        },
        
        handleLogs(data) {
            // Append new logs, maintain limit
            this.logs.push(...data.logs);
            if (this.logs.length > this.filters.limit * 2) {
                this.logs = this.logs.slice(-this.filters.limit);
            }
            
            this.totalCount = data.meta.total_count;
            this.displayedCount = data.meta.displayed_count;
            
            // Auto-scroll if at bottom
            this.$nextTick(() => this.scrollToBottomIfNeeded());
        },
        
        handleStatus(data) {
            this.jobStatus = data.job;
            this.steps = data.steps;
        },
        
        async fetchViaApi() {
            try {
                const lastId = this.logs.length > 0 
                    ? this.logs[this.logs.length - 1].id 
                    : '';
                
                const params = new URLSearchParams({
                    limit: this.filters.limit,
                    ...(this.filters.stepId && { step: this.filters.stepId }),
                    ...(this.filters.level && { level: this.filters.level }),
                    ...(lastId && { since: lastId })
                });
                
                const res = await fetch(`/api/jobs/${jobId}/logs?${params}`);
                const data = await res.json();
                this.handleLogs(data);
                this.lastEventTime = Date.now();
            } catch (err) {
                console.error('API fallback failed:', err);
            }
        },
        
        // UI helpers
        scrollToBottomIfNeeded() {
            const container = this.$refs.logContainer;
            if (!container) return;
            
            const isAtBottom = container.scrollHeight - container.scrollTop <= container.clientHeight + 50;
            if (isAtBottom) {
                container.scrollTop = container.scrollHeight;
            }
        },
        
        formatTimestamp(ts) {
            return new Date(ts).toLocaleTimeString();
        },
        
        levelClass(level) {
            return {
                'debug': 'text-gray-500',
                'info': 'text-blue-600',
                'warn': 'text-yellow-600',
                'error': 'text-red-600'
            }[level] || '';
        }
    }
}
```

### Job Log Viewer Template

Page-specific log viewer using the global library:

```html
<div x-data="jobLogViewer('{{ .JobID }}')" 
     x-init="init()" 
     @beforeunload.window="destroy()"
     class="log-viewer">
    
    <!-- Header with counts -->
    <div class="log-header">
        <span>
            Showing <strong x-text="logs.length"></strong> of 
            <strong x-text="displayedCount"></strong> filtered
            (<strong x-text="totalCount"></strong> total)
        </span>
        <span class="connection-status" :class="{ 'connected': connected }">
            <span x-text="connected ? 'Live' : 'Disconnected'"></span>
        </span>
    </div>
    
    <!-- Filters -->
    <div class="log-filters">
        <select x-model="filters.stepId">
            <option value="">All Steps</option>
            <template x-for="step in steps" :key="step.id">
                <option :value="step.id" x-text="step.name"></option>
            </template>
        </select>
        
        <select x-model="filters.level">
            <option value="">All Levels</option>
            <option value="debug">Debug+</option>
            <option value="info">Info+</option>
            <option value="warn">Warn+</option>
            <option value="error">Error</option>
        </select>
        
        <select x-model="filters.limit">
            <option value="50">50 logs</option>
            <option value="100">100 logs</option>
            <option value="250">250 logs</option>
            <option value="500">500 logs</option>
        </select>
    </div>
    
    <!-- Log entries -->
    <div x-ref="logContainer" class="log-container">
        <template x-for="log in logs" :key="log.id">
            <div class="log-entry" :class="levelClass(log.level)">
                <span class="log-time" x-text="formatTime(log.timestamp)"></span>
                <span class="log-level" x-text="log.level"></span>
                <span class="log-step" x-text="log.step_name" x-show="!filters.stepId"></span>
                <span class="log-message" x-text="log.message"></span>
            </div>
        </template>
    </div>
</div>

<!-- Global JS included in base layout -->
```

## Configuration

Add to Quaero config:

```yaml
logging:
  stream:
    batch_interval_ms: 150    # How often to batch logs to clients
    ping_interval_sec: 5      # Heartbeat interval
    default_limit: 100        # Default logs per request
    max_limit: 1000           # Maximum logs per request
    buffer_size: 100          # Channel buffer for log entries
```

## File Structure

```
./pages/static/
├── js/
│   ├── log-stream.js      # Core SSE library (QuaeroLogs)
│   └── log-components.js  # Alpine.js components
├── css/
│   └── log-stream.css     # Log styling
```

## Migration Steps

### Phase 1: Cleanup
1. **Audit existing code** - identify all WebSocket, polling, and signal code
2. **Remove WebSocket handlers** - delete gorilla/websocket dependencies and handlers
3. **Remove polling JS** - delete setInterval/setTimeout polling logic
4. **Remove signal endpoints** - delete refresh/notify endpoints
5. **Consolidate log endpoints** - merge redundant fetch endpoints

### Phase 2: Server Implementation  
1. **Add LogService** with pub/sub for job and service-level logs
2. **Add SSE handler** for job logs: `GET /api/jobs/{id}/logs/stream`
3. **Add SSE handler** for service logs: `GET /api/service/logs/stream`
4. **Update log writers** to call `LogService.Publish()` when writing logs
5. **Update job/step status changes** to call `LogService.PublishStatus()`
6. **Add fallback API endpoints** for non-SSE clients

### Phase 3: Client Implementation
1. **Create `./pages/static/js/log-stream.js`** - core SSE library
2. **Create `./pages/static/js/log-components.js`** - Alpine.js components
3. **Create `./pages/static/css/log-stream.css`** - log styling
4. **Add script includes** to base layout template
5. **Add service logs panel** to base layout (appears on all pages)
6. **Update job pages** to use new `jobLogViewer()` component
7. **Remove old log-related JS** from individual pages

### Phase 4: Testing & Verification
1. **Test high-volume jobs** - verify batching under load
2. **Test service logs** - verify global panel on all pages
3. **Test filter changes** - verify reconnect behaviour
4. **Test connection recovery** - verify auto-reconnect and fallback
5. **Test memory usage** - verify no leaks with long-running streams

## Testing Checklist

### Job Log Streaming
- [ ] Logs stream in real-time during job execution
- [ ] Total count and displayed count update correctly
- [ ] Status changes flush pending logs immediately
- [ ] Filter changes reconnect with new params (step, level, limit)
- [ ] Connection indicator shows live/disconnected state
- [ ] Auto-scroll works when viewing latest logs

### Service Logs (Global Panel)
- [ ] Service logs panel appears on all pages
- [ ] Panel toggle expands/collapses correctly
- [ ] Unread badge shows count when panel is collapsed
- [ ] Error indicator appears when error logs received
- [ ] Clear button empties log buffer
- [ ] Level filter works correctly
- [ ] Panel persists state across page navigation (optional)

### Reliability
- [ ] Connection auto-recovers after network interruption
- [ ] Fallback API works when SSE silent for 15+ seconds
- [ ] No memory leaks with long-running streams
- [ ] Handles 1000+ logs/second without client overload
- [ ] Works behind nginx/reverse proxy (X-Accel-Buffering: no)
- [ ] Graceful handling when server restarts

### Code Cleanup Verification
- [ ] No WebSocket code remains in codebase
- [ ] No polling logic remains in JS
- [ ] No signal/refresh endpoints remain
- [ ] gorilla/websocket removed from go.mod (if no longer needed)
- [ ] All old log-related JS removed from individual pages