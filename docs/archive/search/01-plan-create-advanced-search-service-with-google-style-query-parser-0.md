I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase has a well-established search architecture with `FTS5SearchService` using SQLite's FTS5 full-text search. The FTS5 index searches both `title` and `content_markdown` fields. The existing service follows constructor-based dependency injection with `DocumentStorage` and `arbor.ILogger`. The task requires creating a new advanced search service with Google-style query parsing that converts to FTS5 syntax while handling application-level filters.

### Approach

Create a new `AdvancedSearchService` in `internal/services/search/` that implements Google-style query parsing. The service will parse queries to identify OR terms (default), AND terms (+prefix), literal phrases (quoted), and qualifiers (document_type:, case:). It will convert parsed queries to FTS5 syntax, execute searches via `DocumentStorage.FullTextSearch()`, and apply application-level filters. The service will implement the existing `interfaces.SearchService` interface for consistency and future MCP integration.

### Reasoning

I explored the existing search service implementation in `fts5_search_service.go`, examined the FTS5 schema configuration in `schema.go`, reviewed the `DocumentStorage.FullTextSearch()` method, and studied the handler patterns in `document_handler.go`. I also checked the app initialization in `app.go` to understand the dependency injection pattern.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant SearchHandler
    participant AdvancedSearchService
    participant QueryParser
    participant DocumentStorage
    participant FTS5Index

    User->>SearchHandler: GET /api/search?q=+cat "on mat" document_type:jira
    SearchHandler->>AdvancedSearchService: Search(ctx, query, opts)
    AdvancedSearchService->>QueryParser: parseQuery(query)
    QueryParser->>QueryParser: Tokenize input
    QueryParser->>QueryParser: Extract qualifiers
    QueryParser->>QueryParser: Identify +terms (AND)
    QueryParser->>QueryParser: Build FTS5 query
    QueryParser-->>AdvancedSearchService: ParsedQuery{fts5Query, filters}
    AdvancedSearchService->>DocumentStorage: FullTextSearch(fts5Query, limit)
    DocumentStorage->>FTS5Index: MATCH query
    FTS5Index-->>DocumentStorage: Ranked results
    DocumentStorage-->>AdvancedSearchService: []*Document
    AdvancedSearchService->>AdvancedSearchService: applyDocumentTypeFilter()
    AdvancedSearchService->>AdvancedSearchService: applyCaseSensitivity()
    AdvancedSearchService-->>SearchHandler: Filtered results
    SearchHandler-->>User: JSON response

## Proposed File Changes

### internal\services\search\advanced_search_service.go(NEW)

References: 

- internal\services\search\fts5_search_service.go
- internal\interfaces\search_service.go
- internal\storage\sqlite\document_storage.go
- internal\models\document.go

Create `AdvancedSearchService` struct with fields: `storage` (interfaces.DocumentStorage), `logger` (arbor.ILogger). Implement constructor `NewAdvancedSearchService()` following the pattern in `fts5_search_service.go`.

Implement `Search()` method that:
1. Calls `parseQuery()` to parse Google-style query into `ParsedQuery` struct
2. Executes FTS5 search via `storage.FullTextSearch()` with converted query
3. Applies application-level filters (document_type, case sensitivity)
4. Returns filtered results

Create internal `ParsedQuery` struct with fields:
- `fts5Query` (string) - Converted FTS5 query
- `documentType` (string) - Extracted from document_type: qualifier
- `caseSensitive` (bool) - Extracted from case:match qualifier
- `originalTerms` ([]string) - For case-sensitive post-filtering

Implement `parseQuery()` method that:
1. Tokenizes input respecting quoted strings using a state machine
2. Identifies qualifiers (key:value patterns) and extracts them
3. Identifies +prefix for required terms
4. Separates terms into required (AND) and optional (OR) groups
5. Returns `ParsedQuery` struct

Implement `buildFTS5Query()` method that:
1. Handles empty query → return empty string
2. Converts required terms: `+cat +dog` → `cat AND dog`
3. Converts optional terms: `cat dog` → `cat OR dog`
4. Combines required and optional: `+cat dog mat` → `cat AND (dog OR mat)`
5. Preserves quoted phrases: `"cat on mat"` → `"cat on mat"`
6. Escapes FTS5 special characters in unquoted terms

Implement `extractQualifiers()` method that:
1. Scans tokens for key:value patterns
2. Recognizes `document_type:jira`, `document_type:confluence`, etc.
3. Recognizes `case:match` flag
4. Removes qualifiers from token list
5. Returns map of qualifiers

Implement `applyDocumentTypeFilter()` method that:
1. Filters results by `SourceType` field matching document_type qualifier
2. Returns filtered document slice

