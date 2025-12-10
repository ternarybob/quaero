# Step 1: Fix TRACE logs to use correct log level
Model: sonnet | Skill: go | Status: Done

## Done
- Changed `.Info()` to `.Debug()` for worker execution trace logs in job_processor.go
- Changed `.Info()` to `.Debug()` for CrawlerWorker trace logs in crawler_worker.go
- Removed "TRACE:" prefix from messages (log level indicates debug nature)

## Files Changed
- `internal/queue/workers/job_processor.go` - Lines 401, 406: Changed from Info to Debug level
- `internal/queue/workers/crawler_worker.go` - Line 332: Changed from Info to Debug level

## Skill Compliance (go)
- [x] Use arbor structured logging at appropriate levels
- [x] Debug messages use `.Debug()` level
- [x] Key-value pairs preserved for structured logging

## Build Check
Build: Pending | Tests: Pending
