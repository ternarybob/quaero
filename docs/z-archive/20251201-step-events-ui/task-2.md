# Task 2: Auto-expand events for running/failed steps

Depends: 1 | Critical: no | Model: sonnet

## Do
- Auto-expand events panel when step status is 'running' or 'failed'
- Initialize expandedStepLogs state based on step status
- Update expansion state when step status changes via WebSocket

## Accept
- [x] Events panel auto-expands for running steps
- [x] Events panel auto-expands for failed steps
- [x] Build compiles without errors
