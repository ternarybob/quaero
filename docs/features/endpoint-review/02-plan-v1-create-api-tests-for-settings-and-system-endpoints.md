I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires creating comprehensive API tests for Settings and System endpoints in a new file `test/api/settings_system_test.go`. The existing `health_check_test.go` provides the template pattern using `common.SetupTestEnvironment()` and `HTTPTestHelper`. The existing `kv_case_insensitive_test.go` has comprehensive KV tests but doesn't use the standard test environment pattern.

**Endpoints to test:**
1. **KV Store** (6 endpoints): GET/POST `/api/kv`, GET/PUT/DELETE `/api/kv/{key}` - case-insensitive keys, masking, CRUD operations
2. **Connectors** (4 endpoints): GET/POST `/api/connectors`, PUT/DELETE `/api/connectors/{id}` - GitHub connector validation
3. **Config** (1 endpoint): GET `/api/config` - returns config with injected keys from KV store
4. **Status** (1 endpoint): GET `/api/status` - application status
5. **Version & Health** (2 endpoints): GET `/api/version`, GET `/api/health` - already tested in `health_check_test.go`
6. **Logs** (3 endpoints): GET `/api/logs/recent`, GET `/api/system/logs/files`, GET `/api/system/logs/content` - log retrieval with filtering

**Key handler behaviors identified:**
- KV: Case-insensitive keys (normalized to lowercase), value masking in list (first 4 + "..." + last 4), full value in GET, upsert returns 201/200 based on create/update, duplicate check returns 409
- Connectors: GitHub connector test on create/update, validation of name/type required
- Config: Returns version, build, port, host, and full config with injected keys from ConfigService
- Status: Returns status from StatusService
- Logs: Recent logs from memory writer (last 100), system logs from files with level filtering and limit

**Test infrastructure available:**
- `common.SetupTestEnvironment()` - starts test server with config
- `HTTPTestHelper` - provides GET, POST, PUT, DELETE methods with status assertion and JSON parsing
- Test config files in `test/config/` directory

### Approach

Create a single comprehensive test file `test/api/settings_system_test.go` that follows the `health_check_test.go` pattern. The file will contain multiple test functions, each testing a specific endpoint or workflow. Use table-driven tests where appropriate for testing multiple scenarios (e.g., KV case-insensitivity, connector validation).

**Test organization:**
1. **KV Store Tests** - Test all CRUD operations, case-insensitivity, masking, upsert behavior, duplicate detection
2. **Connector Tests** - Test CRUD operations, GitHub connector validation, connection testing
3. **Config Test** - Test config retrieval with injected keys
4. **Status Test** - Test status endpoint response structure
5. **Logs Tests** - Test recent logs, log file listing, log content retrieval with filtering

**Key testing patterns:**
- Use `SetupTestEnvironment()` with Badger config (following health_check_test.go)
- Create helper functions for common operations (e.g., creating KV pairs, creating connectors)
- Test both success and error cases (validation errors, not found, conflicts)
- Verify response structures match handler implementations
- Test edge cases (empty values, special characters in keys, invalid JSON)

**Dependencies:**
- Existing test infrastructure in `test/common/setup.go`
- Handler implementations in `internal/handlers/`
- Test config file `test/config/test-quaero-badger.toml`

### Reasoning

I started by reading the user's task requirements and the referenced files. I examined the template file `health_check_test.go` to understand the test pattern using `SetupTestEnvironment()` and `HTTPTestHelper`. I reviewed the existing `kv_case_insensitive_test.go` to understand what KV functionality needs testing. I read all the handler files (`kv_handler.go`, `connector_handler.go`, `config_handler.go`, `status_handler.go`, `system_logs_handler.go`, `search_handler.go`, `api.go`) to understand endpoint behaviors, request/response structures, and error handling. I examined `routes.go` to identify all endpoints that need testing. I also checked the test directory structure to understand existing test organization.

## Mermaid Diagram

