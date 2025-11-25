I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The authentication API has 5 endpoints that need comprehensive test coverage:
1. POST /api/auth - Capture authentication from Chrome extension
2. GET /api/auth/status - Check authentication status
3. GET /api/auth/list - List all credentials (with sanitization)
4. GET /api/auth/{id} - Get specific credential (with sanitization)
5. DELETE /api/auth/{id} - Delete credential

Key testing requirements:
- Credential sanitization (cookies/tokens not exposed in responses)
- WebSocket broadcasting when auth is captured
- Error handling (invalid JSON, missing fields, not found)
- CRUD lifecycle validation
- Response structure verification

The test file should follow patterns from `test/api/settings_system_test.go` which demonstrates:
- Helper functions for resource creation/cleanup
- Table-driven tests with subtests
- HTTPTestHelper usage from TestEnvironment
- Comprehensive error case coverage


### Approach

Create `test/api/auth_test.go` following the established test patterns in `test/api/settings_system_test.go`. The file will include:

1. **Helper functions** for auth credential creation, deletion, and cleanup
2. **Test functions** covering all 5 endpoints with positive and negative cases
3. **Sanitization verification** ensuring cookies/tokens are never exposed
4. **WebSocket broadcast testing** (note: full WebSocket testing deferred to Phase 4)
5. **Error case coverage** for invalid JSON, missing fields, and not found scenarios

The tests will use the existing `TestEnvironment` and `HTTPTestHelper` infrastructure from `test/common/setup.go`, ensuring consistency with other API tests.


### Reasoning

I examined the authentication handler implementation in `internal/handlers/auth_handler.go` to understand all 5 endpoints and their behavior. I reviewed the data structures in `internal/interfaces/auth.go` and `internal/models/auth.go` to understand the auth data flow and credential storage model. I analyzed the existing test patterns in `test/api/settings_system_test.go` to understand the project's testing conventions. I also examined the WebSocket handler to understand the broadcast mechanism for auth updates.


## Mermaid Diagram

sequenceDiagram
    participant Test as Test Suite
    participant API as Auth API
    participant Storage as Auth Storage
    participant WS as WebSocket Handler

    Note over Test,WS: Test 1: POST /api/auth (Capture)
    Test->>API: POST /api/auth (auth data)
    API->>Storage: StoreCredentials()
    API->>WS: BroadcastAuth()
    WS-->>Test: WebSocket message (type: "auth")
    API-->>Test: 200 OK (success message)
    Test->>API: GET /api/auth/list
    API->>Storage: ListCredentials()
    API-->>Test: 200 OK (sanitized list)
    Note over Test: Verify: no cookies/tokens

    Note over Test,WS: Test 2: GET /api/auth/status
    Test->>API: GET /api/auth/status
    API-->>Test: 200 OK (authenticated: true/false)

    Note over Test,WS: Test 3: GET /api/auth/list
    Test->>API: GET /api/auth/list
    API->>Storage: ListCredentials()
    API-->>Test: 200 OK (sanitized array)
    Note over Test: Verify: only safe fields

    Note over Test,WS: Test 4: GET /api/auth/{id}
    Test->>API: GET /api/auth/{id}
    API->>Storage: GetCredentialsByID()
    API-->>Test: 200 OK (sanitized credential)
    Note over Test: Verify: no cookies/tokens

    Note over Test,WS: Test 5: DELETE /api/auth/{id}
    Test->>API: DELETE /api/auth/{id}
    API->>Storage: DeleteCredentials()
    API-->>Test: 200 OK (success message)
    Test->>API: GET /api/auth/{id}
    API-->>Test: 404 Not Found

## Proposed File Changes

### test\api\auth_test.go(NEW)

References: 

- test\common\setup.go
- test\api\settings_system_test.go
- internal\handlers\auth_handler.go
- internal\interfaces\auth.go
- internal\models\auth.go

Create comprehensive API integration tests for all 5 authentication endpoints.

**Package and Imports:**
- Package: `api`
- Standard library: `context`, `encoding/json`, `net/http`, `testing`, `time`
- Project imports: `test/common` for TestEnvironment and HTTPTestHelper

