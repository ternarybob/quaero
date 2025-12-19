# TDD Fix: Job Definition Web Search ASX Test

## Task (TDD Mode)
Fix test errors in `test/ui/job_definition_web_search_asx_test.go` - Tests are IMMUTABLE, only code can be changed.

## Issue Found

The test at lines 248-254 expected `entry.Message` field in log entries:
```go
if strings.Contains(entry.Message, "HTML") ||
    strings.Contains(entry.Message, "has_html") ||
    strings.Contains(entry.Message, "html_len") {
```

But the API returned `StepLog` struct with `Text` field instead of `Message`:
```go
type StepLog struct {
    LineNumber int    `json:"line_number"`
    Level      string `json:"level"`
    Text       string `json:"text"`  // <-- MISMATCH
}
```

## Fix Applied

Modified `internal/handlers/unified_logs_handler.go`:

### 1. Changed StepLog struct (lines 504-509)
**From:**
```go
type StepLog struct {
    LineNumber int    `json:"line_number"`
    Level      string `json:"level"`
    Text       string `json:"text"`
}
```

**To:**
```go
type StepLog struct {
    LineNumber int    `json:"line_number"`
    Level      string `json:"level"`
    Message    string `json:"message"`
}
```

### 2. Updated usages (lines 563-576)
Changed `Text:` to `Message:` in both places where StepLog is constructed.

## Build Verification

```
cmd.exe /c "go build -v ./internal/handlers/..."
# github.com/ternarybob/quaero/internal/handlers - OK

cmd.exe /c "go build -o bin/quaero.exe ./cmd/quaero"
# OK

cmd.exe /c "go build -v ./test/ui/..."
# OK
```

## TDD Compliance

| Rule | Status |
|------|--------|
| Test file unmodified | PASS |
| Fix applied to production code | PASS |
| Build passes | PASS |
| Test compiles | PASS |

## Files Modified

| File | Change |
|------|--------|
| `internal/handlers/unified_logs_handler.go` | Changed `StepLog.Text` to `StepLog.Message` |
