# Complete: Log Level Filter API Integration

Iterations: 1

## Result

Fixed the log level filter button in the step panel to make API calls when filter levels change. Previously, the filter button only performed client-side filtering of already-loaded logs. Now it:

1. **Opens dropdown with checkboxes** (Debug, Info, Warn, Error) - unchanged, already working
2. **Triggers API call on filter change** - NEW: `toggleLevelFilter` now calls `fetchStepLogsWithLevelFilter`
3. **Fetches logs with level filter** - NEW: API call includes `level` parameter
4. **Updates both views** - NEW: Updates `jobLogs` (step panel) and `jobTreeData` (tree view)
5. **Highlights filter button** - existing `btn-primary` class when filter is not "all"

## Architecture Compliance

All requirements from docs/architecture/ verified:
- Uses correct `/api/logs` endpoint with level parameter (QUEUE_LOGGING.md)
- Minimizes API calls - single call per filter toggle (QUEUE_UI.md)
- Respects job hierarchy via `step_job_ids` metadata (manager_worker_architecture.md)

## Files Changed

- `pages/queue.html` - Added `fetchStepLogsWithLevelFilter` and `getStepLevelFilterApiParam` functions, modified `toggleLevelFilter` to call API (both Alpine scope instances)
- `test/ui/job_definition_general_test.go` - Updated ASSERTION 2 to verify API call behavior, added checks for both tree-log-line and terminal-line elements

## User Action Required

Build and tests could not be run in validation environment. Please verify:

```bash
# Build
scripts/build.sh

# Run the specific test
go test ./test/ui/... -v -run TestJobDefinitionErrorGeneratorLogFiltering
```
