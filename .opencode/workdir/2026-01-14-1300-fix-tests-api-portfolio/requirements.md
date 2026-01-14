# Requirements: Fix Portfolio API Tests

## Context
The tests in `test/api/portfolio` are failing with a timeout (30s) and stack traces indicating goroutine leaks or hangs in `net/http` transport (`readLoop`/`writeLoop`).

## Goals
1.  Fix the timeout and goroutine leak issues in `test/api/portfolio`.
2.  Ensure proper cleanup of HTTP resources in tests.

## Constraints
*   Do not modify the application code (`internal/...`) unless necessary for testability.
*   Focus on `test/api/portfolio` and `test/common`.
*   Maintain existing test logic and assertions.

## Tasks
1.  Modify `FetchAndValidateNavexaAPI` in `test/api/portfolio/common_test.go` to use an `http.Client` with `DisableKeepAlives: true`.
2.  Modify `NewHTTPTestHelper` and `NewHTTPTestHelperWithTimeout` in `test/common/setup.go` to use an `http.Client` with `DisableKeepAlives: true`.
