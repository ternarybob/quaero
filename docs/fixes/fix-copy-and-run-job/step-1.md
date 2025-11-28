# Step 1: Replace confirm() with Modal in rerunJob

- Task: task-1.md | Group: 1 | Model: sonnet

## Actions
1. Replaced native `confirm()` with `window.confirmAction()` in `rerunJob` function
2. Updated modal to show appropriate title ("Copy and Queue Job") and message
3. Changed confirmText to "Copy & Queue" with type "primary"
4. Updated notification message to indicate job will run shortly
5. Also updated `deleteJob` function to use modal for consistency

## Files
- `pages/queue.html` - Updated rerunJob (line ~1248-1258) and deleteJob (line ~1391-1412) functions

## Decisions
- Used `type: 'primary'` for rerunJob (positive action) vs `type: 'danger'` for deleteJob (destructive action)
- Message clarifies that job will execute when workers are available
- Moved button disable logic AFTER confirmation for deleteJob to avoid spinner showing during modal

## Verify
Compile: N/A (HTML/JS) | Tests: Pending (Task 3)

## Status: COMPLETE
