I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Architecture:**

✅ **html_scraper provides:** Generic metadata (title, description, Open Graph, JSON-LD) + RawHTML + Markdown + Links
- Cannot extract Jira-specific fields: IssueKey, ProjectKey, Status, Priority, Assignee, Reporter, Labels, Components
- Cannot extract Confluence-specific fields: PageID, SpaceKey, SpaceName, Author, Version, ContentType

✅ **Parsers provide:** Specialized extraction using CSS selectors with multiple fallbacks for resilience
- `jira_parser.go`: 239 lines of Jira-specific extraction logic
- `confluence_parser.go`: 224 lines of Confluence-specific extraction logic

✅ **Helper functions in crawler/helpers.go:** Reusable extraction utilities
- `createDocument()` - Creates goquery.Document from HTML string
- `extractTextFromDoc()` - Tries multiple selectors in priority order
- `extractMultipleTextsFromDoc()` - Collects text from all matching elements
- `extractCleanedHTML()` - Extracts and cleans HTML from selectors
- `extractDateFromDoc()` - Extracts dates with normalization
- `parseJiraIssueKey()`, `parseConfluencePageID()`, `parseSpaceKey()` - Regex-based ID extraction
- `normalizeStatus()` - Status normalization

**Refactoring Strategy:**

The cleanest approach is to **move the parsing logic inline into the transformer methods** rather than trying to enhance html_scraper with source-specific extraction. This keeps the html_scraper generic (as intended) while consolidating the specialized logic where it's actually used.

**Benefits:**
- ✅ Eliminates parser layer (2 files deleted)
- ✅ Maintains extraction quality (same CSS selectors, same fallbacks)
- ✅ Reuses existing helper functions from crawler/helpers.go
- ✅ Keeps html_scraper generic and reusable
- ✅ Reduces indirection (transformers parse directly instead of calling separate parsers)

**Trade-offs:**
- ⚠️ Transformers become longer (add ~150-200 lines each)
- ⚠️ Parsing logic is now in atlassian package instead of crawler package
- ✅ But: This is acceptable since the logic is Jira/Confluence-specific, not generic crawling logic

### Approach

**Consolidate HTML parsing into transformers by moving specialized extraction logic from separate parser files directly into the transformer methods, leveraging html_scraper's RawHTML output and reusing existing helper functions.** This eliminates the parser layer while maintaining the same metadata extraction quality through inline parsing within each transformer.

### Reasoning

I analyzed the current architecture and discovered that `jira_parser.go` and `confluence_parser.go` contain specialized CSS selector-based extraction logic that cannot be replaced by html_scraper's generic metadata extraction. The parsers use helper functions from `crawler/helpers.go` (createDocument, extractTextFromDoc, extractCleanedHTML, etc.) to extract Jira/Confluence-specific fields. The transformers call these parsers at lines 210 in both `jira_transformer.go` and `confluence_transformer.go`. The solution is to move the parsing logic inline into the transformer methods, eliminating the separate parser files while maintaining extraction quality.

## Mermaid Diagram

sequenceDiagram
    participant Crawler as Crawler Service
    participant Scraper as HTML Scraper
    participant Transformer as Transformer<br/>(Jira/Confluence)
    participant Helpers as Shared Helpers<br/>(crawler/helpers.go)
    participant Storage as Document Storage

    Note over Crawler,Storage: BEFORE: Separate Parser Layer

    Crawler->>Scraper: ScrapeURL(url)
    Scraper-->>Crawler: ScrapeResult<br/>(RawHTML, Markdown, Metadata)
    Crawler->>Transformer: CrawlResult
    Transformer->>Transformer: OLD: Call jira_parser.ParseJiraIssuePage()
    Note right of Transformer: Separate parser file<br/>extracts structured data
    Transformer->>Storage: SaveDocument(doc)

    Note over Crawler,Storage: AFTER: Inline Parsing in Transformers

    Crawler->>Scraper: ScrapeURL(url)
    Scraper-->>Crawler: ScrapeResult<br/>(RawHTML, Markdown, Metadata)
    Crawler->>Transformer: CrawlResult
    Transformer->>Helpers: createDocument(html)
    Helpers-->>Transformer: goquery.Document
    Transformer->>Helpers: extractTextFromDoc(doc, selectors)
    Helpers-->>Transformer: IssueKey
    Transformer->>Helpers: extractCleanedHTML(doc, selectors)
    Helpers-->>Transformer: Description HTML
    Transformer->>Helpers: extractDateFromDoc(doc, selectors)
    Helpers-->>Transformer: CreatedDate
    Transformer->>Helpers: parseJiraIssueKey(text)
    Helpers-->>Transformer: Parsed key
    Note right of Transformer: Parse inline using<br/>shared helpers
    Transformer->>Transformer: Build JiraMetadata
    Transformer->>Transformer: Convert HTML to Markdown
    Transformer->>Storage: SaveDocument(doc)

    Note over Crawler,Storage: Parser files deleted, logic consolidated

