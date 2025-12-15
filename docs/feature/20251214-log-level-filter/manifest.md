# Feature: Log Level Filter API Integration

Date: 2025-12-14
Request: "The filter button should filter the level of the logs within the step, not the jobs. Currently, clicking the button simply scrolls to the top of the page. The button should open a list of levels (ERR/WRN/INF), allow the user to select 1 or more levels, and then filter the logs, with a new API call. Noting the current stop status and position of the logs."

## User Intent
Fix the log level filter button in the step panel to make API calls when filter levels change, rather than just client-side filtering. The filter should:
1. Open a dropdown with level checkboxes (Debug, Info, Warn, Error)
2. When levels are changed, fetch new logs from API with level filter
3. Preserve current step status and log position context

## Success Criteria
- [ ] Filter dropdown opens correctly without scrolling to top
- [ ] Toggling filter levels triggers API call with level parameter
- [ ] Logs are re-fetched with the selected level filter applied
- [ ] Step status and context preserved during filter operation
- [ ] Test updated to assert filter functionality works with API calls

## Applicable Architecture Requirements

| Doc | Section | Requirement |
|-----|---------|-------------|
| QUEUE_UI.md | Log Display | Logs should be fetched via REST API when step is expanded |
| QUEUE_UI.md | API Calls | Minimize API calls - use debouncing |
| QUEUE_LOGGING.md | Log Retrieval API | GET /api/jobs/{id}/logs supports `level` filter param |
| QUEUE_LOGGING.md | UI Log Display | Trigger-based fetching with pagination |

## Current State Analysis

The step panel filter button (lines 199-230 in queue.html) uses `toggleLevelFilter()` which:
- Only updates client-side state `stepLevelFilters`
- Does NOT call API to refresh logs with new level filter
- Client-side filtering via `filterLogsByLevels()` only filters already-loaded logs

The tree view filter button (lines 598-628) uses `toggleTreeLogLevel()` which:
- Updates state `treeLogLevelFilter`
- DOES call `fetchStepLogs()` for all expanded steps (lines 4641-4654)
- This is the correct pattern to follow

## Fix Required

Update `toggleLevelFilter()` to also call the log fetch API with the new level filter, similar to `toggleTreeLogLevel()`.
