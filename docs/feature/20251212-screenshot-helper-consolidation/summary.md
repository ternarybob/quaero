# Complete: Screenshot Helper Consolidation
Type: feature | Tasks: 4 | Files: 9

## User Request
"1. Update all tests which have screenshots to use the same library -> test\ui\screenshot_helper.go 2. Update the lib to screenshot the entire page, as many times the screenshot does NOT capture the test/attribution."

## Result
Consolidated all screenshot functionality to `test/ui/screenshot_helper.go`. Removed duplicate implementations from `test/common/setup.go`. All tests now use the single screenshot implementation.

## Skills Used
- go

## Files Changed
| File | Change |
|------|--------|
| test/ui/screenshot_helper.go | Added TakeFullScreenshot, TakeScreenshotInDir, TakeFullScreenshotInDir, TakeBeforeAfterScreenshots, GetScreenshotPath |
| test/common/setup.go | Removed TakeScreenshot, TakeFullScreenshot, TakeBeforeAfterScreenshots, GetScreenshotPath methods |
| test/ui/connector_loading_test.go | Updated to use screenshot_helper functions |
| test/ui/index_test.go | Updated to use screenshot_helper functions |
| test/ui/jobs_test.go | Updated to use screenshot_helper functions |
| test/ui/settings_test.go | Updated to use screenshot_helper functions |
| test/ui/github_jobs_test.go | Updated to use screenshot_helper functions |
| test/ui/logs_test.go | Updated to use screenshot_helper functions |
| test/ui/local_dir_jobs_test.go | Updated to use screenshot_helper functions |
| test/ui/job_framework_test.go | Updated UITestContext.Screenshot() to use TakeFullScreenshotInDir (was viewport-only) |

## Validation: PASS
- Build passes
- Tests pass (TestJobLoggingImprovements - 19s)
- Single implementation in screenshot_helper.go

## API Summary

### screenshot_helper.go Functions
```go
// Standalone (auto-path generation)
TakeScreenshot(ctx, name)        // Viewport screenshot
TakeFullScreenshot(ctx, name)    // Full page screenshot

// With explicit path
TakeScreenshotToPath(ctx, path)      // Viewport to path
TakeFullScreenshotToPath(ctx, path)  // Full page to path

// For TestEnvironment usage (pass env.ResultsDir)
GetScreenshotPath(resultsDir, name)                     // Generate path
TakeScreenshotInDir(ctx, resultsDir, name)              // Viewport in dir
TakeFullScreenshotInDir(ctx, resultsDir, name)          // Full page in dir
TakeBeforeAfterScreenshots(ctx, resultsDir, base, fn)   // Before/after wrapper
```

## Usage
```go
// In test files (package ui)
TakeFullScreenshotInDir(ctx, env.ResultsDir, "screenshot_name")

// Or using UITestContext wrapper
utc.FullScreenshot("screenshot_name")
```
