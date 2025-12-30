# Quaero Test Architecture

This document describes the test architecture and patterns used in the Quaero project. **All new tests MUST follow these patterns.**

## Overview

```
test/
├── api/                    # API integration tests
│   ├── main_test.go        # Test suite setup
│   └── *_test.go           # Individual API tests
├── ui/                     # UI/browser automation tests
│   ├── uitest_context.go   # Core test infrastructure (NOT a test file)
│   └── *_test.go           # Individual UI tests
├── common/                 # Shared test utilities
│   └── setup.go            # Test environment setup
├── config/                 # Test configuration files
└── results/                # Test output (screenshots, logs)
    └── {timestamp}/        # Per-run results directory

internal/                   # Unit tests live alongside code
└── **/
    └── *_test.go           # Unit tests for each package
```

## Browser Automation Standard

### MANDATORY: chromedp Only

**All UI tests MUST use `chromedp` for browser automation.**

```go
import "github.com/chromedp/chromedp"
```

**FORBIDDEN alternatives (will be rejected in code review):**
- selenium / webdriver
- playwright
- puppeteer
- rod
- Direct Chrome DevTools Protocol without chromedp

**Why chromedp?**
1. Native Go - no external dependencies
2. Full Chrome DevTools Protocol support
3. Established infrastructure in `UITestContext`
4. Shared helpers for common operations
5. Consistent patterns across all tests

## UI Tests (`test/ui/`)

### Core Infrastructure: UITestContext

All UI tests MUST use `UITestContext` from `test/ui/uitest_context.go`.

```go
package ui

import (
    "testing"
    "time"

    "github.com/chromedp/chromedp"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
    // MANDATORY: Create context with timeout
    utc := NewUITestContext(t, 5*time.Minute)

    // MANDATORY: Always defer cleanup immediately
    defer utc.Cleanup()

    // Use structured logging
    utc.Log("Starting test for feature X")

    // Navigate using helper
    err := utc.Navigate(utc.JobsURL)
    require.NoError(t, err, "Failed to navigate")

    // Take screenshots at key moments
    utc.Screenshot("initial_state")

    // Use chromedp for browser operations
    var result string
    err = chromedp.Run(utc.Ctx,
        chromedp.WaitVisible(".my-element", chromedp.ByQuery),
        chromedp.Text(".my-element", &result, chromedp.ByQuery),
    )
    require.NoError(t, err)

    // Use testify for assertions
    assert.Equal(t, "expected", result)

    utc.Screenshot("final_state")
}
```

### UITestContext Features

| Method | Purpose |
|--------|---------|
| `NewUITestContext(t, timeout)` | Create test context with browser |
| `utc.Cleanup()` | Release all resources (ALWAYS defer) |
| `utc.Navigate(url)` | Navigate and wait for page load |
| `utc.Screenshot(name)` | Take numbered screenshot |
| `utc.Log(format, args...)` | Structured test logging |
| `utc.Click(selector)` | Click element |
| `utc.GetText(selector)` | Get element text |
| `utc.WaitForElement(sel, timeout)` | Wait for element visibility |
| `utc.TriggerJob(name)` | Trigger job via UI |
| `utc.MonitorJob(name, opts)` | Monitor job until completion |
| `utc.SaveToResults(file, content)` | Save data to results dir |

### Available URLs

```go
utc.BaseURL     // Base server URL
utc.JobsURL     // /jobs page
utc.QueueURL    // /queue page
utc.DocsURL     // /documents page
utc.SettingsURL // /settings page
```

### Reference Implementations

Study these files before writing new UI tests:
- `test/ui/job_core_test.go` - Basic page navigation tests
- `test/ui/job_definition_general_test.go` - Job monitoring patterns
- `test/ui/logs_test.go` - WebSocket tracking patterns

## API Tests (`test/api/`)

### Core Infrastructure

API tests use shared setup from `test/common/setup.go`.

```go
package api

import (
    "testing"

    "github.com/ternarybob/quaero/test/common"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyAPI(t *testing.T) {
    // Setup test environment
    env, err := common.SetupTestEnvironment(t.Name())
    require.NoError(t, err)
    defer env.Cleanup()

    // Use HTTP helpers
    helper := env.NewHTTPTestHelper(t)

    // Make requests
    resp, err := helper.GET("/api/jobs")
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, 200, resp.StatusCode)
}
```

### Reference Implementations

- `test/api/main_test.go` - Test suite setup
- `test/api/jobs_test.go` - Job API tests
- `test/api/health_check_test.go` - Simple endpoint tests

