# Task 2: Load events for completed steps on page load
Depends: 1 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
On page load with completed job, all steps should show their events (not "0 events").

## Skill Patterns to Apply
- Alpine.js reactive components
- Async/await for API calls
- jobLogs cache pattern

## Do
1. In queue.html, add function to auto-load events for completed steps
2. Call this function after initial job list render
3. For each completed step (completed/failed/cancelled status), fetch events from API
4. Store in jobLogs cache so getStepLogs() returns the data

## Accept
- [ ] Completed steps auto-load their events on page load
- [ ] Events display correctly (not showing "No events yet")
- [ ] No duplicate API calls for same step