sequenceDiagram
    participant Test as Test Function
    participant Env as TestEnvironment
    participant Helper as HTTPTestHelper
    participant Server as Test Server
    participant Handler as API Handler
    participant Service as Service Layer
    
    Test->>Env: SetupTestEnvironment()
    Env->>Server: Start test server
    Server-->>Env: Server ready
    Env-->>Test: Return environment
    
    Test->>Env: NewHTTPTestHelper()
    Env-->>Test: Return helper
    
    rect rgb(200, 220, 240)
        Note over Test,Service: KV Store Tests
        Test->>Helper: POST /api/kv (create)
        Helper->>Server: HTTP POST
        Server->>Handler: KVHandler.CreateKVHandler
        Handler->>Service: KVService.Set()
        Service-->>Handler: Success
        Handler-->>Server: 201 Created
        Server-->>Helper: Response
        Helper-->>Test: Assert 201
        
        Test->>Helper: GET /api/kv (list)
        Helper->>Server: HTTP GET
        Server->>Handler: KVHandler.ListKVHandler
        Handler->>Service: KVService.List()
        Service-->>Handler: Masked values
        Handler-->>Server: 200 OK
        Server-->>Helper: Response
        Helper-->>Test: Assert masked values
        
        Test->>Helper: GET /api/kv/{key} (get)
        Helper->>Server: HTTP GET
        Server->>Handler: KVHandler.GetKVHandler
        Handler->>Service: KVService.GetPair()
        Service-->>Handler: Full value
        Handler-->>Server: 200 OK
        Server-->>Helper: Response
        Helper-->>Test: Assert full value
        
        Test->>Helper: PUT /api/kv/{key} (upsert)
        Helper->>Server: HTTP PUT
        Server->>Handler: KVHandler.UpdateKVHandler
        Handler->>Service: KVService.Upsert()
        Service-->>Handler: isNew flag
        Handler-->>Server: 200/201
        Server-->>Helper: Response
        Helper-->>Test: Assert status
        
        Test->>Helper: DELETE /api/kv/{key}
        Helper->>Server: HTTP DELETE
        Server->>Handler: KVHandler.DeleteKVHandler
        Handler->>Service: KVService.Delete()
        Service-->>Handler: Success
        Handler-->>Server: 200 OK
        Server-->>Helper: Response
        Helper-->>Test: Assert 200
    end
    
    rect rgb(220, 240, 200)
        Note over Test,Service: Connector Tests
        Test->>Helper: POST /api/connectors
        Helper->>Server: HTTP POST
        Server->>Handler: ConnectorHandler.CreateConnectorHandler
        Handler->>Service: TestConnection()
        Service-->>Handler: Connection OK
        Handler->>Service: CreateConnector()
        Service-->>Handler: Success
        Handler-->>Server: 201 Created
        Server-->>Helper: Response
        Helper-->>Test: Assert 201
        
        Test->>Helper: GET /api/connectors
        Helper->>Server: HTTP GET
        Server->>Handler: ConnectorHandler.ListConnectorsHandler
        Handler->>Service: ListConnectors()
        Service-->>Handler: Connector list
        Handler-->>Server: 200 OK
        Server-->>Helper: Response
        Helper-->>Test: Assert list
        
        Test->>Helper: DELETE /api/connectors/{id}
        Helper->>Server: HTTP DELETE
        Server->>Handler: ConnectorHandler.DeleteConnectorHandler
        Handler->>Service: DeleteConnector()
        Service-->>Handler: Success
        Handler-->>Server: 204 No Content
        Server-->>Helper: Response
        Helper-->>Test: Assert 204
    end
    
    rect rgb(240, 220, 200)
        Note over Test,Service: System Tests
        Test->>Helper: GET /api/config
        Helper->>Server: HTTP GET
        Server->>Handler: ConfigHandler.GetConfig
        Handler->>Service: ConfigService.GetConfig()
        Service-->>Handler: Config with injected keys
        Handler-->>Server: 200 OK
        Server-->>Helper: Response
        Helper-->>Test: Assert config structure
        
        Test->>Helper: GET /api/status
        Helper->>Server: HTTP GET
        Server->>Handler: StatusHandler.GetStatusHandler
        Handler->>Service: StatusService.GetStatus()
        Service-->>Handler: Status data
        Handler-->>Server: 200 OK
        Server-->>Helper: Response
        Helper-->>Test: Assert status
        
        Test->>Helper: GET /api/logs/recent
        Helper->>Server: HTTP GET
        Server->>Handler: WSHandler.GetRecentLogsHandler
        Handler->>Service: MemoryWriter.GetEntriesWithLimit()
        Service-->>Handler: Log entries
        Handler-->>Server: 200 OK
        Server-->>Helper: Response
        Helper-->>Test: Assert logs
    end
    
    Test->>Env: Cleanup()
    Env->>Server: Stop server
    Server-->>Env: Stopped
    Env-->>Test: Cleanup complete

## Proposed File Changes

### test\api\settings_system_test.go(NEW)

