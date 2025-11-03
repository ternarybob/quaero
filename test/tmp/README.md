# Quaero Test Infrastructure

## Overview

Quaero implements a two-tier test architecture supporting both **mock** and **integration** test modes:

- **Mock Mode**: Uses an in-memory mock server for fast, isolated unit testing
- **Integration Mode**: Tests against the real Quaero service for full-stack validation

The test runner (`cmd/quaero-test-runner/`) orchestrates test execution, managing service lifecycle, and collecting results.

## Test Modes

### Mock Mode (Default)

**When to use:**
- CI/CD pipelines requiring fast feedback
- Rapid development iterations
- Unit testing without external dependencies
- Testing API contracts without database state

**Characteristics:**
- Uses in-memory mock server on port 9999
- No real database or Quaero service required
- Fast execution (no build/startup overhead)
- Isolated and repeatable
- Perfect for testing API endpoint behavior

**How it works:**
- Mock server (`test/mock_server.go`) implements all API endpoints
- Stores data in memory maps (sources, auths, jobs)
- Returns predictable responses
- Automatic cleanup after tests

### Integration Mode

**When to use:**
- End-to-end system validation
- Pre-release testing
- Verifying full stack behavior
- Testing real database interactions
- Validating LLM integration

**Characteristics:**
- Uses real Quaero service on port 8085 (configurable)
- Tests full stack including database, LLM, and all services
- Slower (requires build and service startup)
- Tests actual production code paths

**How it works:**
- Test runner builds Quaero from source
- Starts service in visible console window
- Waits for service readiness
- Runs tests against live service
- Stops service after completion

## Running Tests

### Using Test Runner (Recommended)

The test runner handles everything automatically:

```powershell
# Build test runner (if not already built)
.\scripts\build.ps1

# Run tests (uses mode from config file)
cd bin
.\quaero-test-runner.exe
```

**Configuration:** Edit `bin/quaero-test-runner.toml` to change test mode:

```toml
[test_runner]
test_mode = "mock"  # or "integration"
```

### Direct Test Execution

For development or debugging:

```powershell
# Mock mode (no service needed)
$env:TEST_MODE="mock"
cd test
go test -v ./api

# Integration mode (requires service running)
$env:TEST_MODE="integration"
# Start service first in separate terminal:
.\scripts\build.ps1 -Run
# Then run tests:
cd test
go test -v ./api
```

### Running Specific Tests

```powershell
# Run single test
go test -v -run TestListSources ./test/api

# Run all API tests
go test -v ./test/api

# Run UI tests (always integration mode)
go test -v ./test/ui
```

## Test Organization

### Load Tests (`test/api/job_load_test.go`)

- **Purpose:** Validates database lock fixes under high-concurrency scenarios
- **Tests SQLITE_BUSY error prevention with retry logic**
- **Tests queue message deletion success rate**
- **Tests job hierarchy integrity under concurrent operations**
- **Tests worker pool staggering effectiveness**
- **Tests system throughput and performance metrics**

**Load Test Scenarios:**
- **Light Load:** 5 parent jobs × 20 child URLs (100 total jobs)
- **Medium Load:** 10 parent jobs × 50 child URLs (500 total jobs)  
- **Heavy Load:** 15 parent jobs × 100 child URLs (1500 total jobs)

**Critical Pass/Fail Criteria:**
- Zero SQLITE_BUSY errors (database lock resilience)
- 100% queue message deletion success rate
- 100% job hierarchy integrity maintenance
- Linear performance scaling validation

**Running Load Tests:**
```powershell
# Run light load test (100 jobs, ~5 minutes)
go test -v ./test/api -run TestJobLoadLight

# Run medium load test (500 jobs, ~10 minutes)
go test -v ./test/api -run TestJobLoadMedium

# Run heavy load test (1500 jobs, ~20 minutes)
go test -v ./test/api -run TestJobLoadHeavy

# Run all load tests (comprehensive validation)
go test -v ./test/api -run TestJobLoad
```

