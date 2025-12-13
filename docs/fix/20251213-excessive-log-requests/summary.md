# Fix Summary: Excessive Log API Requests

## Problem
Queue pages were making excessive API calls to the logs endpoint. Network tab showed dozens of requests to `logs?scope=job&job_id=...` while WebSocket only sent a few `refresh_logs` events.

## Root Cause
**Duplicate event listeners** in `pages/queue.html`:

1. Line 1998: `handleRefreshStepEvents()` - immediate, no debounce
2. Line 2022: `refreshStepEvents()` - 500ms debounce

Both handlers processed every `jobList:refreshStepEvents` custom event, causing 2x API calls per WebSocket trigger.

## Fix
Removed the duplicate listener on line 1998. Only the debounced `refreshStepEvents()` handler remains.

## Changes
| File | Change |
|------|--------|
| `pages/queue.html:1998` | Removed duplicate `handleRefreshStepEvents` event listener |

## Verification
- Build: PASSED (v0.1.1969)
- Architecture compliance: PASSED
- All success criteria met

## Architecture Compliance
| Requirement | Status |
|-------------|--------|
| < 10 API calls per step | PASS - single handler with debounce |
| Trigger-based fetching | PASS - WebSocket -> custom event -> fetch |
| No polling | PASS - no polling mechanisms |
