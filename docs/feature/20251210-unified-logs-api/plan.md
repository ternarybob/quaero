# Plan: Unified Logs API with Step Parameter
Type: feature | Workdir: ./docs/feature/20251210-unified-logs-api/

## User Intent (from manifest)
Extend the existing `/api/logs` endpoint to support a `step` query parameter that allows filtering logs for a specific step job ID. The endpoint should be the single access point for all log types:
- `/api/logs?scope=job&job_id={managerID}&step={stepJobId}&order=asc&size=100`

Currently: UI uses `/api/jobs/{stepJobId}/logs` for step logs
Target: UI uses `/api/logs?scope=job&job_id={stepJobId}&size=100&order=asc`

## Active Skills
- go (handler pattern, service layer, error handling)
- frontend (Alpine.js state, API calls)

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Extend UnifiedLogsHandler to support direct job_id logs (no aggregation) | - | no | sonnet | go |
| 2 | Add size parameter alias for limit | 1 | no | sonnet | go |
| 3 | Update UI to use /api/logs instead of /api/jobs/{id}/logs | 1,2 | no | sonnet | frontend |
| 4 | Build and verify | 3 | no | sonnet | - |

## Order
[1,2] → [3] → [4]

## Analysis
The existing `/api/logs` endpoint with `scope=job` uses `GetAggregatedLogs` which collects logs from parent + all descendants. For step logs, we need simpler direct access:
- When `job_id` is provided and `include_children=false`, just get logs for that specific job
- Add `size` as an alias for `limit` (user preference)
- The existing `/api/jobs/{id}/logs` endpoint already does this via `logService.GetLogs()`

The simplest approach: UI should use existing `/api/logs?scope=job&job_id={stepJobId}&include_children=false`
