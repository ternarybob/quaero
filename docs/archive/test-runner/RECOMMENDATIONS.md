# Test Runner Recommendations & Action Items

**Date:** 2025-10-23  
**Status:** Review Complete  
**Overall Rating:** ⭐⭐⭐⭐ (4.5/5)

---

## Executive Summary

The Quaero test runner is a **well-designed, production-ready test automation system** with excellent CLI interface and comprehensive documentation.

**Key Strengths:**
- ✅ Automatic service lifecycle management
- ✅ Dual testing modes (mock/integration)
- ✅ Selective test execution via CLI flags
- ✅ Screenshot capture for UI tests
- ✅ Excellent documentation

**Key Issues to Address:**
- 🔴 Test server started but not used by main tests
- 🟡 No port conflict detection
- 🟡 Test timeouts hardcoded
- 💡 No parallel test execution

---

## Priority Recommendations

### 🔴 High Priority (Implement First)

#### 1. Clarify Test Server Purpose
**Issue:** Test server (port 3333) is started but not used by actual tests.

**Current Code:**
```go
// cmd/quaero-test-runner/main.go:199
testServerURL := fmt.Sprintf("http://localhost:%d", config.TestServer.Port)
// Started but tests use service URL instead
```

**Recommended Fix:**
```go
// Only start test server in mock mode or for connectivity checks
if config.TestRunner.TestMode == "mock" {
    fmt.Println("STEP 0: Starting mock test server...")
    testServer := StartTestServer(config.TestServer.Port)
    defer testServer.Shutdown(ctx)
} else {
    fmt.Println("STEP 0: Skipping test server (integration mode uses real service)")
}
```

**Impact:** Reduces resource usage, clarifies architecture

---

#### 2. Add Port Conflict Detection
**Issue:** No check if service port already in use by dev instance.

**Recommended Implementation:**
```go
// Add to cmd/quaero-test-runner/main.go
import "net"

func checkPortAvailable(port int) error {
    addr := fmt.Sprintf("localhost:%d", port)
    conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
    if err == nil {
        conn.Close()
        return fmt.Errorf("port %d is already in use", port)
    }
    return nil
}

// Before starting service:
if err := checkPortAvailable(servicePort); err != nil {
    fmt.Printf("ERROR: %v\n", err)
    fmt.Printf("Stop existing service: taskkill /F /IM quaero.exe\n")
    os.Exit(1)
}
```

**Impact:** Prevents confusing errors, better user experience

---

#### 3. Make Test Timeouts Configurable
**Issue:** Test timeouts hardcoded in `test/helpers.go`.

**Configuration Enhancement:**
```toml
# quaero-test-runner.toml
[test_runner]
test_timeout_seconds = 60
http_timeout_seconds = 30
llm_timeout_seconds = 120  # For chat/embedding tests
```

**Code Changes:**
```go
// test/helpers.go
func getConfiguredTimeout() time.Duration {
    if timeoutStr := os.Getenv("TEST_TIMEOUT_SECONDS"); timeoutStr != "" {
        if seconds, err := strconv.Atoi(timeoutStr); err == nil {
            return time.Duration(seconds) * time.Second
        }
    }
    return 60 * time.Second // Default
}

func NewHTTPTestHelper(t *testing.T, baseURL string) *HTTPTestHelper {
    return &HTTPTestHelper{
        BaseURL: baseURL,
        Client:  &http.Client{Timeout: getConfiguredTimeout()},
        T:       t,
    }
}
```

**Impact:** Supports slow LLM operations, flexible for CI environments

---

### 🟡 Medium Priority (Next Sprint)

#### 4. Add Coverage Report Generation
**Enhancement:** Generate code coverage reports alongside test results.

**Implementation:**
```go
// In runTestSuite()
coverFile := filepath.Join(suiteDir, "coverage.out")
cmd := exec.Command("go", "test", "-v", "-count=1", 
    "-coverprofile="+coverFile, "./"+suite.Path)

// After test run:
if _, err := os.Stat(coverFile); err == nil {
    htmlFile := filepath.Join(suiteDir, "coverage.html")
    exec.Command("go", "tool", "cover", 
        "-html="+coverFile, "-o="+htmlFile).Run()
}
```

**Configuration:**
```toml
[coverage]
enabled = true
output_format = "html"  # html, json, or both
minimum_coverage = 70.0  # Fail if below threshold
```

