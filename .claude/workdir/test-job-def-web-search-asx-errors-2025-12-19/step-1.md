# WORKER Step 1 - Fix Test Helper Structs

## Changes Made

### File: `test/ui/uitest_context.go`

#### 1. Added `StepID` field to `apiJobTreeStep` (line 900)

**Before:**
```go
type apiJobTreeStep struct {
    Name   string `json:"name"`
    Status string `json:"status"`
}
```

**After:**
```go
type apiJobTreeStep struct {
    StepID string `json:"step_id,omitempty"`
    Name   string `json:"name"`
    Status string `json:"status"`
}
```

#### 2. Added `apiLogEntry` struct and `Logs` field to `apiJobTreeLogsStep`

**Added new struct:**
```go
type apiLogEntry struct {
    LineNumber int    `json:"line_number"`
    Level      string `json:"level"`
    Message    string `json:"message"`
}
```

**Updated `apiJobTreeLogsStep`:**
```go
type apiJobTreeLogsStep struct {
    StepName   string        `json:"step_name"`
    Logs       []apiLogEntry `json:"logs"`
    TotalCount int           `json:"total_count"`
}
```

## Build Verification

```
cmd.exe /c "go vet ./test/ui/..."
# (no errors)

cmd.exe /c "go build -v ./test/ui/..."
# github.com/ternarybob/quaero/test/ui
```

## Anti-Creation Compliance

- **EXTEND > MODIFY > CREATE**: ✓ Extended existing structs
- **No new files created**: ✓ Only modified `uitest_context.go`
- **Follows existing patterns**: ✓ Uses same struct style as existing test helpers
