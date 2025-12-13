# Validation 1: Architecture Compliance
Iteration: 1 | Status: PASS

## Requirements Check

### From manifest.md Success Criteria

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Database is completely removed when `reset_on_startup=true` | PASS | Logging changed to INFO level for visibility, code review confirms `os.RemoveAll` is used |
| Frontend clears all job state on WebSocket reconnection after server restart | PASS | `handleServerRestart()` clears `_pendingStepIds`, `_stepEventFetchInFlight`, `allJobs`, `jobTreeData`, `jobLogs`, etc. |
| No API calls for non-existent jobs after restart | PASS | Frontend clears pending job IDs before any can be processed |
| Build passes | PASS | `go build` succeeded |
| Architecture compliance verified | PASS | See detailed checks below |

### Architecture Document Compliance

#### QUEUE_UI.md - State Management

> jobTreeData, jobTreeExpandedSteps, jobLogs should be cleared on reset

**Compliance:**
- [x] `handleServerRestart()` explicitly clears:
  - `this.allJobs = []`
  - `this.filteredJobs = []`
  - `this.jobTreeData = {}`
  - `this.jobTreeExpandedSteps = {}`
  - `this.jobLogs = {}`
  - `this.jobLogsLoading = {}`
  - `this.jobTreeLoading = {}`
  - `this._pendingStepExpansions = {}`

#### QUEUE_UI.md - WebSocket Events

> Handle reconnection gracefully

**Compliance:**
- [x] WebSocketManager detects server restart via `serverInstanceId` change
- [x] `_server_restart` event emitted to subscribers
- [x] Frontend clears state and reloads fresh data via `loadJobs()`

#### QUEUE_SERVICES.md - Event Service

> Clean event handling on restart

**Compliance:**
- [x] Server generates new `serverInstanceID` on each startup
- [x] Frontend detects restart before processing any stale events
- [x] Pending step IDs cleared immediately on restart detection

## Code Verification

### Server-side Changes

```go
// websocket.go:51
serverInstanceID string // Unique ID generated on startup - clients use to detect server restart

// websocket.go:60
serverInstanceID: uuid.New().String(),

// websocket.go:219
ServerInstanceID string `json:"serverInstanceId"` // Unique ID per server startup - clients clear state on change
```

### Frontend Changes

```javascript
// websocket-manager.js - restart detection
if (type === 'status' && payload && payload.serverInstanceId) {
    const newInstanceId = payload.serverInstanceId;
    if (this.serverInstanceId !== null && this.serverInstanceId !== newInstanceId) {
        // Server has restarted - notify subscribers to clear state
        console.log('[WSManager] Server restart detected');
        this.subscribers['_server_restart'].forEach(cb => cb({...}));
    }
    this.serverInstanceId = newInstanceId;
}
```

### Database Reset Logging

```go
// connection.go:25 - now INFO level instead of DEBUG
logger.Info().Str("path", config.Path).Msg("Database reset: deleting existing database (reset_on_startup=true)")
```

## Expected Behavior After Fix

1. Server starts with `reset_on_startup=true`
2. Database is deleted (logged at INFO level)
3. New `serverInstanceID` generated
4. Browser connects via WebSocket
5. Server sends `status` message with `serverInstanceId`
6. If `serverInstanceId` changed from previous value:
   - WebSocketManager emits `_server_restart` event
   - `queue.html` receives `jobList:serverRestart` event
   - `handleServerRestart()` clears ALL job-related state
   - Fresh `loadJobs()` call fetches from clean database
7. No stale job ID API calls occur

## Validation Result

**PASS** - Fix addresses both root causes:
1. Frontend now detects server restart and clears state
2. Database reset is now visible in logs at INFO level
