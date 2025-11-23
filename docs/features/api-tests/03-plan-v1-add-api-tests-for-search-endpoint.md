I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State:**
- Search handler (`internal/handlers/search_handler.go`) implements `GET /api/search` with query parameter `q`, pagination (`limit`, `offset`), and FTS5 disabled error handling (503)
- Unit tests exist (`internal/handlers/search_handler_test.go`) but use mocks - no API integration tests
- Documents API tests (`test/api/documents_test.go`) provide excellent patterns: helper functions, table-driven subtests, cleanup strategies
- Search service supports three modes: `"fts5"`, `"advanced"` (default), `"disabled"` - configured via `[search]` section in config
- Test config (`test/config/test-quaero.toml`) uses default search mode (`"advanced"`), meaning FTS5 is enabled by default in tests

**Gap:**
- No API integration tests for `/api/search` endpoint
- Missing coverage for: query parameter handling, pagination (limit/offset), limit clamping (max 100, default 50), empty query, result structure, brief truncation (200 chars)
- FTS5 disabled scenario (503 error) difficult to test in integration tests without config modification - will document as limitation

**Test Infrastructure:**
- `TestEnvironment` and `HTTPTestHelper` available in `test/common/setup.go`
- Document helper functions in `test/api/documents_test.go` can be reused for creating searchable test data
- Cleanup strategy: `cleanupAllDocuments()` via `/api/documents/clear-all` endpoint


### Approach

Create new test file `test/api/search_test.go` following established patterns from `documents_test.go` and `settings_system_test.go`. Structure with one main test function per scenario (basic search, pagination, limit clamping, empty query, response structure, brief truncation) using table-driven subtests where appropriate. Reuse document helper functions from `documents_test.go` for test data creation. Include comprehensive cleanup before/after tests. Document FTS5 disabled scenario limitation in comments (requires config modification, not suitable for standard integration tests).


### Reasoning

Read search handler implementation to understand endpoint behavior (query params, pagination, error handling). Examined existing API test patterns in documents and settings tests to identify helper function structure and cleanup strategies. Searched for FTS5 configuration to understand search service modes and default behavior in tests. Verified test infrastructure availability (TestEnvironment, HTTPTestHelper) and document creation helpers for reuse.


## Mermaid Diagram

sequenceDiagram
    participant Test as Test Suite
    participant Env as TestEnvironment
    participant API as Search API
    participant Docs as Documents API
    participant DB as BadgerDB

    Note over Test,DB: Test Setup
    Test->>Env: SetupTestEnvironment()
    Env->>API: Start test server (port 18085)
    Test->>Docs: cleanupAllDocuments()
    Docs->>DB: DELETE all documents

    Note over Test,DB: Create Test Data
    Test->>Test: createTestDocument(content)
    Test->>Docs: POST /api/documents
    Docs->>DB: Store document
    Docs-->>Test: Return document ID

    Note over Test,DB: Execute Search Tests
    Test->>API: GET /api/search?q=keyword&limit=10&offset=0
    API->>DB: FullTextSearch(query, limit)
    DB-->>API: Return matching documents
    API->>API: Truncate content to brief (200 chars)
    API-->>Test: {results, count, query, limit, offset}

    Note over Test,DB: Verify Response
    Test->>Test: Assert status code 200
    Test->>Test: Assert response structure
    Test->>Test: Assert brief truncation
    Test->>Test: Assert pagination params

    Note over Test,DB: Test Cleanup
    Test->>Docs: cleanupAllDocuments()
    Docs->>DB: DELETE all documents
    Test->>Env: Cleanup()
    Env->>API: Stop test server

## Proposed File Changes

### test\api\search_test.go(NEW)

References: 

- internal\handlers\search_handler.go
- test\api\documents_test.go
- test\api\settings_system_test.go
- test\common\setup.go

Create comprehensive API integration tests for search endpoint following patterns from `test/api/documents_test.go` and `test/api/settings_system_test.go`.

**Test Structure:**

