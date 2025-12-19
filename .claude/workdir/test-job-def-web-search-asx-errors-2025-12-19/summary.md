# Summary: Test Job Definition Web Search ASX Errors

## Task
Fix errors in `test/ui/job_definition_web_search_asx_test.go`

## Error Identified

Running `go vet ./test/ui/...` revealed:
```
step.StepID undefined (type apiJobTreeStep has no field or method StepID)
```

The test file used API response fields that weren't defined in the test helper structs.

## Root Cause

Test helper structs in `test/ui/uitest_context.go` were incomplete:

1. `apiJobTreeStep` was missing `StepID` field (API returns `step_id`)
2. `apiJobTreeLogsStep` was missing `Logs` field (API returns logs array)
3. No struct for individual log entries (needed for `entry.Message` access)

## Fix Applied

Modified `test/ui/uitest_context.go`:

### 1. Added `StepID` to `apiJobTreeStep`
```go
type apiJobTreeStep struct {
    StepID string `json:"step_id,omitempty"`  // Added
    Name   string `json:"name"`
    Status string `json:"status"`
}
```

### 2. Added `apiLogEntry` struct
```go
type apiLogEntry struct {
    LineNumber int    `json:"line_number"`
    Level      string `json:"level"`
    Message    string `json:"message"`
}
```

### 3. Added `Logs` field to `apiJobTreeLogsStep`
```go
type apiJobTreeLogsStep struct {
    StepName   string        `json:"step_name"`
    Logs       []apiLogEntry `json:"logs"`      // Added
    TotalCount int           `json:"total_count"`
}
```

## Build Verification

```
.\scripts\build.ps1
# Main executable built: bin/quaero.exe
# MCP server built: bin/quaero-mcp/quaero-mcp.exe

go vet ./test/ui/...
# (no errors)

go build -v ./test/ui/...
# (no errors)
```

## Files Modified

| File | Change |
|------|--------|
| `test/ui/uitest_context.go` | Added missing fields to API response structs |

## Previous Fix (Also Applied)

Earlier in this session, `internal/handlers/unified_logs_handler.go` was also fixed:
- Changed `StepLog.Text` to `StepLog.Message` (JSON field `"text"` → `"message"`)

Both fixes ensure the test helper structs match the actual API responses.
