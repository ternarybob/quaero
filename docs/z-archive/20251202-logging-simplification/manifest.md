# Fix: Simplify Worker and Step Manager Logging

- Slug: logging-simplification | Type: fix | Date: 2025-12-02
- Request: "The worker and step manager logging should be simple. I don't think there is a reason to separate events and logging. Implement Option A - Single method that auto-resolves context."
- Prior: none

## User Intent
Simplify the logging API by merging `AddJobLog` and `AddJobLogWithEvent` into a single method that:
1. Always stores logs to the database
2. Always publishes logs to WebSocket for real-time UI updates
3. Auto-resolves step context (stepName, managerID) from the job's parent chain
4. Workers just call one simple method without choosing or passing options

## Success Criteria
- [ ] Single `AddJobLog` method replaces both `AddJobLog` and `AddJobLogWithEvent`
- [ ] No `JobLogOptions` struct needed - context is auto-resolved
- [ ] All workers updated to use the simplified API
- [ ] Logs appear in both DB and WebSocket UI
- [ ] Build passes
- [ ] Tests pass
