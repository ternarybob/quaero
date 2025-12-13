# Step 2: Remove AddJobLogWithEvent and JobLogOptions

Model: sonnet | Status: ✅

## Done
- Deleted `JobLogOptions` struct
- Deleted `AddJobLogWithEvent` method
- Deleted old `shouldPublishLogToUI` function (replaced with `shouldPublishLogLevel`)
- All functionality now consolidated in `AddJobLog`

## Files Changed
- `internal/queue/manager.go` - Removed ~100 lines of obsolete code

## Build Check
Build: ✅ | Tests: ⏭️
