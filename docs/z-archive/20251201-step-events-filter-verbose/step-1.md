# Step 1: Add log level filter in AddJobLogWithEvent

Model: sonnet | Status: ✅

## Done

- Added `shouldPublishLogToUI` function to filter logs by level
- Only INFO, WARN, ERROR, FATAL, PANIC levels are published to UI
- DEBUG, TRACE, DBG, TRC levels are filtered out by default
- DB storage (AppendLog) continues to receive all logs regardless of level

## Files Changed

- `internal/queue/manager.go` - Added shouldPublishLogToUI function and modified AddJobLogWithEvent to use it

## Verify

Build: ✅ | Tests: ⏭️
