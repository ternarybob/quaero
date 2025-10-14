# Quaero Testing Infrastructure

This directory contains the Go-native testing infrastructure for Quaero.

## Test Structure

```
test/
├── main_test.go          # Integration test fixture (setup/teardown)
├── helpers.go            # Common test utilities
├── run_tests.go          # Go-native test runner
├── api/                  # API integration tests
│   ├── sources_api_test.go
│   └── chat_api_test.go
├── ui/                   # UI tests (chromedp)
│   ├── homepage_test.go
│   └── chat_test.go
├── results/              # Test results (timestamped)
└── archive/              # Archived old tests
```

## Running Tests

### Run All Tests

```bash
cd test
go run run_tests.go
```

### Run Specific Test Suite

**API Tests:**
```bash
cd test
go test -v ./api
```

**UI Tests:**
```bash
cd test
go test -v ./ui
```

### Run Individual Test

```bash
cd test
go test -v ./api -run TestListSources
```

## Test Types

### 1. API Tests (`test/api/`)

Integration tests that make HTTP requests to the running server.

**Features:**
- Tests all REST API endpoints
- CRUD operations verification
- Request/response validation
- Error handling verification

**Example:**
```go
func TestCreateSource(t *testing.T) {
    h := test.NewHTTPTestHelper(t)

    source := map[string]interface{}{
        "name": "Test Source",
        "type": "jira",
    }

    resp, err := h.POST("/api/sources", source)
    if err != nil {
        t.Fatalf("Failed: %v", err)
    }

    h.AssertStatusCode(resp, http.StatusOK)
}
```

### 2. UI Tests (`test/ui/`)

Browser automation tests using chromedp.

**Features:**
- Page load verification
- Element presence checks
- Navigation testing
- JavaScript functionality

**Example:**
```go
func TestHomepageTitle(t *testing.T) {
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    var title string
    err := chromedp.Run(ctx,
        chromedp.Navigate(test.GetTestServerURL()),
        chromedp.Title(&title),
    )

    // Assertions...
}
```

## Test Fixture

The `main_test.go` file provides a `TestMain` function that:

1. **Setup:**
   - Creates test data directory
   - Initializes test configuration (port 18085)
   - Starts the server in background
   - Waits for server readiness

2. **Runs all tests**

3. **Teardown:**
   - Stops the server gracefully
   - Cleans up test data directory

## Test Utilities

### HTTPTestHelper

Helper for making HTTP requests and assertions:

```go
h := test.NewHTTPTestHelper(t)

// Make requests
resp, err := h.GET("/api/status")
resp, err := h.POST("/api/sources", sourceData)
resp, err := h.PUT("/api/sources/123", updatedData)
resp, err := h.DELETE("/api/sources/123")

// Assertions
h.AssertStatusCode(resp, http.StatusOK)
h.AssertJSONField(resp, "status", "success")

// Parse JSON
var result map[string]interface{}
h.ParseJSONResponse(resp, &result)
```

### Retry Helper

Retry operations with exponential backoff:

```go
err := test.Retry(func() error {
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    if resp.StatusCode != 200 {
        return fmt.Errorf("not ready")
    }
    return nil
}, 10, 500*time.Millisecond)
```

## Configuration

Tests use a separate configuration:
- **Port:** 18085 (avoids conflicts with dev server)
- **Database:** `./testdata/test_quaero.db` (temporary)
- **LLM Mode:** mock (no real LLM required)

## Test Results

Test results are saved to timestamped directories:

```
test/results/
└── run-2025-10-14_20-30-45/
    ├── api_tests.log
    └── ui_tests.log
```

## Writing New Tests

### API Test Template

```go
package api

import (
    "net/http"
    "testing"
    "github.com/ternarybob/quaero/test"
)

func TestMyFeature(t *testing.T) {
    h := test.NewHTTPTestHelper(t)

    // Arrange
    data := map[string]interface{}{
        "field": "value",
    }

    // Act
    resp, err := h.POST("/api/endpoint", data)
    if err != nil {
        t.Fatalf("Failed: %v", err)
    }

    // Assert
    h.AssertStatusCode(resp, http.StatusOK)

    var result map[string]interface{}
    h.ParseJSONResponse(resp, &result)

    if result["status"] != "success" {
        t.Errorf("Expected success, got: %v", result)
    }
}
```

### UI Test Template

```go
package ui

import (
    "context"
    "testing"
    "time"
    "github.com/chromedp/chromedp"
    "github.com/ternarybob/quaero/test"
)

func TestMyPage(t *testing.T) {
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    url := test.GetTestServerURL() + "/mypage"

    var title string
    err := chromedp.Run(ctx,
        chromedp.Navigate(url),
        chromedp.WaitVisible(`body`, chromedp.ByQuery),
        chromedp.Title(&title),
    )

    if err != nil {
        t.Fatalf("Failed: %v", err)
    }

    if title != "Expected Title" {
        t.Errorf("Wrong title: %s", title)
    }
}
```

## Best Practices

1. **Use table-driven tests** for multiple similar test cases
2. **Clean up resources** (delete created entities)
3. **Use descriptive test names** (TestFeature_Scenario_ExpectedResult)
4. **Log useful information** with `t.Logf()`
5. **Fail fast** with `t.Fatalf()` for setup failures
6. **Use subtests** with `t.Run()` for organization

## Troubleshooting

### Tests Fail to Start Server

- Check if port 18085 is already in use
- Verify database path is writable
- Check logs for initialization errors

### UI Tests Fail

- Ensure chromedp is installed: `go get github.com/chromedp/chromedp`
- Chrome/Chromium must be installed
- Increase timeouts if tests are flaky

### Tests are Slow

- Run specific suites instead of all tests
- Use `-short` flag to skip long-running tests
- Parallelize independent tests with `t.Parallel()`

## CI/CD Integration

To integrate with CI/CD:

```bash
# In your CI pipeline
cd test
go run run_tests.go

# Or use Go's native test runner
go test -v ./...
```

The test runner exits with code 0 on success, non-zero on failure.
