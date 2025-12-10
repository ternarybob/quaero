# Task 1: Update loadJobLogs to use /api/logs with pagination
Depends: - | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
Change Job page to use unified `/api/logs?scope=job&job_id={id}` endpoint with limit=1000 and cursor-based pagination.

## Skill Patterns to Apply
- Alpine.js reactive state
- Async fetch patterns
- Cursor-based pagination

## Do
1. Add pagination state: `logsCursor`, `hasMoreLogs`, `logsPage`
2. Update `loadJobLogs()` to use `/api/logs?scope=job&job_id={id}&include_children={isParent}&limit=1000`
3. Handle `next_cursor` response for pagination
4. Add `loadMoreLogs()` and `loadPreviousLogs()` methods

## Accept
- [ ] Uses /api/logs endpoint
- [ ] Limits to 1000 logs per request
- [ ] Stores cursor for pagination
