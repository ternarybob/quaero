# Task 1: Fix TRACE logs to use correct log level
Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Fixes "TRACE logs are marked as INFO" - ensures debug/trace statements use appropriate log levels so production logs remain clean.

## Skill Patterns to Apply
- Use arbor structured logging at appropriate levels
- `.Trace()` for internal diagnostics that are rarely needed
- `.Debug()` for development debugging
- `.Info()` only for significant operational events

## Do
1. Change `job_processor.go:401` from `.Info()` to `.Debug()` for "TRACE: About to call worker.Execute"
2. Change `job_processor.go:406` from `.Info()` to `.Debug()` for "TRACE: worker.Execute returned"
3. Change `crawler_worker.go:332` from `.Info()` to `.Debug()` for "TRACE: CrawlerWorker.Execute called"
4. Remove "TRACE:" prefix from messages since log level indicates trace/debug nature

## Accept
- [ ] No "TRACE:" messages at INFO level
- [ ] Debug messages use `.Debug()` or `.Trace()` level
- [ ] Code compiles without errors
