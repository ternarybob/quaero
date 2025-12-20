# Summary: Emailer Test Improvements

## Task
1. Improve email assertion to verify markdown-to-HTML conversion
2. Align test job definition with production job pattern (bin/web-search-asx-cba.toml)

## Changes

### Production Code (`internal/queue/workers/email_worker.go`)

Added explicit job log when HTML is generated from markdown:
```go
if htmlBody != "" {
    w.jobMgr.AddJobLog(ctx, stepID, "info",
        fmt.Sprintf("HTML email body generated (%d bytes) from markdown content", len(htmlBody)))
}
```

This makes the HTML conversion observable via the logs API, which the test can assert on.

### Test Job Definition (`test/config/job-definitions/web-search-asx.toml`)

Expanded from 3 steps to 4 steps:

| Step | Type | Purpose |
|------|------|---------|
| fetch_stock_data | asx_stock_data | Fetches real-time ASX data |
| search_asx_gnp | web_search | Web research |
| summarize_results | summary | Generates rich markdown with tables |
| email_summary | email | Sends HTML email |

The summary prompt now requests tables and structured formatting to produce meaningful markdown content for HTML conversion testing.

### Test File (`test/ui/job_definition_web_search_asx_test.go`)

**Assertion 2**: Updated to expect 4 steps
- Added `fetch_stock_data` to expected steps

**Assertion 5**: Improved HTML verification
- Looks for specific log: `"HTML email body generated (X bytes) from markdown content"`
- Reports HTML body size in test output
- Clearer error messages

## Build Verification

```
go build ./cmd/quaero     # PASS
go vet ./test/ui/...       # PASS
go build ./test/ui/...     # PASS
```

## Files Modified

| File | Change |
|------|--------|
| `internal/queue/workers/email_worker.go` | Added HTML conversion job log |
| `test/config/job-definitions/web-search-asx.toml` | Expanded to 4 steps with richer content |
| `test/ui/job_definition_web_search_asx_test.go` | Updated step count and improved email assertion |
