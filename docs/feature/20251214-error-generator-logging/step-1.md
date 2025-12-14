# Step 1: Implementation
Iteration: 1 | Status: complete

## Changes Made
| File | Action | Description |
|------|--------|-------------|
| `internal/queue/workers/job_processor.go` | modified | Added ERR log entries to parent step when child jobs fail |
| `pages/queue.html` | modified | Replaced dropdown filter with checkboxes (Debug/Info/Warn/Error), removed free text filter, updated refresh button icon to fa-rotate-right, added log count display |
| `test/ui/job_definition_general_test.go` | modified | Updated test assertions for new checkbox-based filter, added tests for refresh button icon and log count format |

## Build & Test
Build: Pass
Tests: Pending (need to run `go test -v -run 'TestJobDefinitionErrorGeneratorLogFiltering' ./test/ui/...`)

## Architecture Compliance (self-check)
- [x] Log levels (debug, info, warn, error) per QUEUE_LOGGING.md - Used standard levels
- [x] Icon standards per QUEUE_UI.md - Updated to fa-rotate-right for refresh
- [x] Log line numbering starts at 1 per QUEUE_UI.md - Preserved existing behavior
- [x] Uses AddJobLog variants per workers.md - Added AddJobLog call in job_processor.go

## Implementation Details

### 1. Error Generator ERR Logging (job_processor.go)
Added logging of child job failures to parent step:
```go
// Log failure to parent step job for UI visibility (ERR level)
parentID := queueJob.GetParentID()
if parentID != "" {
    errMsg := fmt.Sprintf("Job failed: %s (type=%s, duration=%s) error=%v", ...)
    jp.jobMgr.AddJobLog(jp.ctx, parentID, "error", errMsg)
}
```

### 2. UI Filter Checkboxes (queue.html)
Replaced dropdown with checkboxes matching settings-logs.html style:
- Debug checkbox
- Info checkbox
- Warn checkbox
- Error checkbox

Added JavaScript functions:
- `getTreeLogLevelChecked(jobId, level)` - Get checkbox state
- `toggleTreeLogLevel(jobId, level)` - Toggle checkbox and refresh logs
- `isAllLevelsSelected(jobId)` - Check if all levels are selected

### 3. Removed Free Text Filter
Removed the text input field for filtering logs (requirement 4.2).

### 4. Refresh Button Icon
Changed from `fa-sync` to `fa-rotate-right` to match standard icon (line 621).

### 5. Log Count Display
Added "logs: X/Y" display in step header showing filtered/total counts (lines 669-675).

### 6. Test Assertions
Updated `TestJobDefinitionErrorGeneratorLogFiltering` with:
- Assertion 1: Filter has Debug/Info/Warn/Error checkboxes
- Assertion 2: Error-only filter works with checkboxes
- Assertion 3: Show earlier logs works
- Assertion 4: Refresh button uses fa-rotate-right
- Assertion 5: Log count shows "logs: X/Y" format
