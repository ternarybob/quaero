# Task 1: Fix ExecuteJobDefinition to mark job failed when validation fails
Depends: - | Critical: no | Model: sonnet

## Do
- In `internal/queue/manager.go` ExecuteJobDefinition function:
  1. Track whether any validation errors occurred (even with on_error="continue")
  2. Track whether any child jobs were successfully created
  3. At the end of step processing, if validation errors occurred AND no children were created:
     - Set job status to "failed"
     - Set error message describing the validation failure

## Accept
- [ ] Job with validation failure and on_error="continue" shows "failed" status when no children created
- [ ] Error message includes the validation failure reason
- [ ] Build passes
