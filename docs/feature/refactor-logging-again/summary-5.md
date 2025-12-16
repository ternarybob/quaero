# Summary: Fix Real-Time SSE Log Streaming (Iteration 5)

## Issue

Logs still not updating in real-time. User reported:
- "Show Previous Logs" counter IS incrementing (per second)
- But actual logs and log count (top right) are NOT changing

## Root Causes Found

### Root Cause 1: Missing scope/jobId in SSE URL (Fixed in Iteration 4)

The `connect()` method in `log-stream.js` was not including `scope` and `jobId` in the query string, causing SSE to connect to service logs instead of job logs.

### Root Cause 2: SSE Log Sorting Breaks Display Order (NEW FIX)

SSE logs don't include `line_number` in the event payload. The sorting code:

```javascript
mergedLogs.sort((a, b) => (a.line_number || 0) - (b.line_number || 0));
```

All SSE logs have `line_number: 0` (default), so they sorted to the BEGINNING of the array. Then `getFilteredTreeLogs` does:

```javascript
filteredLogs = filteredLogs.slice(-limit);  // Takes last N logs
```

This shows the OLDEST logs (high line numbers from API) instead of the NEW SSE logs (all at line_number 0).

## Fixes Applied

### 1. SSE URL Fix (`pages/static/js/log-stream.js`) - Iteration 4

Added `scope` and `jobId` to `buildQueryParams` call in `connect()`.

### 2. Remove Line Number Sorting (`pages/queue.html`) - NEW

```javascript
// BEFORE (broken):
const mergedLogs = [...existingLogs, ...uniqueNewLogs];
mergedLogs.sort((a, b) => (a.line_number || 0) - (b.line_number || 0));

// AFTER (fixed):
// Append new logs at the end - SSE logs arrive in real-time order (newest last)
// Don't sort by line_number because SSE logs don't have line_number set
const mergedLogs = [...existingLogs, ...uniqueNewLogs];
```

### 3. Enhanced Debug Logging (`pages/queue.html`, `pages/static/js/log-stream.js`)

Added comprehensive console.log statements to trace:
- When SSE `logs` event is received
- Raw data received by `handleSSELogs`
- Step name matching and available steps
- Whether logs are duplicates
- When Alpine reactivity is triggered

## Console Output to Verify Fix

After restarting and hard-refreshing, the browser console should show:

```
[QuaeroLogs] Connecting to SSE: /api/logs/stream?scope=job&job_id=XXX&limit=100
[QuaeroLogs] SSE connected
[QuaeroLogs] SSE logs event received: { logsCount: 10, meta: {...} }
[Queue] SSE handleSSELogs called for job: abc123-d raw data: {"logs":[...],"meta":{...}}
[Queue] SSE processing 10 logs
[Queue] SSE log entry: { level: "info", step_name: "fast_generator", line_number: 0, msg: "..." }
[Queue] SSE treeData has 2 steps: ["fast_generator", "slow_generator"]
[Queue] SSE matching step: { stepName: "fast_generator", stepIdx: 0, ... }
[Queue] SSE unique new logs: 10 of 10 total
[Queue] SSE merged logs: { step: "fast_generator", newCount: 10, totalCount: 10 }
[Queue] SSE triggering Alpine reactivity update
```

## Build Status

✅ Build passes

## Testing

1. Restart the server
2. Hard-refresh browser (Ctrl+F5) to clear cached JS
3. Create a Test Job Generator job
4. Expand the job to see steps
5. Watch browser console for the debug messages
6. Logs should now appear in step panels in real-time
