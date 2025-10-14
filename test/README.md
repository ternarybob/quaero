# Quaero Test Infrastructure

This directory contains integration and UI tests for Quaero.

## Test Organization

Tests are organized by category in subdirectories:

```
test/
├── api/              # API integration tests
├── ui/               # UI browser automation tests
│   ├── *_test.go     # UI test files
│   ├── screenshot_helper.go  # Screenshot capture utility
│   └── browser_validation_test.go  # Browser automation validation
├── runner/           # Test runner executable
│   ├── main.go       # Test runner main
│   └── testserver.go # Local test server (port 3333)
├── helpers.go        # Shared test utilities (imported by api/ and ui/)
└── results/          # Test output directories
    └── {testname}-{datetime}/  # Individual test run results
        ├── test.log           # Test output
        ├── summary.txt        # Test summary
        └── screenshots/       # Screenshots (UI tests only)
```

### Why helpers.go is in test/

The `helpers.go` file is located in `test/` (not `test/runner/`) because:
- It's imported by both `test/api/` and `test/ui/` packages
- The runner is `package main` (executable), so it can't be imported
- Keeping it in `test/` as `package test` allows api and ui packages to import it
- Contains shared HTTP testing utilities used across all test suites

## Prerequisites

**Important:** The test runner handles everything automatically!

The test runner will:
1. Start a local test server on port 3333 (for browser validation)
2. Check connectivity (local and internet)
3. Build the application using `scripts/build.ps1`
4. Start the service in a **visible window** titled "Quaero Service"
5. Wait for service readiness on `http://localhost:8085`
6. Run all test suites
7. Save results to `test/results/{testname}-{datetime}/`
8. Capture screenshots for UI tests
9. Stop the service and cleanup

**You do NOT need to:**
- Manually build the application
- Manually start the service
- Manually stop the service

The runner handles the complete test lifecycle.

## Running Tests

### Using the Test Runner (Recommended)

```powershell
cd test/runner
go run main.go
```

This will:
- Run all test suites (API + UI)
- Save output to `test/results/{testname}-{datetime}.log`
- Display a summary of results

### Running Tests Directly

```powershell
# Run all API tests
cd test
go test -v ./api

# Run all UI tests
cd test
go test -v ./ui

# Run specific test
cd test
go test -v ./api -run TestChatHealth
```

### Using Environment Variables

By default, tests connect to `http://localhost:8085`. To test against a different URL:

```powershell
$env:TEST_SERVER_URL="http://localhost:8080"
cd test/runner
go run main.go
```

## Configuration

Tests connect to a **running service** configured via `bin/quaero.toml`.

**CRITICAL**: Tests do NOT build or start the service automatically. You must:

1. **Build the application**
   ```powershell
   .\scripts\build.ps1
   ```

2. **Start the service** (in a separate window)
   ```powershell
   .\scripts\build.ps1 -Run
   # OR
   .\bin\quaero.exe serve
   ```

3. **Run tests**
   ```powershell
   cd test/runner
   go run main.go
   ```

### How Tests Find the Service

Tests automatically detect the service URL using this priority:

1. **Environment variable** (highest priority)
   ```powershell
   $env:TEST_SERVER_URL="http://localhost:8080"
   ```

2. **bin/quaero.toml** (reads host and port from config)

3. **Default** - `http://localhost:8085` (if config not found)

### Database Conflicts

**WARNING**: Tests and dev service may conflict if they share the same database.

Solutions:
- Use separate database paths in `bin/quaero.toml` for testing
- Or ensure dev service is stopped before running tests
- Or use TEST_SERVER_URL to point to a different service instance

## Test Results

All test output is saved to timestamped directories:
```
test/results/{testname}-{datetime}/
├── test.log        # Full test output (all test details)
└── screenshots/    # Screenshots from UI tests (if applicable)
    ├── homepage-{timestamp}.png
    ├── navigation-sources-{timestamp}.png
    ├── navigation-chat-{timestamp}.png
    └── ...
```

Example directory structure:
```
test/results/
├── api_tests-2025-10-14_22-15-30/
│   └── test.log
└── ui_tests-2025-10-14_22-16-45/
    ├── test.log
    └── screenshots/
        ├── homepage-2025-10-14_22-16-46.png
        ├── chat-page-2025-10-14_22-16-50.png
        ├── navigation-sources-2025-10-14_22-17-00.png
        ├── navigation-jobs-2025-10-14_22-17-05.png
        ├── navigation-documents-2025-10-14_22-17-10.png
        └── navigation-chat-2025-10-14_22-17-15.png
```

### Browser Validation

The test runner includes a browser validation test that runs against a local test server (port 3333) to verify:
- ChromeDP/browser automation is working correctly
- Browser can load pages and interact with elements
- This isolates browser issues from Quaero site issues

If browser validation passes but Quaero UI tests fail, the issue is with the Quaero site, not the browser automation.

## Writing Tests

### API Tests

API tests should:
- Be in `test/api/` directory
- Use `package api`
- Use the `test.NewHTTPTestHelper(t, baseURL)` helper
- Connect to the running service

Example:
```go
package api

import (
    "net/http"
    "os"
    "testing"
    "github.com/ternarybob/quaero/test"
)

func getTestServerURL() string {
    if url := os.Getenv("TEST_SERVER_URL"); url != "" {
        return url
    }
    return "http://localhost:8085"
}

func TestMyAPI(t *testing.T) {
    h := test.NewHTTPTestHelper(t, getTestServerURL())

    resp, err := h.GET("/api/endpoint")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    h.AssertStatusCode(resp, http.StatusOK)
}
```

### UI Tests

UI tests should:
- Be in `test/ui/` directory
- Use `package ui`
- Use ChromeDP for browser automation
- Connect to the running service

## Test Utilities

### HTTPTestHelper

Helper for making HTTP requests and assertions:

```go
h := test.NewHTTPTestHelper(t, baseURL)

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

## Troubleshooting

### Tests Fail to Connect

**Problem**: Tests fail with "connection refused" or timeout errors.

**Solution**: Ensure the service is running:
```powershell
# In a separate window
.\bin\quaero.exe serve

# Or
.\scripts\build.ps1 -Run
```

### Port Already in Use

**Problem**: Service fails to start because port 8085 is in use.

**Solution**:
1. Check what's using the port: `netstat -ano | findstr :8085`
2. Kill the process or change the port in `bin/quaero.toml`

### Tests Pass But Service Not Working

**Problem**: Tests pass but manual testing shows issues.

**Solution**: Tests may be using mock/test data. Verify:
1. Service is running with correct config (`bin/quaero.toml`)
2. Database is populated with test data
3. All dependencies are properly configured

## CI/CD Integration

For automated testing:

```powershell
# Build
.\scripts\build.ps1

# Start service in background
Start-Process -FilePath ".\bin\quaero.exe" -ArgumentList "serve" -NoNewWindow

# Wait for service to be ready
Start-Sleep -Seconds 5

# Run tests
cd test/runner
go run main.go

# Cleanup
Stop-Process -Name "quaero"
```

## See Also

- [Build Guide](../scripts/README.md) - Building and deployment
- [CLAUDE.md](../CLAUDE.md) - Development standards and architecture
