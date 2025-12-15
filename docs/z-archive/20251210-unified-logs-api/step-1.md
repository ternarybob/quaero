# Step 1: Extend UnifiedLogsHandler for direct job logs
Model: sonnet | Skill: go | Status: ✅

## Done
- Added `size` parameter alias for `limit` in query parsing
- Added fast path for `include_children=false` that uses direct `GetLogs`/`GetLogsByLevel` instead of expensive `GetAggregatedLogs`
- Added `models` import for `JobLogEntry` type
- Response includes all log fields: timestamp, full_timestamp, level, message, job_id, step_name, source_type, originator, phase

## Files Changed
- `internal/handlers/unified_logs_handler.go` - Added size alias, fast path for direct job logs

## Skill Compliance (go)
- [x] Context passed to service calls
- [x] Structured logging with arbor
- [x] Error handling with informative messages
- [x] No business logic in handler - delegates to service

## Build Check
Build: ⏳ | Tests: ⏭️
