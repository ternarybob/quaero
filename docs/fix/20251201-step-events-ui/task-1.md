# Task 1: Move events panel from parent to step rows

Depends: - | Critical: no | Model: sonnet

## Do
- Remove events panel from parent job card (lines ~588-624)
- Add events panel to step row template (after step metadata)
- Filter logs by step_name: `getStepLogs(jobId, stepName)`
- Track expanded state per step: `expandedStepLogs[jobId:stepName]`

## Accept
- [x] Events panel appears under each step row
- [x] Logs are filtered to show only logs for that step
- [x] Build compiles without errors