**Test Fixtures:**
Load tests use shared fixtures from `test_fixtures.go`:
- `LoadTestConfig()` - Load test configuration with cleanup
- `InitializeTestApp()` - Full application with real database
- `createLoadTestHTTPServer()` - Test HTTP server with configurable responses
- `validateJobHierarchy()` - Parent-child relationship validation
- `queryQueueMessageCount()` - Direct database queries for validation

**Results Documentation:**
Load test results are documented in `docs/redesign-job-queue-post/load-test-results.md`

**When to Run Load Tests:**
- Before major releases
- After making concurrency-related changes
- When validating database lock fixes
- As part of performance regression testing
- Monthly production readiness validation

```
test/
├── README.md              # This file
├── helpers.go             # HTTP test utilities, mode detection
├── mock_server.go         # Mock API server for isolated testing
├── api/                   # API endpoint tests
│   ├── sources_api_test.go
│   ├── auth_api_test.go
│   ├── job_load_test.go
│   ├── test_fixtures.go
│   └── ...
└── ui/                    # Browser automation tests (ChromeDP)
    ├── config.toml        # UI test configuration
    ├── quaero.toml        # Service configuration for tests
    ├── setup.go           # Test infrastructure and service lifecycle
    ├── main_test.go       # Test suite setup
    ├── homepage_test.go   # Homepage tests
    ├── job_deletion_modal_test.go  # Job deletion tests
    ├── bin/               # Built test binaries & data (gitignored)
    └── results/           # Test-specific result directories (gitignored)
        └── {TestName}-{datetime}/
            ├── service.log
            ├── test.log
            └── *.png
```

### API Tests (`test/api/`)

- Test HTTP endpoints and API contracts
- Support both mock and integration modes
- Use `test.MustGetTestServerURL()` to get correct server
- Automatic mode detection via TEST_MODE environment variable

**Example:**

```go
func TestExample(t *testing.T) {
    h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

    resp, err := h.GET("/api/sources")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    h.AssertStatusCode(resp, http.StatusOK)
}
```
### Test Fixtures (`test/api/test_fixtures.go`)

Shared helper functions for load testing and complex test scenarios:

**Configuration Helpers:**
- `LoadTestConfig()` - Load test configuration with cleanup function
- `InitializeTestApp()` - Initialize full application with all services

**Job Creation Helpers:**
- `createLoadTestJobDefinition()` - Generate job definition with specified child URL count
- `createLoadTestHTTPServer()` - Create HTTP server with configurable responses
- `createLoadTestSource()` - Generate source configuration for load testing

**Validation Helpers:**
- `validateJobHierarchy()` - Verify parent-child relationships remain intact
- `queryJobCount()` - Query crawl_jobs table for count by status
- `queryQueueMessageCount()` - Query goqite table for pending message count
- `waitForJobCompletion()` - Poll job until terminal status

**Database Access Helpers:**
- `getDirectDBConnection()` - Open direct SQLite connection for queries
- `parseLogForPattern()` - Parse log file for regex pattern matches
- `countLogOccurrences()` - Count occurrences of pattern in log file

**Usage Example:**
```go
func TestMyLoadScenario(t *testing.T) {
    config, cleanup := LoadTestConfig(t)
    defer cleanup()
    
    app := InitializeTestApp(t, config)
    defer app.Close()
    
    server := createLoadTestHTTPServer(t)
    defer server.Close()
    
    // Use helper functions for test setup
    jobDef := createLoadTestJobDefinition("test-job", "test-source", 50)
    
    // Use validation helpers
    err := validateJobHierarchy(ctx, app.StorageManager.JobStorage(), parentID, 50)
    if err != nil {
        t.Errorf("Job hierarchy validation failed: %v", err)
    }
}
```

