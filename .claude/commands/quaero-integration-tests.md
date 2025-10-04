# quaero-integration-tests

Creates comprehensive integration test suite for Quaero, based on aktis-parser test structure with API and UI testing.

## Usage

```
/quaero-integration-tests <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: Test Infrastructure Setup

1. **Test Directory Structure**
   ```
   quaero/tests/
   ├── go.mod                  # Separate module for tests
   ├── go.sum
   ├── config.toml             # Test configuration
   ├── run-tests.ps1           # PowerShell test runner
   ├── run-tests.sh            # Bash test runner (optional)
   ├── api/                    # API integration tests
   │   ├── api_health_test.go
   │   ├── auth_flow_test.go
   │   ├── confluence_api_test.go
   │   ├── jira_api_test.go
   │   ├── github_api_test.go
   │   ├── storage_api_test.go
   │   └── query_api_test.go
   ├── cli/                    # CLI integration tests
   │   ├── collect_test.go
   │   ├── query_test.go
   │   └── serve_test.go
   ├── e2e/                    # End-to-end tests
   │   ├── full_workflow_test.go
   │   ├── auth_to_query_test.go
   │   └── vision_workflow_test.go
   ├── fixtures/               # Test data
   │   ├── auth_payload.json
   │   ├── confluence_page.html
   │   ├── jira_issues.json
   │   └── sample_images/
   └── results/                # Test results (gitignored)
       └── run_2025-10-04_15-30-45/
           ├── api/
           ├── cli/
           └── e2e/
   ```

2. **Test Configuration** (`tests/config.toml`)
   ```toml
   [test]
   timeout_seconds = 60
   quaero_url = "http://localhost:8080"
   quaero_binary = "../bin/quaero"

   [api]
   enabled = true

   [cli]
   enabled = true

   [e2e]
   enabled = true

   [services]
   ravendb_url = "http://localhost:8081"
   ollama_url = "http://localhost:11434"
   ```

### Phase 2: Test Runner Implementation

1. **PowerShell Test Runner** (`tests/run-tests.ps1`)
   - Based on aktis-parser/tests/run-tests.ps1
   - Build application
   - Start Quaero server
   - Run test suites (API, CLI, E2E)
   - Collect results
   - Generate reports
   - Stop server
   - Cleanup

2. **Test Runner Features**
   ```powershell
   # Run all tests
   ./run-tests.ps1 -Type all

   # Run specific suite
   ./run-tests.ps1 -Type api
   ./run-tests.ps1 -Type cli
   ./run-tests.ps1 -Type e2e

   # Run specific test
   ./run-tests.ps1 -Type api -Test TestAuthFlow

   # Run source-specific tests
   ./run-tests.ps1 -Type all -Filter Confluence
   ```

### Phase 3: API Integration Tests

1. **Health & Status Tests** (`tests/api/api_health_test.go`)
   ```go
   func TestAPIAvailability(t *testing.T) {
       resp, err := client.Get(config.Test.QuaeroURL + "/health")
       require.NoError(t, err)
       assert.Equal(t, 200, resp.StatusCode)
   }

   func TestServerStatus(t *testing.T) {
       resp, err := client.Get(config.Test.QuaeroURL + "/api/status")
       require.NoError(t, err)

       var status Status
       json.NewDecoder(resp.Body).Decode(&status)
       assert.NotEmpty(t, status.Version)
   }
   ```

2. **Auth Flow Tests** (`tests/api/auth_flow_test.go`)
   ```go
   func TestExtensionAuthFlow(t *testing.T) {
       // Load fixture
       authData := loadFixture(t, "auth_payload.json")

       // Send to server
       resp, err := client.Post(
           config.Test.QuaeroURL+"/api/auth",
           "application/json",
           bytes.NewBuffer(authData),
       )

       require.NoError(t, err)
       assert.Equal(t, 200, resp.StatusCode)

       // Verify auth was stored
       // (Would need API endpoint to check)
   }
   ```

3. **Source API Tests**
   - `confluence_api_test.go` - Test Confluence collection
   - `jira_api_test.go` - Test Jira collection
   - `github_api_test.go` - Test GitHub collection

### Phase 4: CLI Integration Tests

1. **Collect Command Tests** (`tests/cli/collect_test.go`)
   ```go
   func TestCollectCommand(t *testing.T) {
       cmd := exec.Command(config.Test.QuaeroBinary, "collect", "--all")
       output, err := cmd.CombinedOutput()

       assert.NoError(t, err)
       assert.Contains(t, string(output), "Collection complete")
   }

   func TestCollectConfluence(t *testing.T) {
       cmd := exec.Command(
           config.Test.QuaeroBinary,
           "collect",
           "--source", "confluence",
       )
       output, err := cmd.CombinedOutput()

       assert.NoError(t, err)
       assert.Contains(t, string(output), "Collected")
   }
   ```

2. **Query Command Tests** (`tests/cli/query_test.go`)
   ```go
   func TestQueryCommand(t *testing.T) {
       // Ensure data is collected first
       setupTestData(t)

       cmd := exec.Command(
           config.Test.QuaeroBinary,
           "query",
           "How to onboard a new user?",
       )
       output, err := cmd.CombinedOutput()

       assert.NoError(t, err)
       assert.NotEmpty(t, string(output))
       assert.Contains(t, string(output), "Source:")
   }
   ```

3. **Serve Command Tests** (`tests/cli/serve_test.go`)
   ```go
   func TestServeCommand(t *testing.T) {
       // Start server in background
       cmd := exec.Command(config.Test.QuaeroBinary, "serve")
       err := cmd.Start()
       require.NoError(t, err)
       defer cmd.Process.Kill()

       // Wait for server to be ready
       time.Sleep(2 * time.Second)

       // Test server is responding
       resp, err := client.Get(config.Test.QuaeroURL + "/health")
       assert.NoError(t, err)
       assert.Equal(t, 200, resp.StatusCode)
   }
   ```

### Phase 5: End-to-End Tests

1. **Full Workflow Test** (`tests/e2e/full_workflow_test.go`)
   ```go
   func TestFullQuaeroWorkflow(t *testing.T) {
       // 1. Start server
       startServer(t)
       defer stopServer(t)

       // 2. Send auth from extension
       sendAuth(t)

       // 3. Wait for collection
       waitForCollection(t, 30*time.Second)

       // 4. Query the data
       answer := queryQuaero(t, "How to onboard a new user?")

       // 5. Verify answer quality
       assert.NotEmpty(t, answer.Text)
       assert.Greater(t, len(answer.Sources), 0)
       assert.Contains(t, answer.Text, "user")
   }
   ```

2. **Auth to Query Test** (`tests/e2e/auth_to_query_test.go`)
   ```go
   func TestAuthToQueryFlow(t *testing.T) {
       // Complete flow from auth reception to answer generation
       // Tests: Auth → Storage → Collection → RAG → LLM → Answer
   }
   ```

3. **Vision Workflow Test** (`tests/e2e/vision_workflow_test.go`)
   ```go
   func TestVisionWorkflow(t *testing.T) {
       // Test image processing in queries
       // Collect docs with images
       // Query about diagram
       // Verify vision model used
   }
   ```

### Phase 6: Test Utilities

1. **Common Test Functions** (`tests/common.go`)
   ```go
   package main

   // Load test configuration
   func LoadTestConfig() (*TestConfig, error)

   // Start Quaero server
   func StartServer(t *testing.T) *exec.Cmd

   // Stop Quaero server
   func StopServer(t *testing.T, cmd *exec.Cmd)

   // Load test fixture
   func LoadFixture(t *testing.T, filename string) []byte

   // Wait for service to be ready
   func WaitForService(url string, timeout time.Duration) error

   // Create test HTTP client
   func NewTestClient(timeout time.Duration) *http.Client
   ```

2. **Fixtures and Test Data** (`tests/fixtures/`)
   - Sample auth payloads
   - Mock Confluence pages
   - Mock Jira issues
   - Sample images for vision testing

### Phase 7: Test Reports

1. **Results Collection**
   - Timestamped results directory
   - Individual test logs
   - Screenshots (for UI tests)
   - Test summary file

2. **Summary Report Format**
   ```
   Test Run Summary
   ================
   Timestamp: 2025-10-04_15-30-45
   Type: all
   Total: 25
   Passed: 24
   Failed: 1

   Test Results:
   [PASS] API Tests (12/12) - api/test.log
   [PASS] CLI Tests (8/8) - cli/test.log
   [FAIL] E2E Tests (4/5) - e2e/test.log
   ```

## Test Template (aktis-parser pattern)

Based on C:\development\aktis\aktis-parser\tests\

### run-tests.ps1 Template
```powershell
# Quaero Test Runner
param(
    [Parameter(Mandatory=$false)]
    [string]$Type = "all",
    [Parameter(Mandatory=$false)]
    [string]$Test = $null,
    [Parameter(Mandatory=$false)]
    [string]$Filter = $null
)

