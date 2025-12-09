# Step 1: Modify Manager.AddJobLog to auto-resolve context and publish events

Model: sonnet | Status: ✅

## Done
- Modified `AddJobLog` in `manager.go` to:
  1. Store log to database (existing behavior)
  2. Auto-resolve step context using new `resolveJobContext` helper
  3. Publish `EventJobLog` to WebSocket for INFO+ levels
- Added `resolveJobContext` helper that:
  - Gets job from storage
  - Extracts stepName, managerID from job metadata
  - Falls back to parent chain resolution if not in metadata
  - Returns sourceType from job type
- Added `shouldPublishLogLevel` function (simplified from old `shouldPublishLogToUI`)

## Files Changed
- `internal/queue/manager.go` - Rewrote AddJobLog method (lines 660-815)

## Build Check
Build: ✅ | Tests: ⏭️