## Proposed File Changes

### internal\services\atlassian\jira_transformer.go(MODIFY)

References: 

- internal\services\crawler\jira_parser.go(DELETE)
- internal\services\crawler\helpers.go(MODIFY)

**Refactor parseJiraIssue method to parse HTML directly instead of calling external parser:**

**Lines 192-328 (parseJiraIssue method):**

Replace the call to `crawler.ParseJiraIssuePage()` at line 210 with inline parsing logic:

1. **Remove parser call** (line 210): Delete `issueData, err := crawler.ParseJiraIssuePage(html, pageURL, t.logger)`

2. **Add inline parsing logic** (after line 207):
   - Call `crawler.createDocument(html)` to create goquery.Document (reuse helper from `crawler/helpers.go`)
   - Extract IssueKey using `crawler.extractTextFromDoc()` with selectors: `[data-test-id="issue.views.issue-base.foundation.breadcrumbs.current-issue.item"]`, `#key-val`, `#issuekey-val`
   - Fallback: Parse IssueKey from page title using `crawler.parseJiraIssueKey()` helper
   - Extract ProjectKey by splitting IssueKey on "-" and taking first part
   - Extract Summary using selectors: `[data-test-id="issue.views.issue-base.foundation.summary.heading"]`, `#summary-val`, `h1[data-test-id="issue-view-heading"]`
   - Fallback: Extract from title tag and strip " - Jira" suffix
   - Extract Description HTML using `crawler.extractCleanedHTML()` with selectors: `[data-test-id="issue.views.field.rich-text.description"]`, `#description-val`, `.user-content-block`
   - Extract IssueType using selectors: `[data-test-id="issue.views.field.issue-type"] span`, `#type-val`, `.issue-type-icon`
   - Extract Status using selectors: `[data-test-id="issue.views.field.status"] span`, `#status-val`, `.status`, `.aui-lozenge`, then normalize with `crawler.normalizeStatus()`
   - Extract Priority using selectors: `[data-test-id="issue.views.field.priority"] span`, `#priority-val`, `.priority-icon`
   - Extract Assignee using selectors: `[data-test-id="issue.views.field.assignee"] span`, `#assignee-val`, `.user-hover`, return empty string if "Unassigned"
   - Extract Reporter using selectors: `[data-test-id="issue.views.field.reporter"] span`, `#reporter-val`
   - Extract Labels using `crawler.extractMultipleTextsFromDoc()` with selectors: `[data-test-id="issue.views.field.labels"] a`, `#labels-val .labels a`, `.labels .lozenge`
   - Extract Components using selectors: `[data-test-id="issue.views.field.components"] a`, `#components-val a`
   - Extract CreatedDate using `crawler.extractDateFromDoc()` with selectors: `[data-test-id="issue.views.field.created"] time`, `#created-val time`
   - Extract UpdatedDate using selectors: `[data-test-id="issue.views.field.updated"] time`, `#updated-val time`
   - Extract ResolutionDate using selectors: `[data-test-id="issue.views.field.resolved"] time`

3. **Add validation** (after extraction):
   - Check that IssueKey is not empty, return error if missing: "missing required field: IssueKey"
   - Check that Summary is not empty, return error if missing: "missing required field: Summary"
   - Log warning with missing fields if validation fails

4. **Keep existing logic** (lines 232-328):
   - URL resolution (line 232)
   - HTML to markdown conversion (line 235)
   - Date parsing to *time.Time (lines 254-275)
   - Metadata building (lines 278-292)
   - Document creation (lines 314-325)

**Import changes:**
- Keep existing import of `"github.com/ternarybob/quaero/internal/services/crawler"` for helper function access
- The helpers are already exported (capitalized) so they can be called from atlassian package

**Note:** This consolidates ~150 lines of parsing logic from `jira_parser.go` directly into the transformer, eliminating the need for the separate parser file while maintaining identical extraction behavior.

