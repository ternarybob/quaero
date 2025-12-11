# Task 5: Test end-to-end with running job
Depends: 4 | Critical: no | Model: sonnet | Skill: -

## Addresses User Intent
Validates the fix works correctly by running an actual job and verifying UI updates match backend state.

## Skill Patterns to Apply
N/A - no skill for this task

## Do
1. Build the application with changes
2. Start a multi-step job (e.g., codebase_assessment or similar)
3. Observe UI queue page:
   - Verify step status changes from pending → running → completed
   - Verify status indicators (colors/icons) update correctly
   - Verify no stuck "pending" status for completed steps
4. Check browser console for WebSocket messages
5. Verify no error toasts or console errors

## Accept
- [ ] Build succeeds with no errors
- [ ] Running job shows correct step transitions in UI
- [ ] Completed steps show green/completed status
- [ ] No status indicator mismatch between UI and backend
- [ ] Console shows job_update messages being received
