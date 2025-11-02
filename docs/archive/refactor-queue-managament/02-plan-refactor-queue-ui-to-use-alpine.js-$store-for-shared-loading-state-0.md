I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Architecture:**

The loading state synchronization uses a custom event pattern:
- `jobList.isLoading` is watched via `$watch` (lines 1643-1649)
- When `isLoading` changes, a `jobList:loadingStateChange` custom event is dispatched
- `queueStatsHeader` listens for this event (lines 1554-1555) and updates its local `loading` property
- Event listener cleanup is handled via `$watch('$el')` (lines 1558-1563)

**Problems with Current Approach:**
1. **Indirection**: State changes require event dispatching and listening
2. **Memory Management**: Event listeners must be manually cleaned up
3. **Debugging Complexity**: Event flow requires extensive logging to trace
4. **Potential Race Conditions**: Event listeners might miss events if registered after dispatch
5. **Boilerplate Code**: Requires handler functions, event registration, and cleanup logic

**Alpine.js Store Benefits:**
1. **Reactive by Default**: Components automatically react to store changes
2. **No Event Listeners**: Direct property access eliminates listener management
3. **Centralized State**: Single source of truth for shared state
4. **Simpler Code**: Less boilerplate, no cleanup needed
5. **Better Performance**: Alpine's reactivity system is optimized for this use case

**Implementation Strategy:**

Create a global `queueState` store during Alpine initialization that holds the shared `isLoading` state. Both components will access this store directly—`jobList` will write to it, and `queueStatsHeader` will read from it reactively using `$store.queueState.isLoading`.

### Approach

Refactor the loading state synchronization from custom event dispatching to Alpine.js global store (`Alpine.store()`) for reactive shared state management between `queueStatsHeader` and `jobList` components. This eliminates the need for event listeners and provides a cleaner, more maintainable architecture with automatic reactivity.

### Reasoning

I explored the repository structure and read the `queue.html` file, focusing on the `queueStatsHeader` component (lines 1524-1586) and `jobList` component (lines 1588-2866). I analyzed the current event-based communication pattern where `jobList` uses `$watch` to dispatch `jobList:loadingStateChange` events, and `queueStatsHeader` listens for these events to update its `loading` state. The user has already implemented defensive checks and comprehensive logging as suggested in the previous phase.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Template as HTML Template
    participant QueueStatsHeader as queueStatsHeader Component
    participant Store as Alpine.store('queueState')
    participant JobList as jobList Component
    participant API

    Note over Store: Store initialized with<br/>isLoading: false

    User->>Template: Click Refresh Button
    Template->>JobList: Trigger loadJobs()
    
    Note over JobList: loadJobs() method starts
    JobList->>JobList: Set this.isLoading = true
    JobList->>Store: Set isLoading = true
    Note over Store: Store state updated<br/>isLoading: true
    
    Store-->>Template: Reactive update triggered
    Template->>Template: Re-render with $store.queueState.isLoading
    Note over Template: Button shows spinner<br/>"Loading..." text appears
    
    JobList->>API: Fetch /api/jobs
    
    alt Success
        API-->>JobList: Return jobs data
        JobList->>JobList: Update allJobs, totalJobs
        JobList->>JobList: renderJobs()
    else Error
        API-->>JobList: Return error
        JobList->>JobList: Handle error, fallback data
    end
    
    Note over JobList: Finally block executes
    JobList->>JobList: Set this.isLoading = false
    JobList->>Store: Set isLoading = false
    JobList->>Store: Set lastUpdateTime = now
    Note over Store: Store state updated<br/>isLoading: false
    
    Store-->>Template: Reactive update triggered
    Template->>Template: Re-render with $store.queueState.isLoading
    Note over Template: Button shows refresh icon<br/>"Loading..." text hidden
    
    Note over QueueStatsHeader,Store: No event listeners needed!<br/>Alpine reactivity handles everything

## Proposed File Changes

### pages\queue.html(MODIFY)

**Create Alpine.js Global Store for Queue State (around line 1518-1520)**

Inside the `document.addEventListener('alpine:init', ...)` handler, before registering the `queueStatsHeader` and `jobList` components, create a global Alpine store to manage shared queue state.

**Implementation Details:**

1. Add `Alpine.store('queueState', { ... })` call immediately after the `console.log('[Queue] Alpine.js initialized')` line
2. Initialize the store with the following properties:
   - `isLoading: false` - Tracks whether job list is currently loading
   - `lastUpdateTime: null` - Timestamp of last successful update (optional, for debugging)

