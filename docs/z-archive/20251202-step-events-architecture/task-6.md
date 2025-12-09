# Task 6: Create integration test for WebSocket event routing

Depends: 5 | Critical: no | Model: sonnet

## Addresses User Intent

Create a new API test that starts a job, monitors WebSocket output, and verifies events have correct step context.

## Do

1. Create new test file `test/api/websocket_step_events_test.go`
2. Test should:
   - Start a multi-step job
   - Connect to WebSocket
   - Collect all events during job execution
   - Verify each event has correct step_name
   - Verify events from step A don't have step B's name
   - Verify step completion events are properly tagged
3. Test architecture compliance:
   - Verify no events have empty step_name when they should have one
   - Verify step_progress events include step_name
   - Verify job_log events include step context

## Accept

- [ ] New test file exists at test/api/websocket_step_events_test.go
- [ ] Test verifies events have correct step_name
- [ ] Test verifies events are properly segregated by step
- [ ] Test passes or documents current issues
