# Step 2: Fix child refresh interval to not poll when all children present
Model: sonnet | Skill: frontend | Status: Done

## Done
- Added `_childFetchInFlight` Set to track which parent IDs have fetch requests in progress
- Modified interval to skip parents already being fetched (prevents duplicate concurrent requests)
- Uses `.finally()` to clear in-flight tracking when fetch completes (success or error)
- Removed noisy debug logging when nothing to fetch

## Files Changed
- `pages/queue.html` - Lines 1873-1899: Added in-flight tracking, modified interval logic

## Skill Compliance (frontend)
- [x] Alpine.js patterns followed
- [x] Minimize network requests (no duplicates for same parent)
- [x] Async/await properly handled with .finally()

## Build Check
Build: Pending | Tests: Pending
