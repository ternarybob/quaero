I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The queue management UI is stuck showing "Loading..." because the `queueStatsHeader.loading` flag is not being cleared when `jobList.isLoading` changes from `true` to `false`. 

**Root Cause Analysis:**

1. The `queueStatsHeader` component listens for `jobList:loadingStateChange` events (line 1545-1547)
2. The `jobList` component uses Alpine.js `$watch` to dispatch this event when `isLoading` changes (lines 1627-1631)
3. The event listener lacks defensive checks for missing `e.detail` or `e.detail.isLoading`
4. There's no console logging to debug whether events are being dispatched or received
5. The `finally` block (lines 1848-1852) correctly sets `this.isLoading = false`, but we can't verify if the `$watch` handler fires afterward

**Current State:**
- WebSocket connection: ✅ Working (shows "Connected")
- Queue stats: ✅ Working (shows "12 Pending, 2 Workers")
- Job list loading: ❌ Stuck in loading state
- Loading state event: ❌ Not clearing properly

**Solution Approach:**
Add defensive programming and comprehensive logging to diagnose and fix the event synchronization issue without changing the overall architecture.

### Approach

Fix the loading state synchronization between `queueStatsHeader` and `jobList` Alpine.js components by adding defensive checks to the event listener, implementing comprehensive console logging for debugging the event flow, and ensuring the `finally` block in `loadJobs()` always dispatches the loading state change event correctly.

### Reasoning

I explored the repository structure to understand the project layout, then read the `queue.html` file to analyze the Alpine.js components. I focused on the `queueStatsHeader` and `jobList` components, specifically examining the loading state management, event dispatching via `$watch`, and the event listener implementation. I identified the lack of defensive checks and debugging logs as the primary issues preventing proper loading state synchronization.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant queueStatsHeader
    participant jobList
    participant API
    participant $watch

    User->>jobList: Click Refresh Button
    Note over jobList: loadJobs() called
    jobList->>jobList: Log: "loadJobs called"
    jobList->>jobList: Set isLoading = true
    jobList->>jobList: Log: "Setting isLoading to true"
    
    jobList->>$watch: isLoading changed (false → true)
    $watch->>$watch: Log: "isLoading changed: true"
    $watch->>queueStatsHeader: Dispatch jobList:loadingStateChange
    $watch->>$watch: Log: "Dispatched event"
    
    queueStatsHeader->>queueStatsHeader: Receive event
    queueStatsHeader->>queueStatsHeader: Log: "Received loading state change: true"
    queueStatsHeader->>queueStatsHeader: Defensive check: e.detail exists?
    queueStatsHeader->>queueStatsHeader: Set loading = true
    Note over queueStatsHeader: Button shows "Loading..."
    
    jobList->>API: Fetch /api/jobs
    
    alt Success
        API-->>jobList: Return jobs data
        jobList->>jobList: Log: "Successfully loaded X jobs"
        jobList->>jobList: Update allJobs, totalJobs
        jobList->>jobList: Log: "Cleared isInitialLoad flag"
    else Error
        API-->>jobList: Return error
        jobList->>jobList: Log: "Error loading jobs"
        jobList->>jobList: Fallback to cached data
        jobList->>jobList: Log: "Rendered fallback data"
    else Aborted
        API-->>jobList: Request aborted
        jobList->>jobList: Log: "Request aborted"
        jobList->>jobList: Return early
    end
    
    Note over jobList: Finally block ALWAYS executes
    jobList->>jobList: Log: "Finally block executing"
    jobList->>jobList: Log: "isLoading before clear: true"
    jobList->>jobList: Set isLoading = false
    jobList->>jobList: Log: "isLoading after clear: false"
    
    jobList->>$watch: isLoading changed (true → false)
    $watch->>$watch: Log: "isLoading changed: false"
    $watch->>queueStatsHeader: Dispatch jobList:loadingStateChange
    $watch->>$watch: Log: "Dispatched event"
    
    queueStatsHeader->>queueStatsHeader: Receive event
    queueStatsHeader->>queueStatsHeader: Log: "Received loading state change: false"
    queueStatsHeader->>queueStatsHeader: Defensive check: e.detail exists?
    queueStatsHeader->>queueStatsHeader: Set loading = false
    Note over queueStatsHeader: Button shows "Refresh" icon

## Proposed File Changes

### pages\queue.html(MODIFY)

**Fix Event Listener in queueStatsHeader Component (lines 1544-1547)**

Add defensive checks to handle missing or malformed event data:

1. Check if `e.detail` exists before accessing `isLoading` property
2. Validate that `e.detail.isLoading` is a boolean value
3. Add console logging to track when events are received and what values are being set
4. Log warnings if event data is malformed

**Implementation Details:**
- Wrap the assignment in a conditional check: `if (e.detail && typeof e.detail.isLoading === 'boolean')`
- Add console log: `console.log('[Queue] queueStatsHeader received loading state change:', e.detail?.isLoading)`
- Add warning log if event is malformed: `console.warn('[Queue] Received malformed loadingStateChange event:', e)`
- Ensure `this.loading` is only updated with valid boolean values

**Example Pattern:**
```
window.addEventListener('jobList:loadingStateChange', (e) => {
    // Log receipt
    // Check e.detail exists
    // Check e.detail.isLoading is boolean
    // Update this.loading
    // Log warning if malformed
});
```
**Add Console Logging to $watch Handler in jobList Component (lines 1627-1631)**

