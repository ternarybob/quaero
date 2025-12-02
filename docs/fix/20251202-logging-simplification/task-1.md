# Task 1: Modify Manager.AddJobLog to auto-resolve context and publish events

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
This is the core change that merges logging and events into a single method with auto-context resolution.

## Do
1. Add `resolveJobContext` helper method to Manager that:
   - Takes a jobID
   - Looks up job from storage
   - Resolves stepName, managerID, parentID from job metadata or parent chain
   - Caches results if needed (consider memoization for repeated lookups)

2. Modify `AddJobLog` to:
   - Store log to DB (keep existing behavior)
   - Call `resolveJobContext` to get step context
   - Publish `EventJobLog` to WebSocket if level >= INFO
   - Keep existing level filtering logic from `shouldPublishLogToUI`

3. Move `shouldPublishLogToUI` logic inline (simplify since we no longer need opts parameter)

## Accept
- [ ] `AddJobLog` stores logs to database
- [ ] `AddJobLog` auto-resolves step context from job hierarchy
- [ ] `AddJobLog` publishes to WebSocket for INFO/WARN/ERROR levels
- [ ] DEBUG/TRACE logs are stored but not published to WebSocket
- [ ] No breaking changes to existing callers
