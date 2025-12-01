# Step 7: Update UI queue.html

Model: opus | Status: ✅

## Done

- Added WebSocket handler for `step_progress` events (from StepMonitor)
- Added WebSocket handler for `manager_progress` events (from ManagerMonitor)
- Added Alpine.js event listeners for new progress events
- Added `updateStepProgress()` method to handle step progress updates
- Added `updateManagerProgress()` method to handle manager progress updates
- Synced changes to bin/pages/queue.html and test/bin/pages/queue.html

## WebSocket Events Added

```javascript
// Handle step progress events (from StepMonitor - monitors jobs under a step)
if (message.type === 'step_progress' && message.payload) {
    // Update step job with progress data
    window.dispatchEvent(new CustomEvent('jobList:updateStepProgress', { detail: {...} }));
}

// Handle manager progress events (from ManagerMonitor - monitors steps under a manager)
if (message.type === 'manager_progress' && message.payload) {
    // Update manager job with overall progress
    window.dispatchEvent(new CustomEvent('jobList:updateManagerProgress', { detail: {...} }));
}
```

## Alpine.js Methods Added

- `updateStepProgress(progress)` - Updates step job with child job statistics
- `updateManagerProgress(progress)` - Updates manager job with step statistics

## Progress Text Format

- Step: `"X pending, Y running, Z completed, W failed"` (from StepMonitor)
- Manager: `"X/Y steps, Z/W jobs"` (completed/total)

## Files Changed

- `pages/queue.html` - Added WebSocket handlers and Alpine.js methods
- `bin/pages/queue.html` - Synced copy
- `test/bin/pages/queue.html` - Synced copy

## Verify

Build: ✅ | Tests: ⏭️