### internal\services\atlassian\confluence_transformer.go(MODIFY)

References: 

- internal\services\crawler\confluence_parser.go(DELETE)
- internal\services\crawler\helpers.go(MODIFY)

**Refactor parseConfluencePage method to parse HTML directly instead of calling external parser:**

**Lines 192-317 (parseConfluencePage method):**

Replace the call to `crawler.ParseConfluencePage()` at line 210 with inline parsing logic:

1. **Remove parser call** (line 210): Delete `pageData, err := crawler.ParseConfluencePage(html, pageURL, t.logger)`

2. **Add inline parsing logic** (after line 207):
   - Call `crawler.createDocument(html)` to create goquery.Document (reuse helper from `crawler/helpers.go`)
   - Extract PageID using `crawler.parseConfluencePageID(pageURL)` helper
   - Fallback: Try meta tag `meta[name="ajs-page-id"]` content attribute
   - Fallback: Try data attribute `#main-content` data-page-id attribute
   - Extract PageTitle using `crawler.extractTextFromDoc()` with selectors: `#title-text`, `[data-test-id="page-title"]`, `h1.page-title`
   - Fallback: Extract from title tag and strip " - Confluence" suffix
   - Extract SpaceKey using `crawler.parseSpaceKey(pageURL)` helper
   - Extract SpaceName using selectors: `[data-test-id="breadcrumbs"] a[href*="/spaces/"]`, `.aui-nav-breadcrumbs a`
   - Fallback: Use SpaceKey as SpaceName if not found
   - Extract Content HTML using `crawler.extractCleanedHTML()` with selectors: `#main-content .wiki-content`, `.page-content .wiki-content`, `[data-test-id="page-content"]`
   - Extract Author using selectors: `[data-test-id="page-metadata-author"] a`, `.author a`
   - Fallback: Try meta tag `meta[name="confluence-author"]` content attribute
   - Extract Version using selectors: `[data-test-id="page-metadata-version"]`, `.page-metadata .version`
   - Parse version number from text using regex `\d+`, default to 1 if not found
   - Determine ContentType: return "blogpost" if URL contains "/blogposts/", otherwise "page"
   - Extract LastModified using `crawler.extractDateFromDoc()` with selectors: `[data-test-id="page-metadata-modified"] time`, `.last-modified time`
   - Extract CreatedDate using selectors: `[data-test-id="page-metadata-created"] time`, `.created time`
   - Extract Labels using `crawler.extractMultipleTextsFromDoc()` with selectors: `[data-test-id="page-metadata-labels"] a`, `.labels a.label`, `.aui-label`

3. **Add validation** (after extraction):
   - Check that PageID is not empty, return error if missing: "missing required field: PageID"
   - Check that PageTitle is not empty, return error if missing: "missing required field: PageTitle"
   - Log warning with missing fields if validation fails

4. **Keep existing logic** (lines 232-316):
   - URL resolution (line 232)
   - HTML to markdown conversion (line 235)
   - Date parsing to *time.Time (lines 254-268)
   - Metadata building (lines 271-281)
   - Document creation (lines 303-314)

**Import changes:**
- Keep existing import of `"github.com/ternarybob/quaero/internal/services/crawler"` for helper function access
- Add import for `"regexp"` and `"strconv"` for version parsing
- The helpers are already exported (capitalized) so they can be called from atlassian package

**Note:** This consolidates ~140 lines of parsing logic from `confluence_parser.go` directly into the transformer, eliminating the need for the separate parser file while maintaining identical extraction behavior.

### internal\services\crawler\jira_parser.go(DELETE)

References: 

- internal\services\atlassian\jira_transformer.go(MODIFY)

**Delete the entire Jira parser file as parsing logic has been moved inline into jira_transformer.go:**

This file contains:
- `JiraIssueData` struct (lines 12-29) - No longer needed, data is extracted directly in transformer
- `ParseJiraIssuePage()` function (lines 32-78) - Logic moved inline to `jira_transformer.go` parseJiraIssue method
- Helper extraction functions (lines 80-220) - Replaced by calls to shared helpers in `crawler/helpers.go`
- Validation function (lines 222-238) - Moved inline to transformer

All functionality is preserved through:
1. Inline parsing in `jira_transformer.go` using the same CSS selectors
2. Reuse of shared helper functions from `crawler/helpers.go` (createDocument, extractTextFromDoc, extractCleanedHTML, extractDateFromDoc, parseJiraIssueKey, normalizeStatus)
3. Inline validation in transformer method

