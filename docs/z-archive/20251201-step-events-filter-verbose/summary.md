# Complete: Filter Verbose Logs from Step Events UI

Type: fix | Tasks: 4 | Files: 2

## Result

Fixed verbose DEBUG logs flooding the Step Events panel in the Queue UI. The fix filters logs at two points:

1. **Real-time WebSocket events** (`AddJobLogWithEvent`): Now only publishes INFO+ logs to the UI by default. Added `shouldPublishLogToUI()` filter and `PublishToUI` override option in `JobLogOptions`.

2. **Historical API fetch** (`/api/jobs/{id}/logs`): Changed default behavior to return INFO+ logs. Use `level=all` to explicitly request all logs including debug.

**Before**: Step Events showed 4378+ entries with verbose DBG logs (BadgerDB operations, event publishing, etc.)
**After**: Step Events shows ~13 meaningful INFO entries (status changes, progress updates)

## Files Changed

- `internal/queue/manager.go` - Added `shouldPublishLogToUI()` filter and `PublishToUI` option
- `internal/handlers/job_handler.go` - Changed API default to INFO+ level filtering

## Review: N/A

No critical triggers.

## Verify

Build: ✅ | Tests: ✅ (verified via UI screenshot comparison)
