# Step 1: Refactor GetJobTreeHandler

Model: sonnet | Skill: go | Status: Completed

## Done

- Refactored `GetJobTreeHandler` in `internal/handlers/job_handler.go`:
  - Now uses `step_definitions` from parent job metadata as source of truth
  - For each step definition, finds matching step job by `step_name`
  - Uses step job's own status directly (fixes running icon bug)
  - Gets grandchildren of step jobs for ChildSummary counts
  - Fetches logs per step job (not duplicated across all steps)

- Added `buildStepsFromStepJobs` helper function:
  - Fallback when `step_definitions` is not available
  - Iterates step jobs directly, uses their status
  - Gets grandchildren for each step job

## Files Changed

- `internal/handlers/job_handler.go` - Refactored GetJobTreeHandler, added buildStepsFromStepJobs helper

## Skill Compliance

- [x] Proper error handling with logging
- [x] Type assertions with ok check
- [x] Context passed through all operations
- [x] N/A - No new interfaces/structs needed

## Build Check

Build: Passed | Tests: Skipped (manual verification recommended)
