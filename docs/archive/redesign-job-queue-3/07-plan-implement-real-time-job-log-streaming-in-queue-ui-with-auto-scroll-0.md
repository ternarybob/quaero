I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State

**Existing Infrastructure:**
- Job logs modal exists with basic UI (lines 613-670 in queue.html)
- jobLogsModal Alpine component handles state and API calls (lines 2595-2699)
- WebSocket connection established with message routing (lines 963-1080)
- GetAggregatedJobLogsHandler API provides parent+child log aggregation with enrichment
- BroadcastLog() method in WebSocket handler broadcasts log entries in real-time
- Terminal CSS styles defined for log display (quaero.css lines 405-445)
- serviceLogs component in common.js demonstrates auto-scroll pattern (lines 27-178)

**Current Modal Features:**
- Log level filtering (all, error, warning, info, debug)
- "Errors Only" toggle button
- Refresh button
- Terminal-style display
- Loading state
- Uses single-job API endpoint (`/api/jobs/{id}/logs`)

**Missing Features:**
1. Real-time WebSocket log streaming
2. Auto-scrolling (newest logs at bottom)
3. Use of aggregated logs API (parent+child logs)
4. "Clear Logs" button
5. "Download Logs" button
6. Job context enrichment (job name, URL, depth in log display)
7. Include children toggle

## Architecture Decisions

**1. WebSocket Integration Strategy:**
- Add log event handler in connectJobsWebSocket() function (line 983)
- Filter incoming log events by currentJobId in modal
- Append real-time logs to existing logs array (no deduplication needed - logs are append-only)
- Use event-driven pattern: dispatch custom event to modal component

**2. API Migration:**
- Switch from `/api/jobs/{id}/logs` to `/api/jobs/{id}/logs/aggregated`
- Add includeChildren parameter (default: true)
- Use enriched log format with job context (job_name, job_url, job_depth, job_type)
- Maintain backward compatibility with existing log parsing

**3. Auto-Scroll Implementation:**
- Follow serviceLogs pattern from common.js (lines 102-113)
- Use autoScroll boolean flag (default: true)
- Detect user scroll: disable auto-scroll when user scrolls up
- Re-enable auto-scroll when user scrolls to bottom
- Use $nextTick + requestAnimationFrame for reliable scrolling

**4. State Management:**
- Add state variables: autoScroll, includeChildren, isStreaming, logBuffer
- Use logBuffer for batching WebSocket logs (prevent UI thrashing)
- Flush buffer every 500ms or when 10 logs accumulated
- Maintain chronological order (logs already sorted by API and WebSocket)

**5. Clear Logs:**
- Client-side only: clears logs array in modal
- Does not delete from database
- Provides "fresh start" for viewing new logs

**6. Download Logs:**
- Generate text file from logs array
- Format: `[HH:MM:SS] [LEVEL] [Job Name] Message`
- Use Blob + download link pattern
- Filename: `job-logs-{jobId}-{timestamp}.txt`

**7. Log Enrichment Display:**
- Show job context for each log entry (optional, collapsed by default)
- Format: `[Job: {name}] [{url}] [Depth: {depth}]`
- Use different colors for parent vs child job logs
- Add job type badge next to log level

## Performance Considerations

**Throttling:**
- Batch WebSocket logs (flush every 500ms or 10 logs)
- Limit displayed logs to 1000 entries (configurable)
- Use virtual scrolling if > 1000 logs (future enhancement)

**Memory Management:**
- Trim logs array when exceeds maxLogs (1000)
- Remove oldest logs (FIFO)
- Clear logs when modal closes

**WebSocket Filtering:**
- Filter logs by jobId on client side (server broadcasts all logs)
- For parent jobs with includeChildren=true, accept logs from parent and all children
- Use Set for efficient child job ID lookup

## Edge Cases

**1. Modal Closed During Streaming:**
- Stop listening to WebSocket log events
- Clear log buffer
- Preserve logs array for re-opening

**2. Job Completion:**
- Continue streaming logs until modal closed
- Show "Job completed" indicator
- Disable "Include Children" toggle for completed jobs

**3. WebSocket Disconnection:**
- Show disconnected indicator
- Logs continue to display (from API)
- Resume streaming when reconnected

**4. Empty Logs:**
- Show "No logs available" message
- Provide "Refresh" button
- Explain that logs may take a few seconds to appear