**Impact:** No external references to this file exist outside of `jira_transformer.go` line 210, which is being refactored to remove the dependency.

### internal\services\crawler\confluence_parser.go(DELETE)

References: 

- internal\services\atlassian\confluence_transformer.go(MODIFY)

**Delete the entire Confluence parser file as parsing logic has been moved inline into confluence_transformer.go:**

This file contains:
- `ConfluencePageData` struct (lines 14-28) - No longer needed, data is extracted directly in transformer
- `ParseConfluencePage()` function (lines 31-71) - Logic moved inline to `confluence_transformer.go` parseConfluencePage method
- Helper extraction functions (lines 73-205) - Replaced by calls to shared helpers in `crawler/helpers.go`
- Validation function (lines 207-223) - Moved inline to transformer

All functionality is preserved through:
1. Inline parsing in `confluence_transformer.go` using the same CSS selectors
2. Reuse of shared helper functions from `crawler/helpers.go` (createDocument, extractTextFromDoc, extractCleanedHTML, extractDateFromDoc, parseConfluencePageID, parseSpaceKey)
3. Inline validation in transformer method
4. Version parsing using regex (moved inline with import of regexp and strconv packages)

**Impact:** No external references to this file exist outside of `confluence_transformer.go` line 210, which is being refactored to remove the dependency.

### internal\services\crawler\helpers.go(MODIFY)

References: 

- internal\services\atlassian\jira_transformer.go(MODIFY)
- internal\services\atlassian\confluence_transformer.go(MODIFY)

**Export helper functions to make them accessible from atlassian package:**

The helper functions in this file are already exported (capitalized function names), so no changes are needed. However, add documentation comments to clarify that these helpers are now used by both crawler and transformer packages:

**Add package-level comment** (after line 1):
```
// Package crawler provides HTML parsing utilities and helpers.
// These helpers are used by both the crawler service and specialized transformers
// (jira_transformer, confluence_transformer) for extracting structured data from HTML.
```

**Enhance function documentation:**

- `createDocument()` (line 12): Add comment: "Used by transformers to parse HTML into goquery.Document for CSS selector-based extraction"
- `extractTextFromDoc()` (line 37): Add comment: "Used by transformers to extract text using multiple fallback selectors for resilience"
- `extractMultipleTextsFromDoc()` (line 49): Add comment: "Used by transformers to extract arrays (labels, components) from HTML"
- `extractCleanedHTML()` (line 70): Add comment: "Used by transformers to extract and clean HTML content (descriptions, page content)"
- `extractDateFromDoc()` (line 85): Add comment: "Used by transformers to extract and normalize dates from time elements"
- `parseJiraIssueKey()` (line 127): Add comment: "Used by jira_transformer to extract issue keys from text using regex"
- `parseConfluencePageID()` (line 136): Add comment: "Used by confluence_transformer to extract page IDs from URLs"
- `parseSpaceKey()` (line 149): Add comment: "Used by confluence_transformer to extract space keys from URLs"
- `normalizeStatus()` (line 158): Add comment: "Used by jira_transformer to normalize status values to canonical forms"

These documentation updates clarify that the helpers are shared utilities, not just internal to the crawler service.

### docs\refactor_crawler.md(MODIFY)

**Update refactoring documentation to reflect parser consolidation:**

Add a new section documenting the parser consolidation refactoring:

**Section: Parser Consolidation (October 2025)**

**Completed Changes:**
1. ✅ Deleted `jira_parser.go` - Parsing logic moved inline to `jira_transformer.go`
2. ✅ Deleted `confluence_parser.go` - Parsing logic moved inline to `confluence_transformer.go`
3. ✅ Transformers now parse HTML directly using shared helpers from `crawler/helpers.go`
4. ✅ Eliminated parser layer while maintaining identical extraction quality

**Architecture After Consolidation:**
- **Crawler Service:** Uses `html_scraper.go` for generic HTML scraping, produces `ScrapeResult` with markdown, metadata, links, and RawHTML
- **Specialized Transformers:** Parse RawHTML directly using CSS selectors and shared helpers from `crawler/helpers.go`
- **Shared Helpers:** `crawler/helpers.go` provides reusable extraction utilities (createDocument, extractTextFromDoc, extractCleanedHTML, extractDateFromDoc, regex-based ID extraction, status normalization)

