I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Polling Infrastructure (Legacy - To Be Removed)**
- `queue.html` lines 1176-1204: `startAutoRefresh()` function with 5-second interval
- Polls when WebSocket disconnected OR as 30-second backup when connected
- Checks for running jobs before polling to reduce unnecessary requests
- Called during page initialization (line 1224)
- Cleaned up on page unload (lines 1242-1244)
- **Status:** Redundant - WebSocket with exponential backoff reconnection is sufficient

**WebSocket Infrastructure (Primary - Keep)**
- `queue.html` lines 670-743: `connectJobsWebSocket()` with exponential backoff
- Handles `job_status_change` messages for real-time updates
- Reconnection logic: 1s → 2s → 4s → ... → 30s max delay
- Updates job list in-place without full page reload via `updateJobInList()`
- **Status:** Fully operational and reliable

**Log Filtering (Already Removed)**
- `common.js` lines 27-172: `serviceLogs` Alpine.js component
- No filtering logic - displays all logs received from WebSocket
- Server-side filtering via WebSocketWriter (implemented in previous phases)
- `service-logs.html`: No filter UI elements (no dropdowns, checkboxes, etc.)
- **Status:** Already clean - server filters, client displays

**Test Infrastructure (Missing)**
- `queue_test.go`: Empty file (1 blank line)
- No tests for WebSocket-based job updates
- No tests for real-time status changes
- **Status:** Needs creation from scratch

## Why Remove Polling?

**Redundancy Arguments:**
1. **WebSocket Reliability:** Exponential backoff reconnection handles temporary disconnections gracefully
2. **Backup Polling Unnecessary:** 30-second backup polling adds complexity without meaningful benefit
3. **Fallback Polling Covered:** If WebSocket fails completely, users can manually refresh the page
4. **Reduced Server Load:** Eliminates periodic HTTP requests when WebSocket is working
5. **Simpler Code:** Removes 29 lines of polling logic and associated state variables

**Risk Mitigation:**
- WebSocket reconnection is battle-tested (already in production)
- Manual page refresh is always available as ultimate fallback
- Server-side health monitoring can detect WebSocket issues
- Browser DevTools can verify WebSocket connection status

## Design Decisions

**Decision 1: Complete Polling Removal (Not Partial)**
- **Option A:** Remove all polling (recommended)
  - Pros: Simplest, cleanest, reduces server load
  - Cons: No automatic fallback if WebSocket fails silently
- **Option B:** Keep fallback polling only (not backup)
  - Pros: Safety net for WebSocket failures
  - Cons: Adds complexity, masks WebSocket issues
- **Choice:** Option A - WebSocket reconnection is sufficient, manual refresh is ultimate fallback

**Decision 2: Test Scope**
- **Option A:** Basic WebSocket message handling tests
  - Pros: Quick to implement, covers core functionality
  - Cons: Doesn't test full integration
- **Option B:** Full end-to-end integration tests
  - Pros: Comprehensive coverage, catches integration issues
  - Cons: Slower, more complex, requires test server
- **Choice:** Option B - Integration tests are essential for WebSocket reliability verification

**Decision 3: Log Filtering Documentation**
- **Option A:** Just remove code, no documentation
  - Pros: Clean, simple
  - Cons: Future developers might re-add client-side filtering
- **Option B:** Add comments explaining server-side filtering architecture
  - Pros: Prevents regression, documents design decision
  - Cons: Adds comment noise
- **Choice:** Option B - Brief comments prevent architectural regression

### Approach

**Cleanup and Simplification Strategy**

Remove legacy polling infrastructure now that WebSocket-based real-time updates are fully operational. The system has evolved from polling-based to event-driven architecture, making the fallback polling mechanism redundant. Client-side log filtering was already removed in previous phases - this task confirms and documents that state.

**Three-Point Approach:**
1. **Remove Polling Fallback** - Delete autoRefreshInterval logic since WebSocket provides reliable real-time updates with exponential backoff reconnection
2. **Verify Log Filtering Removal** - Confirm client-side filtering is gone (already complete - logs flow directly from server to display)
3. **Add Integration Tests** - Create tests to verify WebSocket-based job updates work correctly without polling

This maintains the clean architecture where the server controls all filtering and the UI is a simple display layer.

### Reasoning

I explored the codebase by reading queue.html (1499 lines with embedded JavaScript), common.js (serviceLogs Alpine.js component), service-logs.html (log display template), and queue_test.go (empty file). I identified the auto-refresh polling logic at lines 1176-1204 in queue.html, confirmed that client-side log filtering doesn't exist in the serviceLogs component (logs are displayed as-is from server), and found that the test file needs to be created from scratch. I also examined the WebSocket connection management (lines 670-743) to understand how real-time updates work, confirming that the polling mechanism is now redundant.

