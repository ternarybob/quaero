# Task 2: Verify step_progress events include correct step_id
Depends: 1 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Ensure the WebSocket events contain the correct step_id for the unified log aggregator to trigger UI refresh.

## Skill Patterns to Apply
- Review existing event payload structure

## Do
1. Verify `publishStepProgress` sends correct `step_id` in payload (already does - line 319)
2. Verify `TriggerStepImmediately` receives correct `stepID` from WebSocket handler

## Accept
- [ ] `step_progress` events contain correct `step_id`
- [ ] No changes needed if already correct
