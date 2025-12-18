# ARCHITECT Analysis: Log Timestamp Format Inconsistency

## Problem Statement

The UI shows inconsistent log formats:
- Some logs display `[22:45:06.209] [INF]` (with milliseconds, 3-letter level)
- Other logs display `[22:45:19] [info]` (without milliseconds, lowercase level)

## Root Cause Analysis

### Two Different Data Sources

The log streaming system has **two data paths** that format data inconsistently:

1. **Live streaming logs** (via `log_event`):
   - Source: `internal/logs/consumer.go` -> `transformEvent()` -> `publishLogEvent()`
   - Format: `"15:04:05.000"` timestamp, `INF`/`WRN`/`ERR`/`DBG` levels
   - Correct format with milliseconds

2. **Initial/historical logs** (from memory writer):
   - Source: `internal/handlers/sse_logs_handler.go` -> `sendInitialServiceLogs()`
   - Format: Parses arbor memory writer output `"LEVEL | Oct 2 16:27:13 | MESSAGE"`
   - **Bug**: Maps levels to lowercase (`info`, `warn`, `error`, `debug`) instead of 3-letter codes
   - **Bug**: Timestamp extracted is `16:27:13` without milliseconds

### Specific Code Locations

**BUG LOCATION 1** - `internal/handlers/sse_logs_handler.go:760-771`:
```go
// Map level
logLevel := "info"  // <-- BUG: Should be "INF"
switch levelStr {
case "ERR", "ERROR", "FATAL", "PANIC":
    logLevel = "error"  // <-- BUG: Should be "ERR"
case "WRN", "WARN":
    logLevel = "warn"   // <-- BUG: Should be "WRN"
case "INF", "INFO":
    logLevel = "info"   // <-- BUG: Should be "INF"
case "DBG", "DEBUG":
    logLevel = "debug"  // <-- BUG: Should be "DBG"
}
```

**CORRECT IMPLEMENTATION** - `internal/handlers/unified_logs_handler.go:137-148`:
```go
// Map level to 3-letter format for consistency
logLevel := "INF" // Default
switch levelStr {
case "ERR", "ERROR", "FATAL", "PANIC":
    logLevel = "ERR"
case "WRN", "WARN":
    logLevel = "WRN"
case "INF", "INFO":
    logLevel = "INF"
case "DBG", "DEBUG":
    logLevel = "DBG"
}
```

**Timestamp Issue**:
- Memory writer format: `"Oct  2 16:27:13"` (no milliseconds in arbor memory writer)
- Consumer format: `"15:04:05.000"` (has milliseconds)
- This is a limitation of arbor's memory writer, not something we can fix in this codebase

## Recommendation: EXTEND Existing Pattern

**Do NOT create new code.** The correct pattern already exists in `unified_logs_handler.go:137-148`.

**Fix**: Modify `sse_logs_handler.go:760-771` to use the same 3-letter format.

## Files to Modify

1. `internal/handlers/sse_logs_handler.go` - Fix level mapping in `sendInitialServiceLogs()` (lines 760-771)

## Anti-Creation Compliance

- No new files needed
- No new functions needed
- Simple modification to match existing pattern
- Already have correct implementation in `unified_logs_handler.go`

## Build Verification Required

After modification, run:
- Windows: `.\scripts\build.ps1`
- Linux: `./scripts/build.sh`
