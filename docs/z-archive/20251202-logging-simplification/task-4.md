# Task 4: Update monitors to use simplified AddJobLog

Depends: 2 | Critical: no | Model: sonnet

## Addresses User Intent
Ensure monitors work correctly with the simplified logging API.

## Do
Review and verify these files still work:
- state/monitor.go - already uses `AddJobLog`, should work unchanged
- state/step_monitor.go - already uses `AddJobLog`, should work unchanged
- state/runtime.go - already uses `AddJobLog`, should work unchanged

No code changes expected - this is a verification task.

## Accept
- [ ] Monitors compile without errors
- [ ] Monitors use `AddJobLog` correctly
