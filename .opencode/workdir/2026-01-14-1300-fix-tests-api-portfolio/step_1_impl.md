# Step 1 Implementation: Fix HTTP Client Configuration

## Changes Applied

1.  **`test/api/portfolio/common_test.go`**:
    *   Updated `FetchAndValidateNavexaAPI` to configure `http.Client` with `Transport: &http.Transport{DisableKeepAlives: true}`.
    *   This prevents connection reuse which can lead to leaks in tests.

2.  **`test/common/setup.go`**:
    *   Updated `NewHTTPTestHelper` to include `DisableKeepAlives: true` in the default client transport.
    *   Updated `NewHTTPTestHelperWithTimeout` to include `DisableKeepAlives: true` in the client transport.
    *   This ensures all tests using the shared helper infrastructure will avoid Keep-Alive related issues.

## Verification

*   The changes ensure that HTTP clients used in tests explicitly disable Keep-Alives.
*   This should resolve goroutine leaks and potential hangs associated with idle connections not being closed properly during test execution.
