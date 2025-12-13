# Task 4: Update websocket_job_events_test.go to verify context

Depends: 3 | Critical: no | Model: sonnet

## Addresses User Intent

Update test to verify WebSocket messages show proper `[step]` and `[worker]` context - User Intent #6

## Do

1. Update `test/api/websocket_job_events_test.go`:
   - Add verification that job_log events have `originator` field
   - Check that originator is "step" or "worker" (not empty)
   - Update log output to show full message content, not just "Received WebSocket message: type=step_progress"
   - Add assertions for originator field presence

2. Create or update test cases to verify:
   - Step logs have originator="step"
   - Worker logs have originator="worker"
   - No duplicate log messages for same event

## Accept

- [ ] Test verifies originator field in job_log events
- [ ] Test logs show actual message content
- [ ] Test checks for [step] and [worker] originator values
- [ ] Test compiles and runs
