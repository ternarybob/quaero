I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State

**Polling Mechanism (Lines 955-969):**
- `startAutoRefresh()` sets up 5-second interval
- Only polls when `hasRunningJobs` is true
- Calls `loadJobs()` and `loadStats()` on each tick
- Started in DOMContentLoaded (line 985)

**Existing WebSocket Pattern (Lines 1008-1102):**
- Alpine.js `queueStatsHeader` component for queue statistics
- Connects to `/ws` endpoint
- Handles `queue_stats` message type
- Fixed 5-second reconnection delay on disconnect
- Demonstrates working WebSocket connection management

**Job State Management (Lines 266-281):**
- `allJobs` - array of all jobs from server
- `filteredJobs` - filtered subset for display
- `selectedJobIds` - Set for batch operations
- `currentPage`, `totalJobs`, `pageSize` - pagination state

**Job Rendering (Lines 541-660):**
- `renderJobs()` creates job cards from `filteredJobs`
- Extracts document count from `job.progress.completed_urls`
- Shows status badges, action buttons based on job status
- Preserves selection state during re-render

**Server Message Format (websocket.go:128-141):**
- `JobStatusUpdate` struct with fields: job_id, status, source_type, entity_type, result_count, failed_count, total_urls, completed_urls, pending_urls, error, duration, timestamp
- Broadcast via `BroadcastJobStatusChange()` method
- Events published for: created, started, completed, failed, cancelled

## Design Decisions

**1. WebSocket Connection Management:**
- Create standalone connection manager (not Alpine.js) for flexibility
- Connect on page load, before initial data fetch
- Store connection state: `wsConnected`, `wsReconnectAttempts`

**2. Exponential Backoff:**
- Start with 1 second, double on each failure, max 30 seconds
- Reset attempts counter on successful connection
- Better than fixed 5-second delay (reduces server load during outages)

**3. Polling Fallback:**
- Keep existing `startAutoRefresh()` mechanism
- Modify to check `wsConnected` flag
- Only poll when WebSocket is disconnected OR as backup every 30 seconds
- Ensures data freshness even if WebSocket silently fails

**4. Job Update Strategy:**
- `updateJobInList(update)` finds job by ID in `allJobs`
- Updates fields: status, result_count, failed_count, progress object
- Re-applies filters to determine if job should be in `filteredJobs`
- Calls `renderJobs()` to update UI without full page reload
- Preserves pagination, selection, and scroll position

**5. Event Handling:**
- `job_status_change` - updates existing job in list
- `job_created` - adds new job if it matches current filters
- `job_completed` - updates job and refreshes stats
- All events trigger `updateJobInList()` with appropriate data

**6. Stats Synchronization:**
- WebSocket updates keep job list fresh
- Stats still updated via existing mechanisms (loadStats on actions)
- Consider adding stats update on job status changes (increment/decrement counters)

### Approach

**WebSocket-First with Polling Fallback**

Add WebSocket connection management for real-time job status updates while maintaining the existing polling mechanism as a fallback. Follow the established `queueStatsHeader` Alpine.js pattern but implement as a standalone connection manager to handle job lifecycle events. Use exponential backoff for reconnection to reduce server load during outages.

### Reasoning

I explored queue.html to understand the current polling mechanism (lines 955-969), existing WebSocket pattern in queueStatsHeader Alpine.js component (lines 1008-1102), job rendering logic (lines 541-660), state management (lines 266-281), and action functions (lines 787-953). I also examined websocket.go to understand the JobStatusUpdate message format (lines 128-141) and BroadcastJobStatusChange method (lines 609-641). This revealed that the backend infrastructure is complete and the UI just needs to consume the events.

## Mermaid Diagram

