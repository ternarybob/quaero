# Step 2: Fix panic recovery to ensure log flush before exit
Model: sonnet | Skill: go | Status: Done

## Done
- Changed `jp.logger.Fatal()` to `jp.logger.Error()` + `common.WriteCrashFile()` in job_processor.go
- Changed `m.logger.Fatal()` to `m.logger.Error()` + `common.WriteCrashFile()` in monitor.go
- Changed `m.logger.Fatal()` to `m.logger.Error()` + `common.WriteCrashFile()` in step_monitor.go
- Added import for `github.com/ternarybob/quaero/internal/common` in all three files
- Added import for `os` in job_processor.go for `os.Exit(1)`

## Files Changed
- `internal/queue/workers/job_processor.go` - Lines 178-194: Updated panic recovery to use crash file
- `internal/queue/state/monitor.go` - Lines 68-86: Updated panic recovery to use crash file
- `internal/queue/state/step_monitor.go` - Lines 54-72: Updated panic recovery to use crash file

## Skill Compliance (go)
- [x] Use arbor structured logging (Error level for crash logging)
- [x] Wrap errors with context (crash files include full stack trace)
- [x] Don't panic on errors - use proper recovery
- [x] Constructor injection preserved

## Build Check
Build: Pending | Tests: Pending
