# Step 1: Implement START/COMPLETE-only event fetching in queue.html

## Changes Made

Modified `pages/queue.html` to only fetch step events on START (first trigger) and COMPLETE (finished=true), skipping all intermediate triggers during step execution.

## Implementation Details

### 1. Added State Tracking Variable (Line 1810)
```javascript
_stepEventsFetchedOnStart: {}, // Track which steps have fetched events on START (prevents fetching during execution)
```

### 2. Modified `refreshStepEvents()` Function (Lines 3629-3694)

**Old Behavior:**
- Used time-based throttling (500ms)
- Still fetched multiple times during execution (whenever 500ms elapsed)
- Created unnecessary API load

**New Behavior:**
- Tracks first fetch for each step ID in `_stepEventsFetchedOnStart` map
- On **first trigger** (step not in map): Fetch events and mark as fetched [START]
- On **finished=true**: Always fetch final events [COMPLETE]
- On **subsequent triggers** (in map, not finished): Skip with console.log
- Clears tracking entry when step completes (allows re-fetch if step runs again)

### 3. Key Logic Changes

```javascript
const isStart = !this._stepEventsFetchedOnStart[stepJobId];
const isComplete = finished === true;

// ONLY fetch on START or COMPLETE
if (!isStart && !isComplete) {
    console.log('[Queue] Skipping refresh for step', stepJobId, '- middle of execution');
    continue;
}

// Mark as fetched on start
if (isStart) {
    this._stepEventsFetchedOnStart[stepJobId] = true;
}

// Clear tracking when step is complete (allows re-fetch if step runs again)
if (isComplete) {
    delete this._stepEventsFetchedOnStart[stepJobId];
}
```

### 4. Console Logging
- Skip messages: `[Queue] Skipping refresh for step <id> - middle of execution (not START, not COMPLETE)`
- Fetch messages: `[Queue] Refreshed <count> events for step <name> (job <id>) [START]` or `[COMPLETE]`

## Testing Recommendations

1. Monitor browser console for skip messages during step execution
2. Verify only 2 fetches per step: one at START, one at COMPLETE
3. Check Network tab to confirm reduced API calls
4. Ensure events still display correctly in Step Events panel

## Files Modified

- `C:\development\quaero\pages\queue.html` (lines 1810, 3629-3694)

## Acceptance Criteria Met

- ✅ `refreshStepEvents()` only fetches on START (first trigger)
- ✅ `refreshStepEvents()` only fetches on COMPLETE (finished=true)
- ✅ Middle-of-execution triggers are skipped
- ✅ Console logs show skip behavior for debugging
- ✅ Tracking map is cleared when step completes
