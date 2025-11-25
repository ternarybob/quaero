I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

I've analyzed the codebase to understand the API endpoint structure and test patterns:

**Endpoints to Test:**
- **Auth**: POST `/api/auth`, GET `/api/auth/status`, GET `/api/auth/list`, GET `/api/auth/{id}`, DELETE `/api/auth/{id}`
- **Documents**: GET/POST `/api/documents`, GET `/api/documents/stats`, GET `/api/documents/tags`, GET/DELETE `/api/documents/{id}`, POST `/api/documents/{id}/reprocess`, DELETE `/api/documents/clear-all`
- **Search**: GET `/api/search?q=query&limit=50&offset=0`

**Test Template Pattern** (from `health_check_test.go` and `settings_system_test.go`):
1. Use `common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")`
2. Create `HTTPTestHelper` via `env.NewHTTPTestHelper(t)`
3. Use `helper.GET()`, `helper.POST()`, `helper.PUT()`, `helper.DELETE()`
4. Use `helper.AssertStatusCode()` and `helper.ParseJSONResponse()`
5. Use `require.NoError()` for must-pass, `assert.Equal()` for expectations
6. Include `t.Log()` for step-by-step tracing
7. Add helper functions for common operations (create, delete, cleanup)

**Data Models:**
- `AtlassianAuthData` with cookies, tokens, userAgent, baseUrl, timestamp
- `Document` with id, source_type, title, content_markdown, metadata, url, tags
- `DocumentStats` with counts by source
- Search returns documents with brief (truncated content)

**Key Behaviors:**
- Auth endpoints sanitize responses (no cookies/tokens in GET responses)
- Document reprocess is a no-op after Phase 5 (embeddings removed)
- Search returns 503 if FTS5 disabled
- Document list supports pagination and filtering
- Clear-all is a danger zone operation


### Approach

Create a single comprehensive test file `test/api/core_data_test.go` that strictly follows the `health_check_test.go` template pattern. The file will be organized into three main sections:

1. **Helper Functions** - Reusable functions for creating/deleting auth credentials and documents
2. **Auth Tests** - 5 test functions covering all auth endpoints
3. **Document Tests** - 8 test functions covering all document endpoints
4. **Search Tests** - 2 test functions covering search with/without results

Each test will:
- Use `SetupTestEnvironment()` with Badger config
- Create `HTTPTestHelper` for HTTP operations
- Include step-by-step `t.Log()` statements
- Use `require.NoError()` for critical assertions
- Use `assert.Equal()` for value comparisons
- Clean up resources with `defer env.Cleanup()`

The tests will validate:
- Success cases (200, 201, 204 status codes)
- Error cases (400, 404, 503 status codes)
- Response structure and data types
- Edge cases (empty queries, missing IDs, invalid JSON)
- Pagination and filtering behavior


### Reasoning

I explored the repository structure, read the template test file (`health_check_test.go`), examined the handler implementations (`auth_handler.go`, `document_handler.go`, `search_handler.go`), reviewed the data models and interfaces, and studied the existing comprehensive test file (`settings_system_test.go`) to understand the exact pattern and conventions used in this codebase.


## Mermaid Diagram

sequenceDiagram
    participant Test as Test Function
    participant Env as TestEnvironment
    participant Helper as HTTPTestHelper
    participant API as Quaero API
    participant Storage as Badger Storage

    Test->>Env: SetupTestEnvironment(name, config)
    Env->>Storage: Initialize Badger
    Env->>API: Start test server
    Env-->>Test: Return env + cleanup

    Test->>Helper: env.NewHTTPTestHelper(t)
    Helper-->>Test: Return helper

    rect rgb(200, 220, 255)
        Note over Test,Storage: Auth Tests
        Test->>Helper: POST /api/auth (credentials)
        Helper->>API: HTTP POST
        API->>Storage: StoreCredentials()
        Storage-->>API: Success
        API-->>Helper: 200 OK
        Helper-->>Test: Response + parsed JSON
        Test->>Test: Assert status, fields

        Test->>Helper: GET /api/auth/list
        Helper->>API: HTTP GET
        API->>Storage: ListCredentials()
        Storage-->>API: Credentials array
        API-->>Helper: 200 OK (sanitized)
        Helper-->>Test: Response + parsed JSON
        Test->>Test: Assert no cookies/tokens

        Test->>Helper: DELETE /api/auth/{id}
        Helper->>API: HTTP DELETE
        API->>Storage: DeleteCredentials(id)
        Storage-->>API: Success
        API-->>Helper: 200 OK
        Helper-->>Test: Response
    end

    rect rgb(200, 255, 220)
        Note over Test,Storage: Document Tests
        Test->>Helper: POST /api/documents (doc)
        Helper->>API: HTTP POST
        API->>Storage: SaveDocument()
        Storage-->>API: Success
        API-->>Helper: 201 Created
        Helper-->>Test: Response + doc ID

        Test->>Helper: GET /api/documents?limit=20
        Helper->>API: HTTP GET
        API->>Storage: ListDocuments(opts)
        Storage-->>API: Documents array
        API-->>Helper: 200 OK + pagination
        Helper-->>Test: Response + parsed JSON
        Test->>Test: Assert pagination fields

        Test->>Helper: GET /api/documents/stats
        Helper->>API: HTTP GET
        API->>Storage: GetStats()
        Storage-->>API: Stats object
        API-->>Helper: 200 OK
        Helper-->>Test: Response + stats

        Test->>Helper: DELETE /api/documents/clear-all
        Helper->>API: HTTP DELETE
        API->>Storage: ClearAll()
        Storage-->>API: Count deleted
        API-->>Helper: 200 OK
        Helper-->>Test: Response + count
    end

    rect rgb(255, 220, 200)
        Note over Test,Storage: Search Tests
        Test->>Helper: GET /api/search?q=test
        Helper->>API: HTTP GET
        API->>Storage: FullTextSearch(query)
        Storage-->>API: Documents array
        API-->>Helper: 200 OK + results
        Helper-->>Test: Response + parsed JSON
        Test->>Test: Assert results, brief field
    end

    Test->>Env: defer env.Cleanup()
    Env->>API: Stop server
    Env->>Storage: Close Badger

