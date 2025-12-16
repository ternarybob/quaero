# Summary: Fix SSE Line Number Type Assertion Bug

## Issue
Line numbers displayed in the UI were always 1, 2, 3... (UI-generated) instead of the actual server-provided line numbers (e.g., 3457, 3458, 3459...).

Screenshot showed:
- `logs: 100/3556` badge (correct total)
- Line numbers 1-14 on the left (WRONG - should be ~3457-3556)
- Log content "Processing item 1111/1200" (middle of processing)

## Root Cause
**Type assertion bug in `handleJobLogEvent`** at `internal/handlers/sse_logs_handler.go:257`

**Before (BUG):**
```go
lineNumber := 0
if ln, ok := payload["line_number"].(float64); ok {  // FAILS - it's an int!
    lineNumber = int(ln)
}
```

The `payload["line_number"]` is an `int` (from `job_manager.go`), NOT a `float64`.
The type assertion `.(float64)` silently fails, leaving `lineNumber = 0`.

When SSE sends logs to the client with `line_number: 0`, the UI falls back:
```html
x-text="log.line_number || (logIdx + 1)"  <!-- Falls back to logIdx + 1 when line_number is 0 -->
```

**After (FIXED):**
```go
lineNumber := 0
switch ln := payload["line_number"].(type) {
case int:
    lineNumber = ln
case float64:
    lineNumber = int(ln)
}
```

Now handles both `int` (direct Go map) and `float64` (JSON decode) types.

## Why This Only Affected Real-Time Updates
1. **Initial API load** (`/api/logs`): Returns logs from database with correct `LineNumber` field - WORKED
2. **SSE real-time updates**: Event payload passes `line_number` as `int`, but handler expected `float64` - BROKEN

This is why the line numbers would show correctly on initial load, but reset to 1, 2, 3... as SSE updates replaced the logs.

## Test Added
`TestLogLineNumbersAreServerProvided` in `test/ui/job_test_generator_streaming_test.go`

Assertions:
1. Line numbers are sequential (no gaps)
2. First line number is NOT 1 when showing last N of M logs (catches UI-generation bug)
3. Last line number is close to total count
4. Displayed count matches actual DOM elements

## Build Status
✅ Build passes