**Helper Functions:**

1. `createTestAuthData() map[string]interface{}` - Returns sample auth data matching `AtlassianAuthData` structure from `internal/interfaces/auth.go`:
   - cookies: Array of cookie objects with name, value, domain, path, expires, secure, httpOnly, sameSite
   - tokens: Map with cloudId and other tokens
   - userAgent: Sample user agent string
   - baseUrl: Test base URL (e.g., "https://test.atlassian.net")
   - timestamp: Current Unix timestamp

2. `captureTestAuth(t *testing.T, env *common.TestEnvironment, authData map[string]interface{}) string` - Helper to POST auth data and return credential ID:
   - POST to `/api/auth` with authData
   - Assert 200 OK status
   - Parse response and extract credential ID from storage (via GET /api/auth/list)
   - Return credential ID for use in subsequent tests

3. `deleteTestAuth(t *testing.T, env *common.TestEnvironment, id string)` - Helper to DELETE auth credential:
   - DELETE to `/api/auth/{id}`
   - Assert 200 OK status
   - Verify success message in response

4. `cleanupAllAuth(t *testing.T, env *common.TestEnvironment)` - Cleanup helper to delete all auth credentials:
   - GET `/api/auth/list` to retrieve all credentials
   - DELETE each credential by ID
   - Used in test cleanup to ensure clean state

**Test Functions:**

1. `TestAuthCapture(t *testing.T)` - Test POST /api/auth endpoint:
   - **Subtest: "Success"** - Valid auth data capture:
     - Create test auth data with `createTestAuthData()`
     - POST to `/api/auth`
     - Assert 200 OK status
     - Verify response contains `status: "success"` and success message
     - Verify credential stored by listing credentials (GET /api/auth/list)
     - Verify stored credential has correct baseUrl and site_domain
     - Cleanup: Delete created credential
   
   - **Subtest: "InvalidJSON"** - Invalid JSON payload:
     - POST malformed JSON to `/api/auth`
     - Assert 400 Bad Request status
     - Verify error message mentions "Invalid request body"
   
   - **Subtest: "MissingFields"** - Missing required fields:
     - POST auth data with missing baseUrl field
     - Assert 400 or 500 status (depending on handler validation)
     - Verify appropriate error message
   
   - **Subtest: "EmptyCookies"** - Empty cookies array:
     - POST auth data with empty cookies array
     - Assert 200 OK (should still succeed, just no cookies)
     - Verify credential created
     - Cleanup: Delete created credential

2. `TestAuthStatus(t *testing.T)` - Test GET /api/auth/status endpoint:
   - **Subtest: "NotAuthenticated"** - No credentials stored:
     - Ensure no credentials exist (cleanup first)
     - GET `/api/auth/status`
     - Assert 200 OK status
     - Verify response contains `authenticated: false`
   
   - **Subtest: "Authenticated"** - Credentials exist:
     - Create test auth credential with `captureTestAuth()`
     - GET `/api/auth/status`
     - Assert 200 OK status
     - Verify response contains `authenticated: true`
     - Cleanup: Delete created credential

3. `TestAuthList(t *testing.T)` - Test GET /api/auth/list endpoint:
   - **Subtest: "EmptyList"** - No credentials stored:
     - Ensure no credentials exist (cleanup first)
     - GET `/api/auth/list`
     - Assert 200 OK status
     - Verify response is empty array `[]`
   
   - **Subtest: "SingleCredential"** - One credential stored:
     - Create test auth credential with `captureTestAuth()`
     - GET `/api/auth/list`
     - Assert 200 OK status
     - Verify response is array with 1 element
     - Verify credential has expected fields: id, name, site_domain, service_type, base_url, created_at, updated_at
     - **CRITICAL: Verify sanitization** - Assert cookies and tokens fields are NOT present in response (per `internal/handlers/auth_handler.go` lines 113-135)
     - Cleanup: Delete created credential
   
   - **Subtest: "MultipleCredentials"** - Multiple credentials stored:
     - Create 3 test auth credentials with different baseUrls
     - GET `/api/auth/list`
     - Assert 200 OK status
     - Verify response is array with 3 elements
     - Verify all credentials have sanitized fields (no cookies/tokens)
     - Cleanup: Delete all created credentials

