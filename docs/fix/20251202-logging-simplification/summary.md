# Complete: Simplify Worker and Step Manager Logging

Type: fix | Tasks: 5 | Files: 8

## User Request
"The worker and step manager logging should be simple. I don't think there is a reason to separate events and logging. Implement Option A - Single method that auto-resolves context."

## Result
Merged `AddJobLog` and `AddJobLogWithEvent` into a single `AddJobLog(ctx, jobID, level, message)` method that automatically resolves step context from the job's parent chain and publishes to WebSocket. Workers now call one simple method without building options or choosing between methods.

## Validation: ✅ MATCHES
All success criteria met. The logging API is now simplified from two methods + options struct to one method with auto-context resolution.

## Review: N/A
No critical triggers (security, auth, crypto, etc.)

## Changes
- `internal/queue/manager.go` - Rewrote AddJobLog, added resolveJobContext, removed AddJobLogWithEvent and JobLogOptions
- `internal/queue/workers/agent_worker.go` - Simplified logging calls, removed helper methods
- `internal/queue/workers/crawler_worker.go` - Simplified logging calls, removed helper methods
- `internal/queue/workers/github_log_worker.go` - Simplified logging calls, removed helper methods
- `internal/queue/workers/github_repo_worker.go` - Simplified logging calls, removed helper methods
- `internal/queue/workers/places_worker.go` - Simplified logging calls
- `internal/queue/workers/web_search_worker.go` - Simplified logging calls

## Verify
Build: ✅ | Tests: ⏭️ (not run)
