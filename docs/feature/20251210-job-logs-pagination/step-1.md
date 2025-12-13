# Step 1: Update loadJobLogs to use /api/logs with pagination
Model: sonnet | Skill: frontend | Status: ✅

## Done
- Added pagination state: `logsCursor`, `hasMoreLogs`, `logsTotal`, `logsPageSize` (1000), `loadingMore`
- Updated `loadJobLogs()` to use `/api/logs?scope=job&job_id={id}&include_children={true|false}&limit=1000&order=asc`
- Added `loadMoreLogs()` method for cursor-based pagination
- Resets pagination state on fresh load

## Files Changed
- `pages/job.html` - Updated Alpine.js state and loadJobLogs function

## Skill Compliance (frontend)
- [x] Alpine.js reactive state for pagination
- [x] Async fetch patterns with URLSearchParams
- [x] Error handling preserved

## Build Check
Build: ⏳ | Tests: ⏭️
