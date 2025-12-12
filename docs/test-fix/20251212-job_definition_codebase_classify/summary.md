# Test Fix Complete
File: test/ui/job_definition_codebase_classify_test.go
Iterations: 5

## Result: ALL TESTS PASS

## Fixes Applied
| Iteration | Files Changed | Tests Fixed |
|-----------|---------------|-------------|
| 1 | pages/queue.html | Step icon (fa-circle -> fa-clock for pending) |
| 1 | pages/queue.html | getStatusIcon (fa-circle -> fa-clock for pending) |
| 1 | pages/queue.html | getStepLogStartIndex (always return 0 for line 1 start) |
| 1 | pages/queue.html | handleJobUpdate (expanded auto-expand to include 'completed') |
| 2 | pages/queue.html | loadJobTreeData (added pending expansions queue) |
| 3 | pages/queue.html | handleJobUpdate (moved auto-expand OUTSIDE status change check) |
| 4 | pages/queue.html | updateStepProgress (added auto-expand for step_progress events) |
| 5 | pages/queue.html | _processRefreshStepEvents (queue pending expansions before tree loads, expand on status not just logs) |

## Root Causes and Solutions

### 1. Step Icon Mismatch (pending: fa-circle -> fa-clock)
**Root Cause**: The step icon CSS class mapping used `fa-circle` for pending status, but the architecture (QUEUE_UI.md) requires `fa-clock`.

**Fix**: Changed `fa-circle` to `fa-clock` in two locations:
- Line 645: Inline class binding in template
- Line 2677: `getStatusIcon()` function

### 2. Log Line Numbering (starting at 4439 instead of 1)
**Root Cause**: `getStepLogStartIndex()` calculated offset based on total count minus displayed count, causing line numbers to reflect server-side offsets.

**Fix**: Always return 0 from `getStepLogStartIndex()`, so line numbers calculate as `0 + logIdx + 1 = 1, 2, 3, ...`

Per QUEUE_LOGGING.md: "Log lines MUST start at line 1 (not 0, not 5)"

### 3. import_files Step Not Auto-Expanding (Complex Race Condition)
**Root Cause**: Multiple timing issues with fast-completing steps:

1. **Status change check**: Auto-expand was inside `if (oldStatus !== status)` block, so if status was already 'completed' when tree loaded, no expansion happened.

2. **Tree data not loaded**: When `job_update` events arrived before tree data loaded, `handleJobUpdate` skipped expansion because `this.jobTreeData[job_id]` was undefined.

3. **step_progress events not handled**: The orchestrator publishes `EventStepProgress` (not `EventJobUpdate`) for synchronously completing steps. The WebSocket handler routes these through the aggregator which sends `refresh_logs` triggers instead of direct `step_progress` broadcasts.

4. **Logs-only auto-expand**: `_processRefreshStepEvents` only auto-expanded steps `if (newLogs.length > 0)`, missing steps that complete without producing logs.

**Fixes**:
- Added `_pendingStepExpansions` map to queue expansions before tree data loads
- Moved auto-expand logic OUTSIDE the status change check
- Added auto-expand to `updateStepProgress` for step_progress events
- Added pending expansion queueing to `_processRefreshStepEvents`
- Changed auto-expand condition from "has logs" to "has activity (running/completed/failed) OR has logs"

## Architecture Compliance Verified
All fixes comply with docs/architecture/ requirements:
- QUEUE_UI.md: Icons match specification, steps auto-expand on activity
- QUEUE_LOGGING.md: Log lines start at 1
- MANAGER_WORKER_ARCHITECTURE.md: Tree data properly handles step hierarchy

## Final Test Output
```
=== RUN   TestJobDefinitionCodebaseClassify
    ✓ Job triggered: Codebase Classify
    ✓ Job reached terminal status: completed
    --- Running Assertions ---
    Assertion 1: Step Log API calls = 3 (max allowed: 10)
    ✓ PASS: Step Log API calls within limit
    Assertion 2: Checking step icons match parent job icon standard...
    ✓ Step 'code_map' icon correct: fa-spinner for status running
    ✓ Step 'import_files' icon correct: fa-check-circle for status completed
    ✓ Step 'rule_classify_files' icon correct: fa-clock for status pending
    ✓ PASS: All step icons match parent job icon standard
    Assertion 3: Checking log line numbering for all steps...
    Step 'import_files' log lines: [1 2 3]
    Step 'rule_classify_files' log lines: [1 2 3 ... 100]
    Step 'code_map' log lines: [1 2 3 ... 13]
    ✓ PASS: All steps have correct sequential log line numbering starting at 1
    Assertion 4: Step expansion order = [code_map import_files rule_classify_files]
    ✓ PASS: All 3 steps auto-expanded
    === TEST RESULT: PASS ===
--- PASS: TestJobDefinitionCodebaseClassify (164.87s)
PASS
```
