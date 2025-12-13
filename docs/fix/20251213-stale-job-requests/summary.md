# Fix Summary: Stale Job Requests After Restart

## Problem
After server restart with `reset_on_startup=true`, queue.html continued making API calls for job IDs that no longer existed, causing errors:
```
[ERR] Failed to get job|job not found: <uuid>
```

## Root Cause
Frontend retained job IDs from previous session in memory (`_pendingStepIds` set) and continued making API calls when WebSocket reconnected after server restart.

## Fix
Implemented **server instance ID** mechanism for restart detection:

1. **Backend**: Server generates unique UUID (`serverInstanceID`) on startup and includes it in WebSocket `status` messages
2. **Frontend**: WebSocketManager detects when `serverInstanceId` changes (indicating server restart)
3. **State Clear**: On restart detection, frontend clears ALL job-related state before making any API calls

## Changes

| File | Change |
|------|--------|
| `internal/handlers/websocket.go` | Added `serverInstanceID` field and UUID generation |
| `pages/static/websocket-manager.js` | Added restart detection and `onServerRestart` callback |
| `pages/queue.html` | Added `handleServerRestart()` to clear all job state |
| `internal/storage/badger/connection.go` | Changed database reset logging to INFO level |

## State Cleared on Restart
- `_pendingStepIds` - pending step event IDs
- `_stepEventFetchInFlight` - in-flight API requests
- `_childFetchInFlight` - child job fetches
- `allJobs`, `filteredJobs` - job lists
- `jobTreeData`, `jobTreeExpandedSteps` - tree view state
- `jobLogs`, `jobLogsLoading` - log data
- `_pendingStepExpansions` - queued expansions

## Verification
- Build: PASSED
- Architecture compliance: PASSED
- All success criteria met

## Architecture Compliance
| Requirement | Status |
|-------------|--------|
| State management (QUEUE_UI.md) | PASS - all state cleared on restart |
| WebSocket events (QUEUE_UI.md) | PASS - graceful reconnection |
| Event handling (QUEUE_SERVICES.md) | PASS - clean state transition |