Add comprehensive logging to track when the `isLoading` state changes and when events are dispatched:

1. Log the old and new values of `isLoading` when the watch fires
2. Log confirmation that the `jobList:loadingStateChange` event is being dispatched
3. Include timestamp for debugging timing issues
4. Add context about what triggered the change (initial load, retry, success, error)

**Implementation Details:**
- Add console log before dispatching event: `console.log('[Queue] jobList isLoading changed:', { from: oldValue, to: val, timestamp: new Date().toISOString() })`
- Add console log after dispatching event: `console.log('[Queue] Dispatched jobList:loadingStateChange event with isLoading:', val)`
- Consider adding a stack trace for debugging: `console.trace('[Queue] Loading state change stack trace')`

**Note:** The `$watch` callback receives both the new value (`val`) and optionally the old value as parameters. Use both for comprehensive logging.
**Add Logging to loadJobs() Method Finally Block (lines 1848-1852)**

Add console logging to verify the `finally` block executes and the loading state is cleared:

1. Log entry into the `finally` block
2. Log the value of `this.isLoading` before and after setting it to `false`
3. Log confirmation that the `currentFetchController` is being cleared
4. Add context about whether the request succeeded, failed, or was aborted

**Implementation Details:**
- Add console log at start of `finally` block: `console.log('[Queue] loadJobs finally block executing, clearing loading state')`
- Log before clearing: `console.log('[Queue] isLoading before clear:', this.isLoading)`
- Log after clearing: `console.log('[Queue] isLoading after clear:', this.isLoading)`
- Log controller state: `console.log('[Queue] Clearing fetch controller:', this.currentFetchController !== null)`

**Important:** The `finally` block already correctly sets `this.isLoading = false` at line 1850. The logging will help verify this executes and triggers the `$watch` handler.
**Add Logging to loadJobs() Method Entry Point (line 1751-1763)**

Add console logging at the start of `loadJobs()` to track when loading begins:

1. Log entry into the method with timestamp
2. Log the current value of `this.isLoading` before setting it to `true`
3. Log whether a previous request is being aborted
4. Log the current page and filter state for context

**Implementation Details:**
- Add console log at method start (after line 1751): `console.log('[Queue] loadJobs called at', new Date().toISOString())`
- Log before setting loading state: `console.log('[Queue] Setting isLoading to true, current value:', this.isLoading)`
- Log abort status: `console.log('[Queue] Aborting previous request:', this.currentFetchController !== null)`
- Log context: `console.log('[Queue] Loading page', this.currentPage, 'with filters:', window.activeFilters)`

**Purpose:** This logging will help trace the complete lifecycle of a load operation and identify if multiple loads are being triggered simultaneously.
**Add Logging to Success Path in loadJobs() (lines 1793-1817)**

Add console logging when the job list loads successfully:

1. Log successful response with job count
2. Log when `isInitialLoad` flag is cleared (line 1806)
3. Log when pending updates are applied (lines 1812-1815)
4. Log timestamp of successful update

**Implementation Details:**
- After line 1794 (parsing response): `console.log('[Queue] Successfully loaded', newJobs.length, 'jobs, total count:', data.total_count)`
- After line 1806 (clearing initial load flag): `console.log('[Queue] Cleared isInitialLoad flag, isLoading will be set to false in finally block')`
- Before line 1813 (applying pending updates): `console.log('[Queue] Applying', this.pendingUpdates.length, 'pending updates')`
- After line 1809 (timestamp update): `console.log('[Queue] Last update time:', this.lastUpdateTime.toISOString())`

**Purpose:** Verify the success path executes completely and reaches the `finally` block.
**Add Logging to Error Path in loadJobs() (lines 1819-1846)**

Add console logging when the job list fails to load:

1. Log error details (already exists at line 1826, but enhance it)
2. Log when falling back to cached data (lines 1829-1834)
3. Log retry scheduling (lines 1839-1845)
4. Ensure error path also reaches `finally` block

**Implementation Details:**
- Enhance existing error log at line 1826: Add more context about the error type and whether it's a network error, timeout, or server error
- After line 1833 (fallback rendering): `console.log('[Queue] Rendered fallback data from last successful load')`
- After line 1837 (retry increment): `console.log('[Queue] Retry count incremented to', this.retryCount, 'of', this.maxRetries)`
- Before line 1844 (scheduling retry): `console.log('[Queue] Scheduling retry in', delay, 'ms')`

**Important:** Verify that errors don't prevent the `finally` block from executing. The `finally` block should always run regardless of success or error.
**Add Logging to AbortError Handling (lines 1820-1824)**

Add console logging when a request is aborted:

1. Enhance the existing log at line 1822 with more context
2. Log the state of `isLoading` when abort occurs
3. Verify that the `finally` block still executes after abort

**Implementation Details:**
- Enhance line 1822: `console.log('[Queue] Request aborted (isLoading:', this.isLoading, '), new request in progress')`
- Add after return statement: Ensure the `finally` block executes even when returning early from abort

**Important:** When a request is aborted (line 1823 returns early), the `finally` block at line 1848 should still execute because it's outside the catch block. Verify this with logging.

**Note:** The early return at line 1823 means the `$watch` handler should fire when `finally` sets `isLoading = false`, but we need to verify this actually happens.