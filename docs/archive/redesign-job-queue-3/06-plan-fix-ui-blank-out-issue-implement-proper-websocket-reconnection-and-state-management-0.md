I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Root Cause Analysis

**The UI blank-out occurs due to a combination of factors:**

1. **Unguarded State Mutations**: `loadJobs()` (lines 1549-1594) immediately replaces `allJobs` and `filteredJobs` arrays, then calls `renderJobs()` which sets `itemsToRender` from the new (potentially empty) arrays. During the fetch, `itemsToRender` is empty, causing the "No jobs found" message to display.

2. **Error Path Bypasses Alpine**: The error handler (lines 1587-1592) directly injects HTML into `#jobs-cards-container`, completely bypassing Alpine.js reactivity and destroying the template structure. This leaves the UI in a broken state.

3. **No Request Deduplication**: Multiple concurrent `loadJobs()` calls can occur from:
   - Page initialization (line 1334)
   - Manual refresh button (line 170)
   - Pagination changes (line 1439)
   - After job rerun (line 1151)
   - After job deletion (line 2236)
   - Filter application (line 843)

4. **WebSocket Reconnection Timing**: The 10-30 second blank-out aligns with WebSocket reconnection delays (exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s max). During disconnection, if a `loadJobs()` call fails or takes too long, the UI goes blank.

5. **No Loading State Preservation**: There's no mechanism to preserve the previous UI state during fetch operations. The template shows "Loading jobs..." only when `itemsToRender.length === 0` AND `filteredJobs.length === 0`, but this condition is met during normal fetches.

## Solution Architecture

**Implement a multi-layered defense strategy:**

1. **Add Loading State Management**: Introduce `isLoading` and `loadError` flags in the Alpine component to track fetch state without clearing existing data.

2. **Implement Request Deduplication**: Use an AbortController to cancel in-flight requests when a new `loadJobs()` is triggered, preventing race conditions.

3. **Preserve Last Successful State**: Keep a snapshot of the last successful job list to display during fetch failures or network issues.

4. **Fix Error Handling**: Replace direct DOM manipulation with Alpine-reactive error state that preserves the template structure.

5. **Add Loading Indicators**: Show a non-intrusive loading indicator (spinner in header) that doesn't clear the existing job list.

6. **Implement Optimistic Updates**: Keep existing jobs visible during refresh, only replacing them when new data successfully loads.

7. **Add Retry Logic**: Implement exponential backoff retry for failed `loadJobs()` requests with user-visible retry button.

8. **WebSocket State Coordination**: Ensure WebSocket reconnection doesn't interfere with ongoing fetch operations.

## Implementation Strategy

**Phase 1: Add Loading State (Non-Breaking)**
- Add `isLoading`, `loadError`, `lastSuccessfulJobs` to Alpine component state
- Modify `loadJobs()` to set `isLoading = true` at start, `false` at end
- Keep existing `allJobs`/`filteredJobs` intact during fetch
- Only update arrays on successful fetch

