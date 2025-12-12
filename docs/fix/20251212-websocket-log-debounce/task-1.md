# Task 1: Add debouncing to fetchStepLogs with per-step tracking
Workdir: ./docs/fix/20251212-websocket-log-debounce/ | Depends: none | Critical: no
Model: opus | Skill: frontend

## Context
This task is part of: Fixing excessive log API calls by adding proper debouncing

## User Intent Addressed
Stop excessive log API calls - The UI is calling the log API way too often. The trigger should be buffered to 1 second interval.

## Input State
Files that exist before this task:
- `pages/queue.html` - Current UI with `fetchStepLogs` that has no debouncing, causing API flooding

## Output State
Files after this task completes:
- `pages/queue.html` - Updated with debounced `fetchStepLogs` that prevents API flooding

## Skill Patterns to Apply
### From frontend patterns:
- **DO:** Use per-step debounce timers to prevent concurrent fetches for same step
- **DO:** Clear existing timer before setting new one (standard debounce pattern)
- **DO:** Allow immediate fetch when status changes (no debounce for critical updates)
- **DON'T:** Use global debounce that blocks all step fetches
- **DON'T:** Add unnecessary state that could cause memory leaks

## Implementation Steps
1. Add `_stepFetchDebounceTimers` map to track pending debounce timers per step
2. Add `_stepFetchInFlight` set to track currently active fetch requests
3. Modify `fetchStepLogs` to:
   - Check if fetch already in flight for this step
   - Clear existing debounce timer for this step
   - Set new debounce timer (1 second) before actual fetch
   - Mark fetch as in-flight when starting, clear when done
4. Add `fetchStepLogsImmediate` for status-change scenarios (no debounce)
5. Update `handleRefreshStepEvents` to use debounced fetch

## Code Specifications
```javascript
// New state tracking
_stepFetchDebounceTimers: {},  // key: jobId:stepName, value: timer ID
_stepFetchInFlight: new Set(), // tracks currently fetching steps

// Modified fetchStepLogs signature stays same
async fetchStepLogs(jobId, stepName, stepIdx, immediate = false)
// immediate=true bypasses debounce for status changes
```

## Accept Criteria
- [ ] No duplicate API calls for same step within 1 second window
- [ ] In-flight requests not duplicated
- [ ] Status change updates can bypass debounce with immediate=true
- [ ] Console logs show debounced behavior
- [ ] Build passes

## Handoff
After completion, next task(s): task-2 (fix step status sync)