**5. Level Filtering with Real-Time Logs:**
- Apply filter to both API logs and WebSocket logs
- Filter WebSocket logs before adding to buffer
- Re-filter existing logs when level changes

## No Backend Changes Required

All functionality can be implemented with existing backend:
- GetAggregatedJobLogsHandler provides all needed data
- BroadcastLog() already broadcasts log entries
- LogService already enriches logs with job context
- No new API endpoints needed

### Approach

Enhance the existing job logs modal to support real-time log streaming via WebSocket, auto-scrolling, and additional controls (Clear/Download). The solution leverages the existing aggregated logs API, WebSocket infrastructure, and Alpine.js patterns already established in the codebase. All changes are frontend-focused with no backend modifications required.

### Reasoning

I explored the queue.html template structure (2563 lines), analyzed the existing jobLogsModal Alpine component (lines 2595-2699), reviewed the WebSocket connection handling (lines 963-1080), examined the GetAggregatedJobLogsHandler API (job_handler.go lines 529-666), studied the LogEntry interface and JobLogEntry model structures, reviewed the terminal CSS styles in quaero.css, and analyzed the serviceLogs component pattern in common.js for auto-scroll implementation reference.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Modal as jobLogsModal
    participant API as Aggregated Logs API
    participant WS as WebSocket
    participant Backend as LogService

    Note over User,Backend: Initial Load Flow
    User->>Modal: Click "View Logs" button
    Modal->>Modal: openModal(jobId)
    Modal->>Modal: Set includeChildren=true, autoScroll=true
    Modal->>API: GET /api/jobs/{id}/logs/aggregated?include_children=true&order=asc
    API-->>Modal: {logs: [...], metadata: {...}, count: 150}
    Modal->>Modal: Parse enriched logs (job_name, job_url, etc.)
    Modal->>Modal: Store child job IDs from metadata
    Modal->>Modal: scrollToBottom() if autoScroll=true
    Modal->>User: Display logs in terminal

    Note over User,Backend: Real-Time Streaming Flow
    Backend->>WS: BroadcastLog(entry) - new log generated
    WS->>Modal: WebSocket message {type: 'log', payload: {...}}
    Modal->>Modal: handleWebSocketLog(logData)
    Modal->>Modal: Filter: check if log.job_id matches currentJobId or childJobIds
    alt Log matches job filter
        Modal->>Modal: Apply level filter (if selectedLogLevel !== 'all')
        alt Log passes level filter
            Modal->>Modal: Add to logBuffer[]
            Modal->>Modal: scheduleFlush() if not scheduled
            Note over Modal: Wait 500ms or 10 logs
            Modal->>Modal: flushLogBuffer()
            Modal->>Modal: Append buffer to logs array
            Modal->>Modal: Trim if logs.length > maxLogs (1000)
            Modal->>Modal: scrollToBottom() if autoScroll=true
            Modal->>User: New logs appear at bottom
        end
    end

    Note over User,Backend: User Interaction Flow
    User->>Modal: Scroll up in terminal
    Modal->>Modal: handleScroll() - detect scroll position
    Modal->>Modal: Set autoScroll=false (user scrolled up)
    Modal->>User: Auto-scroll disabled indicator

    User->>Modal: Scroll to bottom
    Modal->>Modal: handleScroll() - detect at bottom
    Modal->>Modal: Set autoScroll=true (re-enable)
    Modal->>User: Auto-scroll enabled indicator

    User->>Modal: Click "Clear Logs"
    Modal->>Modal: clearLogs() - clear logs array
    Modal->>Modal: clearLogBuffer() - flush buffer
    Modal->>User: Empty terminal display

    User->>Modal: Click "Download Logs"
    Modal->>Modal: downloadLogs() - generate text file
    Modal->>Modal: Format: [timestamp] [level] [job_name] message
    Modal->>Modal: Create Blob and download link
    Modal->>User: Browser downloads job-logs-{id}-{timestamp}.txt

    User->>Modal: Toggle "Include Children"
    Modal->>Modal: includeChildren = !includeChildren
    Modal->>API: Reload logs with new parameter
    API-->>Modal: Updated logs (parent only or parent+children)
    Modal->>User: Display updated logs

    User->>Modal: Close modal
    Modal->>Modal: closeModal()
    Modal->>Modal: Stop streaming (isStreaming=false)
    Modal->>Modal: Clear buffer and child job IDs
    Modal->>User: Modal hidden

## Proposed File Changes

