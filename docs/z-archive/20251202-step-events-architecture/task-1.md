# Task 1: Audit and fix crawler_worker direct publishing

Depends: - | Critical: yes:architectural-change | Model: opus

## Addresses User Intent

Ensures crawler worker does NOT publish events directly to WebSocket/UI. All events must flow through Job Manager.

## Do

1. Review crawler_worker.go for all direct EventService.Publish calls
2. Remove `publishCrawlerProgressUpdate()` direct publishing - use Job Manager's unified logging
3. Remove `publishJobSpawnEvent()` direct publishing - use Job Manager
4. Ensure all logging uses `logWithEvent()` which routes through `AddJobLogWithEvent`
5. Verify step context (step_name, manager_id) is properly set in all events

## Accept

- [ ] No direct `w.eventService.Publish()` calls in crawler_worker.go
- [ ] All events use Job Manager's `AddJobLogWithEvent` or similar unified methods
- [ ] Step context is included in all published events
- [ ] Code compiles without errors
