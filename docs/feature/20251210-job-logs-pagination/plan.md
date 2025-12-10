# Plan: Job Page Logs Pagination with Unified API
Type: feature | Workdir: ./docs/feature/20251210-job-logs-pagination/

## User Intent (from manifest)
Update the Job page to use the unified `/api/logs` endpoint with pagination (1000 logs per page).

## Active Skills
- frontend (Alpine.js state, pagination patterns)

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Update loadJobLogs to use /api/logs with pagination | - | no | sonnet | frontend |
| 2 | Add pagination state and controls to UI | 1 | no | sonnet | frontend |
| 3 | Build and verify | 2 | no | sonnet | - |

## Order
[1] → [2] → [3]
