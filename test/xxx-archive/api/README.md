# API Integration Tests

Self-contained API tests with automatic service lifecycle management.

## Overview

This test suite provides comprehensive API integration testing with:
- **Self-contained setup**: No external dependencies outside test/api
- **Automatic service management**: Build, start, wait for ready, cleanup
- **Results collection**: Parent/child directory structure for organized test artifacts
- **Test isolation**: Runs on port 19085 (separate from dev:8085 and UI tests:18085)

## Directory Structure

```
test/api/
├── setup.go              # Test environment setup (self-contained)
├── setup.toml            # Test configuration
├── test-config.toml      # Runtime service configuration
├── bin/                  # Build artifacts (gitignored)
│   ├── quaero.exe       # Test service binary
│   ├── quaero.toml      # Copied from test-config.toml
│   └── data/            # Test database
├── results/              # Test results (gitignored)
│   └── {Suite}-{datetime}/    # Parent directory per suite
│       └── {TestName}/        # Child directory per test
│           ├── service.log    # Service output
│           └── test.log       # Test execution log
└── *_test.go             # Test files
```

## Running Tests

### Run all API tests
```powershell
cd test/api
go test -v ./...
```

### Run specific test
```powershell
cd test/api
go test -v -run TestSourcesAPI
```

### Run with timeout for longer test suites
```powershell
cd test/api
go test -timeout 10m -v ./...
```

## Test Environment

Each test automatically:
1. **Builds** the application using `go build`
2. **Starts** the service on port 19085
3. **Waits** for service readiness
4. **Captures** all logs to timestamped results directory
5. **Stops** the service after test completion

## Writing Tests

### Basic Test Structure

```go
func TestMyAPI(t *testing.T) {
    // Setup test environment (builds & starts service)
    env, err := SetupTestEnvironment("TestMyAPI")
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }
    defer env.Cleanup()

    // Create HTTP helper
    h := env.NewHTTPTestHelper(t)

    // Make API requests
    resp, err := h.GET("/api/endpoint")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    // Assert response
    h.AssertStatusCode(resp, http.StatusOK)

    // Parse JSON response
    var result map[string]interface{}
    if err := h.ParseJSONResponse(resp, &result); err != nil {
        t.Fatalf("Parse failed: %v", err)
    }

    // Verify data
    if result["field"] != "expected" {
        t.Errorf("Expected 'expected', got: %v", result["field"])
    }
}
```

### HTTP Helper Methods

```go
// GET request
resp, err := h.GET("/api/endpoint")

// POST request with JSON body
data := map[string]interface{}{"key": "value"}
resp, err := h.POST("/api/endpoint", data)

// PUT request with JSON body
resp, err := h.PUT("/api/endpoint/123", data)

// DELETE request
resp, err := h.DELETE("/api/endpoint/123")

// Parse JSON response
var result MyStruct
err := h.ParseJSONResponse(resp, &result)

// Assert status code
h.AssertStatusCode(resp, http.StatusOK)
```

## Test Organization

Tests are organized by API area:
- `auth_api_test.go` - Authentication endpoints
- `sources_api_test.go` - Source management
- `job_api_test.go` - Job management
- `search_api_test.go` - Search functionality
- `chat_api_test.go` - Chat/RAG endpoints
- `config_api_test.go` - Configuration

## Results Collection

All test results are automatically collected in `test/api/results/`:

```
results/
├── TestSources-20250104-150405/    # Suite parent directory
│   ├── TestSourcesCreate/          # Individual test
│   │   ├── service.log
│   │   └── test.log
│   ├── TestSourcesUpdate/
│   │   ├── service.log
│   │   └── test.log
│   └── TestSourcesDelete/
│       ├── service.log
│       └── test.log
└── TestJobs-20250104-150410/       # Another suite
    ├── TestJobCreate/
    │   ├── service.log
    │   └── test.log
    └── TestJobStatus/
        ├── service.log
        └── test.log
```

## Configuration

### setup.toml
- Build settings (source dir, binary output)
- Service settings (port, timeout, endpoints)
- Output settings (results directory)

### test-config.toml
- Runtime service configuration
- Port 19085 (isolated from other instances)
- Mock LLM mode (no model loading)
- Minimal configuration for fast startup

## Important Notes

- ✅ Tests are **self-contained** - no dependencies outside test/api
- ✅ Service lifecycle is **automatic** - no manual start/stop
- ✅ Each test gets its own **timestamped results directory**
- ✅ Tests run on **port 19085** - isolated from dev and UI tests
- ✅ Results are **preserved** for debugging
- ❌ DO NOT manually start the service before running tests
- ❌ DO NOT import from `github.com/ternarybob/quaero/test` package

## Troubleshooting

### Port Already in Use
The test will automatically shutdown any existing service on port 19085.

### Service Won't Start
Check `results/{Suite}-{datetime}/{TestName}/service.log` for startup errors.

### Test Failures
Review both:
- `service.log` - Service output and errors
- `test.log` - Test execution and assertions

### Build Failures
Ensure you're in the `test/api` directory when running tests. The setup expects relative paths from that location.