References: 

- test\api\health_check_test.go
- test\api\kv_case_insensitive_test.go
- test\common\setup.go
- internal\handlers\kv_handler.go
- internal\handlers\connector_handler.go
- internal\handlers\config_handler.go
- internal\handlers\status_handler.go
- internal\handlers\system_logs_handler.go
- internal\handlers\api.go
- internal\handlers\search_handler.go

Create comprehensive API tests for Settings and System endpoints using the `health_check_test.go` template pattern.

**File Structure:**
- Package declaration: `package api`
- Imports: `testing`, `net/http`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`, `github.com/ternarybob/quaero/test/common`
- Test functions for each endpoint group

**Test Functions to Implement:**

1. **TestKVStore_CRUD** - Test complete KV store lifecycle:
   - POST `/api/kv` with key="TEST_KEY", value="test-value-123", description="Test key" → Assert 201 Created
   - GET `/api/kv` → Assert 200 OK, verify list contains key with masked value ("test...123")
   - GET `/api/kv/test_key` (lowercase) → Assert 200 OK, verify full unmasked value returned
   - PUT `/api/kv/Test_Key` (mixed case) with value="updated-value-456" → Assert 200 OK, verify created=false
   - GET `/api/kv/TEST_KEY` → Assert 200 OK, verify updated value
   - DELETE `/api/kv/test_key` → Assert 200 OK
   - GET `/api/kv/TEST_KEY` → Assert 404 Not Found

2. **TestKVStore_CaseInsensitive** - Test case-insensitive key handling:
   - POST `/api/kv` with key="GOOGLE_API_KEY" → Assert 201 Created
   - GET `/api/kv/google_api_key` (lowercase) → Assert 200 OK, verify value matches
   - GET `/api/kv/Google_Api_Key` (mixed case) → Assert 200 OK, verify value matches
   - PUT `/api/kv/GOOGLE_api_KEY` with new value → Assert 200 OK, verify created=false
   - GET `/api/kv` → Assert only 1 key exists (not 3 duplicates)

3. **TestKVStore_Upsert** - Test PUT upsert behavior:
   - PUT `/api/kv/NEW_KEY` with value="value-1" → Assert 201 Created, verify created=true
   - PUT `/api/kv/new_key` with value="value-2" → Assert 200 OK, verify created=false
   - GET `/api/kv/NEW_KEY` → Assert value="value-2"

4. **TestKVStore_DuplicateValidation** - Test duplicate key detection:
   - POST `/api/kv` with key="DUPLICATE_KEY" → Assert 201 Created
   - POST `/api/kv` with key="DUPLICATE_KEY" → Assert 409 Conflict, verify error message contains "already exists"
   - POST `/api/kv` with key="duplicate_key" (lowercase) → Assert 409 Conflict (case-insensitive duplicate)

5. **TestKVStore_ValueMasking** - Test value masking in list endpoint:
   - POST `/api/kv` with key="SHORT", value="abc" → Assert 201 Created
   - POST `/api/kv` with key="LONG", value="sk-1234567890abcdef" → Assert 201 Created
   - GET `/api/kv` → Assert SHORT value="••••••••", LONG value="sk-1...cdef"
   - GET `/api/kv/SHORT` → Assert full value="abc" (unmasked)
   - GET `/api/kv/LONG` → Assert full value="sk-1234567890abcdef" (unmasked)

6. **TestKVStore_ValidationErrors** - Test validation error cases:
   - POST `/api/kv` with empty key → Assert 400 Bad Request, error="Key is required"
   - POST `/api/kv` with empty value → Assert 400 Bad Request, error="Value is required"
   - POST `/api/kv` with invalid JSON → Assert 400 Bad Request, error="Invalid request body"
   - GET `/api/kv/` (empty key) → Assert 400 Bad Request
   - PUT `/api/kv/nonexistent` with empty value → Assert 404 Not Found (description-only update on missing key)

7. **TestConnectors_CRUD** - Test connector lifecycle:
   - POST `/api/connectors` with name="Test GitHub", type="github", config={"token":"test-token","owner":"test-owner","repo":"test-repo"} → Assert 201 Created (note: may fail connection test if token invalid, adjust test accordingly)
   - GET `/api/connectors` → Assert 200 OK, verify list contains connector
   - Extract connector ID from response
   - PUT `/api/connectors/{id}` with updated name → Assert 200 OK
   - DELETE `/api/connectors/{id}` → Assert 204 No Content
   - GET `/api/connectors` → Assert connector no longer in list

8. **TestConnectors_Validation** - Test connector validation:
   - POST `/api/connectors` with empty name → Assert 400 Bad Request, error="Name and Type are required"
   - POST `/api/connectors` with empty type → Assert 400 Bad Request
   - POST `/api/connectors` with invalid JSON → Assert 400 Bad Request
   - POST `/api/connectors` with type="github" but missing config → Assert 400 Bad Request (GitHub connector validation)

9. **TestConnectors_GitHubConnectionTest** - Test GitHub connector connection testing:
   - POST `/api/connectors` with type="github" and invalid token → Assert 400 Bad Request, error contains "Connection test failed"
   - Note: This test may need to be skipped or mocked if no valid GitHub token available

10. **TestConfig_Get** - Test config endpoint:
    - GET `/api/config` → Assert 200 OK
    - Verify response structure: {"version", "build", "port", "host", "config"}
    - Verify version and build are non-empty strings
    - Verify port matches test environment port
    - Verify config object contains expected sections (server, sqlite, etc.)
    - Note: Injected keys from KV store should appear in config if ConfigService is properly initialized

11. **TestStatus_Get** - Test status endpoint:
    - GET `/api/status` → Assert 200 OK
    - Verify response structure contains expected status fields
    - Note: Exact structure depends on StatusService implementation, verify against `internal/services/status/service.go`

12. **TestVersion_Get** - Test version endpoint (duplicate of health_check_test.go, but included for completeness):
    - GET `/api/version` → Assert 200 OK
    - Verify response: {"version", "build", "git_commit"}
    - Verify all fields are non-empty strings

13. **TestHealth_Get** - Test health endpoint (duplicate of health_check_test.go, but included for completeness):
    - GET `/api/health` → Assert 200 OK
    - Verify response: {"status": "ok"}

14. **TestLogsRecent_Get** - Test recent logs endpoint:
    - GET `/api/logs/recent` → Assert 200 OK
    - Verify response is array of log entries
    - Verify each entry has expected structure (level, timestamp, message)
    - Note: Logs may be empty if no recent activity, test should handle empty array

15. **TestSystemLogs_ListFiles** - Test log file listing:
    - GET `/api/system/logs/files` → Assert 200 OK
    - Verify response is array of log file info objects
    - Verify each file has name, size, modified_at fields
    - Note: May be empty if no log files exist, test should handle empty array

16. **TestSystemLogs_GetContent** - Test log content retrieval:
    - GET `/api/system/logs/content?filename=quaero.log&limit=10` → Assert 200 OK or 404 if file doesn't exist
    - If 200 OK: Verify response is array of log entries, verify limit is respected (max 10 entries)
    - GET `/api/system/logs/content?filename=quaero.log&limit=50&levels=ERROR,WARN` → Assert 200 OK or 404
    - If 200 OK: Verify only ERROR and WARN level logs returned
    - GET `/api/system/logs/content` (missing filename) → Assert 400 Bad Request, error="Filename is required"

**Helper Functions:**
- `createKVPair(t *testing.T, helper *common.HTTPTestHelper, key, value, description string) string` - Creates KV pair and returns key
- `deleteKVPair(t *testing.T, helper *common.HTTPTestHelper, key string)` - Deletes KV pair
- `createConnector(t *testing.T, helper *common.HTTPTestHelper, name, connectorType string, config map[string]interface{}) string` - Creates connector and returns ID
- `deleteConnector(t *testing.T, helper *common.HTTPTestHelper, id string)` - Deletes connector

**Test Setup Pattern (same as health_check_test.go):**
```go
func TestKVStore_CRUD(t *testing.T) {
    env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
    require.NoError(t, err, "Failed to setup test environment")
    defer env.Cleanup()
    
    helper := env.NewHTTPTestHelper(t)
    
    // Test implementation...
}
```

**Important Notes:**
- Use `require.NoError()` for setup/critical operations that should fail the test immediately
- Use `assert.Equal()` for value comparisons that should continue test execution
- Use `helper.AssertStatusCode()` for HTTP status assertions
- Use `helper.ParseJSONResponse()` for parsing JSON responses
- Log test progress with `t.Logf()` for debugging
- Handle cases where services may not be fully initialized (e.g., ConfigService, StatusService)
- Some tests may need to be skipped if external dependencies unavailable (e.g., GitHub API)
- Follow Go testing conventions: test function names start with `Test`, use underscores for readability