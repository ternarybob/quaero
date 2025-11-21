I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State:**
- Badger storage implementation exists in `internal/storage/badger/`
- Test framework supports custom config files via `SetupTestEnvironment(testName, customConfigPath...)`
- Health endpoint exists at `/api/health` in `internal/handlers/api.go`
- Default test config uses SQLite with `reset_on_startup = true`
- API tests run on port 19085 (automatically configured)

**Key Requirements:**
- Configure storage type as "badger" instead of "sqlite"
- Set Badger path to temporary directory for test isolation
- Follow existing test patterns for consistency
- Ensure proper cleanup with `defer env.Cleanup()`


### Approach

Create a health check API test that uses Badger storage instead of SQLite. The test will:
1. Create a Badger-specific test config file
2. Use `SetupTestEnvironment` to start the service with Badger storage
3. Test the `/api/health` endpoint using `HTTPTestHelper`
4. Verify the response is `{"status": "ok"}` with HTTP 200

This follows the established test pattern from `kv_case_insensitive_test.go` but uses the full service startup approach with `SetupTestEnvironment`.


### Reasoning

I explored the codebase structure, examined the existing test patterns in `kv_case_insensitive_test.go`, reviewed the Badger storage implementation, analyzed the test framework in `test/common/setup.go`, and studied the health endpoint handler in `internal/handlers/api.go`. I also checked the test config structure to understand how to configure Badger storage.


## Proposed File Changes

### test\config\test-quaero-badger.toml(NEW)

References: 

- test\config\test-quaero.toml
- internal\common\config.go

Create a test configuration file that overrides the base config to use Badger storage:

1. Set `storage.type = "badger"`
2. Configure `storage.badger.path` to use a test-specific directory (will be in temp directory)
3. Keep other settings minimal - inherit from base `test-quaero.toml`
4. Add comment explaining this config is for Badger storage tests

This file will be referenced by `test/api/health_check_test.go` when calling `SetupTestEnvironment`.

### test\api\health_check_test.go(NEW)

References: 

- test\api\kv_case_insensitive_test.go
- test\common\setup.go
- internal\handlers\api.go

Create a comprehensive health check test that uses Badger storage:

1. **Test Function: `TestHealthCheckWithBadger`**
   - Use `common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")` to start service with Badger config
   - Add `defer env.Cleanup()` for proper resource cleanup
   - Create `HTTPTestHelper` using `env.NewHTTPTestHelper(t)`

2. **Health Endpoint Test**
   - Make GET request to `/api/health` using `http.GET("/api/health")`
   - Assert status code is 200 using `http.AssertStatusCode(resp, http.StatusOK)`
   - Parse JSON response into `map[string]string`
   - Verify `status` field equals `"ok"`

3. **Storage Verification**
   - Log that Badger storage was initialized successfully
   - Verify service started with correct storage type

4. **Error Handling**
   - Use `require.NoError` for setup failures (fail fast)
   - Use `assert.Equal` for response validation
   - Include descriptive error messages

5. **Documentation**
   - Add package comment explaining this tests health endpoint with Badger storage
   - Add function comment describing test purpose and approach
   - Reference the Badger config file used

Follow the pattern from `test/api/kv_case_insensitive_test.go` for imports and test structure, but use `SetupTestEnvironment` instead of direct database creation.