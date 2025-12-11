# Step 1: Create /api/jobs/{id}/structure endpoint
Model: opus | Skill: go | Status: ✅

## Done
- Added `JobStructureResponse` and `StepStatus` structs to job_handler.go
- Created `GetJobStructureHandler` method on `JobHandler`
- Added route `/api/jobs/{id}/structure` in routes.go

## Files Changed
- `internal/handlers/job_handler.go` - Added structs and handler method (lines 1375-2042)
- `internal/server/routes.go` - Added route for /structure endpoint (lines 173-177)

## Skill Compliance
- [x] Handler delegates to service layer (jobManager, jobStorage, logService)
- [x] Error handling with context logging
- [x] JSON encoding via json.NewEncoder

## Build Check
Build: ✅ | Tests: ⏭️ (skipped - will test after all tasks complete)
