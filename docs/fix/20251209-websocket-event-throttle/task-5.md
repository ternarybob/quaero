# Task 5: Refactor UI Event Handling

Skill: frontend | Status: pending | Depends: task-3, task-4

## Objective
Update queue.html to handle trigger-based event fetching instead of direct WebSocket payload.

## Changes

### File: `pages/queue.html`

1. Add handler for new WebSocket message type (around line 1176):
```javascript
// Handle refresh_step_events trigger
if (message.type === 'refresh_step_events' && message.payload) {
    const { step_ids, timestamp } = message.payload;
    console.log('[Queue] Received refresh trigger for steps:', step_ids);

    // Dispatch event to Alpine component
    window.dispatchEvent(new CustomEvent('jobList:refreshStepEvents', {
        detail: { step_ids, timestamp }
    }));
}
```

2. Add event listener in Alpine component (around line 1842):
```javascript
window.addEventListener('jobList:refreshStepEvents', (e) => this.refreshStepEvents(e.detail));
```

3. Add refresh method to Alpine component:
```javascript
// Fetch events from API for specified steps
async refreshStepEvents(detail) {
    const { step_ids } = detail;

    for (const stepId of step_ids) {
        try {
            const response = await fetch(`/api/jobs/${stepId}/events?limit=100`);
            if (!response.ok) continue;

            const data = await response.json();

            // Find the step job in allJobs
            const step = this.allJobs.find(j => j.id === stepId);
            if (step) {
                // Update step's events/logs
                step._events = data.events || [];
                step._lastEventFetch = new Date().toISOString();
            }
        } catch (err) {
            console.error('[Queue] Failed to fetch events for step:', stepId, err);
        }
    }

    this.throttledRenderJobs();
}
```

4. Modify step panel template to show events from `_events` property:
- Find the events display section in step details
- Update to read from `step._events` instead of WebSocket payload
- Add "Last updated: X" indicator using `step._lastEventFetch`

5. Keep existing step_progress handler for backward compatibility but mark as deprecated:
```javascript
// Legacy: Direct step_progress updates (kept for backward compat)
// New approach: UI fetches via API on refresh_step_events trigger
if (message.type === 'step_progress' && message.payload) {
    // Keep minimal update for status changes, but don't push full event data
    const stepProgress = message.payload;
    // Only update status, not events
    window.dispatchEvent(new CustomEvent('jobList:updateStepStatus', {
        detail: {
            step_id: stepProgress.step_id,
            status: stepProgress.status
        }
    }));
}
```

## UI Behavior

1. On page load: Initial events fetched via API (existing behavior)
2. During processing: WebSocket sends `refresh_step_events` trigger
3. On trigger: UI fetches latest events from `/api/jobs/{step_id}/events`
4. Events panel updates with fresh data

## Validation
- Build compiles successfully
- UI updates when triggers received
- Events display correctly in step panels
- No performance degradation with 100+ jobs