Write-Host "Test Runner for Quaero" -ForegroundColor Cyan

# 1. Build application
# 2. Start services (RavenDB, Ollama if needed)
# 3. Start Quaero server
# 4. Run tests
# 5. Collect results
# 6. Stop server
# 7. Generate summary
```

### Test File Template
```go
package main

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFeature(t *testing.T) {
    // Skip if disabled
    if !config.API.Enabled {
        t.Skip("API tests disabled")
    }

    // Setup
    client := NewTestClient(config.Test.TimeoutSeconds)

    // Execute
    result, err := performAction(client)

    // Assert
    require.NoError(t, err)
    assert.NotEmpty(t, result)

    // Log
    t.Logf("Test passed: %v", result)
}
```

## Examples

### Create Integration Tests
```
/quaero-integration-tests C:\development\quaero
```

### Run All Tests
```powershell
cd C:\development\quaero\tests
./run-tests.ps1 -Type all
```

### Run Specific Suite
```powershell
./run-tests.ps1 -Type api
./run-tests.ps1 -Type e2e
```

## Validation

After implementation, verifies:
- ✓ Test directory structure created
- ✓ Test runner (PowerShell) functional
- ✓ API integration tests present
- ✓ CLI integration tests present
- ✓ E2E tests present
- ✓ Test configuration file created
- ✓ Fixtures directory populated
- ✓ Results directory gitignored
- ✓ Tests can run independently
- ✓ Tests can run in CI/CD

## Output

Provides detailed report:
- Files created
- Test suites implemented
- Test runner setup
- Configuration created
- Sample test run results

---

**Agent**: quaero-integration-tests

**Prompt**: Create comprehensive integration test suite for Quaero at {{args.[0]}}, based on the aktis-parser test structure at C:\development\aktis\aktis-parser\tests.

## Implementation Tasks

1. **Create Test Directory Structure**
   - tests/ with separate go.mod
   - api/, cli/, e2e/ subdirectories
   - fixtures/ for test data
   - results/ for test output (gitignored)

2. **Implement Test Runner** (`tests/run-tests.ps1`)
   - Based on aktis-parser/tests/run-tests.ps1
   - Build application
   - Start Quaero server
   - Run test suites with filtering
   - Collect and organize results
   - Generate summary report
   - Clean up processes

3. **Create API Integration Tests** (`tests/api/`)
   - api_health_test.go - Health and status checks
   - auth_flow_test.go - Extension auth reception
   - confluence_api_test.go - Confluence collection
   - jira_api_test.go - Jira collection
   - github_api_test.go - GitHub collection
   - storage_api_test.go - Storage operations
   - query_api_test.go - RAG query endpoint

4. **Create CLI Integration Tests** (`tests/cli/`)
   - collect_test.go - Test collect command
   - query_test.go - Test query command
   - serve_test.go - Test serve command
   - version_test.go - Test version command

5. **Create E2E Tests** (`tests/e2e/`)
   - full_workflow_test.go - Complete auth→query flow
   - auth_to_query_test.go - Auth reception to answer
   - vision_workflow_test.go - Image processing workflow
   - multi_source_test.go - Query across sources

6. **Setup Test Configuration** (`tests/config.toml`)
   - Test timeouts
   - Service URLs
   - Feature flags
   - Test data paths

7. **Create Test Utilities** (`tests/common.go`)
   - Server start/stop helpers
   - Fixture loaders
   - Wait for service helpers
   - HTTP client creation

8. **Add Test Fixtures** (`tests/fixtures/`)
   - auth_payload.json
   - confluence_page.html
   - jira_issues.json
   - sample_images/

## Code Quality Standards

- Follow aktis-parser test patterns
- Use testify for assertions
- Comprehensive error checking
- Proper cleanup (defer)
- Timeout handling
- Result logging
- Screenshot capture on failure

## Success Criteria

✓ Test runner executes successfully
✓ Can run all tests
✓ Can run specific test suites
✓ Can filter tests
✓ Results collected properly
✓ Summary report generated
✓ Server started/stopped correctly
✓ All test types passing
✓ CI/CD compatible
