# Complete: Job Page Logs Pagination with Unified API
Type: feature | Tasks: 3 | Files: 1

## User Request
"Review the Job page. The total logs needs to be paged, and limited to 1000 per page. Hence also uses the same log api end point."

## Result
Updated the Job page (`/job?id={id}`) to use the unified `/api/logs` endpoint with pagination:
- Uses `/api/logs?scope=job&job_id={id}&include_children={true|false}&limit=1000&order=asc`
- Supports cursor-based pagination via `next_cursor` response field
- Shows "Load More Logs" button when more logs are available
- Displays log count information

## Skills Used
- frontend (Alpine.js state management, pagination UI)

## Validation: ✅ MATCHES
All success criteria met.

## Review: N/A
No critical triggers.

## Verify
Build: ✅ | Tests: ⏭️

## Files Changed
- `pages/job.html` - Updated to use unified API with pagination controls
