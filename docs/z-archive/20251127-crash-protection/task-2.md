# Task 2: Add Process-Level Crash Protection

## Metadata
- **ID:** 2
- **Group:** 2
- **Mode:** concurrent
- **Skill:** @go-coder
- **Complexity:** medium
- **Model:** claude-sonnet-4-5-20250929
- **Critical:** no
- **Depends:** 1
- **Blocks:** 5

## Paths
```yaml
sandbox: /tmp/3agents/task-2/
source: C:/development/quaero/
output: C:/development/quaero/docs/fixes/20251127-crash-protection/
```

## Files to Modify
- `cmd/quaero/main.go` - Add top-level crash protection
- `internal/common/crash.go` - New file for crash protection utilities

## Requirements
Implement process-level crash protection that survives even fatal panics:

1. **Create crash protection module** (`internal/common/crash.go`):
   ```go
   // InstallCrashHandler sets up process-level crash protection
   // - Installs custom panic handler
   // - Sets up SIGABRT handling on Unix
   // - Creates crash dump file on fatal error
   func InstallCrashHandler(logDir string)

   // WriteCrashFile writes diagnostic info to crash file
   // Should be called from panic recovery before os.Exit
   func WriteCrashFile(logDir string, panicVal interface{}, stackTrace string)
   ```

2. **Top-level panic wrapper in main.go**:
   - Wrap entire main function in deferred panic recovery
   - Write crash file before process exits
   - Include: timestamp, panic value, full stack trace, goroutine dump

3. **Crash file format** (`crash-{timestamp}.log`):
   ```
   === QUAERO CRASH REPORT ===
   Time: 2025-11-27T16:05:22Z
   Panic: {panic value}

   === STACK TRACE ===
   {full stack trace}

   === ALL GOROUTINES ===
   {all goroutines dump}

   === SYSTEM INFO ===
   NumGoroutine: N
   NumCPU: N
   GOOS: windows
   ```

4. **Ensure crash file is written**:
   - Use direct os.OpenFile, not buffered writers
   - Sync file before returning
   - Handle nested panics gracefully

## Acceptance Criteria
- [ ] crash.go created with InstallCrashHandler and WriteCrashFile
- [ ] main.go wraps entire startup in crash protection
- [ ] Crash file includes all goroutine stacks
- [ ] Crash file includes system info
- [ ] Tested manually with intentional panic
- [ ] Compiles successfully

## Context
The current panic recovery in job_processor.go logs to Fatal, but if the logger itself has issues or the panic occurs outside the job processor, no diagnostic information is captured.

## Dependencies Input
From Task 1: Locations where crashes might occur

## Output for Dependents
- crash.go module can be used by other components
- Pattern for panic recovery with crash file
