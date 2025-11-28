# Plan: Fix Copy and Run Job Modal and Execution

## Analysis

### Current State
1. **Popup Issue**: The "Copy and queue job" button on the Queue page uses a browser `confirm()` dialog instead of the existing modal component.
2. **Job Not Running Issue**: The `RerunJob` function in `internal/services/crawler/service.go` saves the job to storage but does NOT enqueue it to the queue for processing, so the job remains in "pending" status forever.

### Dependencies
- `pages/queue.html` - Contains the `rerunJob()` function using native `confirm()`
- `pages/static/common.js` - Contains the `window.confirmAction()` modal helper already in use for `cancelJob`
- `internal/services/crawler/service.go` - Contains the `RerunJob` function that needs to enqueue the job
- `internal/handlers/job_handler.go` - Handler calls `RerunJob`
- `test/ui/queue_test.go` - Existing UI tests

### Approach
1. **Modal Fix**: Replace the native `confirm()` call in `rerunJob()` with `window.confirmAction()` (already exists)
2. **Execution Fix**: Modify `RerunJob` to enqueue the job after saving it, using the existing queue infrastructure
3. **Tests**: Add new tests for the modal confirmation and job execution

### Risks
- Low: The modal component already works (used by `cancelJob`)
- Medium: Enqueue logic needs to match the existing job service pattern

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Replace confirm() with modal in rerunJob | none | no | low | sonnet |
| 2 | Fix RerunJob to enqueue job for execution | none | no | medium | sonnet |
| 3 | Add tests for modal and job running | 1, 2 | no | medium | sonnet |

## Order
Sequential: [1] -> [2] -> [3] -> Validate -> Review (if needed) -> Summary
