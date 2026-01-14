# Architect Analysis: Portfolio API Test Fixes

## Problem Analysis
The tests fail with a 30s timeout and a stack trace showing `net/http` transport goroutines (`readLoop`/`writeLoop`). This indicates that HTTP connections are remaining open (idle) after requests, and the test process is either waiting for them to close or failing leak detection checks.

The 30s timeout specifically aligns with the configured timeout in `FetchAndValidateNavexaAPI`.

The root cause is the use of `http.Client` with the default Transport, which enables HTTP Keep-Alives. In a test environment, especially one making external calls or spawning many ephemeral clients, these idle connections can accumulate and cause leaks or hangs.

## Proposed Solution
We need to disable Keep-Alives for HTTP clients used in tests. This ensures that the underlying TCP connection is closed immediately after the response is read, terminating the background `readLoop` and `writeLoop` goroutines.

### Affected Areas
1.  **`test/api/portfolio/common_test.go`**: The `FetchAndValidateNavexaAPI` function creates a local `http.Client`.
2.  **`test/common/setup.go`**: The `NewHTTPTestHelper` and `NewHTTPTestHelperWithTimeout` methods create clients for test helpers.

### Implementation Details

#### 1. `FetchAndValidateNavexaAPI`
Change the client initialization to:
```go
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        DisableKeepAlives: true,
    },
}
```

#### 2. `HTTPTestHelper` Initialization
In `test/common/setup.go`, update `NewHTTPTestHelper` and `NewHTTPTestHelperWithTimeout` to similarly inject a Transport with `DisableKeepAlives: true`.

## Verification
After applying the changes, the tests in `test/api/portfolio` should run without timing out or leaking goroutines.
