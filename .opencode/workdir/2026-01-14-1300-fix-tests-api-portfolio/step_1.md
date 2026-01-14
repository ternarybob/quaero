# Step 1: Fix HTTP Client Configuration

## Objective
Configure `http.Client` instances in test helpers to disable Keep-Alives, preventing goroutine leaks and hangs.

## Actions
1.  **Edit `test/api/portfolio/common_test.go`**:
    *   In `FetchAndValidateNavexaAPI`, replace the `http.Client` creation with one that includes a Transport with `DisableKeepAlives: true`.

2.  **Edit `test/common/setup.go`**:
    *   In `NewHTTPTestHelper`, add `Transport: &http.Transport{DisableKeepAlives: true}` to the client.
    *   In `NewHTTPTestHelperWithTimeout`, add `Transport: &http.Transport{DisableKeepAlives: true}` to the client.

## Verification
*   Run the tests in `test/api/portfolio` using `go test -v -timeout 20m ./test/api/portfolio/...` (or a specific test like `TestWorkerNavexaPortfolios`).
*   Ensure no timeouts or goroutine leaks occur.
