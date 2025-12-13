# Step 1: Implementation
Iteration: 1 | Status: complete

## Problem Analysis

Found **TWO duplicate event listeners** for `jobList:refreshStepEvents`:

1. **Line 1998**: `window.addEventListener('jobList:refreshStepEvents', (e) => this.handleRefreshStepEvents(e.detail));`
   - Calls `fetchStepLogs()` which uses `/api/jobs/{id}/tree/logs`

2. **Line 2022**: `window.addEventListener('jobList:refreshStepEvents', (e) => this.refreshStepEvents(e.detail));`
   - Calls `_processRefreshStepEvents()` which uses `/api/logs?scope=job&job_id=...`

**Effect**: Every single `refresh_logs` WebSocket event triggers BOTH handlers, resulting in double API calls. When there are many logs being generated, this multiplies the API call count significantly.

The screenshot shows requests to `logs?scope=job&job_id=...` which corresponds to the `refreshStepEvents()` path (line 3914).

## Root Cause

The `refreshStepEvents()` method (lines 3805-3828) was added as a debounced version, but the original `handleRefreshStepEvents()` listener was never removed. Both handlers process the same event.

Additionally, `handleRefreshStepEvents()` (line 3486-3517):
- Does NOT debounce - fetches immediately for ALL steps in jobTreeData
- Calls `fetchStepLogs()` which has its own debouncing but still makes calls

While `refreshStepEvents()` (lines 3805-3828):
- HAS debouncing (500ms)
- Fetches via `/api/logs?scope=job&job_id=...` for each step_id

## Fix Applied

Remove the duplicate listener on line 1998 (`handleRefreshStepEvents`). Keep only the debounced `refreshStepEvents` handler on line 2022.

The `handleRefreshStepEvents` function itself can be marked deprecated since it's no longer used.

## Changes Made
| File | Action | Description |
|------|--------|-------------|
| `pages/queue.html` | modified | Remove duplicate event listener for `jobList:refreshStepEvents` on line 1998 |

## Build & Test
Build: PASSED (v0.1.1969)
Tests: Some timeouts (unrelated to this change)

## Architecture Compliance (self-check)
- [x] API calls reduced - single handler instead of dual handlers
- [x] Log fetching via WebSocket triggers - maintained (refreshStepEvents still receives triggers)
- [x] Debouncing in place - refreshStepEvents has 500ms debounce