sequenceDiagram
    participant Browser as Browser UI
    participant WS as WebSocket Connection
    participant Server as WebSocket Handler
    participant EventBus as Event Service
    participant JobWorker as Job Worker

    Note over Browser,JobWorker: Page Load & Initialization
    Browser->>Browser: DOMContentLoaded
    Browser->>Browser: loadFiltersFromStorage()
    Browser->>Browser: connectJobsWebSocket()
    Browser->>WS: new WebSocket('/ws')
    WS->>Server: Connect
    Server-->>WS: Connection established
    WS->>Browser: onopen (wsConnected = true)
    Browser->>Browser: renderFilterChips()
    Browser->>Server: GET /api/jobs/stats
    Server-->>Browser: Stats data
    Browser->>Server: GET /api/jobs?filters
    Server-->>Browser: Job list
    Browser->>Browser: renderJobs()
    Browser->>Browser: startAutoRefresh() (fallback)

    Note over Browser,JobWorker: Job Status Change (Running → Completed)
    JobWorker->>EventBus: Publish(EventJobCompleted)
    EventBus->>Server: EventSubscriber receives event
    Server->>Server: Transform to JobStatusUpdate
    Server->>WS: BroadcastJobStatusChange(update)
    WS->>Browser: message: {type: "job_status_change", payload: {...}}
    Browser->>Browser: onmessage handler
    Browser->>Browser: updateJobInList(payload)
    Browser->>Browser: Find job in allJobs by ID
    Browser->>Browser: Update job.status, counts, progress
    Browser->>Browser: matchesActiveFilters(job)
    Browser->>Browser: Update filteredJobs array
    Browser->>Browser: renderJobs() (in-place update)
    Browser->>Server: GET /api/jobs/stats (update counters)
    Server-->>Browser: Updated stats

    Note over Browser,JobWorker: WebSocket Disconnection
    WS->>Browser: onclose
    Browser->>Browser: wsConnected = false
    Browser->>Browser: Calculate backoff delay (1s → 2s → 4s → ... → 30s)
    Browser->>Browser: setTimeout(connectJobsWebSocket, delay)
    Note over Browser: Polling fallback activates
    Browser->>Server: GET /api/jobs (every 5s while disconnected)
    Server-->>Browser: Job list
    Browser->>Browser: renderJobs()

    Note over Browser,JobWorker: WebSocket Reconnection
    Browser->>WS: new WebSocket('/ws')
    WS->>Server: Connect
    Server-->>WS: Connection established
    WS->>Browser: onopen (wsConnected = true, attempts = 0)
    Note over Browser: Polling becomes backup only

    Note over Browser,JobWorker: User Action (Cancel Job)
    Browser->>Server: POST /api/jobs/{id}/cancel
    Server->>JobWorker: Cancel job
    JobWorker->>EventBus: Publish(EventJobCancelled)
    EventBus->>Server: EventSubscriber receives event
    Server->>WS: BroadcastJobStatusChange(update)
    WS->>Browser: message: {type: "job_status_change", payload: {...}}
    Browser->>Browser: updateJobInList(payload)
    Browser->>Browser: renderJobs() (no manual loadJobs needed)

## Proposed File Changes

### pages\queue.html(MODIFY)

References: 

- internal\handlers\websocket.go

**Location 1: After line 274 (autoRefreshInterval declaration)**

Add WebSocket connection state variables:
- `let jobsWS = null;` - WebSocket connection for job updates
- `let wsConnected = false;` - connection status flag
- `let wsReconnectAttempts = 0;` - counter for exponential backoff
- `const WS_MAX_RECONNECT_DELAY = 30000;` - max 30 seconds between reconnects
- `const WS_INITIAL_RECONNECT_DELAY = 1000;` - start with 1 second

**Location 2: After line 417 (end of loadJobs function)**

Add `updateJobInList(update)` function:
- Accept `update` parameter (JobStatusUpdate from WebSocket)
- Find job in `allJobs` by `update.job_id`
- If job not found and status is 'pending', fetch full job data from `/api/jobs/{id}` and insert
- If job found, update fields:
  - `job.status = update.status`
  - `job.result_count = update.result_count`
  - `job.failed_count = update.failed_count`
  - Update `job.progress` object: `completed_urls`, `pending_urls`, `total_urls`
  - If `update.error`, set `job.error = update.error`
- Re-apply filters: check if updated job matches `activeFilters` (status, source, entity)
- Update `filteredJobs` array:
  - If job matches filters and not in `filteredJobs`, insert at correct position (sorted by created_at DESC)
  - If job doesn't match filters, remove from `filteredJobs`
  - If job already in `filteredJobs`, update in place