**Impact:** Track code coverage, enforce quality standards

---

#### 5. Add Test Retry Logic
**Enhancement:** Retry flaky tests automatically.

**Configuration:**
```toml
[test_runner]
retry_failed_tests = false
max_retries = 2
retry_delay_seconds = 5
```

**Implementation:**
```go
func runTestSuiteWithRetry(suite TestSuite, config *TestRunnerConfig) TestResult {
    maxRetries := config.TestRunner.MaxRetries
    if maxRetries == 0 {
        maxRetries = 1
    }
    
    var result TestResult
    for attempt := 1; attempt <= maxRetries; attempt++ {
        result = runTestSuite(suite, ...)
        if result.Success {
            return result
        }
        
        if attempt < maxRetries {
            fmt.Printf("⚠ Test failed (attempt %d/%d), retrying...\n", 
                attempt, maxRetries)
            time.Sleep(time.Duration(config.TestRunner.RetryDelay) * time.Second)
        }
    }
    
    return result
}
```

**Impact:** Handle flaky tests, more reliable CI/CD

---

#### 6. Improve Error Messages
**Enhancement:** Add helpful troubleshooting hints to error messages.

**Example:**
```go
if err := waitForService(serviceURL, startupTimeout); err != nil {
    fmt.Printf("ERROR: Service did not become ready: %v\n", err)
    fmt.Println("\nTroubleshooting:")
    fmt.Println("  1. Check if port is already in use:")
    fmt.Printf("     netstat -an | findstr :%d\n", servicePort)
    fmt.Println("  2. Check service logs in the service window")
    fmt.Println("  3. Verify database path is writable")
    fmt.Printf("  4. Try increasing startup_timeout_seconds in config\n")
    os.Exit(1)
}
```

**Impact:** Faster debugging, better developer experience

---

### 💡 Low Priority (Future Enhancements)

#### 7. Add Parallel Test Execution
**Enhancement:** Run test suites in parallel for faster feedback.

**Configuration:**
```toml
[test_runner]
parallel_suites = true
max_parallel = 2
```

**Implementation:**
```go
var wg sync.WaitGroup
resultsChan := make(chan TestResult, len(suites))
semaphore := make(chan struct{}, config.TestRunner.MaxParallel)

for _, suite := range suites {
    wg.Add(1)
    go func(s TestSuite) {
        defer wg.Done()
        semaphore <- struct{}{}        // Acquire
        defer func() { <-semaphore }() // Release
        
        result := runTestSuite(s, ...)
        resultsChan <- result
    }(suite)
}

wg.Wait()
close(resultsChan)
```

**Note:** Requires careful service isolation or mock mode.

**Impact:** Faster test execution, better CI/CD performance

---

#### 8. Add Performance Tracking
**Enhancement:** Track test duration over time, alert on regressions.

**Implementation:**
```go
type PerformanceMetrics struct {
    TestSuite string    `json:"test_suite"`
    Timestamp time.Time `json:"timestamp"`
    Duration  float64   `json:"duration_seconds"`
    TestCount int       `json:"test_count"`
    Passed    int       `json:"passed"`
    Failed    int       `json:"failed"`
}

// Save to test/results/performance.json
func savePerformanceMetrics(metrics PerformanceMetrics) {
    // Append to historical data
    // Check for regressions (e.g., >20% slower than average)
}
```

**Impact:** Detect performance regressions early

---

#### 9. Add Test Tags Support
**Enhancement:** Fine-grained test filtering with tags.

**Example:**
```go
// +build smoke

package api

func TestAuthHealth(t *testing.T) {
    // Smoke test
}
```

**CLI Usage:**
```bash
quaero-test-runner --tags smoke
quaero-test-runner --tags integration,slow
```

**Impact:** Flexible test execution for different scenarios

---

## Configuration Enhancements

### Suggested Complete Configuration