## Unit Tests (`internal/**/`)

Unit tests live alongside the code they test.

```go
// internal/services/myservice/service_test.go
package myservice

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFunction(t *testing.T) {
    // Setup
    svc := NewService(...)

    // Execute
    result, err := svc.DoSomething()

    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Naming Convention

- Test files: `{filename}_test.go`
- Test functions: `Test{FunctionName}` or `Test{Feature}_{Scenario}`

## Assertion Library: testify

**MANDATORY: Use testify for all assertions.**

```go
import (
    "github.com/stretchr/testify/assert"  // Non-fatal assertions
    "github.com/stretchr/testify/require" // Fatal assertions
)

// require - stops test on failure
require.NoError(t, err, "critical operation failed")
require.NotNil(t, result)

// assert - continues test on failure
assert.Equal(t, expected, actual)
assert.True(t, condition)
assert.Contains(t, haystack, needle)
```

## Test Results

Test results are saved to `test/results/{timestamp}/`:

```
test/results/
└── 2024-12-30-1045/
    ├── test.log              # All test logs
    ├── service.log           # Service output logs
    ├── output.md             # Worker-generated content (document content_markdown)
    ├── job_definition.json   # Job definition used (for reproducibility)
    ├── output_1.md           # Numbered outputs (for multi-run comparison)
    ├── output_1.json         # Extracted JSON from output (if applicable)
    ├── 01_initial_state.png
    ├── 02_after_navigation.png
    ├── 03_final_state.png
    └── captured_data.json
```

### Worker Test Output Files

For tests that execute workers (like `TestWorkerASXStockData`, `TestWorkerSummaryWithSchema`):

| File | Description |
|------|-------------|
| `output.md` | Primary output - actual worker-generated content (document content_markdown) |
| `output.json` | Document metadata - structured data/schema fields from the document |
| `job_definition.json` | The job definition used to run the test |
| `output_N.md` | Numbered outputs for multi-run comparison |
| `output_N.json` | Numbered metadata for multi-run comparison |

**Important**: `output.md` contains the actual worker output (document content), NOT test logs. Test logs go to `test.log`.

## Anti-Patterns (Will Be Rejected)

### Browser Automation

```go
// ❌ Using alternative browser libraries
import "github.com/tebeka/selenium"        // FORBIDDEN
import "github.com/playwright-community/playwright-go" // FORBIDDEN
import "github.com/go-rod/rod"             // FORBIDDEN

// ❌ Creating your own browser context
ctx, cancel := chromedp.NewExecAllocator(...) // Use UITestContext!
```

### Test Infrastructure

```go
// ❌ Not using UITestContext
func TestBad(t *testing.T) {
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()
    // ... direct chromedp usage without UITestContext
}

// ❌ Custom test helpers that duplicate UITestContext
type MyTestContext struct { ... }  // Use UITestContext!

// ❌ Custom screenshot/logging functions
func myScreenshot(ctx context.Context, ...) { ... }  // Use utc.Screenshot()!
```

### Assertions

```go
// ❌ Using different assertion libraries
import "github.com/onsi/gomega"  // Use testify!

// ❌ Manual assertions without testify
if result != expected {
    t.Errorf("...")  // Use assert.Equal!
}
```

### Missing Cleanup

```go
// ❌ Missing defer cleanup
func TestBad(t *testing.T) {
    utc := NewUITestContext(t, timeout)
    // Missing: defer utc.Cleanup()
    // ... test code ...
}
```

## Checklist for New Tests

Before writing any new test:

- [ ] Read this document
- [ ] Study 2-3 existing tests in the target directory
- [ ] Identify reusable helpers from `uitest_context.go` (for UI) or `common/` (for API)
- [ ] Use the established patterns exactly

For UI tests specifically:

- [ ] Import `github.com/chromedp/chromedp`
- [ ] Use `NewUITestContext(t, timeout)`
- [ ] Add `defer utc.Cleanup()` immediately after creation
- [ ] Use `utc.Log()` for all logging
- [ ] Use `utc.Screenshot()` for screenshots
- [ ] Use `utc.Navigate()` for navigation
- [ ] Check all chromedp errors
- [ ] Use testify (assert/require) for assertions

## See Also

- `.claude/skills/monitoring/SKILL.md` - Detailed UI testing patterns
- `.claude/skills/go/SKILL.md` - Go code patterns
- `docs/architecture/ARCHITECTURE.md` - System architecture