## Mermaid Diagram

sequenceDiagram
    participant Browser as Browser UI
    participant WS as WebSocket
    participant Server as Server
    participant EventBus as Event Bus

    Note over Browser,EventBus: Current State (With Polling - To Remove)
    Browser->>Browser: DOMContentLoaded
    Browser->>WS: connectJobsWebSocket()
    WS->>Server: Connect
    Server-->>WS: Connected
    Browser->>Browser: startAutoRefresh() ❌
    Browser->>Browser: setInterval(5s) ❌
    
    loop Every 5 seconds ❌
        Browser->>Browser: Check: wsConnected?
        alt WebSocket Disconnected
            Browser->>Server: GET /api/jobs (polling fallback) ❌
            Server-->>Browser: Job list
        else WebSocket Connected (30s backup)
            Browser->>Server: GET /api/jobs (backup poll) ❌
            Server-->>Browser: Job list
        end
    end

    Note over Browser,EventBus: Desired State (WebSocket Only - After Cleanup)
    Browser->>Browser: DOMContentLoaded
    Browser->>WS: connectJobsWebSocket()
    WS->>Server: Connect
    Server-->>WS: Connected ✅
    Note over Browser: No polling started ✅
    
    EventBus->>Server: Job status changed
    Server->>WS: job_status_change event
    WS->>Browser: Real-time update ✅
    Browser->>Browser: updateJobInList() ✅
    
    alt WebSocket Disconnects
        WS->>Browser: onclose
        Browser->>Browser: Exponential backoff (1s→2s→4s→...→30s) ✅
        Browser->>WS: Reconnect attempt
        WS->>Server: Connect
        Server-->>WS: Connected ✅
    end
    
    alt User Needs Fresh Data
        Browser->>Browser: Manual page refresh ✅
        Browser->>Server: GET /queue (full page load)
        Server-->>Browser: Fresh HTML + data
        Browser->>WS: connectJobsWebSocket() ✅
    end

## Proposed File Changes

### pages\queue.html(MODIFY)

**Location 1: Lines 274-278 (State Variables)**

Remove polling-related state variables:
- Delete line 274: `let autoRefreshInterval = null;` (no longer needed)
- Delete lines 1177-1178: `let lastPollTime = 0;` and `const BACKUP_POLL_INTERVAL = 30000;` (polling state)
- Keep WebSocket state variables: `jobsWS`, `wsConnected`, `wsReconnectAttempts` (lines 275-277)

**Rationale:** WebSocket with exponential backoff reconnection is the sole update mechanism. Polling state variables are no longer referenced.

---

**Location 2: Lines 1176-1204 (Auto-Refresh Function)**

Delete the entire `startAutoRefresh()` function:
- Remove lines 1176-1204 completely (29 lines)
- This includes:
  - Function declaration and interval setup
  - Polling condition logic (fallback + backup)
  - `loadJobs()` and `loadStats()` calls
  - Console logging for polling triggers

**Rationale:** WebSocket provides real-time updates. Polling is redundant and adds unnecessary server load. Manual page refresh is available as ultimate fallback.

---

**Location 3: Line 1224 (Initialization)**

Remove polling initialization:
- Delete line 1224: `startAutoRefresh();`
- Keep all other initialization: filters, WebSocket, rendering, data loading

**Rationale:** WebSocket connection (line 1219) is sufficient for real-time updates. No polling needed during initialization.

---

**Location 4: Lines 1242-1244 (Cleanup)**

Remove polling cleanup:
- Delete lines 1242-1244: `if (autoRefreshInterval) { clearInterval(autoRefreshInterval); }`
- Keep WebSocket cleanup (lines 1247-1251)

**Rationale:** No polling interval to clean up. WebSocket cleanup remains essential.

---

**Location 5: Add Architecture Comment (After Line 1219)**

Add brief comment explaining the WebSocket-first architecture:
```javascript
// Real-time updates via WebSocket (no polling fallback)
// - Server pushes job status changes via job_status_change events
// - Exponential backoff reconnection handles temporary disconnections
// - Manual page refresh available as ultimate fallback
```

**Rationale:** Documents design decision to prevent future developers from re-adding polling. Explains why there's no fallback mechanism.

### pages\static\common.js(MODIFY)

References: 

- internal\handlers\websocket_writer.go

**Location: Lines 27-37 (serviceLogs Component Initialization)**

Add architecture comment explaining server-side filtering:

After line 32 (before `init()` method), add comment block:
```javascript
// Architecture Note: Log Filtering
// - Server filters logs before broadcasting (WebSocketWriter with min_level and exclude_patterns)
// - Client displays all received logs without filtering
// - This maintains clean separation: server controls filtering, client is display layer
// - See: internal/handlers/websocket_writer.go for server-side filtering logic
```