**When to Use Fixtures:**
- Load testing scenarios requiring complex setup
- Tests needing temporary database isolation
- Validation of database operations under load
- Integration tests requiring full application context


### UI Tests (`test/ui/`)

Browser automation tests using ChromeDP with full service lifecycle management.

**Architecture:**
- **Self-contained**: Everything runs within `test/ui/` directory (CI/CD friendly)
- **Isolated**: Uses port 18085 (separate from dev server on 8085)
- **Fresh builds**: Builds Quaero binary using `go build` for each test run
- **No external scripts**: No dependency on `scripts/build.ps1` or shell scripts
- Each test manages its own service instance via `SetupTestEnvironment()`
- Configuration driven by `test/ui/setup.toml`
- Automatic build, service startup, port checking, and graceful shutdown
- Results saved to test-specific timestamped directories

**Running UI Tests:**

```powershell
# Run all UI tests
cd test/ui
go test -v

# Run specific test
go test -v -run TestHomepageTitle -timeout 5m

# Run with coverage
go test -v -coverprofile=coverage.out
```

**Test Infrastructure:**

Each test follows this pattern:

```go
func TestExample(t *testing.T) {
    // Setup: Starts service, creates results directory
    env, err := SetupTestEnvironment("TestName")
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }
    defer env.Cleanup()

    // Log to both console and test.log
    env.LogTest(t, "Starting test...")

    // Get service URL
    url := env.GetBaseURL()

    // Use ChromeDP for browser automation
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    // Take screenshots
    err = env.TakeScreenshot(ctx, "screenshot-name")

    // Results automatically saved to env.GetResultsDir()
}
```

**Configuration (`test/ui/config.toml`):**

```toml
[build]
# Source directory for main.go (relative to test/ui)
source_dir = "../../cmd/quaero"
# Binary output path (relative to test/ui)
binary_output = "./bin/quaero"
# Config file for the test service (relative to test/ui)
config_file = "./quaero.toml"

[service]
startup_timeout_seconds = 30
# Port 18085 isolates tests from dev server on 8085
port = 18085
host = "localhost"
shutdown_endpoint = "/api/shutdown"

[output]
results_base_dir = "./results"
```

**Service Configuration (`test/ui/quaero.toml`):**

Minimal configuration for fast test startup:
- Port 18085 (isolated from development)
- Test database: `./bin/data/quaero-test.db` (created automatically)
- Mock LLM mode (no model loading)
- All sources and jobs disabled
- Database reset on startup

**Service Lifecycle:**

1. **Application Build**: Builds binary using `go build -o ./bin/quaero ../../cmd/quaero` (CI/CD compatible)
2. **Port Check**: Detects if service already running on port 18085
3. **Graceful Shutdown**: Stops existing service via `/api/shutdown` endpoint
4. **Service Start**: Launches fresh service with test configuration
5. **Readiness Wait**: Polls until service responds to health checks
6. **Test Execution**: Runs test with live service
7. **Cleanup**: Stops service and closes log files

**CI/CD Benefits:**
- ✅ No external script dependencies
- ✅ Platform-independent (uses `go build`)
- ✅ Self-contained in `test/ui/` directory
- ✅ Isolated port (18085) prevents conflicts
- ✅ Fresh builds ensure tests run against latest code
- ✅ Fast startup with minimal configuration

**Test Results Structure:**

Each test creates a timestamped directory:

```
test/ui/results/
└── {TestName}-{YYYYMMDD-HHMMSS}/
    ├── service.log       # Service startup, HTTP logs, shutdown
    ├── test.log          # Test execution log with timestamps
    └── *.png             # Screenshots captured during test
```

**Example Results:**
```
test/ui/results/HomepageTitle-20251104-081635/
├── service.log    # Full service output (13KB)
├── test.log       # Test execution log (385B)
└── homepage.png   # Screenshot (92KB)
```

**Helper Methods:**