### pages\queue.html(MODIFY)

References: 

- pages\static\quaero.css(MODIFY)
- pages\static\common.js
- internal\handlers\job_handler.go
- internal\handlers\websocket.go

**Enhance Job Logs Modal UI (lines 613-670):**

**Add Include Children Toggle (after line 631):**
- Add checkbox control: `<label class="form-checkbox"><input type="checkbox" x-model="includeChildren" @change="loadLogs()"><i class="form-icon"></i> Include Child Jobs</label>`
- Place between log level filter and error toggle
- Binds to Alpine state variable

**Add Auto-Scroll Toggle (after line 637):**
- Add button: `<button class="btn btn-sm" :class="autoScroll ? 'btn-primary' : ''" @click="toggleAutoScroll()" title="Toggle auto-scroll"><i class="fas fa-arrow-down"></i> Auto-Scroll</button>`
- Shows active state when enabled
- Place after refresh button

**Add Clear Logs Button (after auto-scroll toggle):**
- Add button: `<button class="btn btn-sm" @click="clearLogs()" title="Clear displayed logs"><i class="fas fa-trash"></i> Clear</button>`
- Client-side only operation

**Add Download Logs Button (after clear button):**
- Add button: `<button class="btn btn-sm" @click="downloadLogs()" title="Download logs as text file"><i class="fas fa-download"></i> Download</button>`
- Generates text file from current logs

**Add Streaming Indicator (in modal header, after title):**
- Add indicator: `<span x-show="isStreaming" class="label label-success" style="margin-left: 0.5rem;"><i class="fas fa-circle" style="animation: pulse 2s infinite;"></i> Live</span>`
- Shows when WebSocket is connected and streaming
- Add CSS animation for pulse effect

**Enhance Terminal Display (lines 649-662):**
- Add `x-ref="logContainer"` to terminal div for scroll control
- Add `@scroll="handleScroll()"` to detect user scrolling
- Modify log entry template to include job context:
  ```html
  <div class="terminal-line">
    <span class="terminal-time" x-text="`[${log.timestamp}]`"></span>
    <span :class="log.levelClass" x-text="`[${log.level.toUpperCase()}]`"></span>
    <span x-show="log.job_name" class="terminal-job-context" x-text="`[${log.job_name}]`"></span>
    <span x-text="log.message"></span>
  </div>
  ```
- Add job context display (job name) between level and message
- Use conditional display for job context (only show if available)

**Add Log Count Display (in modal footer, before Close button):**
- Add text: `<span class="text-secondary" style="margin-right: auto;" x-text="`${logs.length} logs displayed`"></span>`
- Shows current log count

**Add CSS for Job Context and Pulse Animation (in <style> section, lines 21-99):**
- Add `.terminal-job-context { color: #58a6ff; font-weight: 500; margin-right: 0.5rem; }`
- Add `@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }`
- Styles for job context display and streaming indicator
**Refactor jobLogsModal Alpine Component (lines 2595-2699):**

**Add State Variables (after line 2599):**
- `includeChildren: true` - Toggle for including child job logs
- `autoScroll: true` - Auto-scroll to bottom flag
- `isStreaming: false` - WebSocket streaming active flag
- `logBuffer: []` - Buffer for batching WebSocket logs
- `flushTimer: null` - Timer for flushing log buffer
- `maxLogs: 1000` - Maximum logs to display
- `childJobIds: new Set()` - Set of child job IDs for filtering
- `wsEventListener: null` - Reference to WebSocket event listener for cleanup

**Update init() Method (lines 2601-2606):**
- Keep existing modal open event listener
- Add WebSocket log event listener: `this.wsEventListener = (e) => this.handleWebSocketLog(e.detail)`
- Register listener: `window.addEventListener('jobLogs:newLog', this.wsEventListener)`
- Add cleanup on component destroy

**Refactor openModal() Method (lines 2608-2615):**
- Keep existing logic for setting currentJobId and opening modal
- Add: `this.isStreaming = wsConnected` (check global WebSocket state)
- Add: `this.childJobIds.clear()` (reset child job IDs)
- Call `this.loadLogs()` as before
- After loadLogs completes, fetch child job IDs if includeChildren=true

**Update closeModal() Method (lines 2617-2623):**
- Keep existing logic
- Add: `this.isStreaming = false`
- Add: `this.clearLogBuffer()` (flush and clear buffer)
- Add: `this.childJobIds.clear()`

