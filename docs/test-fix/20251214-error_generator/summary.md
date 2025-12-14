# Test Fix Summary
File: test/ui/error_generator_test.go
Test: TestJobDefinitionErrorGeneratorLogFiltering
Iterations: 5
Date: 2025-12-14

## Result: PARTIAL SUCCESS

### Assertions Status
1. **Assertion 1 (Filter Dropdown)**: ✅ PASS - Filter dropdown exists with All, Warn+, Error options
2. **Assertion 2 (Error Filter)**: ✅ PASS - Error filter functionality works (shows 0 logs when no errors)
3. **Assertion 3 (Show Earlier Logs)**: ⚠️ SKIP - Test skips when no earlier logs available

## Fixes Applied

| Iteration | Files Changed | Description |
|-----------|---------------|-------------|
| 1 | `pages/queue.html` | Added `load-earlier-logs-btn` class and data attributes to the "Show earlier logs" button |
| 1-5 | `pages/queue.html` | Added `window.loadEarlierLogs` global function for programmatic click handling |

## Architecture Compliance
All fixes comply with:
- `docs/architecture/QUEUE_UI.md` - Log fetching via API maintained
- `docs/architecture/QUEUE_LOGGING.md` - Trigger-based fetching preserved

## Technical Details

### What Was Fixed
- Added infrastructure to support programmatic clicks on "Show earlier logs" button
- Button now has class `load-earlier-logs-btn` and data attributes for jobId, stepName, stepIndex
- Global function `window.loadEarlierLogs(jobId, stepName, stepIndex)` exposed for test access

### Remaining Issue
The test scenario doesn't reliably create enough logs on the **step job** to trigger the "Show earlier logs" button (requires > 200 logs). The error_generator worker creates logs on individual worker jobs, not aggregated to the step.

This is an **architectural limitation**, not a code bug:
- Step jobs receive only their own logs (~5-10 logs)
- Worker jobs each have their own logs (300 each)
- UI shows step logs, not aggregated worker logs
- To show 200+ logs on a step, logs would need to be aggregated from workers to the step

### Test Behavior
- Test SKIP's gracefully when no "Show earlier logs" button is available
- Test PASS for Assertions 1 and 2
- Test will fully pass when a scenario creates 200+ logs on a single step

## Files Modified
- `pages/queue.html` - Button attributes and global function
- `internal/queue/workers/error_generator_worker.go` - (Reverted - no permanent changes)

## NOT Changed
- `test/ui/error_generator_test.go` - Tests define requirements, not modified

## Recommendation
To fully test Assertion 3, consider:
1. Creating a test-specific worker that logs directly to the step (not child workers)
2. Or modifying the test to use a different job type that generates many step-level logs
3. Or implementing log aggregation from workers to steps (significant architecture change)
