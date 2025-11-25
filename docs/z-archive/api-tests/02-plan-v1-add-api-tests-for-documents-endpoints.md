I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State:**
- Document handler (`internal/handlers/document_handler.go`) implements 8 endpoints: List, Create, Get, Delete, Reprocess, Stats, Tags, DeleteAll
- Routes registered in `internal/server/routes.go` with custom routing logic
- No API integration tests exist for document endpoints (0% coverage)
- Test patterns established in `test/api/auth_test.go` and `test/api/jobs_test.go`
- `TestEnvironment` and `HTTPTestHelper` utilities available in `test/common/setup.go`

**Requirements:**
- Test all 8 document endpoints with comprehensive scenarios
- Follow patterns from `auth_test.go`: helper functions, subtests, cleanup
- Test pagination, filtering (source_type, tags, dates), ordering
- Test error cases: invalid JSON, missing fields, not found, invalid IDs
- Test edge cases: empty results, multiple documents, danger zone operations
- Ensure test isolation with cleanup before/after each test

### Approach

Create comprehensive API integration tests for all 8 document endpoints in `test/api/documents_test.go`, following established patterns from `test/api/auth_test.go` and `test/api/jobs_test.go`. Tests will use `TestEnvironment` and `HTTPTestHelper` for service interaction, include helper functions for document creation/cleanup, and cover happy paths, error cases, pagination, filtering, and edge cases. Each test ensures clean state via pre/post cleanup for isolation.

### Reasoning

Read document handler implementation to understand endpoint behavior and request/response structures. Examined routes.go to understand routing patterns. Reviewed auth_test.go and jobs_test.go to identify established test patterns and conventions. Analyzed TestEnvironment and HTTPTestHelper utilities to understand available testing infrastructure. Reviewed document models to understand data structures and required fields.

## Mermaid Diagram

sequenceDiagram
    participant Test as Test Suite
    participant Env as TestEnvironment
    participant Helper as HTTPTestHelper
    participant API as Document API
    participant DB as BadgerDB

    Note over Test,DB: Test Setup Phase
    Test->>Env: SetupTestEnvironment()
    Env->>Env: Build service, start server
    Test->>Helper: cleanupAllDocuments()
    Helper->>API: DELETE /api/documents/clear-all
    API->>DB: ClearAll()
    DB-->>API: Success
    API-->>Helper: 200 OK

    Note over Test,DB: Test Execution Phase
    
    rect rgb(200, 220, 255)
        Note over Test,API: Create Document Test
        Test->>Helper: POST /api/documents
        Helper->>API: CreateDocumentHandler
        API->>DB: SaveDocument()
        DB-->>API: Success
        API-->>Helper: 201 Created {id, source_type, title}
        Helper-->>Test: Assert 201, parse response
    end

    rect rgb(220, 255, 220)
        Note over Test,API: List Documents Test
        Test->>Helper: GET /api/documents?limit=10&source_type=jira
        Helper->>API: ListHandler
        API->>DB: List(opts)
        DB-->>API: Documents array
        API-->>Helper: 200 OK {documents, total_count, limit, offset}
        Helper-->>Test: Assert 200, verify pagination
    end

    rect rgb(255, 220, 220)
        Note over Test,API: Get Document Test
        Test->>Helper: GET /api/documents/{id}
        Helper->>API: GetDocumentHandler
        API->>DB: GetDocument(id)
        DB-->>API: Document
        API-->>Helper: 200 OK {document}
        Helper-->>Test: Assert 200, verify fields
    end

    rect rgb(255, 240, 200)
        Note over Test,API: Delete Document Test
        Test->>Helper: DELETE /api/documents/{id}
        Helper->>API: DeleteDocumentHandler
        API->>DB: DeleteDocument(id)
        DB-->>API: Success
        API-->>Helper: 200 OK {doc_id, message}
        Helper-->>Test: Assert 200, verify deletion
    end

    rect rgb(240, 220, 255)
        Note over Test,API: Stats Test
        Test->>Helper: GET /api/documents/stats
        Helper->>API: StatsHandler
        API->>DB: GetStats()
        DB-->>API: DocumentStats
        API-->>Helper: 200 OK {stats}
        Helper-->>Test: Assert 200, verify counts
    end

    Note over Test,DB: Test Cleanup Phase
    Test->>Helper: cleanupAllDocuments()
    Helper->>API: DELETE /api/documents/clear-all
    API->>DB: ClearAll()
    Test->>Env: Cleanup()
    Env->>Env: Stop service, close logs

## Proposed File Changes

### test\api\documents_test.go(NEW)

References: 

- test\api\auth_test.go
- test\api\jobs_test.go
- test\common\setup.go
- internal\handlers\document_handler.go
- internal\models\document.go

Create comprehensive API integration tests for all 8 document endpoints following patterns from `test/api/auth_test.go`.