**Phase 2: Request Deduplication**
- Add `currentFetchController` (AbortController) to component state
- Abort previous request before starting new one
- Handle AbortError gracefully (don't show as error)

**Phase 3: Fix Error Handling**
- Replace direct DOM manipulation with `loadError` state variable
- Update template to show error message via Alpine binding
- Add "Retry" button that calls `loadJobs()` again
- Preserve existing job list during errors

**Phase 4: Loading Indicators**
- Add spinner to refresh button when `isLoading = true`
- Add subtle loading overlay to job cards container (optional)
- Show "Refreshing..." text in header stats area
- Ensure indicators don't cause layout shift

**Phase 5: Optimistic State Management**
- Store `lastSuccessfulJobs` snapshot on successful fetch
- Fall back to snapshot if fetch fails
- Show stale data indicator when using snapshot
- Clear snapshot only when new data successfully loads

**Phase 6: Retry Logic**
- Implement exponential backoff for automatic retries (3 attempts)
- Show retry count in error message
- Add manual "Retry Now" button
- Reset retry count on successful fetch

**Phase 7: WebSocket Coordination**
- Ensure `updateJobInList()` doesn't conflict with `loadJobs()`
- Queue WebSocket updates during active fetch
- Apply queued updates after fetch completes
- Add connection state indicator in header

**Phase 8: Testing & Validation**
- Test with network throttling (slow 3G)
- Test with WebSocket disconnection/reconnection
- Test concurrent `loadJobs()` calls
- Test error recovery and retry logic
- Verify no blank screens under any condition

### Approach

Fix the UI blank-out issue by implementing proper state management with loading indicators, request deduplication, error boundaries, and WebSocket reconnection handling. The solution focuses on preventing the UI from clearing during data fetches and ensuring Alpine.js reactivity is preserved even during errors.

### Reasoning

I explored the queue.html template structure (2563 lines), analyzed the Alpine.js jobList component (lines 1408-2431), traced WebSocket connection handling (lines 919-1032), examined the loadJobs() flow (lines 1549-1594), identified all loadJobs() call sites (11 locations), reviewed the error handling path that bypasses Alpine reactivity, and confirmed there's no polling mechanism that could cause periodic blank-outs.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant UI as Queue UI (Alpine.js)
    participant LoadJobs as loadJobs()
    participant API as /api/jobs
    participant WS as WebSocket
    participant State as Component State

    Note over UI,State: BEFORE FIX: Race Condition & Blank Screen

    User->>UI: Click Refresh
    UI->>LoadJobs: Call loadJobs()
    LoadJobs->>State: Clear allJobs = []
    LoadJobs->>State: Clear filteredJobs = []
    State->>UI: itemsToRender = [] (BLANK SCREEN)
    LoadJobs->>API: fetch('/api/jobs')
    
    Note over WS: WebSocket update arrives during fetch
    WS->>UI: job_status_change event
    UI->>State: Mutate allJobs (race condition)
    
    API-->>LoadJobs: Response (or error)
    alt Fetch Success
        LoadJobs->>State: allJobs = newData
        State->>UI: Render jobs
    else Fetch Error
        LoadJobs->>UI: Direct DOM manipulation (breaks Alpine)
        Note over UI: UI STUCK IN BLANK STATE
    end

    Note over UI,State: AFTER FIX: Optimistic Updates & Error Recovery

    User->>UI: Click Refresh
    UI->>LoadJobs: Call loadJobs()
    
    alt Previous Request In-Flight
        LoadJobs->>LoadJobs: Abort previous request
    end
    
    LoadJobs->>State: isLoading = true
    LoadJobs->>State: Keep existing allJobs (no clear)
    State->>UI: Show loading spinner (jobs still visible)
    LoadJobs->>API: fetch with AbortSignal
    
    Note over WS: WebSocket update arrives during fetch
    WS->>UI: job_status_change event
    UI->>State: Check isLoading = true
    State->>State: Queue update in pendingUpdates[]
    
    API-->>LoadJobs: Response (or error)
    alt Fetch Success
        LoadJobs->>State: allJobs = newData
        LoadJobs->>State: lastSuccessfulJobs = newData
        LoadJobs->>State: Apply pendingUpdates[]
        LoadJobs->>State: isLoading = false
        State->>UI: Render updated jobs (smooth transition)
    else Fetch Error
        LoadJobs->>State: loadError = error.message
        LoadJobs->>State: Fall back to lastSuccessfulJobs
        LoadJobs->>State: isLoading = false
        State->>UI: Show error banner + Retry button
        Note over UI: Jobs still visible (no blank screen)
        
        alt Auto Retry (< maxRetries)
            LoadJobs->>LoadJobs: Schedule retry with backoff
        end
    end
    
    User->>UI: Click Retry
    UI->>LoadJobs: retryLoadJobs()
    LoadJobs->>State: retryCount = 0
    LoadJobs->>LoadJobs: Call loadJobs() again

## Proposed File Changes

### pages\queue.html(MODIFY)

References: 

- pages\static\common.js

**Add Loading State Management to Alpine Component (lines 1408-1430)**

Add new state variables to the `jobList` Alpine component:
- `isLoading: false` - Tracks whether a fetch is in progress
- `loadError: null` - Stores error message if fetch fails
- `lastSuccessfulJobs: []` - Snapshot of last successful job list for fallback
- `currentFetchController: null` - AbortController for request cancellation
- `retryCount: 0` - Tracks number of retry attempts
- `maxRetries: 3` - Maximum automatic retry attempts
- `isInitialLoad: true` - Tracks if this is the first load (show loading, not stale data)

These variables enable proper loading state tracking without clearing existing data during fetches.

**Refactor loadJobs() Method (lines 1549-1594)**

**Current Issues:**
- Immediately clears `allJobs` and `filteredJobs` arrays (line 1578-1580)
- Error handler directly manipulates DOM, bypassing Alpine reactivity (lines 1587-1592)
- No request deduplication or abort handling
- No loading state management

**Required Changes:**

1. **Add Request Deduplication (at method start):**
   - Check if `this.currentFetchController` exists
   - If exists, call `this.currentFetchController.abort()` to cancel previous request
   - Create new AbortController: `this.currentFetchController = new AbortController()`
   - Pass `signal: this.currentFetchController.signal` to fetch options

2. **Set Loading State (before fetch):**
   - Set `this.isLoading = true`
   - Clear previous error: `this.loadError = null`
   - Keep existing `allJobs` and `filteredJobs` intact (don't clear them)

3. **Handle Fetch Success:**
   - Store response data in temporary variables: `const newJobs = data.jobs || []`
   - Update `this.allJobs = newJobs` only after successful parse
   - Update `this.filteredJobs = [...newJobs]`
   - Store snapshot: `this.lastSuccessfulJobs = [...newJobs]`
   - Reset retry count: `this.retryCount = 0`
   - Set `this.isInitialLoad = false`
   - Call `this.renderJobs()` to update UI

4. **Handle Fetch Errors:**
   - Catch AbortError separately (don't treat as error): `if (error.name === 'AbortError') return;`
   - For other errors, set `this.loadError = error.message`
   - If `this.lastSuccessfulJobs.length > 0`, fall back to snapshot: `this.allJobs = [...this.lastSuccessfulJobs]`
   - Increment `this.retryCount`
   - If `this.retryCount < this.maxRetries`, schedule automatic retry with exponential backoff: `setTimeout(() => this.loadJobs(), Math.min(1000 * Math.pow(2, this.retryCount), 30000))`
   - Log error with context: `console.error('[Queue] Error loading jobs (attempt ' + this.retryCount + '):', error)`
   - Remove direct DOM manipulation - let template handle error display

5. **Cleanup (in finally block):**
   - Set `this.isLoading = false`
   - Clear fetch controller: `this.currentFetchController = null`

**Add Retry Method:**
- Add new method: `retryLoadJobs()` that resets `this.retryCount = 0` and calls `this.loadJobs()`
- This allows manual retry from UI button

**Update Template to Show Loading State (lines 194-475)**

**Add Loading Indicator in Header (after line 170):**
- Add loading spinner next to refresh button when `isLoading = true`
- Use Alpine `x-show` directive: `<span x-show="isLoading" class="loading loading-sm" style="margin-left: 0.5rem;"></span>`
- Update refresh button to show spinner icon when loading: `:class="{ 'loading': isLoading }"`

**Add Error Display (before jobs container, around line 193):**
- Add error alert that shows when `loadError !== null`
- Use Alpine `x-show` directive: `<template x-if="loadError">`
- Display error message with retry button:
  ```html
  <div class="toast toast-error" style="margin-bottom: 1rem;">
    <i class="fas fa-exclamation-circle"></i>
    <span x-text="'Failed to load jobs: ' + loadError"></span>
    <button class="btn btn-sm btn-primary" @click="retryLoadJobs()" style="margin-left: 1rem;">
      <i class="fas fa-redo"></i> Retry
    </button>
    <span x-show="retryCount > 0" x-text="'(Attempt ' + retryCount + '/' + maxRetries + ')'"></span>
  </div>
  ```

**Update "No Jobs" Message (lines 470-474):**
- Current condition: `x-if="itemsToRender.length === 0"`
- Change to: `x-if="itemsToRender.length === 0 && !isLoading && !loadError"`
- Update message logic:
  - If `isInitialLoad && isLoading`: Show "Loading jobs..." with spinner
  - If `filteredJobs.length === 0 && !isLoading`: Show "No jobs found matching the current filters."
  - Otherwise: Show nothing (let error display handle it)

**Add Stale Data Indicator (after line 193):**
- Show when displaying `lastSuccessfulJobs` after error
- Condition: `x-show="loadError && lastSuccessfulJobs.length > 0"`
- Display: `<div class="toast toast-warning">Showing cached data. <button @click="retryLoadJobs()">Refresh</button></div>`

**Prevent Race Conditions in updateJobInList (lines 2240-2372)**

**Current Issue:**
- `updateJobInList()` mutates `allJobs` and `filteredJobs` while `loadJobs()` might be fetching
- Can cause inconsistent state or lost updates

**Required Changes:**

1. **Check Loading State:**
   - At method start, check `if (this.isLoading)`
   - If loading, queue the update: `this.pendingUpdates.push(update)` (add `pendingUpdates: []` to state)
   - Return early to avoid mutation during fetch

2. **Apply Queued Updates:**
   - In `loadJobs()` success handler, after updating arrays, apply queued updates:
     ```javascript
     if (this.pendingUpdates.length > 0) {
       this.pendingUpdates.forEach(update => this.updateJobInList(update));
       this.pendingUpdates = [];
     }
     ```

3. **Preserve Existing Logic:**
   - Keep all existing update logic (lines 2244-2372)
   - Only add the loading state check at the beginning

**Update Refresh Button (line 170)**

**Current:**
- Simple button that dispatches `jobList:load` event
- No visual feedback during loading

**Required Changes:**
- Add loading state binding: `:disabled="isLoading"`
- Change icon based on loading state: `:class="isLoading ? 'fa-spinner fa-pulse' : 'fa-rotate-right'"`
- Add tooltip that changes during loading: `:title="isLoading ? 'Loading...' : 'Refresh Jobs'"`

**Add Connection State Indicator (lines 154-165)**

**Current:**
- Shows WebSocket connection status (Connected/Disconnected)
- No indication of fetch state

**Required Changes:**
- Add loading indicator next to connection status
- Show "Refreshing..." text when `isLoading = true`
- Use Alpine binding: `<span x-show="isLoading" class="text-gray">Refreshing...</span>`

**Update Page Initialization (lines 1444-1446)**

**Current:**
- Calls `this.loadJobs()` immediately in `init()`
- Sets `isInitialLoad = true` implicitly

**Required Changes:**
- Explicitly set `this.isInitialLoad = true` before calling `this.loadJobs()`
- This ensures proper loading indicator display on first load

**Add Cleanup on Component Destroy:**
- Add cleanup logic to abort in-flight requests when component is destroyed
- In Alpine component, add: `$watch('$el', (el) => { if (!el && this.currentFetchController) this.currentFetchController.abort(); })`
- This prevents memory leaks and orphaned requests
**Improve WebSocket Reconnection Handling (lines 1011-1032)**

**Current Issues:**
- WebSocket close triggers immediate reconnection with exponential backoff
- No coordination with `loadJobs()` state
- Reconnection doesn't refresh job list (correct behavior, but users might expect it)
- No indication that reconnection is in progress

**Required Changes:**

1. **Add Reconnection State:**
   - Add global variable: `let wsReconnecting = false`
   - Set to `true` when scheduling reconnection (line 1023)
   - Set to `false` when connection succeeds (line 929)

2. **Update Connection Status Display:**
   - Modify the connection status label (line 164) to show three states:
     - Connected: Green "Connected"
     - Reconnecting: Yellow "Reconnecting..."
     - Disconnected: Red "Disconnected"
   - Use Alpine binding: `:class="connected ? 'label-success' : (wsReconnecting ? 'label-warning' : 'label-error')"`
   - Update text: `x-text="connected ? 'Connected' : (wsReconnecting ? 'Reconnecting...' : 'Disconnected')"`

3. **Coordinate with Load State:**
   - Don't trigger `loadJobs()` on reconnection (current behavior is correct)
   - WebSocket updates will naturally refresh the UI once reconnected
   - If users want manual refresh, they can use the refresh button

4. **Add Reconnection Feedback:**
   - Show reconnection attempt count in console: `console.log('[Queue] Reconnection attempt ' + wsReconnectAttempts + ', delay: ' + delay + 'ms')`
   - Dispatch custom event for reconnection state: `window.dispatchEvent(new CustomEvent('queueStats:update', { detail: { connected: false, reconnecting: true } }))`

5. **Handle Reconnection Success:**
   - On successful reconnection (line 928), dispatch event: `window.dispatchEvent(new CustomEvent('queueStats:update', { detail: { connected: true, reconnecting: false } }))`
   - Reset reconnection state: `wsReconnecting = false`
   - Log success: `console.log('[Queue] WebSocket reconnected successfully after ' + wsReconnectAttempts + ' attempts')`

**Add Network Error Handling (lines 1002-1009)**

**Current:**
- WebSocket error logs to console
- Sets `wsConnected = false`
- No user-visible feedback beyond connection status

**Required Changes:**

1. **Distinguish Error Types:**
   - Check if error is due to network issues vs server issues
   - Log error details: `console.error('[Queue] WebSocket error:', error.type, error.message)`

2. **Show User-Friendly Error:**
   - If error occurs during initial connection, show toast notification
   - Use `window.showNotification('Unable to connect to server. Retrying...', 'warning')`
   - Don't show notification for every reconnection attempt (would be spammy)

3. **Coordinate with Load State:**
   - If `loadJobs()` is in progress when WebSocket disconnects, let it complete normally
   - Don't cancel in-flight fetch due to WebSocket disconnection

**Add Graceful Degradation (new section after line 1032)**

**Purpose:**
- Ensure UI remains functional even when WebSocket is disconnected
- Provide fallback mechanisms for real-time updates

**Implementation:**

1. **Add Manual Refresh Prompt:**
   - When WebSocket is disconnected for > 30 seconds, show subtle prompt
   - Display: "Real-time updates unavailable. <button>Refresh manually</button>"
   - Use toast notification with action button

2. **Preserve Existing Data:**
   - Never clear job list due to WebSocket disconnection
   - Keep showing last known state
   - Add "Last updated: X seconds ago" timestamp

3. **Add Timestamp Tracking:**
   - Add to Alpine component state: `lastUpdateTime: null`
   - Update on successful `loadJobs()`: `this.lastUpdateTime = new Date()`
   - Display in header: `<span x-show="lastUpdateTime" x-text="'Last updated: ' + formatTimeSince(lastUpdateTime)"></span>`

4. **Add formatTimeSince Helper:**
   - Add method to Alpine component:
     ```javascript
     formatTimeSince(date) {
       const seconds = Math.floor((new Date() - date) / 1000);
       if (seconds < 60) return seconds + 's ago';
       if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
       return Math.floor(seconds / 3600) + 'h ago';
     }
     ```

**Update queueStatsHeader Component (lines 1367-1405)**

**Current:**
- Listens for `queueStats:update` events
- Loads initial stats from API
- No coordination with main job list loading state

**Required Changes:**

1. **Add Reconnecting State:**
   - Add state variable: `reconnecting: false`
   - Update from event: `this.reconnecting = e.detail.reconnecting || false`

2. **Coordinate with Job List:**
   - Listen for job list loading state changes
   - Show loading indicator when job list is refreshing
   - Add event listener: `window.addEventListener('jobList:loadingStateChange', (e) => { this.loading = e.detail.isLoading; })`

3. **Emit Loading State from jobList:**
   - In `loadJobs()`, emit event when loading state changes:
     ```javascript
     window.dispatchEvent(new CustomEvent('jobList:loadingStateChange', { detail: { isLoading: true } }));
     // ... fetch logic ...
     window.dispatchEvent(new CustomEvent('jobList:loadingStateChange', { detail: { isLoading: false } }));
     ```

**Add Error Boundary for Template Rendering (lines 194-475)**

**Purpose:**
- Prevent Alpine.js errors from breaking the entire UI
- Provide fallback UI when rendering fails

**Implementation:**

1. **Wrap Job Cards in Error Boundary:**
   - Add try-catch around template rendering logic
   - Alpine doesn't have built-in error boundaries, so use defensive checks

2. **Add Defensive Checks:**
   - Before accessing nested properties, check existence: `item.job?.status`
   - Use optional chaining throughout template
   - Add fallback values: `item.job?.name || 'Unknown Job'`

3. **Add Render Error State:**
   - Add to Alpine component: `renderError: null`
   - If rendering fails, set error and show fallback UI
   - Display: "Unable to display jobs. <button>Reload page</button>"

4. **Log Render Errors:**
   - Add console logging for debugging: `console.error('[Queue] Render error:', error)`
   - Include context: job ID, error message, stack trace

**Testing Checklist (add as comments in code):**

```javascript
// TESTING CHECKLIST:
// 1. ✓ Slow network (throttle to Slow 3G in DevTools)
// 2. ✓ WebSocket disconnection (close connection in DevTools)
// 3. ✓ Concurrent loadJobs() calls (click refresh rapidly)
// 4. ✓ Error recovery (simulate 500 error, then retry)
// 5. ✓ Pagination during loading (change page while loading)
// 6. ✓ WebSocket updates during loading (trigger job status change)
// 7. ✓ Browser tab backgrounding (check if updates pause)
// 8. ✓ Long-running jobs (verify UI doesn't freeze)
// 9. ✓ Empty job list (verify no blank screen)
// 10. ✓ Filter changes during loading (apply filter while loading)
```