# Plan: GitHub Job Owner Validation and Status Fix
Type: fix | Workdir: ./docs/fix/20251201-github-job-owner-validation/

## Analysis
1. **Owner is required**: The `github_repo` step requires `owner` field - this is correct behavior
2. **Status bug**: When `on_error="continue"` and validation fails, the job is logged as error but continues without setting job status to "failed". When no more steps exist, the job ends up as "completed" even though validation failed.

## Root Cause
In `internal/queue/manager.go` lines 1091-1100:
- When validation fails with `on_error="continue"`, the code logs the error but just `continue`s to next step
- If it was the only/last step, no child jobs are created, parent job status remains "running"
- JobMonitor later sees no children and marks it "completed"

## Solution
When step validation fails, even with `on_error="continue"`:
1. If NO child jobs were created for ANY step, the parent job should be marked as "failed"
2. Track whether any steps successfully created children
3. At end of ExecuteJobDefinition, if no children created and validation errors occurred, mark as failed

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Fix ExecuteJobDefinition to track validation failures and mark job failed if no children created | - | no | sonnet |
| 2 | Add owner field to test connector config for completeness | 1 | no | sonnet |

## Order
[1] → [2]