```toml
[test_runner]
tests_dir = "./test"
output_dir = "./test/results"
build_script = "./scripts/build.ps1"
test_mode = "integration"  # "mock" or "integration"

# Timeouts
test_timeout_seconds = 60
http_timeout_seconds = 30
llm_timeout_seconds = 120

# Parallel execution
parallel_suites = false
max_parallel = 2

# Retry logic
retry_failed_tests = false
max_retries = 2
retry_delay_seconds = 5

[test_server]
port = 3333
enabled_in_integration = false  # Only use in mock mode

[service]
binary = "./bin/quaero.exe"
config = "./bin/quaero.toml"
startup_timeout_seconds = 30
port = 8085  # Optional override
kill_existing = true  # Kill existing service on port
graceful_shutdown_seconds = 5

[coverage]
enabled = true
output_format = "html"  # "html", "json", or "both"
minimum_coverage = 70.0  # Fail if below threshold
include_packages = ["internal/**"]
exclude_packages = ["internal/test/**"]
```

---

## CLI Enhancements

### Suggested Additional Flags

```bash
# Verbose output
--verbose, -v          # Detailed logging

# Debugging
--debug                # Enable debug mode (keep service running on failure)
--no-cleanup           # Don't clean up on exit
--port <port>          # Override service port

# Performance
--parallel             # Run suites in parallel
--timeout <seconds>    # Override test timeout

# Coverage
--coverage             # Generate coverage report
--min-coverage <pct>   # Fail if coverage below threshold

# Retry
--retry                # Retry failed tests
--max-retries <n>      # Maximum retries per test

# Filtering
--tags <tags>          # Filter by test tags (e.g., smoke,integration)
--skip-ui              # Skip UI tests
```

---

## Action Items Checklist

### Immediate Actions (Week 1)
- [ ] Clarify test server purpose (remove or document clearly)
- [ ] Add port conflict detection before service start
- [ ] Make test timeouts configurable via TOML/env vars
- [ ] Update README.md with new configuration options

### Short-term Actions (Sprint 1)
- [ ] Add coverage report generation
- [ ] Add test retry logic for flaky tests
- [ ] Improve error messages with troubleshooting hints
- [ ] Add `--verbose` and `--debug` CLI flags

### Long-term Actions (Next Quarter)
- [ ] Implement parallel test execution support
- [ ] Add performance tracking and regression alerts
- [ ] Add test tagging system for fine-grained filtering
- [ ] Create GitHub Actions workflow templates

---

## Success Metrics

Track these metrics to measure improvements:

| Metric | Current | Target | How to Measure |
|--------|---------|--------|----------------|
| **Test Execution Time** | ~2 min | <90 sec | Time from start to summary |
| **Flaky Test Rate** | Unknown | <5% | Track retry frequency |
| **Port Conflict Errors** | Common | 0 | User error reports |
| **Code Coverage** | Unknown | >70% | Coverage reports |
| **CI/CD Failures** | Unknown | <10% | GitHub Actions stats |

---

## Security Considerations

**Current Security Posture:**
- ⚠️ PowerShell execution with `-ExecutionPolicy Bypass`
- ⚠️ No validation of config file paths
- ⚠️ Service runs with user privileges (not sandboxed)

**Recommendations:**
1. Validate all file paths before execution
2. Add option to run service with limited privileges
3. Document security implications in README
4. Consider sandboxing service in test environment

---

## Documentation Updates Needed

1. **README.md**
   - Add architecture diagram
   - Document new configuration options
   - Add troubleshooting section for port conflicts
   - Add performance benchmarks

2. **AGENTS.md**
   - Update test runner section with new capabilities
   - Document configuration best practices
   - Add CI/CD integration examples

3. **New: PERFORMANCE.md**
   - Document expected test execution times
   - Provide performance tuning tips
   - Document parallel execution caveats

---

## Conclusion

The test runner is already **production-ready** with excellent architecture and documentation. The recommended improvements will enhance:

1. **Developer Experience** - Better error messages, port conflict detection
2. **Reliability** - Retry logic, timeout configuration
3. **Performance** - Parallel execution, performance tracking
4. **Quality** - Coverage reports, regression detection

**Estimated Effort:**
- High Priority: 1-2 days
- Medium Priority: 3-5 days
- Low Priority: 1-2 weeks

**ROI:** High - Small time investment for significant quality and productivity gains.

---

**Next Steps:**
1. Review recommendations with team
2. Prioritize action items
3. Create GitHub issues for each action item
4. Assign ownership and deadlines
5. Track progress in sprint planning

**Reviewed by:** Claude (AI Agent)  
**Date:** 2025-10-23  
**Next Review:** After implementing high-priority recommendations