4. `TestAuthGet(t *testing.T)` - Test GET /api/auth/{id} endpoint:
   - **Subtest: "Success"** - Valid credential ID:
     - Create test auth credential with `captureTestAuth()`
     - GET `/api/auth/{id}` with valid ID
     - Assert 200 OK status
     - Verify response contains expected fields: id, name, site_domain, service_type, base_url, created_at, updated_at
     - **CRITICAL: Verify sanitization** - Assert cookies and tokens fields are NOT present in response (per `internal/handlers/auth_handler.go` lines 168-178)
     - Cleanup: Delete created credential
   
   - **Subtest: "NotFound"** - Invalid credential ID:
     - GET `/api/auth/nonexistent-id`
     - Assert 404 Not Found status
     - Verify error message mentions "Authentication not found"
   
   - **Subtest: "EmptyID"** - Empty ID in path:
     - GET `/api/auth/` (trailing slash, no ID)
     - Assert 400 Bad Request or 404 Not Found status
     - Verify appropriate error message

5. `TestAuthDelete(t *testing.T)` - Test DELETE /api/auth/{id} endpoint:
   - **Subtest: "Success"** - Valid credential ID:
     - Create test auth credential with `captureTestAuth()`
     - DELETE `/api/auth/{id}` with valid ID
     - Assert 200 OK status
     - Verify response contains `status: "success"` and success message
     - Verify credential no longer exists (GET /api/auth/{id} returns 404)
   
   - **Subtest: "NotFound"** - Invalid credential ID:
     - DELETE `/api/auth/nonexistent-id`
     - Assert 500 Internal Server Error status (per `internal/handlers/auth_handler.go` line 201)
     - Verify error message mentions "Failed to delete credentials"
   
   - **Subtest: "EmptyID"** - Empty ID in path:
     - DELETE `/api/auth/` (trailing slash, no ID)
     - Assert 400 Bad Request status
     - Verify error message mentions "Missing auth ID"

6. `TestAuthSanitization(t *testing.T)` - Comprehensive sanitization verification:
   - **Subtest: "ListSanitization"** - Verify cookies/tokens never exposed in list:
     - Create test auth credential with cookies and tokens
     - GET `/api/auth/list`
     - Parse response JSON
     - Assert cookies field is nil or not present
     - Assert tokens field is nil or not present
     - Verify only safe fields are present (id, name, site_domain, service_type, base_url, created_at, updated_at)
     - Cleanup: Delete created credential
   
   - **Subtest: "GetSanitization"** - Verify cookies/tokens never exposed in get:
     - Create test auth credential with cookies and tokens
     - GET `/api/auth/{id}`
     - Parse response JSON
     - Assert cookies field is nil or not present
     - Assert tokens field is nil or not present
     - Verify only safe fields are present
     - Cleanup: Delete created credential

**WebSocket Broadcast Testing:**
- Note: Full WebSocket testing is deferred to Phase 4 (separate test file)
- The auth capture test verifies the handler calls `BroadcastAuth` (per `internal/handlers/auth_handler.go` lines 68-78)
- WebSocket message structure verification will be in `test/api/websocket_test.go`

**Test Setup and Teardown:**
- Each test function should call `cleanupAllAuth()` at the beginning to ensure clean state
- Use `defer cleanupAllAuth()` to ensure cleanup even if test fails
- Follow pattern from `test/api/settings_system_test.go` for consistent test structure

**Error Handling:**
- All HTTP requests should check status codes
- All JSON parsing should handle errors gracefully
- Use `t.Fatalf()` for critical failures that prevent test continuation
- Use `t.Errorf()` for assertion failures that allow test to continue

**Code Quality:**
- Follow Go testing conventions
- Use table-driven tests where appropriate (e.g., error cases)
- Keep test functions under 80 lines (use subtests for organization)
- Use descriptive test names and comments
- Follow patterns from `test/api/settings_system_test.go` for consistency