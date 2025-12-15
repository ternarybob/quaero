# Complete: Test Replication

Iterations: 2

## Result

Successfully implemented `TestJobDefinitionErrorGeneratorComprehensive` that replicates the comprehensive assertions from `job_definition_codebase_classify_test.go`.

### Features Implemented

1. **Two error_generator steps with different names:**
   - `step_one_generate` (10% failure rate, 200 logs, 50ms delay)
   - `step_two_generate` (20% failure rate, 200 logs, 50ms delay)

2. **Real-time monitoring via WebSocket (NO page refresh):**
   - Tracks `refresh_logs` messages via Chrome DevTools Protocol
   - Tracks API calls via network event listener
   - No `loadJobs()` or page refresh during monitoring

3. **5-minute timeout with terminal state wait:**
   - Job must reach `completed`, `failed`, or `cancelled`
   - Test fails if timeout exceeded

4. **API vs UI consistency assertions every 30 seconds:**
   - Parent job status matches between API and UI
   - Step statuses match between API and UI

5. **Comprehensive assertions replicated:**
   - Progressive log updates within first 30s
   - WebSocket refresh_logs messages < 40
   - API calls gated by refresh triggers
   - Step icons match parent job standard
   - All steps have logs
   - Completed/running steps MUST have logs
   - Log line numbering correct
   - Both steps auto-expand
   - UI log counts match API total_count

## Test Results

The test correctly identifies **pre-existing UI bugs**:
- Steps auto-expand ✓
- Step icons correct ✓
- WebSocket messages received ✓
- **Logs not displayed** - UI bug (empty_logs_section)

## Architecture Compliance

All requirements from docs/architecture/ verified:
- Uses WebSocket for real-time updates (QUEUE_UI.md)
- Checks step auto-expand behavior (QUEUE_UI.md)
- Verifies step icons match standard (QUEUE_UI.md)
- Validates log line numbering (QUEUE_LOGGING.md)
- Tracks API call count (QUEUE_UI.md)

## Files Changed

- `test/ui/job_definition_general_test.go`:
  - Added `TestJobDefinitionErrorGeneratorComprehensive` function
  - Added `checkStepExpansionStateForJob` helper function
  - Added imports for `encoding/json`, `strings`, `network`

## User Action Required

The test correctly identifies that expanded steps show `empty_logs_section` even when logs exist in the API. This is a pre-existing UI bug that needs investigation.

To run the test:
```bash
go test -v -timeout 6m -run "TestJobDefinitionErrorGeneratorComprehensive" ./test/ui/...
```