Implement `applyCaseSensitivity()` method that:
1. If case:match is false, return results as-is (FTS5 default)
2. If case:match is true, post-filter results:
   - Check if original terms appear with exact case in Title or ContentMarkdown
   - Use `strings.Contains()` for exact case matching
   - Return only documents with exact case matches

Implement `GetByID()` method that delegates to `storage.GetDocument()` (implements interface).

Implement `SearchByReference()` method that wraps reference in quotes and calls `Search()` (implements interface).

Add comprehensive inline documentation explaining:
- Google-style query syntax (OR default, +AND, "phrase", qualifiers)
- FTS5 conversion rules
- Case sensitivity limitations and post-filtering approach
- Examples of query transformations

### internal\services\search\advanced_search_service_test.go(NEW)

References: 

- internal\services\search\advanced_search_service.go(NEW)
- internal\services\search\fts5_search_service_test.go

Create comprehensive unit tests following the pattern in `fts5_search_service_test.go`.

Create `mockDocumentStorage` implementation (can reuse from fts5_search_service_test.go or create new).

Test suite for `parseQuery()`:
- `TestParseQuery_SimpleOR` - "cat dog mat" → OR terms
- `TestParseQuery_RequiredAND` - "+cat +dog" → AND terms
- `TestParseQuery_MixedANDOR` - "+cat dog mat" → mixed
- `TestParseQuery_QuotedPhrase` - "\"cat on mat\"" → phrase
- `TestParseQuery_Qualifiers` - "document_type:jira cat" → extract qualifier
- `TestParseQuery_CaseMatch` - "case:match Cat" → case flag
- `TestParseQuery_EmptyQuery` - "" → empty result
- `TestParseQuery_OnlyQualifiers` - "document_type:jira" → no search terms

Test suite for `buildFTS5Query()`:
- `TestBuildFTS5_SimpleOR` - Verify "cat OR dog OR mat" output
- `TestBuildFTS5_RequiredAND` - Verify "cat AND dog" output
- `TestBuildFTS5_MixedANDOR` - Verify "cat AND (dog OR mat)" output
- `TestBuildFTS5_QuotedPhrase` - Verify phrase preservation
- `TestBuildFTS5_SpecialChars` - Verify escaping of FTS5 special chars

Test suite for `Search()`:
- `TestSearch_SimpleQuery` - Basic search with OR terms
- `TestSearch_WithDocumentTypeFilter` - Filter by document_type:jira
- `TestSearch_WithCaseSensitive` - Verify case:match post-filtering
- `TestSearch_EmptyQuery` - Return all documents
- `TestSearch_NoResults` - Handle empty result set
- `TestSearch_ComplexQuery` - Combined +terms, phrases, qualifiers

Test suite for edge cases:
- `TestEdgeCase_UnbalancedQuotes` - Handle "cat dog (missing close quote)
- `TestEdgeCase_MultipleQualifiers` - "document_type:jira case:match cat"
- `TestEdgeCase_InvalidQualifier` - "unknown:value cat" (ignore invalid)
- `TestEdgeCase_OnlyPlusSign` - "+" alone
- `TestEdgeCase_SpecialCharacters` - Handle FTS5 operators in query

Use table-driven test pattern for multiple test cases per function.

### internal\services\search\query_parser.go(NEW)

References: 

- internal\services\search\advanced_search_service.go(NEW)

Create a separate `QueryParser` helper struct to encapsulate query parsing logic (optional refactoring for cleaner code organization).

Define `QueryParser` struct with no fields (stateless parser).

Define `Token` struct with fields:
- `value` (string) - Token text
- `tokenType` (TokenType enum) - TERM, PHRASE, QUALIFIER, OPERATOR
- `required` (bool) - True if prefixed with +

Define `TokenType` constants:
- `TokenTypeTerm` - Regular search term
- `TokenTypePhrase` - Quoted phrase
- `TokenTypeQualifier` - key:value pair
- `TokenTypeOperator` - Special operators (future: NOT, etc.)

Implement `Tokenize()` method that:
1. Scans input string character by character
2. Maintains state: IN_TERM, IN_QUOTE, IN_QUALIFIER
3. Handles quote escaping (\")
4. Detects +prefix for required terms
5. Detects : for qualifiers
6. Returns slice of Token structs

Implement `IsQualifier()` helper that checks if token matches key:value pattern.

Implement `EscapeFTS5()` helper that escapes FTS5 special characters:
- Escape: " (double quote), * (wildcard), ^ (boost)
- Do not escape: + (handled separately), - (hyphen in terms is OK)

Implement `SplitQualifier()` helper that splits "key:value" into (key, value) tuple.

Add unit tests in `query_parser_test.go` for tokenization edge cases.