# Task 6: Run tests and iterate to pass

Depends: 5 | Critical: no | Model: sonnet

## Addresses User Intent

Execute tests and iterate until they pass - User Intent #6 (complete verification)

## Do

1. Run `test/api/websocket_job_events_test.go`:
   - Execute all WebSocket event tests
   - Fix any failures related to originator/context verification
   - Iterate until pass

2. Run `test/ui/queue_test.go` -> TestStepEventsDisplay:
   - Execute UI test
   - Fix any failures related to log format/tags
   - Iterate until pass

3. Verify no regressions in other tests

## Accept

- [ ] websocket_job_events_test.go passes
- [ ] TestStepEventsDisplay passes
- [ ] No new test failures introduced
- [ ] Build succeeds
