# Step 4: Fix circular logging loop in refresh triggers
Model: opus | Skill: go | Status: Done

## Root Cause Found
The infinite loop was caused by:
1. `broadcastLogsRefreshTrigger()` logs "Broadcast refresh_logs trigger" at Debug level
2. This Debug log goes to arbor channel → LogConsumer
3. LogConsumer publishes `log_event`
4. `log_event` triggers `RecordEvent()` → `hasPendingLogs = true`
5. 1 second later, aggregator sends `refresh_logs` again
6. Loop repeats infinitely

## Done
- Removed Debug log from `broadcastLogsRefreshTrigger()` (websocket.go:245-247)
- Removed Debug log from `broadcastStepRefreshTrigger()` (websocket.go:203-206)
- Removed Trace and Debug logs from `flushPending()` (log_aggregator.go:112, 120)
- Added comments explaining why logging is not allowed in these functions

## Files Changed
- `internal/handlers/websocket.go` - Removed logging from broadcast functions
- `internal/services/events/log_aggregator.go` - Removed logging from flushPending
- `internal/logs/consumer.go` - Updated comment to clarify skip behavior

## Skill Compliance (go)
- [x] Identified circular dependency in logging
- [x] Comments explain the constraint

## Build Check
Build: Pass | Tests: Skipped