**Refactor loadLogs() Method (lines 2625-2649):**
- Change API endpoint from `/api/jobs/${this.currentJobId}/logs` to `/api/jobs/${this.currentJobId}/logs/aggregated`
- Add query parameters: `include_children=${this.includeChildren}`, `order=asc` (oldest-first for scrolling)
- Keep level parameter: `level=${this.selectedLogLevel}`
- Parse enriched log format (includes job_id, job_name, job_url, job_depth, job_type)
- Store child job IDs from metadata: `Object.keys(data.metadata).forEach(id => this.childJobIds.add(id))`
- Map logs with enrichment: `this.logs = rawLogs.map(log => this._parseEnrichedLogEntry(log))`
- After loading, scroll to bottom if autoScroll=true
- Set isStreaming=true if WebSocket connected

**Add New Method: _parseEnrichedLogEntry() (after _parseLogEntry):**
- Parse enriched log entry from aggregated API
- Extract: timestamp, full_timestamp, level, message, job_id, job_name, job_url, job_depth, job_type, parent_id
- Return object with all fields plus levelClass from _getLevelClass()
- Handle missing fields gracefully (use defaults)

**Add New Method: handleWebSocketLog() (after loadLogs):**
- Signature: `handleWebSocketLog(logData)`
- Filter by job: check if `logData.job_id === this.currentJobId` OR `this.childJobIds.has(logData.job_id)`
- If includeChildren=false, only accept logs from currentJobId
- Apply level filter: skip if selectedLogLevel !== 'all' and log level doesn't match
- Parse log entry: `const entry = this._parseLogEntry(logData)`
- Add to buffer: `this.logBuffer.push(entry)`
- Schedule flush if not already scheduled: `if (!this.flushTimer) this.scheduleFlush()`

**Add New Method: scheduleFlush() (after handleWebSocketLog):**
- Set timer: `this.flushTimer = setTimeout(() => this.flushLogBuffer(), 500)`
- Flush after 500ms or when buffer reaches 10 logs (check in handleWebSocketLog)

**Add New Method: flushLogBuffer() (after scheduleFlush):**
- If buffer empty, return
- Append buffer to logs: `this.logs.push(...this.logBuffer)`
- Trim logs if exceeds maxLogs: `if (this.logs.length > this.maxLogs) this.logs = this.logs.slice(-this.maxLogs)`
- Clear buffer: `this.logBuffer = []`
- Clear timer: `this.flushTimer = null`
- Auto-scroll if enabled: call `this.scrollToBottom()`

**Add New Method: clearLogBuffer() (after flushLogBuffer):**
- Clear buffer: `this.logBuffer = []`
- Clear timer: `if (this.flushTimer) { clearTimeout(this.flushTimer); this.flushTimer = null; }`

**Add New Method: scrollToBottom() (after clearLogBuffer):**
- Use $nextTick for DOM update: `this.$nextTick(() => { ... })`
- Get container: `const container = this.$refs.logContainer`
- Scroll: `if (container) container.scrollTop = container.scrollHeight`
- Use requestAnimationFrame for reliability (like serviceLogs pattern)

**Add New Method: handleScroll() (after scrollToBottom):**
- Get container: `const container = this.$refs.logContainer`
- Check if at bottom: `const isAtBottom = container.scrollHeight - container.scrollTop <= container.clientHeight + 50`
- Update autoScroll: `this.autoScroll = isAtBottom`
- This disables auto-scroll when user scrolls up, re-enables when scrolling to bottom

**Add New Method: toggleAutoScroll() (update existing, line 2695):**
- Toggle flag: `this.autoScroll = !this.autoScroll`
- If enabled, scroll to bottom immediately: `if (this.autoScroll) this.scrollToBottom()`

**Add New Method: clearLogs() (after toggleAutoScroll):**
- Clear logs array: `this.logs = []`
- Clear buffer: `this.clearLogBuffer()`
- Show notification: `window.showNotification('Logs cleared', 'info')`

**Add New Method: downloadLogs() (after clearLogs):**
- Generate text content: `const content = this.logs.map(log => `[${log.timestamp}] [${log.level}] ${log.job_name ? '[' + log.job_name + '] ' : ''}${log.message}`).join('\n')`
- Create Blob: `const blob = new Blob([content], { type: 'text/plain' })`
- Create download link: `const url = URL.createObjectURL(blob)`
- Generate filename: `const filename = `job-logs-${this.currentJobId.substring(0, 8)}-${new Date().toISOString().replace(/[:.]/g, '-')}.txt``
- Trigger download: create temporary anchor element, set href and download attributes, click, remove
- Revoke URL: `URL.revokeObjectURL(url)`
- Show notification: `window.showNotification('Logs downloaded', 'success')`

