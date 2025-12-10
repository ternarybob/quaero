# Plan: Fix Logging Crash and TRACE Level Issues
Type: fix | Workdir: ./docs/fix/20251210-logging-crash-fix/

## User Intent (from manifest)
Fix two logging-related issues:
1. **Silent crash** - Service crashes without generating any log output
2. **TRACE logs at wrong level** - Debug/trace statements using `.Info()` instead of `.Trace()`

## Active Skills
- go

## Analysis

### Issue 1: TRACE logs at INFO level
Found in codebase:
- `internal/queue/workers/job_processor.go:401-410` - "TRACE: About to call worker.Execute" and "TRACE: worker.Execute returned" using `.Info()`
- `internal/queue/workers/crawler_worker.go:332-335` - "TRACE: CrawlerWorker.Execute called" using `.Info()`

### Issue 2: Silent Crash
Panic recovery in multiple locations uses `.Fatal()` which may not flush before `os.Exit()`:
- `internal/queue/workers/job_processor.go:178` - processJobs goroutine
- `internal/queue/state/monitor.go:71` - Job monitor goroutine
- `internal/queue/state/step_monitor.go:57` - Step monitor goroutine

The arbor logger's `.Fatal()` may call `os.Exit(1)` before the log output is flushed to disk.

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Fix TRACE logs to use correct log level | - | no | sonnet | go |
| 2 | Fix panic recovery to ensure log flush before exit | 1 | no | sonnet | go |
| 3 | Build and verify | 2 | no | sonnet | go |

## Order
[1] → [2] → [3]