**Structure:**

1. **Package and Imports** (lines 1-15):
   - Package declaration: `package api`
   - Standard library imports: `fmt`, `net/http`, `testing`, `time`
   - External imports: `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`
   - Internal imports: `github.com/ternarybob/quaero/test/common`

2. **Helper Functions** (lines 17-150):
   - `createTestDocument(sourceType, title string) map[string]interface{}` - Creates sample document data with required fields (id, source_type, title, content_markdown, url, source_id, metadata, tags)
   - `createTestDocumentWithMetadata(sourceType, title string, metadata map[string]interface{}) map[string]interface{}` - Creates document with custom metadata
   - `createAndSaveTestDocument(t *testing.T, env *common.TestEnvironment, doc map[string]interface{}) string` - POSTs document to `/api/documents`, asserts 201 Created, parses response, returns document ID
   - `deleteTestDocument(t *testing.T, env *common.TestEnvironment, id string)` - DELETEs document via `/api/documents/{id}`, asserts 200 OK
   - `cleanupAllDocuments(t *testing.T, env *common.TestEnvironment)` - DELETEs all documents via `/api/documents/clear-all`, logs count, ensures clean state

3. **Test: GET /api/documents (List)** (lines 152-450):
   - `TestDocumentsList(t *testing.T)` - Main test function
   - Setup: `SetupTestEnvironment()`, cleanup before/after
   - Subtests:
     - `EmptyList` - Verify empty array when no documents exist
     - `SingleDocument` - Create 1 document, verify list returns 1 item with correct fields (id, source_type, title, content_markdown, url, created_at, updated_at, metadata, tags)
     - `MultipleDocuments` - Create 5 documents with different source types, verify list returns all 5
     - `Pagination` - Create 25 documents, test limit=10 offset=0, limit=10 offset=10, limit=10 offset=20, verify counts and pagination metadata
     - `FilterBySourceType` - Create documents with source_type "jira" and "confluence", test `?source_type=jira`, verify only jira documents returned
     - `FilterByTags` - Create documents with tags ["urgent", "bug"], ["feature"], ["urgent"], test `?tags=urgent`, verify only documents with "urgent" tag returned
     - `FilterByCreatedAfter` - Create documents with different created_at timestamps, test `?created_after=<timestamp>`, verify only documents created after timestamp returned
     - `FilterByCreatedBefore` - Test `?created_before=<timestamp>`, verify only documents created before timestamp returned
     - `OrderByCreatedAt` - Test `?order_by=created_at&order_dir=desc`, verify documents ordered by created_at descending
     - `OrderByTitle` - Test `?order_by=title&order_dir=asc`, verify documents ordered by title ascending
     - `CombinedFilters` - Test `?source_type=jira&tags=urgent&limit=5`, verify filters combine correctly

4. **Test: POST /api/documents (Create)** (lines 452-600):
   - `TestDocumentsCreate(t *testing.T)` - Main test function
   - Setup: `SetupTestEnvironment()`, cleanup before/after
   - Subtests:
     - `Success` - POST valid document, verify 201 Created, verify response contains id/source_type/title, verify document retrievable via GET
     - `WithMetadata` - POST document with metadata map, verify metadata stored correctly
     - `WithTags` - POST document with tags array, verify tags stored correctly
     - `InvalidJSON` - POST malformed JSON, verify 400 Bad Request
     - `MissingID` - POST document without id field, verify 400 Bad Request with error message
     - `MissingSourceType` - POST document without source_type field, verify 400 Bad Request
     - `EmptyID` - POST document with empty string id, verify 400 Bad Request
     - `EmptySourceType` - POST document with empty string source_type, verify 400 Bad Request
     - `DuplicateID` - Create document, attempt to create another with same ID, verify error (400 or 500)

5. **Test: GET /api/documents/{id} (Get)** (lines 602-720):
   - `TestDocumentsGet(t *testing.T)` - Main test function
   - Setup: `SetupTestEnvironment()`, cleanup before/after
   - Subtests:
     - `Success` - Create document, GET by ID, verify 200 OK, verify all fields match (id, source_type, title, content_markdown, url, metadata, tags, created_at, updated_at)
     - `NotFound` - GET with nonexistent ID, verify 404 Not Found
     - `EmptyID` - GET `/api/documents/`, verify 400 or 404
     - `WithComplexMetadata` - Create document with nested metadata (Jira-style), GET by ID, verify metadata structure preserved

6. **Test: DELETE /api/documents/{id} (Delete)** (lines 722-820):
   - `TestDocumentsDelete(t *testing.T)` - Main test function
   - Setup: `SetupTestEnvironment()`, cleanup before/after
   - Subtests:
     - `Success` - Create document, DELETE by ID, verify 200 OK, verify response contains doc_id and message, verify GET returns 404
     - `NotFound` - DELETE nonexistent ID, verify 500 Internal Server Error (per handler implementation)
     - `EmptyID` - DELETE `/api/documents/`, verify 400 Bad Request
     - `MultipleDeletes` - Create 3 documents, delete all 3, verify each deletion successful, verify list empty

