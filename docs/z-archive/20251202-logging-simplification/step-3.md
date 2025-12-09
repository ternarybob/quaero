# Step 3: Update workers to use simplified AddJobLog

Model: sonnet | Status: ✅

## Done
Updated all workers to use simplified `AddJobLog`:

### agent_worker.go
- Removed `buildJobLogOptions()` method
- Removed `publishAgentJobLog()` helper
- Removed `publishJobError()` helper
- Removed unused `stepName`, `managerID` variables
- Replaced all `AddJobLogWithEvent` calls → `AddJobLog`

### crawler_worker.go
- Removed `buildJobLogOptions()` method
- Removed `logWithEvent()` helper
- Removed `publishLinkDiscoveryEvent()` helper
- Removed `publishJobSpawnEvent()` helper
- Simplified `publishCrawlerProgressUpdate()` to use `AddJobLog`
- Replaced all `AddJobLogWithEvent` calls → `AddJobLog`

### github_log_worker.go
- Removed `buildJobLogOptions()` method
- Simplified `logDocumentSaved()` to use `AddJobLog`

### github_repo_worker.go
- Removed `buildJobLogOptions()` method
- Simplified `logDocumentSaved()` to use `AddJobLog`

### places_worker.go
- Simplified `logJobEvent()` to use `AddJobLog`
- Removed inline `JobLogOptions` usage

### web_search_worker.go
- Simplified `logJobEvent()` to use `AddJobLog`
- Removed inline `JobLogOptions` usage

### database_maintenance_worker.go
- Already uses `AddJobLog` - no changes needed

## Files Changed
- `internal/queue/workers/agent_worker.go` - Removed ~60 lines
- `internal/queue/workers/crawler_worker.go` - Removed ~90 lines
- `internal/queue/workers/github_log_worker.go` - Removed ~30 lines
- `internal/queue/workers/github_repo_worker.go` - Removed ~30 lines
- `internal/queue/workers/places_worker.go` - Simplified ~15 lines
- `internal/queue/workers/web_search_worker.go` - Simplified ~15 lines

## Build Check
Build: ✅ | Tests: ⏭️
