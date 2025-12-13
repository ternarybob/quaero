# Plan: WebSocket Job Logging Consistency

Type: fix | Workdir: docs/fix/20251203-websocket-logging-consistency/

## User Intent (from manifest)

1. All job logs should have consistent context tags: `[step]` for step manager logs, `[worker]` for worker logs
2. Fix duplicate log entries (e.g., "Starting 2 workers..." and "Step finished successfully" appearing twice)
3. Update log format to: `[time] [level] [context] message`
4. Replace emoji log levels with standard tags: `[INF]`, `[DBG]`, `[WRN]`, `[ERR]`
5. Match colors from Service Logs, maintain white/transparent background
6. Update tests to verify WebSocket messages show proper context

## Architecture Analysis

**Current state:**
- Logs stored with `originator` field ("manager", "step", "worker") as metadata
- UI displays `[originator]` tag from metadata
- Emoji prefixes in message text (✓, ✗, ▶)
- Duplicate logs from: StepMonitor calling publishStepLog + UpdateJobStatus also logging

**Target state:**
- Consistent `[step]` and `[worker]` tags from originator field
- Standard text level tags: `[INF]`, `[DBG]`, `[WRN]`, `[ERR]`
- No duplicate log entries
- Format: `[HH:MM:SS] [LVL] [context] message`

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Identify and fix duplicate log sources in StepMonitor | - | no | sonnet |
| 2 | Replace emoji prefixes with clean messages in crawler_worker.go | 1 | no | sonnet |
| 3 | Update queue.html UI to format logs with [LVL] text tags and proper styling | 2 | no | sonnet |
| 4 | Update websocket_job_events_test.go to verify [step] and [worker] context | 3 | no | sonnet |
| 5 | Update queue_test.go TestStepEventsDisplay to verify log formatting | 4 | no | sonnet |
| 6 | Run tests and iterate to pass | 5 | no | sonnet |

## Order

[1] → [2] → [3] → [4] → [5] → [6]

## Key Files

- `internal/queue/state/step_monitor.go` - publishStepLog, duplicate sources
- `internal/queue/workers/crawler_worker.go` - emoji replacements
- `pages/queue.html` - UI log formatting
- `test/api/websocket_job_events_test.go` - API test updates
- `test/ui/queue_test.go` - UI test updates
