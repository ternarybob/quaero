# Task 1: Create /api/jobs/{id}/structure endpoint
Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Provides a lightweight endpoint for UI to fetch current job and step statuses without the overhead of logs and child job details.

## Skill Patterns to Apply
- Separation of concerns: Handler delegates to service layer
- Error handling: Check errors, log with context, return appropriate HTTP codes
- JSON encoding: Use json.NewEncoder for streaming response

## Do
1. Add `JobStructureResponse` and `StepStatus` structs to `job_handler.go`
2. Create `GetJobStructureHandler` method on `JobHandler`
3. Add route `/api/jobs/{id}/structure` in routes.go
4. Implementation should:
   - Get parent job from storage
   - Get step jobs (children)
   - For each step, get log count from log service
   - Return minimal structure response

## Accept
- [ ] Endpoint returns 200 with correct JSON structure
- [ ] Response includes job_id, status, steps array with name/status/log_count
- [ ] Step statuses match actual database state
- [ ] No logs or detailed child job info included (lightweight)
- [ ] Route registered and accessible