1. **Package and Imports:**
   - Package `api`
   - Import: `testing`, `net/http`, `fmt`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`, `github.com/ternarybob/quaero/test/common`

2. **Test Functions:**

   **TestSearchBasic** - Basic search functionality:
   - Subtests:
     - `EmptyDatabase` - Search with no documents, verify empty results array, count=0
     - `SingleDocument` - Create 1 document with searchable content, search for keyword, verify result matches
     - `MultipleDocuments` - Create 5 documents with different content, search for common keyword, verify multiple results
     - `NoResults` - Search for nonexistent keyword, verify empty results array, count=0
     - `EmptyQuery` - Search with empty query string (`q=`), verify returns all documents (or empty results depending on implementation)
   - Setup: `cleanupAllDocuments()` before and after
   - Use `createTestDocument()` and `createAndSaveTestDocument()` from documents_test.go helpers
   - Verify response structure: `{results, count, query, limit, offset}`
   - Assert each result has required fields: `id`, `source_type`, `title`, `content_markdown`, `url`, `brief`, `created_at`, `updated_at`

   **TestSearchPagination** - Pagination with limit and offset:
   - Subtests:
     - `DefaultPagination` - No params, verify limit=50, offset=0 in response
     - `CustomLimit` - `?limit=10`, verify limit=10 in response and results count ≤ 10
     - `CustomOffset` - `?offset=5`, verify offset=5 in response
     - `LimitAndOffset` - `?limit=10&offset=20`, verify both params in response
     - `SecondPage` - Create 25 docs, fetch first page (limit=10, offset=0), then second page (limit=10, offset=10), verify different results
   - Setup: Create 25 documents with searchable content
   - Cleanup: Delete all created documents

   **TestSearchLimitClamping** - Limit validation and clamping:
   - Subtests:
     - `MaxLimitEnforcement` - `?limit=200`, verify clamped to 100 in response
     - `NegativeLimit` - `?limit=-10`, verify defaults to 50
     - `ZeroLimit` - `?limit=0`, verify defaults to 50
     - `InvalidLimit` - `?limit=invalid`, verify defaults to 50
     - `NegativeOffset` - `?offset=-5`, verify clamped to 0
     - `InvalidOffset` - `?offset=bad`, verify defaults to 0
   - Verify status code 200 (graceful handling of invalid params)
   - Verify response structure intact despite invalid params

   **TestSearchResponseStructure** - Response format validation:
   - Subtests:
     - `AllFieldsPresent` - Verify response contains: `results`, `count`, `query`, `limit`, `offset`
     - `ResultFieldsComplete` - Verify each result has: `id`, `source_type`, `source_id`, `title`, `content_markdown`, `url`, `detail_level`, `metadata`, `created_at`, `updated_at`, `brief`
     - `CountMatchesResults` - Verify `count` field equals `len(results)`
     - `QueryEchoed` - Verify `query` field matches request query parameter
   - Create 3 documents with full metadata
   - Search and verify complete response structure

   **TestSearchBriefTruncation** - Brief field truncation logic:
   - Subtests:
     - `ShortContent` - Content < 200 chars, verify brief equals full content (no truncation)
     - `ExactlyTwoHundred` - Content exactly 200 chars, verify brief equals content (no ellipsis)
     - `LongContent` - Content > 200 chars, verify brief is 203 chars (200 + "..."), ends with "..."
     - `VeryLongContent` - Content 500+ chars, verify brief is 203 chars
     - `EmptyContent` - Empty content, verify brief is empty string
   - Create documents with varying content lengths
   - Verify `brief` field truncation matches handler logic (200 char limit)
   - Verify `content_markdown` field always contains full content

   **TestSearchErrorCases** - Error handling:
   - Subtests:
     - `MethodNotAllowed` - POST/PUT/DELETE to `/api/search`, verify 405 Method Not Allowed
   - Note: FTS5 disabled scenario (503 error) requires config modification (`[search] mode = "disabled"`), not suitable for standard integration tests - document as limitation in comments

3. **Helper Functions (reuse from documents_test.go):**
   - Import/reference `createTestDocument()` - creates document with required fields
   - Import/reference `createAndSaveTestDocument()` - POSTs document and returns ID
   - Import/reference `deleteTestDocument()` - DELETEs document by ID
   - Import/reference `cleanupAllDocuments()` - DELETEs all documents via `/api/documents/clear-all`

4. **Test Patterns:**
   - Use `require.NoError()` for setup operations (environment, document creation)
   - Use `assert` for verification (status codes, response fields, counts)
   - Cleanup before and after each test function: `cleanupAllDocuments(t, env)` in setup and defer
   - Use `helper.GET()` for search requests
   - Use `helper.AssertStatusCode()` for status verification
   - Use `helper.ParseJSONResponse()` for response parsing
   - Log test progress: `t.Logf()` for key operations, `t.Log("✓ Test completed")` at end

5. **Documentation:**
   - File header comment: "Package api provides API integration tests for search endpoint"
   - Comment explaining FTS5 disabled scenario limitation: "Note: Testing FTS5 disabled (503 error) requires modifying test config to set [search] mode = 'disabled'. This is not included in standard integration tests to avoid config complexity. The error path is covered by unit tests in internal/handlers/search_handler_test.go."
   - Each test function has descriptive comment explaining what it tests

**Key Assertions:**
- Status code 200 for successful searches
- Response structure: `{results: [], count: int, query: string, limit: int, offset: int}`
- Result structure: `{id, source_type, source_id, title, content_markdown, url, detail_level, metadata, created_at, updated_at, brief}`
- Brief truncation: ≤200 chars = no truncation, >200 chars = 200 chars + "..."
- Limit clamping: ≤0 defaults to 50, >100 clamped to 100
- Offset clamping: <0 defaults to 0
- Count field matches results array length
- Query parameter echoed in response

**Test Isolation:**
- Each test function uses `cleanupAllDocuments()` before and after
- Documents created with unique IDs (timestamp + counter from documents_test.go pattern)
- No shared state between tests