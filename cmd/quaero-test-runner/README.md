# Quaero Test Runner

**The test runner handles EVERYTHING automatically - do NOT run build scripts or start the service manually!**

## What the Test Runner Does

The test runner is a complete test automation system that:

1. ✅ **Builds the application** using `scripts/build.ps1`
2. ✅ **Starts a test server** on port 3333 for browser validation
3. ✅ **Starts the Quaero service** in a visible window
4. ✅ **Waits for service readiness**
5. ✅ **Runs all test suites** (API + UI)
6. ✅ **Captures screenshots** for UI tests
7. ✅ **Saves results** to timestamped directories
8. ✅ **Stops the service** and cleans up

## ⚠️ CRITICAL: What NOT to Do

### ❌ DO NOT run build.ps1 manually before testing:
```powershell
# ❌ WRONG - Don't do this!
.\scripts\build.ps1
cd test/runner && go run main.go

# ❌ WRONG - Don't do this either!
.\scripts\build.ps1 -Run
cd test/runner && go run main.go
```

### ❌ DO NOT start the service manually:
```powershell
# ❌ WRONG - Don't do this!
.\bin\quaero.exe serve
cd test/runner && go run main.go
```

**Why?** The test runner needs to control the service lifecycle. Manual builds/starts will cause conflicts.

## ✅ How to Run Tests

### Option 1: Use Pre-Built Test Runner (Recommended)

```powershell
# The test runner is built automatically by build.ps1
.\scripts\build.ps1

# Now run the test runner (it will build and start service automatically)
cd bin
.\quaero-test-runner.exe
```

### Option 2: Run from Source

```powershell
# Run directly from source (no need to build first!)
cd cmd/quaero-test-runner
go run .
```

**That's it!** The test runner handles everything.

## Configuration

Edit `bin/quaero-test-runner.toml` or `cmd/quaero-test-runner/quaero-test-runner.toml`:

```toml
[test_runner]
# Where test files are located (discovers api/ and ui/ subdirectories automatically)
tests_dir = "./test"

# Where test results will be saved
# Structure: {output_dir}/{test-name}-{datetime}/
output_dir = "./test/results"

# Build script to use (test runner calls this)
build_script = "./scripts/build.ps1"  # Windows
# build_script = "./scripts/build.sh" # Linux/Mac

[test_server]
# Port for browser automation test server
port = 3333

[service]
# Service binary path (built by test runner)
binary = "./bin/quaero.exe"

# Service configuration file
config = "./bin/quaero.toml"

# How long to wait for service to start (seconds)
startup_timeout_seconds = 30

# Optional: Override service port (otherwise uses port from config file)
# port = 8085
```

## Test Results

Results are saved to timestamped directories:

```
test/results/
├── api_tests-2025-10-15_14-30-00/
│   └── test.log                     # Full test output
│
└── ui_tests-2025-10-15_14-32-15/
    ├── test.log                     # Full test output
    └── screenshots/                 # UI test screenshots
        ├── hero-consistency-Home-2025-10-15_14-32-16.png
        ├── hero-consistency-Sources-2025-10-15_14-32-17.png
        ├── hero-consistency-Jobs-2025-10-15_14-32-18.png
        ├── hero-consistency-Documents-2025-10-15_14-32-19.png
        ├── hero-consistency-Chat-2025-10-15_14-32-20.png
        ├── hero-consistency-Auth-2025-10-15_14-32-21.png
        ├── hero-consistency-Config-2025-10-15_14-32-22.png
        └── hero-consistency-Settings-2025-10-15_14-32-23.png
```

## Directory Structure

```
cmd/quaero-test-runner/
├── main.go                      # Test orchestration
├── testserver.go                # Test server for browser validation
├── quaero-test-runner.toml      # Configuration
└── README.md                    # This file

test/
├── helpers.go                   # Shared HTTP test utilities
├── api/                         # API integration tests
│   ├── *_api_test.go
│   └── ...
├── ui/                          # UI browser tests
│   ├── *_test.go
│   ├── screenshot_helper.go     # Screenshot utilities
│   └── hero_consistency_test.go # Hero section consistency test
└── results/                     # Test output (auto-created)
    └── {test-name}-{datetime}/
        ├── test.log
        └── screenshots/
```

## Why helpers.go is in test/ (NOT in cmd/quaero-test-runner/)

**CRITICAL ARCHITECTURAL DECISION:**

```go
// ✅ CORRECT: test/helpers.go
package test

// Used by:
import "github.com/ternarybob/quaero/test"
```

**Reasons:**
1. Both `test/api/` and `test/ui/` packages import it
2. Go packages are imported by their path - `test/` is the correct import path
3. **`cmd/quaero-test-runner/` is `package main`** - cannot be imported by other packages!
4. Test runner orchestrates, helpers provide utilities - clear separation of concerns
5. Standard Go practice for shared test utilities

```go
// ❌ WRONG: cmd/quaero-test-runner/helpers.go
package main

// Cannot be imported - package main is not importable!
// This would break test/api/ and test/ui/ imports
```

## Writing Tests

### API Tests

Located in `test/api/`, these test HTTP endpoints.

**Example:**
```go
package api

import (
    "testing"
    "github.com/ternarybob/quaero/test"
)

func TestMyAPI(t *testing.T) {
    h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

    resp, err := h.GET("/api/endpoint")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    h.AssertStatusCode(resp, http.StatusOK)
}
```