**Store Structure:**
```javascript
Alpine.store('queueState', {
    isLoading: false,
    lastUpdateTime: null
});
```

**Purpose:** This creates a centralized, reactive state container that both `queueStatsHeader` and `jobList` components can access without custom events.

**Note:** Alpine.js stores are globally accessible via `$store.queueState` in component templates and `Alpine.store('queueState')` in JavaScript code. Changes to store properties automatically trigger reactivity in all components that reference them.
**Update queueStatsHeader Component to Read from Store (lines 1524-1586)**

Refactor the `queueStatsHeader` Alpine component to read the loading state directly from the global store instead of listening for custom events.

**Changes Required:**

1. **Remove the `loading` property** from the component's data (line 1530)
   - This property is no longer needed since we'll read from the store

2. **Remove the `loadingStateChangeHandler` setup** in the `init()` method (lines 1533-1541)
   - Delete the handler function assignment
   - This eliminates the need for defensive checks and event handling

3. **Remove the event listener registration** (line 1555)
   - Delete `window.addEventListener('jobList:loadingStateChange', this.loadingStateChangeHandler)`

4. **Remove the cleanup logic** for the event listener (lines 1557-1563)
   - Delete the `$watch('$el')` block that removes the event listener
   - Store-based reactivity doesn't require manual cleanup

5. **Update template references** in the HTML (around lines 90-98)
   - Change all occurrences of `loading` to `$store.queueState.isLoading`
   - This includes: `x-show="loading"`, `:disabled="loading"`, `:title="loading ? ..."`, `:class="loading ? ..."`, etc.

**Benefits:**
- Eliminates ~30 lines of event handling code
- No manual memory management needed
- Automatic reactivity via Alpine's store system
- Simpler, more maintainable code

**Example Template Update:**
```html
<!-- Before -->
<span x-show="loading">Refreshing...</span>
<button :disabled="loading">...</button>

<!-- After -->
<span x-show="$store.queueState.isLoading">Refreshing...</span>
<button :disabled="$store.queueState.isLoading">...</button>
```
**Update jobList Component to Write to Store (lines 1588-2866)**

Refactor the `jobList` Alpine component to write loading state changes directly to the global store instead of dispatching custom events.

**Changes Required:**

1. **Remove the `$watch('isLoading')` handler** (lines 1643-1649)
   - Delete the entire `$watch` block that dispatches `jobList:loadingStateChange` events
   - This eliminates the need for event dispatching and logging

2. **Update all assignments to `this.isLoading`** to also update the store
   - There are two locations where `this.isLoading` is set:
     - Line 1785: `this.isLoading = true;` (in `loadJobs()` method)
     - Line 1882: `this.isLoading = false;` (in `finally` block)

3. **Implement store updates alongside local state updates**
   - When setting `this.isLoading = true`, also set `Alpine.store('queueState').isLoading = true`
   - When setting `this.isLoading = false`, also set `Alpine.store('queueState').isLoading = false`
   - Optionally update `Alpine.store('queueState').lastUpdateTime` when clearing loading state

4. **Keep the local `isLoading` property**
   - Don't remove `this.isLoading` from the component
   - It's still useful for internal component logic and conditional rendering within the `jobList` component itself
   - The store serves as the "source of truth" for cross-component communication

**Implementation Pattern:**
```javascript
// In loadJobs() method (line 1785)
this.isLoading = true;
Alpine.store('queueState').isLoading = true;

// In finally block (line 1882)
this.isLoading = false;
Alpine.store('queueState').isLoading = false;
Alpine.store('queueState').lastUpdateTime = new Date();
```

**Logging Updates:**
- Update existing console.log statements to mention store updates
- Example: `console.log('[Queue] Setting isLoading to true and updating store')`
- This maintains debugging capability while documenting the new architecture

**Why Keep Local State:**
- The `jobList` component uses `isLoading` internally for conditional logic
- Accessing `this.isLoading` is more convenient than `Alpine.store('queueState').isLoading` within the component
- The store acts as a "broadcast" mechanism for other components
**Remove Event Listener from queueStatsHeader HTML Template (around lines 90-98)**

Update the HTML template section that displays the queue statistics header to use the store-based loading state.

**Changes Required:**

1. **Locate the queue statistics header section** (around lines 90-98)
   - This is the section with the refresh button and connection status
   - Look for `x-data="queueStatsHeader"` attribute

