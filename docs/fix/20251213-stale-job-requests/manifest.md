# Fix: Stale Job Requests After Restart
Date: 2025-12-13
Request: "Service after restart and hard page reset is still refreshing job logs for jobs that don't exist"

## User Intent
Fix the bug where the queue.html page makes API calls for job IDs that no longer exist after a service restart with `reset_on_startup=true`. The browser appears to retain state from the previous session.

## Success Criteria
- [x] Database is completely removed when `reset_on_startup=true` (verify logging)
- [x] Frontend clears all job state on WebSocket reconnection after server restart
- [x] No API calls for non-existent jobs after restart
- [x] Build passes
- [x] Architecture compliance verified

## Applicable Architecture Requirements
| Doc | Section | Requirement |
|-----|---------|-------------|
| QUEUE_UI.md | State Management | jobTreeData, jobLogs, etc. should be cleared on reset |
| QUEUE_UI.md | WebSocket Events | Handle reconnection gracefully |
| QUEUE_SERVICES.md | Event Service | Clean event handling on restart |

## Problem Analysis
From screenshot (ksnip_20251213-104729.png):
1. Service Logs show errors: `[ERR] Failed to get job|job not found: {UUID}`
2. All errors have same timestamp (10:47:04) - occurred at startup/reconnection
3. Job Queue shows 0 jobs (database was cleared)
4. Network tab shows API calls happening

Root cause candidates:
1. Frontend WebSocket handler doesn't clear state on reconnection
2. Old job IDs from `_pendingStepIds` set being processed
3. `refreshStepEvents` trying to fetch logs for non-existent jobs

## Solution Approach
1. Add server "instance ID" that changes on restart
2. Frontend detects instance change and clears all job state
3. Improve database reset logging to INFO level for visibility
