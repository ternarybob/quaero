# Step 1: Implementation

Iteration: 1 | Status: complete

## Changes Made

| File | Action | Description |
|------|--------|-------------|
| `test/ui/job_definition_general_test.go` | modified | Added `TestJobDefinitionErrorGeneratorComprehensive` function |
| `test/ui/job_definition_general_test.go` | modified | Added imports for `encoding/json`, `strings`, `network` |
| `test/ui/job_definition_general_test.go` | modified | Added `checkStepExpansionStateForJob` helper function |

## Implementation Details

### New Test: `TestJobDefinitionErrorGeneratorComprehensive`

Replicates the comprehensive assertions from `job_definition_codebase_classify_test.go`:

1. **Two error_generator steps** with different names:
   - `step_one_generate` (10% failure rate)
   - `step_two_generate` (20% failure rate)

2. **Real-time monitoring via WebSocket** (NO page refresh):
   - Uses `network.Enable()` to track WebSocket frames
   - Tracks `refresh_logs` messages via `WebSocketMessageTracker`
   - Tracks API calls via `APICallTracker`

3. **5-minute timeout** with terminal state wait:
   - `jobTimeout := 5 * time.Minute`
   - Monitors until `completed`, `failed`, or `cancelled`

4. **Status change screenshots** (not polling-based):
   - Screenshots on every status change
   - Additional screenshots every 30 seconds during monitoring

5. **API vs UI consistency assertions every 30 seconds**:
   - `assertAPIParentJobStatusMatchesUI`
   - `assertAPIStepStatusesMatchUI`

### Assertions Replicated

| # | Assertion | Source |
|---|-----------|--------|
| 0 | Progressive log updates within first 30s | codebase_classify_test.go |
| 1 | WebSocket refresh_logs messages < 40 | codebase_classify_test.go |
| 1b | /api/logs calls gated by refresh triggers | codebase_classify_test.go |
| 2 | Step icons match parent job standard | codebase_classify_test.go |
| 3 | All steps have logs | codebase_classify_test.go |
| 3b | Completed/running steps MUST have logs | codebase_classify_test.go |
| 4 | Log line numbering correct (starts at 1) | codebase_classify_test.go |
| 5 | Both steps auto-expanded | NEW (for two-step job) |
| 6 | UI log counts match API total_count | codebase_classify_test.go |
| 7 | Job reached terminal state | codebase_classify_test.go |

## Build & Test

Build: Pass
Tests: Not yet run

## Architecture Compliance (self-check)

- [x] Uses WebSocket for real-time updates (QUEUE_UI.md - WebSocket Events)
- [x] Checks step auto-expand (QUEUE_UI.md - Step Expansion)
- [x] Verifies step icons match standard (QUEUE_UI.md - Icon Standards)
- [x] Checks log line numbering (QUEUE_LOGGING.md - Log Line Numbering)
- [x] Tracks API call count for logs (QUEUE_UI.md - API Calls < 10)
