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
│   └── ...
└── ui/                    # Browser automation tests (ChromeDP)
    └── ...
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

- Browser automation using ChromeDP
- Always run in integration mode (require real service)
- Capture screenshots for visual validation
- Saved to timestamped result directories

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

## Test Results

Results are saved to timestamped directories:

```
test/results/
└── {test-name}-{datetime}/
    ├── test.log           # Full test output
    └── screenshots/       # UI test screenshots (if applicable)
```

**Example:**
```
test/results/api_tests-2025-01-20_14-30-45/test.log
test/results/ui_tests-2025-01-20_14-35-22/screenshots/homepage.png
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

1. **Check ChromeDP installation:**
   - Chrome/Chromium must be installed
   - ChromeDP downloads headless chrome automatically

2. **Review screenshots** in test results directory for visual debugging

3. **Increase timeouts** if tests are timing out:
   ```go
   h := test.NewHTTPTestHelperWithTimeout(t, baseURL, 120*time.Second)
   ```

## CI/CD Recommendations

### Fast Feedback Loop (PR Checks)

```yaml
# Use mock mode for fast feedback
test_mode: mock
timeout: 5m
```

### Comprehensive Validation (Nightly/Pre-Release)

```yaml
# Use integration mode for full validation
test_mode: integration
timeout: 30m
```

### Multi-Stage Pipeline

```yaml
stages:
  - name: "Fast Tests"
    test_mode: mock

  - name: "Full Integration"
    test_mode: integration
    depends_on: ["Fast Tests"]
```

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
