I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase has well-established patterns for HTTP handlers with clear examples in `document_handler.go` and `chat_handler.go`. The `AdvancedSearchService` is already implemented with a `Search()` method that returns `[]*models.Document`. Helper functions exist in `helpers.go` for common tasks like `WriteJSON()`, `WriteError()`, and query parameter parsing. The `Document` model contains all necessary fields: ID, Title, ContentMarkdown, URL, SourceType. The task requires creating a simple GET endpoint that wraps the search service and formats results with truncated markdown summaries.

### Approach

Create `SearchHandler` struct following the established handler pattern with constructor-based dependency injection. Implement a single `SearchHandler()` method for GET `/api/search?q=query` that parses query parameters, calls `AdvancedSearchService.Search()`, transforms full documents into brief result objects (truncating ContentMarkdown to 200 characters), and returns JSON. Use existing helper functions from `helpers.go` for consistency. Handle edge cases: empty query, no results, parsing errors, and service errors gracefully.

### Reasoning

I examined the existing handler patterns in `document_handler.go` and `chat_handler.go`, reviewed the `AdvancedSearchService` implementation to understand its interface, studied the `Document` model structure, checked available helper functions in `helpers.go`, and reviewed the routing patterns in `routes.go` and app initialization in `app.go` to understand the dependency injection flow.

## Mermaid Diagram

sequenceDiagram
    participant Client
    participant SearchHandler
    participant AdvancedSearchService
    participant DocumentStorage
    
    Client->>SearchHandler: GET /api/search?q=cat+dog&limit=10
    SearchHandler->>SearchHandler: Parse query params (q, limit, offset)
    SearchHandler->>SearchHandler: Build SearchOptions
    SearchHandler->>AdvancedSearchService: Search(ctx, "cat dog", opts)
    AdvancedSearchService->>AdvancedSearchService: Parse Google-style query
    AdvancedSearchService->>DocumentStorage: FullTextSearch(fts5Query, limit)
    DocumentStorage-->>AdvancedSearchService: []*Document (full objects)
    AdvancedSearchService->>AdvancedSearchService: Apply filters
    AdvancedSearchService-->>SearchHandler: []*Document
    SearchHandler->>SearchHandler: Transform to SearchResult
    SearchHandler->>SearchHandler: Truncate ContentMarkdown to 200 chars
    SearchHandler->>SearchHandler: Build JSON response
    SearchHandler-->>Client: JSON {results, count, query, limit, offset}

## Proposed File Changes

### internal\handlers\search_handler.go(NEW)

References: 

- internal\handlers\document_handler.go
- internal\handlers\chat_handler.go
- internal\handlers\helpers.go
- internal\services\search\advanced_search_service.go
- internal\interfaces\search_service.go
- internal\models\document.go

Create `SearchHandler` struct with fields:
- `searchService` (interfaces.SearchService) - for executing searches
- `logger` (arbor.ILogger) - for structured logging

Implement constructor `NewSearchHandler(searchService interfaces.SearchService, logger arbor.ILogger) *SearchHandler` following the pattern from `document_handler.go` and `chat_handler.go`.

Implement `SearchHandler(w http.ResponseWriter, r *http.Request)` method:

**1. Method Validation:**
- Check if request method is GET using pattern from `document_handler.go:34`
- Return 405 Method Not Allowed if not GET

**2. Query Parameter Parsing:**
- Extract `q` parameter (search query) using `r.URL.Query().Get("q")`
- Extract `limit` parameter with default 50, max 100 (follow pattern from `document_handler.go:61-73`)
- Extract `offset` parameter with default 0 (follow pattern from `document_handler.go:75-79`)
- Log the search request with query, limit, offset using `logger.Info()`

**3. Build SearchOptions:**
- Create `interfaces.SearchOptions` struct with parsed Limit and Offset
- Leave SourceTypes and MetadataFilters empty (not needed for basic search)

**4. Execute Search:**
- Call `searchService.Search(r.Context(), query, opts)`
- Handle errors by logging with `logger.Error().Err(err).Msg()` and returning 500 with error message
- Handle empty results gracefully (return empty array, not error)