- Call `renderJobs()` to update UI
- Log update for debugging: `console.log('[Queue] Job updated via WebSocket:', update.job_id.substring(0, 8), update.status)`

**Location 3: After line 539 (end of removeFilter function)**

Add `connectJobsWebSocket()` function:
- Construct WebSocket URL: `const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'; const wsUrl = \`\${protocol}//\${window.location.host}/ws\`;`
- Create WebSocket: `jobsWS = new WebSocket(wsUrl);`
- **onopen handler:**
  - Set `wsConnected = true`
  - Reset `wsReconnectAttempts = 0`
  - Log: `console.log('[Queue] Jobs WebSocket connected')`
- **onmessage handler:**
  - Parse message: `const message = JSON.parse(event.data);`
  - Handle `job_status_change` type:
    - Extract payload: `const update = message.payload;`
    - Call `updateJobInList(update)`
    - If status is 'completed' or 'failed' or 'cancelled', call `loadStats()` to update counters
  - Handle `job_created` type (future-proof):
    - Call `updateJobInList(message.payload)` to add new job
    - Call `loadStats()` to update counters
  - Log received messages: `console.log('[Queue] WebSocket message:', message.type)`
- **onerror handler:**
  - Set `wsConnected = false`
  - Log: `console.error('[Queue] Jobs WebSocket error:', error)`
- **onclose handler:**
  - Set `wsConnected = false`
  - Calculate reconnect delay with exponential backoff:
    - `const delay = Math.min(WS_INITIAL_RECONNECT_DELAY * Math.pow(2, wsReconnectAttempts), WS_MAX_RECONNECT_DELAY);`
    - Increment `wsReconnectAttempts++`
  - Log: `console.log('[Queue] Jobs WebSocket disconnected, reconnecting in', delay, 'ms')`
  - Schedule reconnection: `setTimeout(() => { connectJobsWebSocket(); }, delay);`

**Location 4: Modify startAutoRefresh function (lines 956-969)**

Update polling logic to work as fallback:
- Keep existing interval setup (lines 957-959)
- Modify interval callback (lines 961-968):
  - Add condition: `if (!wsConnected || /* backup polling every 30s */)` before checking `hasRunningJobs`
  - Add backup polling logic: track last poll time, force poll every 30 seconds even if WebSocket connected
  - Keep existing logic: only poll when `hasRunningJobs` is true
  - Add log: `console.log('[Queue] Polling fallback triggered (WS connected:', wsConnected, ')')`
- **Rationale:** WebSocket is primary, polling is fallback + periodic backup

**Location 5: Modify DOMContentLoaded handler (lines 972-999)**

Add WebSocket initialization:
- After line 979 (loadFiltersFromStorage), before line 982 (renderFilterChips)
- Add: `connectJobsWebSocket();` to establish WebSocket connection early
- Add log: `console.log('[Queue] Initializing WebSocket connection for job updates')`
- Keep existing initialization order: filters → WebSocket → render → load data → start polling

**Location 6: Modify beforeunload handler (lines 1002-1006)**

Add WebSocket cleanup:
- After clearing autoRefreshInterval (line 1004)
- Add: `if (jobsWS) { jobsWS.close(); jobsWS = null; }`
- Add log: `console.log('[Queue] Closing WebSocket connection')`

**Location 7: Modify action functions (lines 787-953)**

Optimistic UI updates (optional enhancement):
- In `cancelJob()` (line 862): After successful response (line 891), don't call `loadJobs()` - WebSocket will update
- In `deleteJob()` (line 909): After successful response (line 938), remove job from `allJobs` and `filteredJobs` immediately, then call `renderJobs()`
- In `rerunJob()` (line 788): Keep existing behavior (needs full refresh to show new job)
- **Rationale:** Reduces redundant API calls, WebSocket provides updates

**Location 8: Add helper function after updateJobInList**

Add `matchesActiveFilters(job)` function:
- Accept `job` parameter
- Check if job matches `activeFilters.status` (if set is not empty)
- Check if job matches `activeFilters.source` (if set is not empty)
- Check if job matches `activeFilters.entity` (if set is not empty)
- Return boolean: true if job matches all active filters
- **Rationale:** Reusable filter logic for updateJobInList and potential future use