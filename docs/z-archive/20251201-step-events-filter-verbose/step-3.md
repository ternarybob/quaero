# Step 3: Change API default to return INFO+ logs

Model: sonnet | Status: ✅

## Done

- Modified `/api/jobs/{id}/logs` handler to default to INFO+ logs
- When no level param: returns INFO, WARN, ERROR, FATAL logs (excludes DEBUG)
- When level=all: returns ALL logs including DEBUG
- Existing level filters (debug, info, warn, error) work as before

## Files Changed

- `internal/handlers/job_handler.go` - Changed GetJobLogsHandler to default to info level filtering

## Verify

Build: ✅ | Tests: ⏭️