7. **Test: POST /api/documents/{id}/reprocess (Reprocess)** (lines 822-900):
   - `TestDocumentsReprocess(t *testing.T)` - Main test function
   - Setup: `SetupTestEnvironment()`, cleanup before/after
   - Subtests:
     - `Success` - Create document, POST to `/api/documents/{id}/reprocess`, verify 200 OK, verify response contains success=true and message (note: endpoint is no-op after Phase 5)
     - `NotFound` - POST reprocess for nonexistent ID, verify 400 Bad Request (per handler implementation)
     - `EmptyID` - POST to `/api/documents//reprocess`, verify 400 Bad Request

8. **Test: GET /api/documents/stats (Stats)** (lines 902-1020):
   - `TestDocumentsStats(t *testing.T)` - Main test function
   - Setup: `SetupTestEnvironment()`, cleanup before/after
   - Subtests:
     - `EmptyDatabase` - Cleanup all documents, GET stats, verify total_documents=0, documents_by_source empty map
     - `SingleDocument` - Create 1 document, GET stats, verify total_documents=1, documents_by_source has correct count
     - `MultipleSourceTypes` - Create 3 jira docs, 2 confluence docs, 1 github doc, GET stats, verify total_documents=6, verify documents_by_source breakdown (jira=3, confluence=2, github=1)
     - `StatsFields` - Verify response contains required fields: total_documents, documents_by_source, jira_documents, confluence_documents, last_updated, average_content_size

9. **Test: GET /api/documents/tags (Tags)** (lines 1022-1120):
   - `TestDocumentsTags(t *testing.T)` - Main test function
   - Setup: `SetupTestEnvironment()`, cleanup before/after
   - Subtests:
     - `EmptyDatabase` - Cleanup all documents, GET tags, verify response contains tags array (empty)
     - `SingleTag` - Create document with tags ["urgent"], GET tags, verify tags array contains "urgent"
     - `MultipleTags` - Create documents with tags ["urgent", "bug"], ["feature"], ["urgent", "enhancement"], GET tags, verify tags array contains unique tags: ["urgent", "bug", "feature", "enhancement"]
     - `DuplicateTags` - Create 3 documents all with tag "urgent", GET tags, verify "urgent" appears only once in response
     - `NoTags` - Create documents without tags field, GET tags, verify empty array or no error

10. **Test: DELETE /api/documents/clear-all (DeleteAll)** (lines 1122-1220):
    - `TestDocumentsClearAll(t *testing.T)` - Main test function
    - Setup: `SetupTestEnvironment()`, cleanup before/after
    - Subtests:
      - `Success` - Create 10 documents, DELETE `/api/documents/clear-all`, verify 200 OK, verify response contains message and documents_affected=10, verify GET list returns empty array
      - `EmptyDatabase` - Cleanup all, DELETE clear-all, verify 200 OK, verify documents_affected=0
      - `VerifyAllDeleted` - Create 5 documents with different source types, DELETE clear-all, verify stats shows total_documents=0, verify list returns empty array

11. **Test: Document Lifecycle** (lines 1222-1350):
    - `TestDocumentLifecycle(t *testing.T)` - Comprehensive end-to-end test
    - Setup: `SetupTestEnvironment()`, cleanup before/after
    - Flow:
      1. Create document via POST, verify 201 Created
      2. Get document via GET, verify fields match
      3. Verify document appears in list
      4. Verify stats incremented
      5. Verify tags endpoint returns document tags
      6. Reprocess document, verify success
      7. Delete document, verify 200 OK
      8. Verify GET returns 404
      9. Verify list no longer contains document
      10. Verify stats decremented

**Test Patterns:**
- All tests use `require.NoError()` for setup operations
- All tests use `assert` for verification
- All tests cleanup before and after for isolation
- All tests use `HTTPTestHelper` for requests
- All tests log completion with `t.Log("✓ Test completed")`
- All subtests have descriptive names matching scenario

**Error Handling:**
- Invalid JSON → 400 Bad Request
- Missing required fields → 400 Bad Request
- Not found → 404 Not Found (GET) or 500 Internal Server Error (DELETE)
- Empty ID → 400 Bad Request

**Pagination/Filtering:**
- Default limit: 20 (per handler implementation)
- Test limit/offset combinations
- Test source_type, tags, created_after, created_before filters
- Test order_by and order_dir parameters
- Verify total_count in response

**Data Cleanup:**
- `cleanupAllDocuments()` called before and after each test
- Individual document cleanup via `deleteTestDocument()` in subtests
- Ensures test isolation and prevents cross-test contamination