- `env.GetBaseURL()` - Returns service URL
- `env.GetResultsDir()` - Returns test results directory
- `env.GetScreenshotPath(name)` - Returns screenshot file path
- `env.TakeScreenshot(ctx, name)` - Captures and saves screenshot
- `env.LogTest(t, format, args...)` - Logs to both console and test.log
- `env.Cleanup()` - Stops service and closes resources

## Writing Tests

### Test Helpers

**GetTestServerURL()** - Returns correct server URL based on mode:
```go
url := test.MustGetTestServerURL()
// Mock mode: http://localhost:9999
// Integration mode: http://localhost:8085
```

**GetTestMode()** - Returns current test mode:
```go
if test.IsMockMode() {
    // Skip tests requiring real service features
    t.Skip("Test requires integration mode")
}
```

**HTTPTestHelper** - Convenience wrapper for HTTP requests:
```go
h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

// HTTP methods
resp, err := h.GET("/api/status")
resp, err := h.POST("/api/sources", sourceData)
resp, err := h.PUT("/api/sources/123", updates)
resp, err := h.DELETE("/api/sources/123")

// Assertions
h.AssertStatusCode(resp, http.StatusOK)
h.AssertJSONField(resp, "status", "ok")

// Parsing
var data map[string]interface{}
err := h.ParseJSONResponse(resp, &data)
```

### Best Practices

1. **Always use test helpers:**
   - Use `test.MustGetTestServerURL()` instead of hardcoded URLs
   - Use `HTTPTestHelper` for consistent request handling

2. **Clean up test data:**
   ```go
   defer func() {
       h.DELETE("/api/sources/" + sourceID)
   }()
   ```

3. **Test both modes when possible:**
   - Write tests that work in both mock and integration modes
   - Use `skipIfMockMode()` for integration-only tests

4. **Use descriptive test names:**
   - `TestCreateSourceWithAuthentication` ✓
   - `TestSource` ✗

5. **Log important information:**
   ```go
   t.Logf("Created source with ID: %s", sourceID)
   ```


## Troubleshooting

### Tests fail with connection errors

**Check test mode:**
```powershell
# Verify environment variable
$env:TEST_MODE
```

**Mock mode issues:**
- Ensure mock server started successfully (check test runner output)
- Port 9999 should be available

**Integration mode issues:**
- Ensure Quaero service is running
- Check service is on expected port (8085 by default)
- Verify service health: `curl http://localhost:8085/api/status`

### Mock server not responding

1. Check if another process is using port 9999
2. Review test runner logs for startup errors
3. Try killing any stale processes:
   ```powershell
   Get-NetTCPConnection -LocalPort 9999 | Select -ExpandProperty OwningProcess | Stop-Process -Force
   ```

### Integration tests fail

1. **Verify service is running:**
   ```powershell
   # Check process
   Get-Process quaero

   # Check port
   netstat -an | findstr :8085
   ```

2. **Check service logs** in the service console window

3. **Rebuild service:**
   ```powershell
   .\scripts\build.ps1 -Clean
   .\scripts\build.ps1 -Run
   ```

### UI tests fail

1. **Build failures:**
   ```powershell
   # Check if Go is installed and in PATH
   go version

   # Manually test build
   cd test/ui
   go build -o ./bin/quaero ../../cmd/quaero
   ```

2. **Port already in use:**
   - UI tests automatically handle existing services on port 18085
   - Check test results in `test/ui/results/{TestName}-{datetime}/service.log`
   - Look for "Service already running" or "failed to shutdown" messages

3. **Service won't start:**
   - Check `service.log` for startup errors
   - Verify config exists: `test/ui/quaero.toml`
   - Ensure bin directory is writable (data directory created automatically)
   - Check binary was built: `ls test/ui/bin/quaero*`

4. **Check ChromeDP installation:**
   - Chrome/Chromium must be installed
   - ChromeDP downloads headless chrome automatically

