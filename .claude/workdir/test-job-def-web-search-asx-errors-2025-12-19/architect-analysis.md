# ARCHITECT ANALYSIS - Test Job Definition Web Search ASX Errors

## Task
Fix errors in `test/ui/job_definition_web_search_asx_test.go`

## Error Analysis

Running `go vet ./test/ui/...` revealed:
```
test\ui\job_definition_web_search_asx_test.go:211:94: step.StepID undefined (type apiJobTreeStep has no field or method StepID)
```

### Root Cause

The test struct `apiJobTreeStep` in `test/ui/uitest_context.go` is incomplete:

**Current struct (lines 898-902):**
```go
type apiJobTreeStep struct {
    Name   string `json:"name"`
    Status string `json:"status"`
}
```

**Actual API response (internal/handlers/job_handler.go:1341-1351):**
```go
type JobTreeStep struct {
    StepID       string           `json:"step_id,omitempty"`
    Name         string           `json:"name"`
    Status       string           `json:"status"`
    DurationMs   int64            `json:"duration_ms"`
    StartedAt    *time.Time       `json:"started_at,omitempty"`
    FinishedAt   *time.Time       `json:"finished_at,omitempty"`
    Expanded     bool             `json:"expanded"`
    ChildSummary *ChildJobSummary `json:"child_summary,omitempty"`
    Logs         []JobTreeLog     `json:"logs"`
    TotalLogs    int              `json:"total_logs,omitempty"`
}
```

### Secondary Issues

The test also uses `entry.Message` (line 252) and `stepLogs.Logs` (line 249), which require:

1. **`apiJobTreeLogsStep`** needs a `Logs` field (currently missing)
2. A new struct for individual log entries with `Message` field

**Current struct:**
```go
type apiJobTreeLogsStep struct {
    StepName   string `json:"step_name"`
    TotalCount int    `json:"total_count"`
}
```

**API response (internal/handlers/unified_logs_handler.go:511-519):**
```go
type StepLogsResponse struct {
    StepName        string    `json:"step_name"`
    StepID          string    `json:"step_id,omitempty"`
    Status          string    `json:"status"`
    Logs            []StepLog `json:"logs"`
    TotalCount      int       `json:"total_count"`
    UnfilteredCount int       `json:"unfiltered_count"`
}

type StepLog struct {
    LineNumber int    `json:"line_number"`
    Level      string `json:"level"`
    Message    string `json:"message"`
}
```

## Fix Recommendation

Modify `test/ui/uitest_context.go` to add the missing fields to the API response structs:

1. Add `StepID` field to `apiJobTreeStep`
2. Add `Logs` field and log entry type to `apiJobTreeLogsStep`

### Anti-Creation Compliance

- **EXTEND > MODIFY > CREATE**: ✓ Extending existing structs
- **No new files**: ✓ Only modifying `uitest_context.go`
- **Follow existing patterns**: ✓ Using same struct style as existing test helpers

### Note on 3agents Rules

The rule "NEVER modify tests to make code pass" refers to changing test assertions or logic to avoid failures. This fix is different - we're **fixing the test's API response structs to match the actual API**, which is a legitimate bug fix in the test helper code.

## Files to Modify

| File | Change |
|------|--------|
| `test/ui/uitest_context.go` | Add `StepID` to `apiJobTreeStep`, add `Logs` field and log entry struct to logs response |
