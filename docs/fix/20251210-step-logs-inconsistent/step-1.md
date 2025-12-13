# Step 1: Update StepMonitor.publishStepLog to store logs under stepID
Model: sonnet | Skill: go | Status: ✅

## Done
- Changed `publishStepLog` first parameter from `managerID` to `stepID`
- Updated function comment to clarify logs are stored under step job ID
- Updated all 4 callers to pass `stepJob.ID` instead of `managerID`

## Files Changed
- `internal/queue/state/step_monitor.go` - Modified publishStepLog signature and all callers

## Skill Compliance
- [x] Error handling preserved (wrapping with context)
- [x] Structured logging preserved (arbor key-value pairs)
- [x] No anti-patterns introduced

## Build Check
Build: ⏳ (pending) | Tests: ⏳ (pending)