**Available HTTP methods:**
- `h.GET(path)`
- `h.POST(path, body)`
- `h.PUT(path, body)`
- `h.DELETE(path)`

**Assertions:**
- `h.AssertStatusCode(resp, statusCode)`
- `h.AssertJSONField(resp, "field", "value")`
- `h.ParseJSONResponse(resp, &result)`

### UI Tests

Located in `test/ui/`, these test browser interactions.

**Example:**
```go
package ui

import (
    "context"
    "testing"
    "github.com/chromedp/chromedp"
    "github.com/ternarybob/quaero/test"
)

func TestMyUI(t *testing.T) {
    serverURL := test.MustGetTestServerURL()

    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    // Navigate to page
    if err := chromedp.Run(ctx,
        chromedp.Navigate(serverURL+"/page"),
        chromedp.WaitReady("body"),
    ); err != nil {
        t.Fatalf("Navigation failed: %v", err)
    }

    // Take screenshot
    if err := TakeScreenshot(ctx, "my-test"); err != nil {
        t.Errorf("Screenshot failed: %v", err)
    }

    // Interact with elements
    var text string
    if err := chromedp.Run(ctx,
        chromedp.Text("#element-id", &text),
    ); err != nil {
        t.Fatalf("Failed to get text: %v", err)
    }

    if text != "Expected" {
        t.Errorf("Expected 'Expected', got '%s'", text)
    }
}
```

**Screenshot utilities:**
- `TakeScreenshot(ctx, name)` - Captures screenshot to results directory
- `GetScreenshotsDir()` - Gets current test run screenshot directory

## Running Individual Tests (Development)

For development/debugging, you can run tests directly WITHOUT the test runner:

```powershell
# ⚠️ WARNING: You must manually start the service first!

# 1. Build and start service in separate window
.\scripts\build.ps1 -Run

# 2. In another window, run tests directly
cd test
go test -v ./api                    # All API tests
go test -v ./ui                     # All UI tests
go test -v ./api -run TestChatHealth  # Specific test
go test -v ./ui -run TestHeroSectionConsistency  # Hero test
```

**When to use direct testing:**
- Debugging a specific test
- Rapid iteration during development
- Running a single test repeatedly

**When to use test runner:**
- CI/CD pipelines
- Full regression testing
- Before commits/PRs
- When you want automated service management

## Troubleshooting

### Test Runner Fails to Build

**Symptoms:** Build errors during test runner execution

**Solution:**
1. Check Go version: `go version` (requires 1.21+)
2. Run `go mod tidy` in project root
3. Check build script exists: `.\scripts\build.ps1`

### Service Fails to Start

**Symptoms:** Test runner reports "Service did not become ready"

**Solution:**
1. Check port 8085 is not in use: `netstat -an | findstr :8085`
2. Kill any existing processes: `taskkill /F /IM quaero.exe`
3. Check service configuration: `bin/quaero.toml`
4. Increase startup timeout in test runner config

### UI Tests Fail

**Symptoms:** Browser automation errors or screenshot issues

**Solution:**
1. Ensure Chrome/Chromium is installed
2. Check screenshots directory has write permissions
3. Review screenshots in `test/results/{test-name}-{datetime}/screenshots/`
4. Run test runner with `-v` for verbose output

### Port Conflicts

**Symptoms:** "Port already in use" errors

**Solution:**
1. **Test server (port 3333):** Change in test runner config
2. **Service (port 8085):** Change in `bin/quaero.toml`
3. Kill conflicting processes: `Get-Process | Where-Object {$_.ProcessName -like '*quaero*'} | Stop-Process -Force`

## Advanced Usage

### Custom Test Server URL

Override the service URL for testing against different instances:

```powershell
# Test against a different server
$env:TEST_SERVER_URL="http://localhost:9090"
cd cmd/quaero-test-runner
go run .
```

### Database Isolation

To avoid conflicts between test runs and dev instances:

1. Create separate database in `bin/quaero.toml`:
```toml
[database]
path = "./data/test.db"  # Separate from dev database
```

2. Or stop dev service before running tests

### Custom Build Scripts

Override the build script in test runner config:

```toml
[test_runner]
build_script = "./my-custom-build.ps1"
```

## CI/CD Integration

**GitHub Actions Example:**

```yaml
- name: Run Tests
  run: |
    cd cmd/quaero-test-runner
    go run .
```

**Azure Pipelines Example:**

```yaml
- script: |
    cd cmd/quaero-test-runner
    go run .
  displayName: 'Run Integration Tests'
```

**Jenkins Example:**

```groovy
stage('Test') {
    steps {
        dir('cmd/quaero-test-runner') {
            bat 'go run .'
        }
    }
}
```

## Summary

**✅ DO:**
- Run the test runner: `cd cmd/quaero-test-runner && go run .`
- Let the test runner build and start the service
- Use configuration file for paths and settings
- Review screenshots in results directories

**❌ DON'T:**
- Run `build.ps1` before the test runner
- Manually start the service before the test runner
- Hardcode paths in tests - use configuration
- Ignore test failures - check logs and screenshots

## See Also

- [CLAUDE.md](../../CLAUDE.md) - Development standards
- [README.md](../../README.md) - Project overview
- [test/helpers.go](../../test/helpers.go) - Shared test utilities
- [test/ui/screenshot_helper.go](../../test/ui/screenshot_helper.go) - Screenshot utilities
