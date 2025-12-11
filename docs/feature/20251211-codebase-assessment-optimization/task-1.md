# Task 1: Refactor GetJobTreeHandler

Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent

Fixes "Steps Not Showing" and "Running Icon Bug" issues. The tree view should display all 3 steps with correct status icons.

## Skill Patterns to Apply

- Go patterns for handler refactoring
- Proper error handling with wrapped errors
- Efficient database queries (batch fetch children)

## Do

1. Modify `GetJobTreeHandler` in `internal/handlers/job_handler.go`:
   - Get `step_definitions` from parent job's metadata
   - For each step definition, find matching step job by `step_name` metadata
   - Use step job's own status for the step (not aggregated from grandchildren)
   - Get grandchildren of each step job for ChildSummary counts
   - Fetch logs for each step job separately using step job ID

2. Update step status logic:
   - Remove the incorrect status override logic (lines 1486-1491)
   - Use step job's actual `Status` field directly
   - This ensures completed steps show checkmark, running shows spinner

3. Handle edge cases:
   - If `step_definitions` is missing, fall back to current behavior
   - If step job not found for a definition, show step as "pending"

## Accept

- [ ] Tree API returns 3 steps when job has 3 step_definitions
- [ ] Each step shows its actual status (completed = checkmark, running = spinner)
- [ ] ChildSummary counts reflect grandchildren of each step job
- [ ] Logs are fetched per step job, not duplicated across all steps
- [ ] Build passes without errors