**Update _parseLogEntry() Method (lines 2651-2662):**
- Keep existing logic for backward compatibility
- Add support for enriched fields if present: job_id, job_name, job_url, job_depth, job_type
- Return object with all available fields

**Add Cleanup on Component Destroy:**
- Add method: `destroy() { if (this.wsEventListener) window.removeEventListener('jobLogs:newLog', this.wsEventListener); this.clearLogBuffer(); }`
- Alpine.js will call this automatically when component is destroyed
**Add WebSocket Log Event Handler (in connectJobsWebSocket function, lines 963-1080):**

**Add Log Message Handler (in jobsWS.onmessage, after line 1042):**
- Add new message type handler:
  ```javascript
  // Handle log events for job logs modal
  if (message.type === 'log' && message.payload) {
      const logData = message.payload;
      // Dispatch to job logs modal if open
      window.dispatchEvent(new CustomEvent('jobLogs:newLog', {
          detail: logData
      }));
  }
  ```
- Place after job_spawn handler (line 1042)
- Before the closing brace of onmessage (line 1046)

**Rationale:**
- WebSocket handler already exists and is connected
- BroadcastLog() in backend sends messages with type='log'
- Modal component filters logs by job ID
- Event-driven pattern maintains loose coupling
- No changes to WebSocket connection logic needed

**Update WebSocket Connection State (in jobsWS.onopen, line 971):**
- After setting wsConnected=true (line 973), dispatch event to modal:
  ```javascript
  window.dispatchEvent(new CustomEvent('jobLogs:streamingStateChange', {
      detail: { isStreaming: true }
  }));
  ```
- This notifies modal that streaming is available

**Update WebSocket Disconnection State (in jobsWS.onclose, line 1057):**
- After setting wsConnected=false (line 1058), dispatch event to modal:
  ```javascript
  window.dispatchEvent(new CustomEvent('jobLogs:streamingStateChange', {
      detail: { isStreaming: false }
  }));
  ```
- This notifies modal that streaming is unavailable

**Add Event Listener in jobLogsModal init():**
- Listen for streaming state changes: `window.addEventListener('jobLogs:streamingStateChange', (e) => { this.isStreaming = e.detail.isStreaming; })`
- Update isStreaming flag when WebSocket connects/disconnects

### pages\static\quaero.css(MODIFY)

**Add CSS Styles for Job Logs Modal Enhancements (after line 445):**

**Add Job Context Styling:**
- Add rule: `.terminal-job-context { color: #58a6ff; font-weight: 500; margin-right: 0.5rem; font-size: 0.9em; }`
- Styles the job name/context display in log entries
- Uses blue color to distinguish from log message
- Slightly smaller font size for visual hierarchy

**Add Pulse Animation for Streaming Indicator:**
- Add keyframes:
  ```css
  @keyframes pulse {
      0%, 100% { opacity: 1; }
      50% { opacity: 0.5; }
  }
  ```
- Used by the "Live" streaming indicator in modal header
- Subtle pulsing effect to indicate active streaming

**Add Auto-Scroll Button Active State:**
- Add rule: `.btn.btn-primary.auto-scroll-active { background-color: #3b82f6; color: white; }`
- Provides visual feedback when auto-scroll is enabled
- Matches existing button styling patterns

**Add Log Count Display Styling:**
- Add rule: `.log-count-display { font-size: 0.9em; color: var(--text-secondary); }`
- Styles the log count text in modal footer
- Subtle, non-intrusive display

**Enhance Terminal Container for Scroll Detection:**
- Update `.terminal` rule to include: `scroll-behavior: smooth;`
- Provides smooth scrolling when auto-scroll is active
- Improves user experience

**Add Responsive Styles for Modal Controls:**
- Add media query for mobile:
  ```css
  @media (max-width: 768px) {
      .modal-body .btn-sm { font-size: 0.7rem; padding: 0.15rem 0.4rem; }
      .terminal-job-context { display: none; }
  }
  ```
- Reduces button size on mobile
- Hides job context on mobile to save space
- Maintains usability on small screens