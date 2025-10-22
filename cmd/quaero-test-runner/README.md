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

## CLI Flags

The test runner now supports CLI flags for selective test execution and discovery.

### Available Flags

- **`--suite <api|ui|all>`** - Filters which test directories to run
  - `api`: Run only API integration tests
  - `ui`: Run only UI browser tests
  - `all`: Run all tests (default)
  
- **`--test <pattern>`** - Go test pattern for `-run` flag
  - Supports regex patterns for matching test function names
  - Examples: `TestAuth`, `TestAuth.*`, `Test(Auth|Config)`
  
- **`--list`** - List available test suites and exit
  - Displays descriptions of all available test suites
  - Shows example usage for each suite
  
- **`--help`** - Show usage information

### Common Usage Examples

```powershell
# Example 1: List available test suites
.\bin\quaero-test-runner.exe --list

# Example 2: Run only API tests
.\bin\quaero-test-runner.exe --suite api

# Example 3: Run only UI tests
.\bin\quaero-test-runner.exe --suite ui

# Example 4: Run specific test by name
.\bin\quaero-test-runner.exe --suite api --test TestAuthList

# Example 5: Run tests matching pattern
.\bin\quaero-test-runner.exe --suite api --test "TestAuth.*"

# Example 6: Run specific UI test
.\bin\quaero-test-runner.exe --suite ui --test TestHeroSectionConsistency

# Example 7: Run job rerun tests
.\bin\quaero-test-runner.exe --suite api --test TestJobRerun

# Example 8: Run all tests (default behavior)
.\bin\quaero-test-runner.exe
# or explicitly:
.\bin\quaero-test-runner.exe --suite all
```

### Running from Source with Flags

You can use CLI flags when running from source:

```powershell
# Run from source with flags
cd cmd/quaero-test-runner
go run . --suite api --test TestAuthList

# Note: If you encounter issues with flag parsing, use -- separator
go run . -- --suite api --test TestAuth
```

### Test Pattern Matching

The `--test` flag uses Go's `-run` regex pattern matching. Here are common patterns:

| Pattern | Matches |
|---------|---------|
| `TestAuth` | TestAuth, TestAuthList, TestAuthentication, etc. |
| `^TestAuth$` | Only TestAuth (exact match) |
| `TestAuth.*List` | TestAuthList, TestAuthStatusList, etc. |
| `Test(Auth\|Config)` | TestAuth*, TestConfig*, etc. |
| `TestAuth/subtest` | Specific subtest within TestAuth |

**Examples:**

```powershell
# Match all authentication tests
.\bin\quaero-test-runner.exe --suite api --test "TestAuth"

# Match exact test name
.\bin\quaero-test-runner.exe --suite api --test "^TestAuthList$"

# Match tests ending with "Health"
.\bin\quaero-test-runner.exe --suite api --test ".*Health$"

# Match multiple test prefixes
.\bin\quaero-test-runner.exe --suite api --test "Test(Auth|Config|Job)"
```

