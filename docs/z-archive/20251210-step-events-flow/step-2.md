# Step 2: Load events for completed steps on page load
Model: sonnet | Skill: frontend | Status: ✅

## Done
- Added `loadCompletedStepEvents()` function to queue.html
- Added `fetchStepEventsById()` helper function
- Called `loadCompletedStepEvents()` after historical logs fetch in loadJobs()
- Targets completed/failed/cancelled steps that don't have cached events

## Files Changed
- `pages/queue.html` - Added functions to auto-load events for completed steps on page load

## Skill Compliance
- [x] Alpine.js reactive patterns followed
- [x] Async/await for API calls
- [x] Uses jobLogs cache pattern consistently
- [x] No duplicate fetches (checks existing logs first)

## Build Check
Build: ⏳ | Tests: ⏭️
