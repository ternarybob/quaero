# Task 1: Update StepMonitor.publishStepLog to store logs under stepID
Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Step logs must be stored under the step job ID so the UI can fetch them when displaying step events.

## Skill Patterns to Apply
- Error handling: wrap errors with context
- Structured logging: use arbor with key-value pairs

## Do
1. Modify `publishStepLog` function signature to accept `stepID` parameter
2. Change `AddJobLogWithContext` call to use `stepID` instead of `managerID`
3. Update all callers of `publishStepLog` to pass the `stepID`

## Accept
- [ ] `publishStepLog` accepts and uses `stepID` for log storage
- [ ] All callers in `monitorStepChildren` pass correct `stepID`
- [ ] Code compiles without errors