**Rationale:** Documents that the absence of client-side filtering is intentional, not an oversight. Prevents future developers from adding client-side filtering logic. References the server-side implementation for context.

---

**Verification Only (No Changes):**

Confirm the following methods do NOT contain filtering logic:
- `addLog()` (lines 85-108): Adds logs directly to array without filtering
- `_parseLogEntry()` (lines 110-121): Parses log format without filtering by level
- `subscribeToWebSocket()` (lines 74-83): Subscribes to all log messages

**Rationale:** Confirms that client-side filtering was already removed in previous phases. This task is verification and documentation, not implementation.

### pages\partials\service-logs.html(MODIFY)

**Verification Only (No Changes Required)**

Confirm the template does NOT contain filtering UI:
- Line 24-26: No filter dropdown or checkboxes
- Line 27-33: Displays all logs in `logs` array without filtering
- Line 9-17: Control buttons (pause, refresh, clear) - no filter controls

**Rationale:** Template is already clean - displays logs as-is from the serviceLogs component. No filtering UI elements exist. This confirms that client-side filtering removal is complete.

**Optional Enhancement (Not Required):**
If desired, add HTML comment documenting server-side filtering:
```html
<!-- Logs are filtered server-side before broadcasting (see WebSocketWriter) -->
```

Place after line 22 (before terminal div). This is optional documentation, not a functional change.

### test\ui\queue_test.go(NEW)

References: 

- test\helpers.go
- test\ui\main_test.go

**Create Integration Tests for WebSocket-Based Job Updates**

Replace the empty file with comprehensive integration tests:

**Package and Imports:**
- Package: `ui_test`
- Imports: `testing`, `time`, `github.com/chromedp/chromedp`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`
- Import test helpers: `github.com/ternarybob/quaero/test`

**Test 1: TestQueuePageWebSocketConnection**
- Navigate to `/queue` page
- Wait for page load and WebSocket connection
- Verify WebSocket connection established (check browser console logs for "[Queue] Jobs WebSocket connected")
- Verify initial job list loaded
- Assert no polling requests occur (monitor network tab for GET /api/jobs requests)
- **Purpose:** Confirms WebSocket connects successfully and polling is not active

**Test 2: TestQueuePageJobStatusUpdate**
- Create a test job via API (POST /api/jobs/definitions/{id}/execute)
- Navigate to `/queue` page
- Wait for job to appear in list
- Monitor for job status changes (pending → running → completed)
- Verify UI updates in real-time without page refresh
- Assert job card updates show correct status, counts, and timestamps
- Verify no polling requests during status updates
- **Purpose:** Confirms WebSocket-based job updates work end-to-end

**Test 3: TestQueuePageWebSocketReconnection**
- Navigate to `/queue` page
- Wait for WebSocket connection
- Simulate WebSocket disconnection (close connection via browser DevTools or server restart)
- Verify reconnection attempt with exponential backoff
- Verify connection re-established within 30 seconds
- Assert job list remains functional after reconnection
- **Purpose:** Confirms exponential backoff reconnection works correctly

**Test 4: TestQueuePageManualRefresh**
- Navigate to `/queue` page with existing jobs
- Create new job via API (not visible in current list)
- Click browser refresh button
- Verify new job appears after manual refresh
- Assert WebSocket reconnects after page reload
- **Purpose:** Confirms manual refresh works as ultimate fallback

**Test 5: TestServiceLogsNoClientFiltering**
- Navigate to any page with service logs component (e.g., `/queue`)
- Trigger various log levels from server (debug, info, warn, error)
- Verify all logs appear in UI without client-side filtering
- Assert no filter dropdown or controls exist in logs UI
- Verify logs display in chronological order
- **Purpose:** Confirms client-side log filtering is absent and logs display correctly

**Test Helpers:**
- `waitForWebSocketConnection(ctx, timeout)` - Polls console logs for connection message
- `waitForJobStatusChange(ctx, jobID, expectedStatus, timeout)` - Waits for job card to show status
- `countNetworkRequests(ctx, urlPattern, duration)` - Monitors network tab for polling requests
- `getJobCardElement(ctx, jobID)` - Finds job card DOM element by job ID
- `getJobCardStatus(ctx, jobID)` - Extracts status from job card

**Test Configuration:**
- Use existing test server infrastructure from `test/helpers.go`
- Run tests against local test instance (not production)
- Use chromedp for browser automation (consistent with other UI tests)
- Set reasonable timeouts: 5s for WebSocket connection, 30s for job completion, 60s for reconnection

**Rationale:** Integration tests verify the complete WebSocket-based update flow works correctly without polling. Tests cover normal operation, reconnection scenarios, and manual refresh fallback. This ensures the removal of polling doesn't break functionality.