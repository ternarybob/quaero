I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase already has most of the integration complete:
- **Routes configured**: Line 24 in `routes.go` serves the search page, line 60 handles the API endpoint
- **Navbar configured**: Line 19 in `navbar.html` includes the Search link with active state handling
- **Handler initialized**: Line 645 in `app.go` creates `SearchHandler` with `SearchService` dependency
- **Service interface**: Both `FTS5SearchService` and `AdvancedSearchService` implement `interfaces.SearchService`
- **Constructor compatibility**: Both services use identical constructor signatures: `(storage interfaces.DocumentStorage, logger arbor.ILogger)`

The only required change is updating line 290 in `app.go` to instantiate `AdvancedSearchService` instead of `FTS5SearchService`. This enables Google-style query parsing (OR default, +AND, "phrases", document_type:qualifier, case:match) while maintaining full backward compatibility with existing code.

### Approach

Replace the existing `FTS5SearchService` with `AdvancedSearchService` in the application initialization. The routes and navbar are already configured correctly, so only the service initialization needs to be updated. This is a simple drop-in replacement since both services implement the same `interfaces.SearchService` interface and have identical constructor signatures.

### Reasoning

I examined the application initialization in `app.go`, verified the routing configuration in `routes.go`, checked the navbar implementation in `navbar.html`, and reviewed the `AdvancedSearchService` constructor to confirm compatibility. I discovered that the routes (`/search` page and `/api/search` endpoint) and navbar link are already properly configured, requiring only a service swap in the initialization code.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Navbar
    participant SearchPage
    participant SearchHandler
    participant AdvancedSearchService
    participant QueryParser
    participant SQLite

    User->>Navbar: Click "SEARCH" link
    Navbar->>SearchPage: Navigate to /search
    SearchPage->>User: Display search UI with syntax help
    
    User->>SearchPage: Enter "+cat dog document_type:jira"
    User->>SearchPage: Click Search button
    
    SearchPage->>SearchHandler: GET /api/search?q=...&limit=50
    SearchHandler->>AdvancedSearchService: Search(ctx, query, opts)
    
    AdvancedSearchService->>QueryParser: Tokenize(query)
    QueryParser-->>AdvancedSearchService: tokens[]
    
    AdvancedSearchService->>QueryParser: ExtractQualifiers(tokens)
    QueryParser-->>AdvancedSearchService: {document_type: "jira"}
    
    AdvancedSearchService->>QueryParser: BuildFTS5Query(tokens)
    QueryParser-->>AdvancedSearchService: "cat AND (dog)"
    
    AdvancedSearchService->>SQLite: FullTextSearch(fts5Query, limit)
    SQLite-->>AdvancedSearchService: documents[]
    
    AdvancedSearchService->>AdvancedSearchService: applyDocumentTypeFilter("jira")
    AdvancedSearchService->>AdvancedSearchService: applyCaseSensitivity()
    
    AdvancedSearchService-->>SearchHandler: filtered documents[]
    SearchHandler->>SearchHandler: Transform to SearchResult (truncate to 200 chars)
    SearchHandler-->>SearchPage: JSON {results, count, query}
    
    SearchPage->>SearchPage: renderResults()
    SearchPage->>SearchPage: renderPagination()
    SearchPage-->>User: Display results with links

## Proposed File Changes

### internal\app\app.go(MODIFY)

References: 

- internal\services\search\advanced_search_service.go
- internal\services\search\fts5_search_service.go
- internal\interfaces\search_service.go
- internal\handlers\search_handler.go

**Update Search Service Initialization (Line 289-293)**

Replace the `FTS5SearchService` initialization with `AdvancedSearchService`:

**Current code (lines 289-293):**
```go
// 3.5 Initialize search service (FTS5-based search)
a.SearchService = search.NewFTS5SearchService(
    a.StorageManager.DocumentStorage(),
    a.Logger,
)
```

**Change to:**
```go
// 3.5 Initialize search service (Advanced search with Google-style query parsing)
a.SearchService = search.NewAdvancedSearchService(
    a.StorageManager.DocumentStorage(),
    a.Logger,
)
```

**Rationale:**
- `AdvancedSearchService` implements the same `interfaces.SearchService` interface as `FTS5SearchService`
- Constructor signatures are identical (both accept `DocumentStorage` and `Logger`)
- Provides enhanced query parsing capabilities:
  - OR search (default): `cat dog` finds documents with any term
  - AND search (+prefix): `+cat +dog` requires all terms
  - Phrase search (quotes): `"cat on mat"` finds exact phrase
  - Qualifiers: `document_type:jira`, `case:match`
  - Mixed queries: `+cat dog "on mat" document_type:jira`
- Maintains full backward compatibility with existing search functionality
- No changes required to `SearchHandler` or any other dependent code

**Testing Verification:**
- Verify application starts without errors
- Test basic search queries via `/search` page
- Test Google-style query syntax (OR, AND, phrases, qualifiers)
- Verify API endpoint `/api/search` returns correct results
- Confirm existing chat service RAG search continues to work (uses same `SearchService` interface)
- Run existing search tests to ensure no regressions

### internal\server\routes.go(MODIFY)

References: 

- internal\handlers\search_handler.go
- pages\search.html

**Verification Only - No Changes Required**

Confirm the following routes are already configured correctly:

**Line 24:** Page route for search UI
```go
mux.HandleFunc("/search", s.app.PageHandler.ServePage("search.html", "search"))
```

**Line 60:** API route for search endpoint
```go
mux.HandleFunc("/api/search", s.app.SearchHandler.SearchHandler)
```

**Status:** ✅ Both routes are already properly configured. The `SearchHandler.SearchHandler` method accepts GET requests with query parameters (`q`, `limit`, `offset`) and returns JSON results. No modifications needed.

**Testing Verification:**
- Access `/search` page in browser - should load search UI
- Test API endpoint: `GET /api/search?q=test&limit=10`
- Verify JSON response structure: `{results: [], count: 0, query: "", limit: 0, offset: 0}`

### pages\partials\navbar.html(MODIFY)

References: 

- pages\search.html

**Verification Only - No Changes Required**

Confirm the Search navigation link is already configured correctly:

**Line 19:** Search link with active state handling
```html
<a href="/search" @click="mobileMenuOpen = false" {{if eq .Page "search"}}class="active"{{end}}>SEARCH</a>
```

**Status:** ✅ The Search link is already properly integrated into the navigation bar with:
- Correct route: `/search`
- Mobile menu support: `@click="mobileMenuOpen = false"`
- Active state highlighting: `{{if eq .Page "search"}}class="active"{{end}}`
- Positioned between DOCUMENTS and CHAT links (logical placement)

**Testing Verification:**
- Click Search link in navbar - should navigate to `/search` page
- Verify active state highlights when on search page
- Test mobile menu closes after clicking Search link
- Confirm navbar status indicator (ONLINE/OFFLINE) works correctly