📖 For advanced patterns, see [Go testing documentation](https://golang.org/pkg/testing/#hdr-Subtests_and_Sub_benchmarks).

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

### ✅ Recommended: Use CLI Flags

The **recommended approach** is to use the test runner's `--suite` and `--test` flags for selective test execution:

```powershell
# Run specific API test
.\bin\quaero-test-runner.exe --suite api --test TestAuthList

# Run specific UI test
.\bin\quaero-test-runner.exe --suite ui --test TestHeroSectionConsistency

# Run tests matching pattern
.\bin\quaero-test-runner.exe --suite api --test "TestAuth.*"
```

**Advantages:**
- ✅ Automatic service management (no manual startup)
- ✅ Proper environment setup (TEST_SERVER_URL, TEST_MODE)
- ✅ Screenshots saved to timestamped results directory
- ✅ Test logs captured automatically
- ✅ Works with both mock and integration modes

### Alternative: Manual go test (Advanced)

For advanced debugging, you can run tests directly WITHOUT the test runner:

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

### Comparison: Test Runner Flags vs Manual go test

| Approach | Service Management | Environment Setup | Screenshots | When to Use |
|----------|-------------------|-------------------|-------------|-------------|
| `--suite` + `--test` flags | ✅ Automatic | ✅ Automatic | ✅ Saved | Development, CI/CD, standard use |
| Manual `go test` | ❌ Manual | ⚠️ Manual | ⚠️ Lost | Advanced debugging only |

**When to use test runner flags:**
- ✅ Standard development workflow
- ✅ Running specific tests during feature development
- ✅ CI/CD pipelines
- ✅ Pre-commit validation
- ✅ When you want automated service management

**When to use manual go test:**
- 🔧 Deep debugging with IDE integration
- 🔧 Custom test flags (e.g., `-bench`, `-cpuprofile`)
- 🔧 Test development with rapid iteration

## Exit Codes

The test runner uses standard exit codes to indicate different outcomes:

| Exit Code | Meaning | When It Occurs |
|-----------|---------|----------------|
| **0** | Success | All tests passed |
| **1** | Test failure | One or more tests failed |
| **2** | No tests executed | Suite/mode configuration resulted in no tests running |

### Exit Code 2 Scenarios

Exit code **2** indicates that no tests were executed, which can occur when:

1. **UI tests requested in mock mode**
   ```powershell
   # Config has test_mode = "mock"
   .\bin\quaero-test-runner.exe --suite ui
   # Exit 2: UI tests require integration mode
   ```

2. **Test pattern matches no tests**
   ```powershell
   .\bin\quaero-test-runner.exe --suite api --test "TestNonExistent"
   # Exit 2: No tests match the pattern
   ```

3. **Suite/mode combination filters out all tests**
   ```powershell
   # Config has test_mode = "mock" and --suite all
   # Exit 2: Only API tests run, but if API path doesn't exist
   ```

**Note:** Exit code 2 is **not** a test failure - it indicates a configuration issue where the runner had nothing to execute.

## Troubleshooting

### No Tests Run When Using --test Flag

**Symptoms:** Test runner reports "no tests to run" or shows 0 tests executed

**Solution:**
1. Check test function name spelling (case-sensitive)
2. Verify regex pattern syntax: `--test "TestAuth.*"`
3. List available tests: `go test -v ./test/api -list ".*"`
4. Try without pattern first: `--suite api` to see all API tests

### UI Tests Skipped in Mock Mode

**Symptoms:** "WARNING: UI tests require integration mode" message

**Solution:**
UI tests require `test_mode = "integration"` in config file:

```toml
[test_runner]
test_mode = "integration"  # Change from "mock"
```

Or run API tests only: `--suite api`


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

### Fast CI Feedback with Test Suites

Run API tests first (fast), then UI tests (slower) for optimal CI feedback:

**GitHub Actions Example with Matrix Strategy:**

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: windows-latest
    strategy:
      matrix:
        suite: [api, ui]
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run ${{ matrix.suite }} Tests
        run: |
          cd cmd/quaero-test-runner
          go run . --suite ${{ matrix.suite }}
      
      - name: Upload Test Results
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: test-results-${{ matrix.suite }}
          path: test/results/
```

**GitHub Actions Example with Sequential Stages:**

```yaml
- name: Run API Tests (Fast)
  run: |
    cd cmd/quaero-test-runner
    go run . --suite api

- name: Run UI Tests (Slower)
  if: success()
  run: |
    cd cmd/quaero-test-runner
    go run . --suite ui
```

**Azure Pipelines Example:**

```yaml
- script: |
    cd cmd/quaero-test-runner
    go run . --suite api
  displayName: 'Run API Tests'

- script: |
    cd cmd/quaero-test-runner
    go run . --suite ui
  displayName: 'Run UI Tests'
  condition: succeeded()
```

**Jenkins Example:**

```groovy
stage('API Tests') {
    steps {
        dir('cmd/quaero-test-runner') {
            bat 'go run . --suite api'
        }
    }
}
stage('UI Tests') {
    when { expression { currentBuild.result == 'SUCCESS' } }
    steps {
        dir('cmd/quaero-test-runner') {
            bat 'go run . --suite ui'
        }
    }
}
```

### Running Specific Tests in CI

```yaml
# Run smoke tests only
- name: Smoke Tests
  run: |
    cd cmd/quaero-test-runner
    go run . --suite api --test "TestAuth.*|TestHealth.*"
```

## Quick Reference

### CLI Flags Summary

| Flag | Values | Default | Description |
|------|--------|---------|-------------|
| `--suite` | api, ui, all | all | Test suite to run |
| `--test` | Go regex pattern | (none) | Filter tests by name pattern |
| `--list` | (boolean) | false | List available suites and exit |
| `--help` | (boolean) | false | Show usage information |

### Common Commands

```powershell
# Discovery
--list                              # List test suites

# Run all tests
(no flags)                          # Default: all tests
--suite all                         # Explicit: all tests

# Run by suite
--suite api                         # API tests only
--suite ui                          # UI tests only

# Run specific tests
--suite api --test TestAuth         # Tests matching "TestAuth"
--suite api --test "^TestAuth$"     # Exact match "TestAuth"
--suite api --test "TestAuth.*"     # Tests starting with "TestAuth"
--suite ui --test TestHero          # UI tests matching "TestHero"

# Pattern matching
--suite api --test "Test(Auth|Job)" # Match multiple prefixes
--suite api --test ".*Health$"      # Match suffix
```

### Test Mode Behavior

| Mode | API Tests | UI Tests | Backend |
|------|-----------|----------|---------|
| **mock** | ✅ Runs | ⚠️ Skipped | In-memory mock |
| **integration** | ✅ Runs | ✅ Runs | Real service + database |

**Note:** Use `--suite api` in mock mode to avoid UI test warnings.

## Summary

**✅ DO:**
- Use CLI flags for selective test execution: `--suite api --test TestAuth`
- Run the test runner: `cd cmd/quaero-test-runner && go run .`
- Let the test runner build and start the service
- Use `--list` to discover available test suites
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
- [Go Testing Documentation](https://golang.org/pkg/testing/) - Test pattern matching