**Rationale:**
- html_scraper provides generic metadata extraction (title, description, Open Graph, JSON-LD) suitable for most use cases
- Jira/Confluence-specific fields (IssueKey, ProjectKey, Status, Priority, PageID, SpaceKey, etc.) require specialized CSS selector-based extraction
- Moving parsing logic inline to transformers eliminates unnecessary indirection and keeps specialized logic where it's used
- Shared helpers in `crawler/helpers.go` remain reusable across transformers

**Benefits:**
- Reduced codebase size (2 files deleted, ~460 lines removed)
- Eliminated parser layer (less indirection)
- Maintained extraction quality (same CSS selectors, same fallbacks)
- Kept html_scraper generic and reusable for future sources
- Transformers are self-contained (parsing + transformation in one place)

**Trade-offs:**
- Transformers are longer (~150-200 lines added to each)
- Parsing logic moved from crawler package to atlassian package
- Acceptable trade-off: Logic is source-specific, not generic crawling logic

**Future Considerations:**
- If GitHub transformer is added, it should follow the same pattern: parse HTML inline using shared helpers from `crawler/helpers.go`
- Consider extracting common parsing patterns into additional shared helpers if duplication emerges across transformers

### docs\architecture.md(MODIFY)

References: 

- internal\services\crawler\html_scraper.go
- internal\services\crawler\helpers.go(MODIFY)
- internal\services\atlassian\jira_transformer.go(MODIFY)
- internal\services\atlassian\confluence_transformer.go(MODIFY)

**Update architecture documentation to reflect parser consolidation:**

**In Section 3 (Content Flow Pipeline):**

Update the transformation flow description:
- **OLD:** "Crawler fetches HTML pages → HTML Parsers extract structured data → Transformers convert HTML to Markdown → Storage serializes to SQLite"
- **NEW:** "Crawler fetches HTML pages → html_scraper extracts generic metadata + RawHTML → Transformers parse RawHTML for source-specific metadata + convert to Markdown → Storage serializes to SQLite"

Update file references:
- **REMOVE:** References to `crawler/jira_parser.go` and `crawler/confluence_parser.go`
- **ADD:** Reference to `crawler/helpers.go` as shared parsing utilities
- **UPDATE:** Transformer descriptions to note they parse HTML directly using shared helpers

**In Section 7 (HTML Parsing Details):**

Replace the parser implementation section with:

**HTML Parsing Architecture:**
- **Generic Parsing:** `html_scraper.go` (lines 536-636) extracts standard metadata (title, description, Open Graph, Twitter Card, JSON-LD, canonical URL) using goquery
- **Specialized Parsing:** Transformers parse RawHTML directly for source-specific fields
  - `jira_transformer.go` parseJiraIssue method: Extracts IssueKey, ProjectKey, Status, Priority, Assignee, Reporter, Labels, Components using CSS selectors with multiple fallbacks
  - `confluence_transformer.go` parseConfluencePage method: Extracts PageID, SpaceKey, SpaceName, Author, Version, ContentType using CSS selectors with multiple fallbacks
- **Shared Helpers:** `crawler/helpers.go` provides reusable extraction utilities:
  - `createDocument()` - Creates goquery.Document from HTML string
  - `extractTextFromDoc()` - Tries multiple selectors in priority order
  - `extractMultipleTextsFromDoc()` - Collects text from all matching elements
  - `extractCleanedHTML()` - Extracts and cleans HTML from selectors
  - `extractDateFromDoc()` - Extracts dates with RFC3339 normalization
  - `parseJiraIssueKey()`, `parseConfluencePageID()`, `parseSpaceKey()` - Regex-based ID extraction
  - `normalizeStatus()` - Status normalization

**Design Philosophy:**
- html_scraper remains generic and reusable for any HTML source
- Source-specific extraction is handled by transformers using shared helpers
- This keeps the crawler layer clean while allowing specialized metadata extraction where needed

**Add new subsection: "Why Inline Parsing in Transformers?"**

Explain the architectural decision:
- Jira/Confluence-specific fields are not available in standard HTML meta tags
- Extraction requires CSS selectors targeting specific page structure elements
- Multiple fallback selectors provide resilience against UI changes
- Inline parsing in transformers eliminates unnecessary abstraction layer
- Shared helpers in `crawler/helpers.go` prevent code duplication
- Future sources (GitHub, etc.) can follow the same pattern