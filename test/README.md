# Quaero Test Suite

## Directory Structure

```
test/
  ├── integration/          # Integration tests (component interaction)
  ├── ui/                   # UI/browser tests (end-to-end workflows)
  │   ├── config.go         # Test configuration
  │   ├── test_config.toml  # Server URL configuration
  │   └── *.go              # ChromeDP-based UI tests
  ├── results/              # Test results (timestamped directories)
  └── run-tests.ps1         # Test runner script (USE THIS)
```

## Running Tests

**IMPORTANT: ALL tests MUST be run through `run-tests.ps1`**

### Basic Usage

```powershell
# Run integration tests (default)
cd test
./run-tests.ps1

# Run UI tests (requires server running)
./run-tests.ps1 -Type ui

# Run all tests
./run-tests.ps1 -Type all

# Run with verbose output
./run-tests.ps1 -Type integration -VerboseOutput

# Run without coverage
./run-tests.ps1 -Type integration -Coverage:$false
```

### Test Types

- **`integration`** - Component interaction tests (fast, no server required)
- **`ui`** - Browser-based end-to-end tests (requires server running)
- **`all`** - All test types

## Test Results

Results are automatically organized in timestamped directories:

```
results/
  ├── integration-2025-10-05_14-30-15/
  │   └── coverage.out
  ├── ui-2025-10-05_14-35-22/
  │   ├── 01_page_loaded.png
  │   ├── 02_button_clicked.png
  │   └── ...
  └── all-2025-10-05_14-40-10/
      └── ...
```

**Format:** `{test-type}-{yyyy-MM-dd_HH-mm-ss}/`

## UI Tests

UI tests use ChromeDP to automate browser interactions and verify the web interface.

### Prerequisites

1. **Start the Quaero server:**
   ```powershell
   cd bin
   ./quaero.exe serve -c ../deployments/local/quaero.toml
   ```

2. **Configure server URL** (optional):
   Edit `test/ui/test_config.toml`:
   ```toml
   server_url = "http://localhost:8080"
   ```

   Or set environment variable:
   ```powershell
   $env:TEST_SERVER_URL = "http://localhost:8080"
   ```

### Available UI Tests

- **Jira Workflow** - Complete Jira data collection workflow
- **Confluence Workflow** - Complete Confluence data collection workflow

### Screenshots

UI tests automatically capture screenshots at key steps. Screenshots are saved to the timestamped results directory with numbered prefixes for sequential ordering.

## Integration Tests

Integration tests verify component interactions without requiring a running server. They use in-memory databases and mock HTTP servers.

## Writing Tests

### Unit Tests

Unit tests should be colocated with the code they test:

```
internal/services/atlassian/
  ├── auth_service.go
  └── auth_service_test.go        ← Same directory
```

Run with: `go test ./internal/services/atlassian/`

### Integration Tests

Add to `test/integration/`:

```go
package integration

import "testing"

func TestFeature(t *testing.T) {
    // Test setup
    // Component interaction tests
    // Assertions
}
```

Run with: `./run-tests.ps1 -Type integration`

### UI Tests

Add to `test/ui/`:

```go
package ui

import (
    "testing"
    "github.com/chromedp/chromedp"
)

func TestFeature(t *testing.T) {
    config, _ := LoadTestConfig()
    serverURL := config.ServerURL

    // ChromeDP test logic
    // Use takeScreenshot() helper for screenshots
}
```

Run with: `./run-tests.ps1 -Type ui`

## Best Practices

1. **Always use run-tests.ps1** - Never run `go test` directly
2. **Check results directory** - Review screenshots and logs after UI tests
3. **Clean up regularly** - Old result directories can be deleted
4. **Server must be running** - UI tests require a live server
5. **Use descriptive test names** - Follow Go naming conventions

## Troubleshooting

### UI Tests Fail with Connection Refused

**Problem:** Server isn't running or wrong port

**Solution:**
```powershell
# Check server is running
curl http://localhost:8080

# Start server if needed
cd bin
./quaero.exe serve
```

### Screenshots Not Captured

**Problem:** `TEST_RUN_DIR` not set

**Solution:** Always use `run-tests.ps1` - it sets this automatically

### Tests Pass But No Results

**Problem:** Running `go test` directly instead of through script

**Solution:** Use `./run-tests.ps1 -Type ui`
