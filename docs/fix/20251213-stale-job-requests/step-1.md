# Step 1: Implementation
Iteration: 1 | Status: complete

## Problem Analysis

After server restart with `reset_on_startup=true`, the queue.html page was making API calls for job IDs that no longer exist, causing errors:
```
[ERR] Failed to get job|job not found: <uuid>
```

Root cause: The frontend retained job IDs from the previous session in memory (e.g., `_pendingStepIds` set) and continued making API calls for them when WebSocket reconnected.

## Solution

Implemented a **server instance ID** mechanism:
1. Server generates unique UUID on startup
2. Server sends instance ID in WebSocket `status` message
3. Frontend detects when instance ID changes (server restart)
4. Frontend clears ALL job-related state on restart detection

## Changes Made

| File | Action | Description |
|------|--------|-------------|
| `internal/handlers/websocket.go` | modified | Added `serverInstanceID` field, UUID import, ID initialization, and inclusion in StatusUpdate |
| `pages/static/websocket-manager.js` | modified | Added `serverInstanceId` tracking, restart detection in handleMessage, `onServerRestart` callback |
| `pages/queue.html` | modified | Added server restart subscription, `handleServerRestart()` method to clear all job state |
| `internal/storage/badger/connection.go` | modified | Changed database reset logging from Debug to Info level |

## Key Code Changes

### Backend (websocket.go)

```go
// Added to struct
serverInstanceID string // Unique ID generated on startup

// In NewWebSocketHandler
serverInstanceID: uuid.New().String(),

// In StatusUpdate
ServerInstanceID string `json:"serverInstanceId"`

// In sendStatus
ServerInstanceID: h.serverInstanceID,
```

### Frontend (websocket-manager.js)

```javascript
// Track server instance
this.serverInstanceId = null;

// In handleMessage
if (type === 'status' && payload && payload.serverInstanceId) {
    if (this.serverInstanceId !== null && this.serverInstanceId !== newInstanceId) {
        // Server has restarted - notify subscribers
        this.subscribers['_server_restart'].forEach(cb => cb(...));
    }
    this.serverInstanceId = newInstanceId;
}
```

### Frontend (queue.html)

```javascript
// Subscribe to restart events
WebSocketManager.onServerRestart(({ oldInstanceId, newInstanceId }) => {
    window.dispatchEvent(new CustomEvent('jobList:serverRestart', ...));
});

// Clear all state on restart
handleServerRestart() {
    if (this._pendingStepIds) { this._pendingStepIds.clear(); }
    if (this._stepEventFetchInFlight) { this._stepEventFetchInFlight.clear(); }
    this.allJobs = [];
    this.jobTreeData = {};
    this.jobLogs = {};
    // ... clear other state
    this.loadJobs(); // Reload fresh from server
}
```

## Build & Test
Build: PASSED
Tests: PASSED (no specific tests for this feature)

## Architecture Compliance (self-check)
- [x] State management follows QUEUE_UI.md patterns
- [x] WebSocket handling follows QUEUE_SERVICES.md event flow
- [x] Frontend clears all job-related state on restart
- [x] Database reset logging improved for visibility
