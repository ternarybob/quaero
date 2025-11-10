# Test Analysis: Chrome Extension Testing

## Implementation Changes
Based on the code review, the Chrome extension has the following key features:
- **Extension manifest**: `cmd/quaero-chrome-extension/manifest.json` - defines permissions and components
- **Side panel UI**: `cmd/quaero-chrome-extension/sidepanel.html` - main user interface
- **Side panel script**: `cmd/quaero-chrome-extension/sidepanel.js` - handles "Capture & Crawl" functionality
- **Background service**: `cmd/quaero-chrome-extension/background.js` - generic auth capture

**Key Functionality:**
- Capture authentication cookies from any authenticated website
- Start a quick crawl job via POST `/api/job-definitions/quick-crawl`
- Display server connection status via WebSocket
- Track last capture time in Chrome storage

## Existing Test Coverage

**UI Tests:** (7 files in `/test/ui/`)
- `homepage_test.go`: Tests homepage title, elements, debug logging, timestamps, navigation
- Other test files for specific UI components

**API Tests:** (2 files in `/test/api/`)
- `quick_crawl_test.go`: Tests quick-crawl endpoint with various scenarios
- `mcp_server_test.go`: Tests MCP server functionality

## Test Patterns Identified

**UI Test Pattern:**
```go
func TestName(t *testing.T) {
    // Setup: create test environment with test name
    env, err := common.SetupTestEnvironment("TestName")
    if err != nil {
        t.Fatalf("Failed to setup test environment: %v", err)
    }
    defer env.Cleanup()

    // Timing and logging
    startTime := time.Now()
    env.LogTest(t, "=== RUN TestName")
    defer func() {
        elapsed := time.Since(startTime)
        if t.Failed() {
            env.LogTest(t, "--- FAIL: TestName (%.2fs)", elapsed.Seconds())
        } else {
            env.LogTest(t, "--- PASS: TestName (%.2fs)", elapsed.Seconds())
        }
    }()

    // Environment details
    env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
    env.LogTest(t, "Results directory: %s", env.GetResultsDir())

    // Create ChromeDP context
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Test logic with ChromeDP actions
    // - Navigate to pages
    // - Wait for WebSocket connection (env.WaitForWebSocketConnection)
    // - Take screenshots (env.TakeScreenshot)
    // - Verify page elements and behavior
}
```

**Key Utilities:**
- `env.SetupTestEnvironment(testName)` - starts service, creates test directory
- `env.LogTest(t, message, args...)` - logs to both test log and console
- `env.WaitForWebSocketConnection(ctx, timeout)` - waits for WS connection
- `env.TakeScreenshot(ctx, name)` - captures page screenshot
- `env.Cleanup()` - stops service and closes resources

## Test Gaps

**Chrome Extension Testing:**
- **No UI tests** for Chrome extension functionality
- **No tests** for "Capture & Crawl" button workflow
- **No tests** for extension installation and loading
- **No tests** for interaction between extension and Quaero server

**Specific Missing Coverage:**
1. Extension loads successfully in ChromeDP
2. Extension side panel displays correctly
3. "Capture & Crawl" button is visible and clickable
4. Quick crawl job is created when button is clicked
5. Job executes and captures the target page
6. Last capture time is updated

## Test Plan

### New Tests Required:
- [x] **TestChromeExtension**: Test Chrome extension installation and "Capture & Crawl" functionality
  - Load extension in ChromeDP with unpacked extension
  - Navigate to test page (https://www.abc.net.au/news)
  - Open extension side panel
  - Verify "Capture & Crawl" button exists
  - Click "Capture & Crawl" button
  - Verify quick crawl job is created (check API response)
  - Verify job status shows "running" or "queued"
  - Wait for job to complete (poll job status)
  - Verify page content was captured (check documents table)
  - Take screenshots at each step

### Tests to Update:
- None - this is a new feature test

### Tests to Run:
- [x] All UI tests in /test/ui (including new extension test)
- [x] All API tests in /test/api

**Challenge: ChromeDP Extension Loading**

ChromeDP supports loading Chrome extensions with the following approach:
```go
// Allocator options to load unpacked extension
extensionPath := "C:\\development\\quaero\\cmd\\quaero-chrome-extension"
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("load-extension", extensionPath),
    chromedp.Flag("disable-extensions-except", extensionPath),
)

allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
defer cancel()

ctx, cancel := chromedp.NewContext(allocCtx)
defer cancel()
```

**Key Considerations:**
1. Extension must be unpacked (directory, not .zip)
2. Path must be absolute
3. Extension will be available but side panel requires explicit navigation
4. Extension storage (Chrome storage API) works in test environment

**Analysis completed:** 2025-11-10T00:00:00Z
