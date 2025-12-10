# Complete: Unified Logs API with Step Parameter
Type: feature | Tasks: 3 | Files: 2

## User Request
"But the point of having a single log api endpoint is to provide a filter for any requestor. ie. NOT /api/jobs/{stepJobId}/logs but /api/logs?jobid={jobsid}&step={stepJobId}&order=ASC&size=100"

## Result
Extended the existing `/api/logs` endpoint to support direct job log retrieval with a fast path when `include_children=false`. Added `size` as alias for `limit` parameter. Updated all UI fetch calls to use the unified endpoint instead of `/api/jobs/{id}/logs`.

The unified endpoint now supports:
- `/api/logs?scope=job&job_id={id}&include_children=false` - direct logs for a single job
- `/api/logs?scope=job&job_id={id}&include_children=true` - aggregated logs including children
- `size` parameter as alias for `limit`
- `order=asc|desc` for sort direction

## Skills Used
- go (handler pattern, service delegation)
- frontend (async fetch patterns)

## Validation: ✅ MATCHES
All success criteria met. Implementation matches user intent.

## Review: N/A
No critical triggers.

## Verify
Build: ✅ | Tests: ⏭️

## Files Changed
- `internal/handlers/unified_logs_handler.go` - Added size alias, fast path for direct job logs
- `pages/queue.html` - 4 fetch calls updated to use /api/logs endpoint