2. **Update all `loading` references to `$store.queueState.isLoading`**
   - `x-show="loading"` → `x-show="$store.queueState.isLoading"`
   - `:disabled="loading"` → `:disabled="$store.queueState.isLoading"`
   - `:title="loading ? 'Loading...' : 'Refresh Jobs'"` → `:title="$store.queueState.isLoading ? 'Loading...' : 'Refresh Jobs'"`
   - `:class="loading ? 'fa-spinner fa-pulse' : 'fa-rotate-right'"` → `:class="$store.queueState.isLoading ? 'fa-spinner fa-pulse' : 'fa-rotate-right'"`

3. **Verify no other template sections reference the old `loading` property**
   - Search the entire file for `x-show="loading"`, `x-text="loading"`, `:disabled="loading"`, etc.
   - Update any remaining references to use `$store.queueState.isLoading`

**Benefits:**
- Template directly reflects the store state
- No intermediate component property needed
- Automatic reactivity when store updates

**Testing Tip:**
- After implementation, open browser console and manually test: `Alpine.store('queueState').isLoading = true`
- The UI should immediately show the loading spinner
- Set to `false` and the spinner should disappear
**Update Console Logging to Reflect Store-Based Architecture (multiple locations)**

Update the console logging statements added in the previous implementation phase to reflect the new store-based architecture.

**Changes Required:**

1. **In `jobList.loadJobs()` method** (around line 1771)
   - Update: `console.log('[Queue] Setting isLoading to true, current value:', this.isLoading)`
   - To: `console.log('[Queue] Setting isLoading to true and updating store, current value:', this.isLoading)`

2. **In `jobList` finally block** (around line 1880)
   - Update: `console.log('[Queue] loadJobs finally block executing, clearing loading state')`
   - To: `console.log('[Queue] loadJobs finally block executing, clearing loading state in component and store')`
   - Add after line 1882: `console.log('[Queue] Store isLoading updated to:', Alpine.store('queueState').isLoading)`

3. **Remove `queueStatsHeader` event logging** (lines 1535, 1539)
   - Delete: `console.log('[Queue] queueStatsHeader received loading state change:', e.detail?.isLoading)`
   - Delete: `console.warn('[Queue] Received malformed loadingStateChange event:', e)`
   - These logs are no longer relevant since events are removed

4. **Remove `jobList` event dispatch logging** (lines 1644, 1648)
   - Delete: `console.log('[Queue] jobList isLoading changed:', { to: val, timestamp: new Date().toISOString() })`
   - Delete: `console.log('[Queue] Dispatched jobList:loadingStateChange event with isLoading:', val)`
   - These logs are no longer relevant since `$watch` and events are removed

**Optional: Add Store Initialization Logging**
- After creating the store (around line 1520), add:
  - `console.log('[Queue] Alpine store initialized:', Alpine.store('queueState'))`
- This helps verify the store is created correctly during initialization

**Purpose:**
- Maintain debugging capability with updated architecture
- Remove obsolete event-related logging
- Document store updates for troubleshooting
**Verify and Test Store-Based Implementation**

**Verification Checklist:**

1. **Store Creation**
   - Verify `Alpine.store('queueState')` is created in the `alpine:init` event handler
   - Verify it's created BEFORE the component registrations
   - Check browser console for initialization log

2. **Component Updates**
   - Verify `queueStatsHeader` no longer has `loading` property
   - Verify `queueStatsHeader` no longer has event listeners
   - Verify `queueStatsHeader` no longer has cleanup logic
   - Verify `jobList` no longer has `$watch('isLoading')` handler
   - Verify `jobList` updates store when setting `this.isLoading`

3. **Template Updates**
   - Search for any remaining references to bare `loading` (without `$store.queueState.`)
   - Verify all loading state checks use `$store.queueState.isLoading`
   - Check both the queue statistics header section and any other sections

4. **Functional Testing**
   - Load the queue management page
   - Click the refresh button
   - Verify the loading spinner appears and disappears correctly
   - Check browser console for updated log messages
   - Verify no JavaScript errors in console

5. **Browser Console Testing**
   - Open browser DevTools console
   - Type: `Alpine.store('queueState')`
   - Verify it returns an object with `isLoading` and `lastUpdateTime` properties
   - Type: `Alpine.store('queueState').isLoading = true`
   - Verify the UI immediately shows the loading spinner
   - Type: `Alpine.store('queueState').isLoading = false`
   - Verify the spinner disappears

**Expected Behavior:**
- Loading state synchronization works identically to before
- No custom events are dispatched or listened to
- Code is simpler and more maintainable
- Debugging is easier with centralized state

**Rollback Plan:**
- If issues arise, the previous event-based implementation can be restored
- The changes are isolated to the `queue.html` file
- No backend or API changes are required