5. **Review test results:**
   ```powershell
   # Navigate to latest test results
   cd test/ui/results
   ls -la

   # View service log
   cat {TestName}-{datetime}/service.log

   # View test log
   cat {TestName}-{datetime}/test.log

   # View screenshots
   ls {TestName}-{datetime}/*.png
   ```

6. **Increase timeouts** if tests are timing out:
   - Edit `test/ui/setup.toml`:
     ```toml
     [service]
     startup_timeout_seconds = 60  # Increase from 30
     ```

7. **Clean up stale services:**
   ```powershell
   # Check for running Quaero processes
   Get-Process quaero -ErrorAction SilentlyContinue

   # Kill if needed
   Stop-Process -Name quaero -Force
   ```

## CI/CD Recommendations

### GitHub Actions Example

```yaml
name: UI Tests

on: [push, pull_request]

jobs:
  ui-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install Chrome
        run: |
          wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add -
          echo "deb http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list
          apt-get update && apt-get install -y google-chrome-stable

      - name: Run UI Tests
        working-directory: test/ui
        run: go test -v -timeout 10m

      - name: Upload Test Results
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: ui-test-results
          path: test/ui/results/
```

### Fast Feedback Loop (PR Checks)

```yaml
# Use mock mode for API tests (fast)
test_mode: mock
timeout: 5m

# UI tests always use real service
# But run on isolated port (18085)
```

### Comprehensive Validation (Nightly/Pre-Release)

```yaml
# Use integration mode for full validation
test_mode: integration
timeout: 30m

# UI tests included in full validation
# Fresh builds ensure latest code
```

### Key CI/CD Features

- ✅ **No Docker required**: Tests build and run natively
- ✅ **Parallel execution**: Tests isolated by port (18085)
- ✅ **Artifact collection**: Screenshots and logs automatically saved
- ✅ **Fast startup**: Minimal config, mock LLM, no source loading
- ✅ **Cross-platform**: Works on Linux, macOS, Windows

## Mock Server Capabilities

The mock server (`test/mock_server.go`) implements these endpoints:

**Sources:**
- `GET /api/sources` - List sources
- `POST /api/sources` - Create source
- `GET /api/sources/{id}` - Get source
- `PUT /api/sources/{id}` - Update source
- `DELETE /api/sources/{id}` - Delete source

**Authentication:**
- `GET /api/auth/list` - List credentials
- `POST /api/auth` - Create credentials
- `GET /api/auth/status` - Get auth status
- `DELETE /api/auth/{id}` - Delete credentials

**Jobs:**
- `POST /api/jobs` - Create job
- `GET /api/jobs/{id}` - Get job status

**System:**
- `GET /api/config` - Get configuration
- `GET /api/status` - Health check

## Extending Tests

### Adding New Mock Endpoints

Edit `test/mock_server.go`:

```go
// Add handler to NewMockServer()
mux.HandleFunc("/api/newEndpoint", ms.handleNewEndpoint)

// Implement handler
func (ms *MockServer) handleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Handle request
    respondJSON(w, http.StatusOK, data)
}
```

### Adding New Test Suites

1. Create test file in `test/api/` or `test/ui/`
2. Add package documentation (see existing files)
3. Use test helpers for consistency
4. Test runs automatically via test runner

### Mode-Specific Tests

```go
func TestIntegrationOnly(t *testing.T) {
    if test.IsMockMode() {
        t.Skip("Requires real database - integration mode only")
    }

    // Test real service features
}
```

## Additional Resources

- **Test Runner Documentation:** `cmd/quaero-test-runner/README.md`
- **Project Guidelines:** `CLAUDE.md`
- **Build Instructions:** `scripts/build.ps1 --help`

## Getting Help

- Check this README for common issues
- Review test runner output for detailed errors
- Check service logs in console window (integration mode)
- Review screenshots in test results (UI tests)
