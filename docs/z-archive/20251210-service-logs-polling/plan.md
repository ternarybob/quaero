# Plan: Fix Service Logs Duplicate Polling
Type: fix | Workdir: ./docs/fix/20251210-service-logs-polling/

## User Intent (from manifest)
Stop unnecessary polling/duplicate requests in Service Logs UI:
1. WebSocket-driven refresh - UI should only fetch logs when backend sends "refresh_logs" trigger
2. Smart backend triggers - Backend should only send trigger if logs occurred in last second
3. Initial page load - Get last 100 logs on page load
4. No duplicate requests - Stop continuous /recent API calls when idle

## Active Skills
- go (backend log aggregator)
- frontend (Alpine.js UI component)

## Analysis

### Current Architecture (already correct):
- LogEventAggregator.StartPeriodicFlush() runs every 1 second
- flushPending() only triggers if hasPendingLogs is true
- UI serviceLogs component fetches on `refresh_logs` WebSocket message
- Initial load calls loadRecentLogs() which fetches last 100

### Root Cause Investigation:
Looking at the network trace, the continuous requests are:
1. `/jobs?parent_id=...` - from 2-second child refresh interval (line 1875 queue.html)
2. `/recent` - from log refresh triggers

The log aggregator **should** only trigger when hasPendingLogs=true, but let me verify the implementation is correct and add better logging to understand when triggers occur.

### Issues Found:
1. The LogEventAggregator doesn't log when it skips (no pending logs) - hard to debug
2. The child refresh interval fires every 2 seconds regardless of whether children are actually missing

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Add debug logging to LogEventAggregator to trace when triggers fire vs skip | - | no | sonnet | go |
| 2 | Fix child refresh interval to not poll when all children present | 1 | no | sonnet | frontend |
| 3 | Build and verify | 2 | no | sonnet | go |

## Order
[1] → [2] → [3]
