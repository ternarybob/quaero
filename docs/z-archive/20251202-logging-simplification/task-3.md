# Task 3: Update workers to use simplified AddJobLog

Depends: 2 | Critical: no | Model: sonnet

## Addresses User Intent
Workers should just call `AddJobLog(jobID, level, message)` without building options.

## Do
Update each worker file:

### agent_worker.go
- Remove `buildJobLogOptions()` method
- Replace `AddJobLogWithEvent(jobID, level, msg, opts)` → `AddJobLog(jobID, level, msg)`
- Remove `publishAgentJobLog` helper or simplify it to just call AddJobLog
- Remove imports of `queue.JobLogOptions`

### crawler_worker.go
- Remove `buildJobLogOptions()` method
- Remove `logWithEvent()` helper method
- Replace all `AddJobLogWithEvent` calls → `AddJobLog`
- Simplify `publishCrawlerProgressUpdate` and similar helpers
- Remove imports of `queue.JobLogOptions`

### github_log_worker.go
- Remove `buildJobLogOptions()` method
- Replace `AddJobLogWithEvent` → `AddJobLog`
- Remove `queue.JobLogOptions` usage

### github_repo_worker.go
- Remove `buildJobLogOptions()` method
- Replace `AddJobLogWithEvent` → `AddJobLog`
- Remove `queue.JobLogOptions` usage

### places_worker.go
- Replace `AddJobLogWithEvent` → `AddJobLog`
- Remove `queue.JobLogOptions` usage

### web_search_worker.go
- Replace `AddJobLogWithEvent` → `AddJobLog`
- Remove `queue.JobLogOptions` usage

### database_maintenance_worker.go
- Already uses `AddJobLog` - no changes needed

## Accept
- [ ] All workers compile without errors
- [ ] All workers use only `AddJobLog` (no `AddJobLogWithEvent`)
- [ ] No `buildJobLogOptions` methods remain
- [ ] No `logWithEvent` helpers remain
- [ ] No `queue.JobLogOptions` references remain