**5. Transform Results:**
- Create `SearchResult` struct (defined in same file) with fields:
  - `ID` (string) - document ID
  - `Title` (string) - document title
  - `Brief` (string) - truncated markdown (first 200 chars)
  - `URL` (string) - link to original document
  - `SourceType` (string) - document source type (jira, confluence, etc.)
- Iterate through documents and transform each:
  - Copy ID, Title, URL, SourceType directly
  - Truncate ContentMarkdown to 200 characters for Brief field
  - If ContentMarkdown > 200 chars, append "..." to indicate truncation
  - Handle empty ContentMarkdown gracefully (use empty string)

**6. Build Response:**
- Create response map with structure:
  - `results` ([]SearchResult) - array of search results
  - `count` (int) - number of results in current response
  - `query` (string) - original search query
  - `limit` (int) - limit used
  - `offset` (int) - offset used
- Follow response pattern from `document_handler.go:102-108`

**7. Return JSON:**
- Set Content-Type header to application/json
- Encode response using `json.NewEncoder(w).Encode(response)`
- Follow pattern from `document_handler.go:110-111`

**Error Handling:**
- Empty query: Allow (returns all documents up to limit)
- Service errors: Log and return 500 with generic error message
- No results: Return empty array with count=0 (not an error)
- Invalid limit/offset: Use defaults (don't error)

**Logging:**
- Log search request with query, limit, offset at Info level
- Log search completion with result count at Debug level
- Log errors at Error level with full error details

### internal\handlers\search_handler_test.go(NEW)

References: 

- internal\handlers\search_handler.go(NEW)
- internal\handlers\document_handler.go

Create comprehensive unit tests for `SearchHandler` following the testing patterns in the codebase.

**Mock SearchService:**
- Create `mockSearchService` struct implementing `interfaces.SearchService`
- Implement `Search()` method that returns configurable results or errors
- Implement `GetByID()` and `SearchByReference()` as no-ops (not used by handler)

**Test Suite:**

`TestSearchHandler_Success` - Verify successful search:
- Mock service returns 3 documents with varying ContentMarkdown lengths
- Verify response structure matches expected format
- Verify Brief field is truncated to 200 chars with "..." suffix
- Verify all fields (ID, Title, URL, SourceType) are copied correctly
- Verify count matches number of results

`TestSearchHandler_EmptyQuery` - Verify empty query handling:
- Call handler with empty q parameter
- Verify service is called with empty string
- Verify results are returned (not an error)

`TestSearchHandler_NoResults` - Verify empty result handling:
- Mock service returns empty array
- Verify response has empty results array
- Verify count is 0
- Verify HTTP status is 200 (not an error)

`TestSearchHandler_ServiceError` - Verify error handling:
- Mock service returns error
- Verify handler returns 500 status
- Verify error is logged
- Verify response contains error message

`TestSearchHandler_MethodNotAllowed` - Verify method validation:
- Send POST request to handler
- Verify 405 Method Not Allowed response

`TestSearchHandler_Pagination` - Verify pagination parameters:
- Call handler with limit=10, offset=20
- Verify SearchOptions passed to service has correct values
- Verify response includes limit and offset

`TestSearchHandler_DefaultPagination` - Verify defaults:
- Call handler without limit/offset parameters
- Verify defaults are used (limit=50, offset=0)

`TestSearchHandler_TruncationEdgeCases` - Verify truncation logic:
- Test document with exactly 200 chars (no truncation)
- Test document with 201 chars (truncate with "...")
- Test document with empty ContentMarkdown
- Test document with very short ContentMarkdown

`TestSearchHandler_InvalidPagination` - Verify invalid parameter handling:
- Call handler with non-numeric limit/offset
- Verify defaults are used (graceful fallback)

**Test Helpers:**
- Create `createTestDocument()` helper to generate test documents
- Create `executeSearchRequest()` helper to make HTTP requests
- Use table-driven tests where appropriate

**Assertions:**
- Verify HTTP status codes
- Verify response JSON structure
- Verify field values and transformations
- Verify logging calls (if using mock logger)

Follow testing patterns from existing handler tests in the codebase.