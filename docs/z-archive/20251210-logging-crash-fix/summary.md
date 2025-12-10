# Complete: Logging Crash and TRACE Level Fix
Type: fix | Tasks: 3 | Files: 4

## User Request
"1. Service crashed, without any logging generated. 2. Appears trace logs are marked as INFO. This should NOT be the case. Clean contextual logging is required."

## Result
Fixed two logging issues: (1) Changed TRACE-prefixed log calls from `.Info()` to `.Debug()` level for clean production logs, and (2) Updated panic recovery handlers to use `common.WriteCrashFile()` instead of `.Fatal()` to ensure crash diagnostics are reliably written to disk before process termination.

## Skills Used
- go

## Validation: MATCHES
All success criteria met - TRACE logs use correct level, crash files written on panic, build passes.

## Review: N/A
No critical triggers.

## Verify
Build: Pass | Tests: Pass

## Files Changed
- `internal/queue/workers/job_processor.go` - Debug log level + crash file panic recovery
- `internal/queue/workers/crawler_worker.go` - Debug log level
- `internal/queue/state/monitor.go` - Crash file panic recovery
- `internal/queue/state/step_monitor.go` - Crash file panic recovery
