# Fix: Excessive Log API Requests
Date: 2025-12-13
Request: "Queue pages making too many log API endpoint requests, independent of WebSocket triggers"

## User Intent
Fix the bug where the queue.html page is making excessive API calls to the logs endpoint, outside of the intended WebSocket-triggered architecture.

## Success Criteria
- [x] API calls to logs endpoint reduced to < 10 per step (per QUEUE_UI.md)
- [x] Log fetching only triggered by WebSocket `refresh_logs` events or manual step expansion
- [x] No polling or duplicate fetch mechanisms
- [x] Build passes
- [x] Architecture compliance verified

## Applicable Architecture Requirements
| Doc | Section | Requirement |
|-----|---------|-------------|
| QUEUE_UI.md | API Calls | Step log API calls should be < 10 per job execution |
| QUEUE_UI.md | Log Display | Fetch logs only when step is expanded |
| QUEUE_UI.md | WebSocket Events | Use WebSocket events for incremental updates |
| QUEUE_LOGGING.md | Log Fetching Strategy | UI uses trigger-based fetching, NOT polling |
| QUEUE_LOGGING.md | Real-time Updates | WebSocket `job_log` events trigger incremental fetch |
| QUEUE_LOGGING.md | Known Issues | UI may make too many API calls for logs |

## Problem Analysis
From screenshots:
1. Network tab shows dozens of requests to `logs?scope=job&job_id=...`
2. WebSocket messages show only a few `refresh_logs` events
3. Disconnect indicates a bug in queue.html causing log fetches outside of WebSocket triggers
