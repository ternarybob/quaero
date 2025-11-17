# UI Test Screenshot Requirements

## Overview
All UI tests MUST capture before/after screenshots to document visual state changes and aid in debugging.

## Requirements

### 1. Every UI Test Must Capture Screenshots
- **Before Screenshot**: Capture the initial state before performing any actions
- **After Screenshot**: Capture the final state after actions complete
- **Error Screenshots**: If an action fails, capture an "after_error" screenshot

### 2. Use Test Environment Helper Methods

The `TestEnvironment` provides helper methods for screenshots:

```go
// Capture a viewport screenshot
env.TakeScreenshot(ctx, "screenshot_name")

// Capture a full-page screenshot (recommended)
env.TakeFullScreenshot(ctx, "screenshot_name")

// Capture before/after automatically with error handling
env.TakeBeforeAfterScreenshots(ctx, "action_name", func() error {
    // Perform your action here
    return nil
})
```

### 3. Screenshot Naming Convention

Screenshots should use descriptive names that indicate:
- The page/feature being tested
- The action being performed
- The state (before/after/error)

Examples:
- `queue_page_before.png`
- `queue_page_after.png`
- `job_deletion_before.png`
- `job_deletion_after.png`
- `settings_save_before.png`
- `settings_save_after_error.png`

### 4. Screenshot Storage

Screenshots are automatically saved to:
```
test/results/{ui|api}/{suite-name}-{timestamp}/{TestName}/
```

Example:
```
test/results/ui/job-20251118-083724/TestJobErrorDisplay_Simple/
  ├── queue_page_before.png
  ├── queue_page_after.png
  ├── service.log
  └── test.log
```

## Example Implementation

### Simple Approach (Manual Before/After)

```go
func TestMyFeature(t *testing.T) {
    env, err := common.SetupTestEnvironment("TestMyFeature")
    if err != nil {
        t.Fatalf("Failed to setup: %v", err)
    }
    defer env.Cleanup()

    // Setup Chrome context
    ctx := context.Background()
    allocCtx, cancel := chromedp.NewExecAllocator(ctx,
        append(chromedp.DefaultExecAllocatorOptions[:],
            chromedp.Flag("headless", true),
            chromedp.Flag("disable-gpu", true),
            chromedp.Flag("no-sandbox", true),
        )...)
    defer cancel()

    chromeCtx, cancel := chromedp.NewContext(allocCtx)
    defer cancel()

    // Navigate to page
    serverURL := env.GetBaseURL()
    err = chromedp.Run(chromeCtx,
        chromedp.Navigate(serverURL+"/my-page"),
        chromedp.Sleep(2*time.Second),
    )
    if err != nil {
        t.Fatalf("Failed to navigate: %v", err)
    }

    // Take BEFORE screenshot
    if err := env.TakeFullScreenshot(chromeCtx, "my_feature_before"); err != nil {
        t.Fatalf("Failed to take before screenshot: %v", err)
    }
    t.Logf("✓ Before screenshot: %s", env.GetScreenshotPath("my_feature_before"))

    // Perform action
    err = chromedp.Run(chromeCtx,
        chromedp.Click("#my-button", chromedp.ByQuery),
        chromedp.Sleep(1*time.Second),
    )
    if err != nil {
        t.Fatalf("Failed to perform action: %v", err)
    }

    // Take AFTER screenshot
    if err := env.TakeFullScreenshot(chromeCtx, "my_feature_after"); err != nil {
        t.Fatalf("Failed to take after screenshot: %v", err)
    }
    t.Logf("✓ After screenshot: %s", env.GetScreenshotPath("my_feature_after"))

    // Assertions...
    t.Logf("✅ TEST COMPLETED: Screenshots in %s", env.ResultsDir)
}
```

### Advanced Approach (Using Helper)

```go
func TestMyFeature(t *testing.T) {
    env, err := common.SetupTestEnvironment("TestMyFeature")
    if err != nil {
        t.Fatalf("Failed to setup: %v", err)
    }
    defer env.Cleanup()

    // Setup Chrome context
    ctx := context.Background()
    allocCtx, cancel := chromedp.NewExecAllocator(ctx, ...)
    defer cancel()

    chromeCtx, cancel := chromedp.NewContext(allocCtx)
    defer cancel()

    // Navigate to page
    serverURL := env.GetBaseURL()
    err = chromedp.Run(chromeCtx,
        chromedp.Navigate(serverURL+"/my-page"),
        chromedp.Sleep(2*time.Second),
    )
    if err != nil {
        t.Fatalf("Failed to navigate: %v", err)
    }

    // Use helper for before/after screenshots with action
    err = env.TakeBeforeAfterScreenshots(chromeCtx, "my_feature", func() error {
        return chromedp.Run(chromeCtx,
            chromedp.Click("#my-button", chromedp.ByQuery),
            chromedp.Sleep(1*time.Second),
        )
    })
    if err != nil {
        t.Fatalf("Action failed: %v", err)
    }

    // Assertions...
    t.Logf("✅ TEST COMPLETED: Screenshots in %s", env.ResultsDir)
}
```

## Benefits

1. **Visual Debugging**: See exactly what the UI looked like before and after the test
2. **Regression Detection**: Compare screenshots across test runs to detect visual regressions
3. **Documentation**: Screenshots serve as visual documentation of expected behavior
4. **Troubleshooting**: When tests fail in CI, screenshots help identify the issue

## Existing Tests

The following tests already implement screenshots following the template:
- `TestJobErrorDisplay_Simple` - Demonstrates error display feature with proper screenshot workflow
  - Takes "queue-initial" screenshot after WebSocket connection
  - Takes "job-in-queue" screenshot showing job with error
  - Uses env.LogTest() for all logging
  - Follows homepage_test.go template exactly

## Todo

All other UI tests in this directory (homepage_test.go, etc.) should be reviewed to ensure they follow the template and screenshot requirements.
