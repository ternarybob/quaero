# WORKER Step 3: Update Console Log Timestamp Default

## Change Made

Updated the default console log timestamp format from `"15:04:05"` to `"15:04:05.000"` to include milliseconds, matching the SSE log format.

## Files Modified

### 1. `internal/common/logger.go:119-120`

```go
// Before:
// Default time format if not specified
timeFormat := "15:04:05"

// After:
// Default time format if not specified (HH:MM:SS.mmm for alignment with SSE logs)
timeFormat := "15:04:05.000"
```

### 2. `internal/app/app.go:387`

```go
// Before:
TimeFormat: "15:04:05",

// After:
TimeFormat: "15:04:05.000",
```

### 3. `internal/common/config.go:76`

```go
// Before:
TimeFormat    string   `toml:"time_format"`     // Time format for logs (e.g. "15:04:05" or "2006-01-02 15:04:05")

// After:
TimeFormat    string   `toml:"time_format"`     // Time format for logs (default: "15:04:05.000")
```

## TOML Configuration

Users can still customize via `config.toml`:
```toml
[logging]
time_format = "15:04:05.000"  # Default (with milliseconds)
# time_format = "15:04:05"    # Without milliseconds
# time_format = "2006-01-02 15:04:05"  # With date
```

## Build Result

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

BUILD PASSED