## Proposed File Changes

### test\api\core_data_test.go(NEW)

References: 

- test\api\health_check_test.go
- test\api\settings_system_test.go
- test\common\setup.go
- internal\handlers\auth_handler.go
- internal\handlers\document_handler.go
- internal\handlers\search_handler.go

Create comprehensive API integration tests for Auth, Documents, and Search endpoints following the `health_check_test.go` template pattern.

**Structure:**
- Package declaration and imports (testing, http, testify, common)
- Helper functions section with:
  - `createTestAuth()` - Creates auth credential via POST `/api/auth`
  - `deleteTestAuth()` - Deletes auth credential via DELETE `/api/auth/{id}`
  - `createTestDocument()` - Creates document via POST `/api/documents`
  - `deleteTestDocument()` - Deletes document via DELETE `/api/documents/{id}`
  - `cleanupTestAuth()` - Cleanup helper with error suppression
  - `cleanupTestDocument()` - Cleanup helper with error suppression

**Auth Tests (5 functions):**
1. `TestAuth_CaptureAuth()` - POST `/api/auth` with valid/invalid AtlassianAuthData
   - Valid: 200 status, success message
   - Invalid JSON: 400 status
   - Missing fields: 400 status

2. `TestAuth_GetStatus()` - GET `/api/auth/status`
   - Returns authenticated boolean
   - 200 status

3. `TestAuth_ListCredentials()` - GET `/api/auth/list`
   - Returns array of sanitized credentials
   - Verifies no cookies/tokens in response
   - 200 status

4. `TestAuth_GetCredential()` - GET `/api/auth/{id}`
   - Valid ID: 200 status, credential data
   - Invalid ID: 404 status
   - Empty ID: 400 status

5. `TestAuth_DeleteCredential()` - DELETE `/api/auth/{id}`
   - Valid ID: 200 status, success message
   - Invalid ID: 200 status (idempotent)
   - Empty ID: 400 status

**Document Tests (8 functions):**
1. `TestDocuments_Create()` - POST `/api/documents`
   - Valid document: 201 status, returns id/source_type/title
   - Missing id: 400 status
   - Missing source_type: 400 status
   - Invalid JSON: 400 status

2. `TestDocuments_List()` - GET `/api/documents`
   - Default pagination (limit=20, offset=0)
   - Custom pagination (limit=5, offset=2)
   - Filter by source_type
   - Filter by tags (comma-separated)
   - Returns documents array, total_count, limit, offset

3. `TestDocuments_GetStats()` - GET `/api/documents/stats`
   - Returns total_documents, documents_by_source, last_updated
   - 200 status

4. `TestDocuments_GetTags()` - GET `/api/documents/tags`
   - Returns tags array
   - 200 status

5. `TestDocuments_GetDocument()` - GET `/api/documents/{id}`
   - Valid ID: 200 status, full document data
   - Invalid ID: 404 status
   - Empty ID: 400 status

6. `TestDocuments_DeleteDocument()` - DELETE `/api/documents/{id}`
   - Valid ID: 200 status, success message
   - Invalid ID: 200 status (idempotent)
   - Empty ID: 400 status

7. `TestDocuments_Reprocess()` - POST `/api/documents/{id}/reprocess`
   - Valid ID: 200 status, success message (no-op after Phase 5)
   - Invalid ID: 400 status
   - Logs warning about embeddings removed

8. `TestDocuments_ClearAll()` - DELETE `/api/documents/clear-all`
   - Creates 3 test documents
   - Clears all: 200 status, documents_affected count
   - Verifies all deleted via stats endpoint

**Search Tests (2 functions):**
1. `TestSearch_WithResults()` - GET `/api/search?q=query`
   - Creates test document with searchable content
   - Searches with query: 200 status
   - Returns results array, count, query, limit, offset
   - Verifies brief field (truncated content)
   - Tests pagination (limit, offset)

2. `TestSearch_FTS5Disabled()` - GET `/api/search?q=query`
   - Tests behavior when FTS5 is disabled
   - Expected: 503 status or empty results (depends on config)
   - Verifies error message mentions FTS5

**Implementation Details:**
- Each test uses `SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")`
- All HTTP calls via `HTTPTestHelper` methods
- JSON payloads constructed as `map[string]interface{}`
- Response parsing via `helper.ParseJSONResponse()`
- Status code validation via `helper.AssertStatusCode()`
- Step-by-step logging with `t.Log()`
- Cleanup with `defer env.Cleanup()`
- Helper functions suppress errors for cleanup operations

**Test Data:**
- Auth: AtlassianAuthData with cookies array, tokens map, baseUrl, userAgent, timestamp
- Documents: id="doc_test_123", source_type="test", title="Test Doc", content_markdown="Test content", metadata={"key": "value"}
- Search: Documents with searchable keywords in title/content

**Error Handling:**
- Invalid JSON: 400 Bad Request
- Missing required fields: 400 Bad Request
- Resource not found: 404 Not Found
- FTS5 disabled: 503 Service Unavailable
- Empty IDs: 400 Bad Request

**Assertions:**
- `require.NoError()` for HTTP call errors and JSON parsing
- `assert.Equal()` for status codes, field values, counts
- `assert.NotEmpty()` for required fields
- `assert.Contains()` for error messages
- `assert.Len()` for array lengths