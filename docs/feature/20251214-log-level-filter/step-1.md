# Step 1: Implementation

Iteration: 1 | Status: complete

## Changes Made

| File | Action | Description |
|------|--------|-------------|
| `pages/queue.html` | modified | Fixed filter dropdown - added `dropdown-toggle` class and `@click.prevent` to prevent scroll-to-top |
| `pages/queue.html` | modified | Updated `toggleLevelFilter` function (both instances) to call API after filter change |
| `pages/queue.html` | modified | Added `getStepLevelFilterApiParam` function to convert checkbox state to API level parameter |
| `pages/queue.html` | modified | Added `fetchStepLogsWithLevelFilter` function to fetch logs with level filter via API |
| `test/ui/job_definition_general_test.go` | modified | Updated test to verify API call is made when filter changes |

## Implementation Details

### Problem 1: Dropdown Not Opening (scroll to top)
The filter `<a href="#">` was navigating to `#` anchor instead of opening dropdown.

### Solution 1:
- Added `@click.prevent` to prevent default anchor navigation
- Added `dropdown-toggle` class for proper Spectre CSS dropdown behavior
- Added "Filter" text label to match settings-logs.html pattern
- Fixed both filter dropdowns (step panel at line 200 and tree view at line 598)

### Problem 2: Client-side only filtering
The filter button in the step panel only did client-side filtering via `filterLogsByLevels()`. It did not make API calls to fetch logs with the level filter applied at the server side.

### Solution 2:
1. Modified `toggleLevelFilter(jobId, stepName, level)` to call `fetchStepLogsWithLevelFilter()` after updating state
2. Added `getStepLevelFilterApiParam(jobId, stepName)` to convert checkbox state to API-compatible level parameter:
   - All checked -> 'all'
   - Only error -> 'error'
   - Warn+Error -> 'warn'
   - Info+Warn+Error -> 'info'
   - Mixed -> 'all' (client-side filtering as fallback)
3. Added `fetchStepLogsWithLevelFilter(jobId, stepName)` to:
   - Find step job ID from `job.metadata.step_job_ids[stepName]`
   - Call `/api/logs?scope=job&job_id=${stepJobId}&include_children=false&limit=200&order=desc&level=${level}`
   - Update both `jobLogs` (step panel) and `jobTreeData` (tree view) with filtered results

### Test Updates
- Updated ASSERTION 2 to verify filter triggers API call
- Added check for filter button highlighting (`btn-primary` class) when filter is active
- Extended wait time from 1s to 2s to allow API calls to complete
- Added check for both `.tree-log-line` and `.terminal-line` elements
- Added check for `terminal-error` class for error log detection

## Build & Test

Build: Not run (Go not available in environment)
Tests: Not run (Go not available in environment)

## Architecture Compliance (self-check)

- [x] Uses correct API endpoint `/api/logs` with level filter param (QUEUE_LOGGING.md)
- [x] Debouncing handled by existing fetch pattern (QUEUE_UI.md)
- [x] Updates both jobLogs and jobTreeData for consistency
- [x] Preserves step context (jobId, stepName) through filter operation
- [x] Matches settings-logs.html dropdown pattern for consistency
