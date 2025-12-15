# Task 1: Extend UnifiedLogsHandler for direct job logs
Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Enable `/api/logs?scope=job&job_id={stepJobId}` to return logs for a specific job without aggregation overhead.

## Skill Patterns to Apply
- Constructor DI pattern
- Context everywhere
- Structured logging with arbor
- Wrap errors with context

## Do
1. In `getJobLogs()`, when `include_children=false`, use `logService.GetLogs()` or `GetLogsByLevel()` directly
2. This avoids the expensive `GetAggregatedLogs()` for single-job queries
3. Add `size` parameter as alias for `limit`

## Accept
- [ ] `/api/logs?scope=job&job_id=X&include_children=false` returns only that job's logs
- [ ] `size` parameter works as alias for `limit`
- [ ] Build succeeds
