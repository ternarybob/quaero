# Step 1: Standardize Queue Workers

## Task Reference
- **Task File:** task-1.md
- **Group:** 1 (sequential)
- **Dependencies:** none

## Params
- Sandbox: `/tmp/3agents/task-1/`
- Source: `C:/development/quaero/`
- Output: `C:/development/quaero/docs/features/20251127-logging-standardization/`

## Actions Taken
1. Reviewed all 5 queue worker files
2. Found `job_processor.go` was already correctly using Info for start/stop, Debug for registration, Trace for processing
3. Found `agent_worker.go` was already correctly using Debug for job execution start/end, Trace for detailed steps
4. Found `github_log_worker.go` was already correctly using Debug for job processing
5. Found `database_maintenance_worker.go` was already correctly using Debug for operations
6. Fixed `crawler_worker.go` - changed 2 Info logs (limit messages) to Debug

## Files Modified
- `internal/queue/workers/crawler_worker.go` - Changed Info to Debug for:
  - Line 350-353: "Reached max pages limit" - interim status, not significant event
  - Line 395-399: "Reached maximum depth" - interim status, not significant event
  - Also updated matching publishCrawlerJobLog calls to use "debug" level

## Decisions Made
- **Keep existing patterns**: The workers were already mostly well-structured
- **Info reserved for**: Job start (handled by AddJobLog), job completion summaries
- **Debug for**: All interim status updates including limit reached messages

## Acceptance Criteria
- [x] job_processor.go uses Info only for processor start/stop
- [x] crawler_worker.go uses Info only for job start/end (via AddJobLog)
- [x] agent_worker.go uses Info only for job start/end
- [x] github_log_worker.go uses Info only for job start/end
- [x] database_maintenance_worker.go uses Info only for operation start/end
- [x] All detailed tracing moved from Debug to Trace (already in place)
- [ ] Compiles successfully (will verify in Phase 3)

## Verification
To be completed in Phase 3

## Output for Dependents
Established pattern:
- Info: Only AddJobLog for significant user-facing events
- Debug: Interim status, progress updates, condition checks
- Trace: Detailed internal tracing, function entry/exit

## Status: COMPLETE
