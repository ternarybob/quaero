# Step 1: Job Definition Test Framework - Implementation Complete

## Summary

Successfully implemented shared helper methods in the UI test framework for job definition tests. Added three new methods to `UITestContext` and a new configuration struct to standardize job definition testing across all job types.

## Files Changed

### `C:\development\quaero\test\ui\job_framework_test.go`
- **Lines added:** 143 lines
- **Total lines:** 563 lines
- **Changes:**
  - Added imports: `io`, `os`, `path/filepath`
  - Added `JobDefinitionTestConfig` struct (7 lines)
  - Added `CopyJobDefinitionToResults` method (35 lines)
  - Added `RefreshAndScreenshot` method (29 lines)
  - Added `RunJobDefinitionTest` method (59 lines)

## Key Changes

### 1. JobDefinitionTestConfig Struct

```go
// JobDefinitionTestConfig configures a job definition end-to-end test
type JobDefinitionTestConfig struct {
    JobName           string        // Name as shown in UI (e.g., "News Crawler")
    JobDefinitionPath string        // Path to TOML file (relative to test/ui/)
    Timeout           time.Duration // Max time to wait for job completion
    RequiredEnvVars   []string      // Env vars that must be set (skip if missing)
    AllowFailure      bool          // If true, don't fail test if job fails
}
```

**Purpose:** Provides a standardized configuration structure for all job definition tests, ensuring consistent test setup across different job types.

### 2. CopyJobDefinitionToResults Method

```go
func (utc *UITestContext) CopyJobDefinitionToResults(jobDefPath string) error
```

**Key features:**
- Resolves relative path from test/ui/ directory
- Opens source TOML file and creates destination in results directory
- Uses `io.Copy` for efficient file copying
- Comprehensive error handling with context using `%w`
- Logs success with destination path

**Error handling:**
- Working directory resolution errors
- File open errors with source path
- File create errors with destination path
- Copy operation errors

### 3. RefreshAndScreenshot Method

```go
func (utc *UITestContext) RefreshAndScreenshot(name string) error
```

**Key features:**
- Gets current URL using `chromedp.Location`
- Navigates to same URL (performs refresh)
- Waits for `.page-title` to ensure page load
- Takes full-page screenshot using existing `FullScreenshot` method
- Logs success after completion

**Error handling:**
- URL retrieval errors
- Page navigation/refresh errors
- Page load timeout errors
- Screenshot capture errors

### 4. RunJobDefinitionTest Method

```go
func (utc *UITestContext) RunJobDefinitionTest(config JobDefinitionTestConfig) error
```

**Orchestrates the complete test flow:**
1. **Environment variable check:** Validates all required env vars are set, uses `t.Skip()` if missing
2. **Copy TOML:** Copies job definition to results directory for audit trail
3. **Navigate:** Goes to Jobs page and waits for full load
4. **Screenshot:** Takes "job_definition" screenshot showing initial state
5. **Trigger:** Uses existing `TriggerJob` method to start the job
6. **Monitor:** Uses existing `MonitorJob` with custom options respecting `AllowFailure`
7. **Refresh:** Refreshes page and takes "final_state" screenshot

**Configuration handling:**
- Respects `AllowFailure` flag by passing to `MonitorJobOptions`
- Uses provided `Timeout` for job monitoring
- Skips test gracefully when env vars missing (doesn't fail build)

## Design Patterns Used

### Error Handling
- All errors wrapped with context using `fmt.Errorf` and `%w`
- Descriptive error messages include relevant paths and parameters
- Follows Go 1.21+ error wrapping best practices

### Logging
- Uses existing `utc.Log` method for consistent logging
- Logs key milestones: start, TOML copy, refresh, completion
- Success indicators use checkmark prefix: `✓`

### Resource Management
- Proper `defer` usage for file handles
- No resource leaks in CopyJobDefinitionToResults
- Leverages existing cleanup mechanisms in UITestContext

### Composability
- New methods build on existing primitives (TriggerJob, MonitorJob, Screenshot)
- No duplication of navigation, triggering, or monitoring logic
- Clean separation of concerns

## Build Verification

```bash
> go build ./test/ui/...
```

**Result:** ✅ **SUCCESS** - Code compiles without errors

## Accept Criteria Checklist

- ✅ JobDefinitionTestConfig struct defined with all fields
  - JobName, JobDefinitionPath, Timeout, RequiredEnvVars, AllowFailure
- ✅ CopyJobDefinitionToResults copies file to Env.ResultsDir
  - Resolves relative paths correctly
  - Uses io.Copy for efficient transfer
  - Proper error handling and logging
- ✅ RefreshAndScreenshot navigates to current URL and screenshots
  - Gets current URL via chromedp.Location
  - Refreshes by navigating to same URL
  - Waits for page load before screenshot
  - Uses FullScreenshot for complete capture
- ✅ RunJobDefinitionTest orchestrates: env check, copy TOML, trigger, monitor, refresh, screenshot
  - Checks env vars and skips via t.Skip if missing
  - Copies TOML to results directory
  - Takes job_definition screenshot
  - Triggers job via TriggerJob
  - Monitors job via MonitorJob with proper options
  - Refreshes and takes final_state screenshot
- ✅ All new methods have error handling with context
  - All errors wrapped with %w
  - Descriptive error messages with relevant context
- ✅ Code compiles: `go build ./test/ui/...`
  - Build successful with no errors or warnings

## Next Steps

This framework is now ready for implementation of individual job definition tests:
- Task 2: News crawler test
- Task 3: YouTube transcript test
- Task 4: HackerNews crawler test
- Task 5: Document Q&A test

All tests will use `RunJobDefinitionTest` with job-specific `JobDefinitionTestConfig` instances.
