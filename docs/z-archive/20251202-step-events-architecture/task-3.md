# Task 3: Fix step_monitor event publishing to include step context

Depends: 1 | Critical: yes:architectural-change | Model: opus

## Addresses User Intent

Step Manager must properly tag all events with step_name so UI can filter correctly. This is the core issue causing events to appear in wrong step panels.

## Do

1. Review step_monitor.go `publishStepProgress()` method
2. Ensure ALL events published include `step_name` field
3. Verify the step_name is correctly passed to WebSocket handlers
4. Check that step completion messages include step_name
5. Audit all event payloads to ensure step context is present

## Accept

- [ ] All step_progress events include step_name in payload
- [ ] Step completion messages are tagged with correct step_name
- [ ] Events can be filtered by step_name in WebSocket output
- [ ] Code compiles without errors
