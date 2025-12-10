# Task 2: Fix panic recovery to ensure log flush before exit
Depends: 1 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Fixes "Service crashed without any logging generated" - ensures panic recovery logs are flushed to output before the process terminates.

## Skill Patterns to Apply
- Use arbor structured logging
- Error handling must not swallow crashes silently
- Context should be preserved in error messages

## Do
1. In `job_processor.go` panic recovery (line 176-184):
   - Change `.Fatal()` to `.Error()` to avoid immediate exit
   - Add explicit `os.Stderr.Sync()` or similar to flush output
   - Then call `os.Exit(1)` or re-panic after flush

2. In `monitor.go` panic recovery (line 67-77):
   - Same pattern: `.Error()` + flush + exit

3. In `step_monitor.go` panic recovery (line 53-63):
   - Same pattern: `.Error()` + flush + exit

## Accept
- [ ] Panic in processJobs produces visible log output
- [ ] Panic in job monitor produces visible log output
- [ ] Panic in step monitor produces visible log output
- [ ] Log messages include stack trace and context
- [ ] Code compiles without errors
