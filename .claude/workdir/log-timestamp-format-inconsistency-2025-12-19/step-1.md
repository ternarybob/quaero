# WORKER Step 1: Fix Level Mapping in SSE Logs Handler

## Change Made

Modified `internal/handlers/sse_logs_handler.go` lines 760-771 to use 3-letter level format consistent with live streaming logs.

## Before (Bug)

```go
// Map level
logLevel := "info"
switch levelStr {
case "ERR", "ERROR", "FATAL", "PANIC":
    logLevel = "error"
case "WRN", "WARN":
    logLevel = "warn"
case "INF", "INFO":
    logLevel = "info"
case "DBG", "DEBUG":
    logLevel = "debug"
}
```

## After (Fix)

```go
// Map level to 3-letter format for consistency with live streaming logs
logLevel := "INF"
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

## Pattern Source

Copied exact pattern from `unified_logs_handler.go:137-148` which already has the correct implementation.

## Build Result

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

BUILD PASSED
