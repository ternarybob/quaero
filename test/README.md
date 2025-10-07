# Quaero Test Suite

## Directory Structure

```
test/
  ├── api/                   # API integration tests
  ├── ui/                    # UI/browser tests (end-to-end workflows)
  │   ├── config.go          # Test configuration
  │   ├── *_test.go          # ChromeDP-based UI tests
  │   └── ui_test.go         # Test utilities and helpers
  ├── unit/                  # Unit tests
  ├── results/               # Test results (timestamped directories)
  └── run-tests.ps1          # Test runner script (USE THIS)
```

## Running Tests

**IMPORTANT: ALL tests MUST be run through `run-tests.ps1`**

### Basic Usage

```powershell
# Run all tests (default)
cd test
./run-tests.ps1

# Run specific test type
./run-tests.ps1 -type unit
./run-tests.ps1 -type api
./run-tests.ps1 -type ui
./run-tests.ps1 -type all

# Run specific test with pattern matching
./run-tests.ps1 -type ui -script PageLayout
./run-tests.ps1 -script navbar

# Run with verbose output
./run-tests.ps1 -type ui -verboseoutput

# Run without coverage
./run-tests.ps1 -type ui -coverage:$false
```

### Test Types

- **`unit`** - Fast unit tests for individual components
- **`api`** - API integration tests with database interactions
- **`ui`** - Browser-based end-to-end tests (builds and starts server automatically)
- **`all`** - All test types (default)

## Test Results

Results are automatically organized in timestamped directories:

```
results/
  ├── unit-2025-10-07_14-30-15/
  │   ├── test-output.log
  │   └── coverage.out
  ├── api-2025-10-07_14-35-22/
  │   ├── test-output.log
  │   └── coverage.out
  ├── ui-2025-10-07_14-40-10/
  │   ├── test-output.log
  │   ├── coverage.out
  │   ├── 01_navbar_home.png
  │   ├── 02_navbar_jira_data.png
  │   └── ...
  └── ui-PageLayout-2025-10-07_14-57-36/
      ├── test-output.log
      ├── coverage.out
      └── 06_navbar_settings.png
```

**Format:** `{test-type}-{script-filter}-{yyyy-MM-dd_HH-mm-ss}/`

## UI Tests

UI tests use ChromeDP to automate browser interactions and verify the web interface.

### Automatic Server Management

**The test runner automatically:**
1. Builds the application using `./scripts/build.ps1`
2. Reads configuration from `bin/quaero.toml` to get the server port
3. Starts the Quaero server in the background
4. Waits for the server to be ready
5. Runs the tests
6. Stops the server when tests complete

**No manual server setup required!**

### Available UI Tests

- **PageLayoutConsistency** - Tests navbar, footer, and service status consistency across all pages
- **JiraCompleteWorkflow** - Complete Jira data collection workflow
- **ConfluenceCompleteWorkflow** - Complete Confluence data collection workflow
- **ConfluenceCascade** - Tests cascading Confluence operations
- **JiraGetIssues** - Tests Jira issue retrieval

### Screenshots

UI tests automatically capture screenshots at key steps. Screenshots are saved to the timestamped results directory with numbered prefixes (e.g., `01_navbar_home.png`, `02_navbar_jira_data.png`) for sequential ordering.

## API Tests

API tests verify server endpoints and database interactions. They test the REST API functionality without requiring browser automation.

## Writing Tests

### Unit Tests

Unit tests should be colocated with the code they test:

```
internal/services/atlassian/
  ├── auth_service.go
  └── auth_service_test.go        ← Same directory
```

Run with: `go test ./internal/services/atlassian/`

### API Tests

Add to `test/api/`:

```go
package api

import "testing"

func TestAPIFeature(t *testing.T) {
    // Test setup
    // HTTP request/response testing
    // Database interaction verification
    // Assertions
}
```

Run with: `./run-tests.ps1 -type api`

### UI Tests

Add to `test/ui/`:

```go
package ui

import (
    "testing"
    "github.com/chromedp/chromedp"
)

func TestUIFeature(t *testing.T) {
    config, _ := LoadTestConfig()
    serverURL := config.ServerURL

    // ChromeDP test logic
    // Use takeScreenshot() helper for screenshots
    // Test navigation, form interactions, etc.
}
```

Run with: `./run-tests.ps1 -type ui`

## Best Practices

1. **Always use run-tests.ps1** - Never run `go test` directly
2. **Use specific test types** - Run `-type ui` for UI tests, `-type api` for API tests
3. **Filter with -script** - Use pattern matching to run specific tests
4. **Check results directory** - Review screenshots and logs after tests
5. **Clean up regularly** - Old result directories can be deleted
6. **Use descriptive test names** - Follow Go naming conventions

## Common Test Commands

```powershell
# Run all UI tests
./run-tests.ps1 -type ui

# Run only page layout tests
./run-tests.ps1 -type ui -script PageLayout

# Run navbar-related tests across all types
./run-tests.ps1 -script navbar

# Run API tests with verbose output
./run-tests.ps1 -type api -verboseoutput
```

## Troubleshooting

### Build Fails

**Problem:** Build script fails during test setup

**Solution:**
```powershell
# Check build manually
cd ..
./scripts/build.ps1
```

### UI Tests Fail with "Server not ready"

**Problem:** Server failed to start or took too long

**Solution:** Check if port is already in use:
```powershell
# Check what's using port 8085
netstat -an | findstr :8085

# Kill existing quaero process
Get-Process quaero -ErrorAction SilentlyContinue | Stop-Process -Force
```

### Screenshots Not Captured

**Problem:** `TEST_RUN_DIR` not set or ChromeDP issues

**Solution:** Always use `run-tests.ps1` - it sets environment variables automatically

### Tests Pass But No Results

**Problem:** Running `go test` directly instead of through script

**Solution:** Use `./run-tests.ps1 -type ui`
