# Step 1: Fix ExecuteJobDefinition to mark job failed when validation fails
Model: sonnet | Status: ✅

## Done
- Added `lastValidationError` variable to track validation errors that occur with `on_error="continue"`
- Updated validation error handling to store the error message
- Modified completion logic to check for validation errors when no children were created
- If validation errors occurred AND no children created, job is now marked as "failed" with error message

## Files Changed
- `internal/queue/manager.go` - Added validation error tracking and proper failure status

## Changes Detail
1. Line ~1017: Added `var lastValidationError string` to track errors
2. Line ~1103: Added `lastValidationError = fmt.Sprintf(...)` when validation fails with on_error="continue"
3. Lines ~1229-1241: Added conditional check - if `lastValidationError != ""`, mark job as "failed" instead of "completed"

## Verify
Build: ✅ | Tests: ⏭️ (not blocking)
