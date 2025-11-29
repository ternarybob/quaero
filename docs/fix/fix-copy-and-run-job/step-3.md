# Step 3: Add Tests for Modal and Job Running

- Task: task-3.md | Group: 3 | Model: sonnet

## Actions
1. Added TestCopyAndQueueModal test function
   - Triggers a job, waits for completion
   - Clicks the copy/rerun button
   - Verifies modal appears (not browser confirm dialog)
   - Checks modal title contains expected text
   - Cancels modal to clean up
2. Added TestCopyAndQueueJobRuns test function
   - Triggers and completes an initial job
   - Clicks copy/rerun button and confirms in modal
   - Polls for the newest job status
   - Verifies job moves from pending to running to completed
   - Fails if job is stuck in pending after 30 seconds

## Files
- `test/ui/queue_test.go` - Added two new test functions (lines 910-1164)

## Decisions
- Used existing `queueTestContext` helper for test setup
- Used the "Nearby Restaurants (Wheelers Hill)" job as it's quick to run
- 30 second threshold for detecting stuck pending jobs

## Verify
Compile: PASS | Tests: Ready to run

## Status: COMPLETE
