# Step 2: Fix RerunJob to Enqueue Job for Execution

- Task: task-2.md | Group: 2 | Model: sonnet

## Actions
1. Added queue enqueue logic after saving job in RerunJob function
2. Serialize job state to JSON for queue payload
3. Create QueueMessage with job ID, type, and payload
4. Call queueManager.Enqueue() to add job to processing queue
5. Added graceful error handling - job is still saved even if enqueue fails
6. Updated logging from Debug to Info for successful enqueue

## Files
- `internal/services/crawler/service.go` - Added enqueue logic at line ~1054-1083

## Decisions
- Return success even if enqueue fails (job is saved, can be manually triggered later)
- Use Info level logging for successful enqueue to help with debugging
- Use s.ctx (service context) for enqueue call to ensure proper cancellation handling

## Verify
Compile: PASS | Tests: Pending (Task 3)

## Status: COMPLETE
