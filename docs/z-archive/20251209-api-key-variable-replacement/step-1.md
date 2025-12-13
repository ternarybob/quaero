# Step 1: Fix ValidateAPIKeys to handle {xxx} pattern
Model: sonnet | Status: ✅

## Done
- Modified ValidateAPIKeys to detect and extract key name from `{xxx}` pattern
- Added check: if apiKeyName starts with `{` and ends with `}`, extract the inner key name
- Uses extracted key name for ResolveAPIKey lookup instead of literal placeholder

## Files Changed
- `internal/jobs/service.go` - Added variable reference pattern handling in ValidateAPIKeys (lines 397-403)

## Build Check
Build: ⏳ | Tests: ⏭️
