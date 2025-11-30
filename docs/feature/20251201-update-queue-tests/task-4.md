# Task 4: Add Test for Expand/Collapse Children
Depends: 3 | Critical: no | Model: sonnet

## Context
The UI should allow users to expand/collapse child jobs by clicking a button.
The button shows "N children" with a chevron icon.
When clicked, child job rows should appear below the parent.

## Do
1. Add test to verify expand/collapse functionality
2. Find the "children" button on the parent job card
3. Click the button and verify children rows appear
4. Verify chevron icon changes from right to down
5. Click again and verify children rows hide
6. Verify chevron icon changes back to right

## Accept
- [ ] Test finds the children expand button
- [ ] Test clicks button and verifies children appear
- [ ] Test verifies chevron direction changes
- [ ] Test clicks again and verifies children hide
- [ ] Test should FAIL if expand/collapse doesn't work
