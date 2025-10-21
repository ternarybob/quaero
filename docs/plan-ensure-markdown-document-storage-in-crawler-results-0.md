I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Implementation Status

### ✅ ALREADY IMPLEMENTED - Complete Markdown Storage Pipeline

**1. HTML Scraping with Markdown Conversion** (`html_scraper.go`)
- Line 144: Creates markdown converter with base URL for link resolution
- Lines 364-385: `convertContentToMarkdown()` method converts HTML to markdown
- Line 384: Populates `result.Markdown` field
- Uses `github.com/JohannesKaufmann/html-to-markdown` library

**2. Markdown Storage in CrawlResult Metadata** (`types.go`)
- Lines 134-190: `ToCrawlResult()` method converts `ScrapeResult` to `CrawlResult`
- Line 155: **Stores markdown in `metadata["markdown"]`** ✅
- Line 156: Also stores HTML in `metadata["html"]`
- Line 157: Stores text content in `metadata["text_content"]`

**3. Metadata Propagation in Crawler Service** (`service.go`)
- Lines 1148-1158: Executes HTML scraping via `scraper.ScrapeURL()`
- Line 1160: Converts `ScrapeResult` to `CrawlResult` via `ToCrawlResult()`
- Lines 1219-1226: **Merges scrape metadata (including markdown) into item.Metadata** ✅
- Lines 986-998: Propagates metadata to final `CrawlResult.Metadata`

**4. Markdown Extraction in Transformers** (`helpers.go`)
- Lines 198-266: `convertHTMLToMarkdown()` helper function
- Handles HTML-to-markdown conversion with fallback to `stripHTMLTags()`
- Line 234: Applies fallback when conversion produces empty output (configurable)
- Used by both Jira and Confluence transformers

**5. Document Creation with Markdown** (`jira_transformer.go` & `confluence_transformer.go`)
- **Jira**: Line 345 calls `convertHTMLToMarkdown()`, Line 429 stores in `Document.ContentMarkdown`
- **Confluence**: Line 325 calls `convertHTMLToMarkdown()`, Line 398 stores in `Document.ContentMarkdown`
- Both transformers extract HTML from `CrawlResult`, convert to markdown, and store in document

**6. Database Persistence** (`document_storage.go`)
- Line 84: `SaveDocument()` persists `doc.ContentMarkdown` to database
- Line 544: `scanDocument()` retrieves `contentMarkdown` from database
- Line 560: Populates `doc.ContentMarkdown` field
- Lines 628-629: `scanDocuments()` handles batch retrieval

### 🔍 Root Cause of User's Issue

The user reports: "job completes without scanning links, without saving documents"

**This is NOT a markdown storage problem.** The issue is caused by:
1. **Empty seed URLs** - The previous phase should have implemented `generateSeedURLs()` to create initial crawl URLs from `source.BaseURL`
2. **No URLs in queue** - Without seed URLs, the crawler has nothing to process
3. **Immediate completion** - Job completes with 0 results because queue is empty

**Evidence from code:**
- `job_helper.go` line 153 (from previous analysis): Passes empty array `[]string{}` to `StartCrawl()`
- Comment says "crawler derives URLs from source config" but this was never implemented
- The markdown storage pipeline only runs AFTER pages are crawled

### 📊 Data Flow Verification

```
1. HTMLScraper.ScrapeURL() 
   → Converts HTML to markdown (line 384)
   → Returns ScrapeResult with Markdown field populated

2. ScrapeResult.ToCrawlResult()
   → Stores markdown in metadata["markdown"] (line 155)
   → Returns CrawlResult

3. Service.makeRequest()
   → Merges metadata into item.Metadata (lines 1219-1226)
   → Markdown is now in CrawlResult.Metadata["markdown"]

4. Transformers (Jira/Confluence)
   → Extract HTML from CrawlResult (via selectResultBody helper)
   → Convert HTML to markdown (via convertHTMLToMarkdown helper)
   → Store in Document.ContentMarkdown (lines 429/398)

5. DocumentStorage.SaveDocument()
   → Persists ContentMarkdown to database (line 84)
   → Retrieves ContentMarkdown from database (line 560)
```

### ⚠️ Why This Phase is Unnecessary

The user's task asks to:
1. ✅ "Ensure `ScrapeResult.Markdown` is populated" - **Already done** (line 384)
2. ✅ "Store markdown in `CrawlResult.Metadata["markdown"]`" - **Already done** (line 155)
3. ✅ "Extract and store markdown in documents table" - **Already done** (lines 429/398)
4. ✅ "Verify markdown is persisted" - **Already done** (line 84)

**All requirements are already met.** The implementation is complete and correct.

### Approach

**VERIFICATION ONLY - No Implementation Needed**

After comprehensive code analysis, the markdown storage pipeline is **already fully implemented and working correctly**. The entire flow from HTML scraping → markdown conversion → metadata storage → document transformation → database persistence is complete and functional.

The user's reported issue ("job completes without scanning links, without saving documents") is **NOT related to markdown storage** - it's caused by the empty seed URLs bug that should have been fixed in the previous phase "CRITICAL: Implement seed URL generation from source configuration".

This phase focuses on **verification and documentation** to confirm the existing implementation is correct, rather than making changes.

### Reasoning

Performed comprehensive code analysis by reading all five files specified by the user: `html_scraper.go`, `service.go`, `jira_transformer.go`, `confluence_transformer.go`, and `document_storage.go`. Also examined supporting files: `types.go` and `helpers.go`. Traced the complete data flow from HTML scraping through markdown conversion to database persistence. Confirmed that every step of the pipeline is already implemented and functional.

## Mermaid Diagram

sequenceDiagram
    participant Crawler as Crawler Service
    participant Scraper as HTMLScraper
    participant Converter as html-to-markdown
    participant Types as ToCrawlResult()
    participant Transformer as Jira/Confluence Transformer
    participant Helper as convertHTMLToMarkdown()
    participant Storage as DocumentStorage
    participant DB as SQLite Database

    Note over Crawler,DB: ✅ ALREADY IMPLEMENTED - Markdown Storage Pipeline

    Crawler->>Scraper: ScrapeURL(url)
    Scraper->>Scraper: Fetch HTML content
    Scraper->>Converter: ConvertString(html)
    Converter-->>Scraper: markdown string
    Scraper->>Scraper: result.Markdown = markdown
    Note over Scraper: Line 384: Markdown populated
    Scraper-->>Crawler: ScrapeResult{Markdown: "..."}

    Crawler->>Types: scrapeResult.ToCrawlResult()
    Types->>Types: metadata["markdown"] = s.Markdown
    Note over Types: Line 155: Markdown stored in metadata
    Types-->>Crawler: CrawlResult{Metadata: {"markdown": "..."}}

    Crawler->>Crawler: Merge metadata into item.Metadata
    Note over Crawler: Lines 1219-1226: Metadata propagation
    Crawler->>Crawler: result.Metadata = item.Metadata
    Note over Crawler: Lines 986-998: Final result construction

    Note over Crawler,DB: Job completes, results stored in memory

    Note over Transformer: Collection event triggered (every 5 min)

    Transformer->>Crawler: GetJobResults(jobID)
    Crawler-->>Transformer: []*CrawlResult with metadata

    Transformer->>Transformer: selectResultBody(result)
    Note over Transformer: Extract HTML from metadata["html"]
    
    Transformer->>Helper: convertHTMLToMarkdown(html, baseURL)
    Helper->>Converter: ConvertString(html)
    Converter-->>Helper: markdown string
    
    alt Conversion produces empty output
        Helper->>Helper: stripHTMLTags(html)
        Note over Helper: Fallback if enableEmptyOutputFallback=true
    end
    
    Helper-->>Transformer: contentMarkdown
    
    Transformer->>Transformer: Create Document{ContentMarkdown: contentMarkdown}
    Note over Transformer: Lines 429/398: Markdown stored in Document
    
    Transformer->>Storage: SaveDocument(doc)
    Storage->>DB: INSERT content_markdown = ?
    Note over Storage: Line 84: Persist markdown
    DB-->>Storage: Success
    Storage-->>Transformer: Success

    Note over Transformer,DB: ✅ Markdown stored in database

    Note over Storage,DB: Later: Retrieve documents

    Storage->>DB: SELECT content_markdown FROM documents
    DB-->>Storage: content_markdown value
    Storage->>Storage: doc.ContentMarkdown = contentMarkdown.String
    Note over Storage: Line 560: Populate markdown field
    Storage-->>Transformer: Document{ContentMarkdown: "..."}

## Proposed File Changes

### internal\services\crawler\html_scraper.go(MODIFY)

**VERIFICATION ONLY - No changes needed.**

Confirm that the markdown conversion pipeline is working correctly:

1. **Line 384**: `result.Markdown = markdown` - Verify this line populates the Markdown field after successful conversion
2. **Lines 364-385**: `convertContentToMarkdown()` method - Verify it:
   - Checks `config.OutputFormat` for markdown/both modes
   - Extracts main content via `extractMainContent()`
   - Converts HTML to markdown using `mdConverter.ConvertString()`
   - Handles errors gracefully with logging
3. **Line 144**: Markdown converter initialization - Verify base URL is passed for relative link resolution

**Testing:**
- Run a crawl job with a valid source and seed URLs (after previous phase fixes empty seed URL bug)
- Check logs for "Converting HTML to markdown" messages
- Verify `ScrapeResult.Markdown` is populated in debug logs
- Confirm no conversion errors in logs

**Expected Behavior:**
- HTML content is converted to markdown format
- Markdown is stored in `ScrapeResult.Markdown` field
- Conversion uses base URL for resolving relative links
- Errors are logged but don't crash the scraper

### internal\services\crawler\types.go(MODIFY)

References: 

- internal\services\crawler\html_scraper.go(MODIFY)

**VERIFICATION ONLY - No changes needed.**

Confirm that `ToCrawlResult()` correctly stores markdown in metadata:

1. **Line 155**: `metadata["markdown"] = s.Markdown` - Verify markdown is stored in metadata map
2. **Line 156**: `metadata["html"] = s.HTML` - Verify HTML is also stored for transformers
3. **Line 157**: `metadata["text_content"] = s.TextContent` - Verify plain text is stored
4. **Lines 139-142**: Body content selection - Verify HTML/RawHTML is used for Body (not markdown)
5. **Lines 145-158**: Metadata merging - Verify all ScrapeResult fields are copied to metadata

**Testing:**
- After a successful scrape, inspect `CrawlResult.Metadata` in debugger or logs
- Verify `metadata["markdown"]` contains converted markdown content
- Verify `metadata["html"]` contains cleaned HTML
- Verify `CrawlResult.Body` contains HTML (not markdown) for transformer compatibility

**Expected Behavior:**
- Markdown is accessible via `CrawlResult.Metadata["markdown"]`
- HTML is accessible via `CrawlResult.Metadata["html"]` and `CrawlResult.Body`
- Transformers can choose between HTML parsing or markdown extraction
- All metadata fields are preserved during conversion

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\services\crawler\types.go(MODIFY)
- internal\services\crawler\html_scraper.go(MODIFY)

**VERIFICATION ONLY - No changes needed.**

Confirm that metadata (including markdown) is propagated correctly:

1. **Line 1160**: `crawlResult := scrapeResult.ToCrawlResult()` - Verify conversion happens
2. **Lines 1219-1226**: Metadata merging loop - Verify markdown is copied to `item.Metadata`
3. **Lines 1221-1225**: Selective merging - Verify critical fields (job_id, source_type, entity_type) are preserved
4. **Lines 986-998**: Result construction - Verify `item.Metadata` is assigned to `result.Metadata`
5. **Lines 1192-1210**: HTML storage in response_body - Verify HTML (not markdown) is stored for transformers

**Testing:**
- Add debug logging after line 1226 to inspect `item.Metadata` keys
- Verify `item.Metadata["markdown"]` exists and contains content
- Verify `item.Metadata["html"]` exists and contains HTML
- Check that `result.Metadata` (returned from `executeRequest()`) includes markdown

**Expected Behavior:**
- Markdown flows from `ScrapeResult` → `CrawlResult` → `item.Metadata` → final result
- Metadata is preserved across the entire request pipeline
- HTML is stored in `response_body` for transformer compatibility
- Job-specific metadata (job_id, source_type) is not overwritten

### internal\services\atlassian\jira_transformer.go(MODIFY)

References: 

- internal\services\atlassian\helpers.go(MODIFY)
- internal\services\crawler\helpers.go

**VERIFICATION ONLY - No changes needed.**

Confirm that Jira transformer correctly extracts and stores markdown:

1. **Line 130**: `body := selectResultBody(result)` - Verify HTML is extracted from CrawlResult
2. **Line 250**: `descriptionHTML := crawler.ExtractCleanedHTML(...)` - Verify description HTML is extracted
3. **Line 342**: `docURL := resolveDocumentURL(...)` - Verify URL is resolved for link resolution
4. **Line 345**: `contentMarkdown := convertHTMLToMarkdown(descriptionHTML, docURL, ...)` - Verify conversion happens
5. **Line 429**: `ContentMarkdown: contentMarkdown` - Verify markdown is stored in Document

**Testing:**
- Run a Jira crawl job and trigger transformation via collection event
- Check logs for "Parsed Jira issue from HTML" messages
- Verify "Jira issue markdown conversion completed" logs show non-zero markdown_length
- Query documents table and verify `content_markdown` column is populated
- Check for "Markdown conversion produced empty output" warnings (indicates conversion issues)

**Expected Behavior:**
- HTML description is extracted from Jira issue pages
- HTML is converted to markdown using base URL for link resolution
- Markdown is stored in `Document.ContentMarkdown` field
- Empty output warnings trigger fallback to HTML stripping (if enabled)
- Documents are saved to database with markdown content

### internal\services\atlassian\confluence_transformer.go(MODIFY)

References: 

- internal\services\atlassian\helpers.go(MODIFY)
- internal\services\crawler\helpers.go

**VERIFICATION ONLY - No changes needed.**

Confirm that Confluence transformer correctly extracts and stores markdown:

1. **Line 132**: `body := selectResultBody(result)` - Verify HTML is extracted from CrawlResult
2. **Line 255**: `contentHTML := crawler.ExtractCleanedHTML(...)` - Verify page content HTML is extracted
3. **Line 322**: `docURL := resolveDocumentURL(...)` - Verify URL is resolved for link resolution
4. **Line 325**: `contentMarkdown := convertHTMLToMarkdown(contentHTML, docURL, ...)` - Verify conversion happens
5. **Line 398**: `ContentMarkdown: contentMarkdown` - Verify markdown is stored in Document

**Testing:**
- Run a Confluence crawl job and trigger transformation via collection event
- Check logs for "Parsed Confluence page from HTML" messages
- Verify "Confluence page markdown conversion completed" logs show non-zero markdown_length
- Query documents table and verify `content_markdown` column is populated
- Check for "Markdown conversion produced empty output" warnings (indicates conversion issues)

**Expected Behavior:**
- HTML content is extracted from Confluence pages
- HTML is converted to markdown using base URL for link resolution
- Markdown is stored in `Document.ContentMarkdown` field
- Empty output warnings trigger fallback to HTML stripping (if enabled)
- Documents are saved to database with markdown content

### internal\storage\sqlite\document_storage.go(MODIFY)

References: 

- internal\models\document.go

**VERIFICATION ONLY - No changes needed.**

Confirm that markdown is correctly persisted to and retrieved from the database:

1. **Line 84**: `doc.ContentMarkdown` - Verify markdown is included in INSERT statement
2. **Lines 56-61**: Smart upsert logic - Verify markdown is preserved/replaced based on detail_level
3. **Line 544**: `&contentMarkdown` - Verify markdown is scanned from database
4. **Line 560**: `doc.ContentMarkdown = contentMarkdown.String` - Verify markdown is populated in Document
5. **Lines 628-629**: Batch scanning - Verify markdown is retrieved for multiple documents

**Testing:**
- After transformation completes, query the database directly:
  ```sql
  SELECT id, source_type, source_id, title, 
         LENGTH(content_markdown) as markdown_length,
         SUBSTR(content_markdown, 1, 100) as markdown_preview
  FROM documents
  WHERE source_type IN ('jira', 'confluence')
  LIMIT 10;
  ```
- Verify `markdown_length` is > 0 for documents
- Verify `markdown_preview` shows markdown-formatted content (with `#`, `*`, `[links]()`, etc.)
- Use API endpoint `GET /api/documents` to retrieve documents and verify `content_markdown` field
- Check that markdown is preserved across updates (upsert logic)

**Expected Behavior:**
- Markdown is stored in `content_markdown` column as TEXT
- Markdown is retrieved correctly when loading documents
- Smart upsert preserves full content when upserting metadata-only documents
- Markdown is accessible via API and storage interface methods

### internal\services\atlassian\helpers.go(MODIFY)

References: 

- internal\services\crawler\types.go(MODIFY)

**VERIFICATION ONLY - No changes needed.**

Confirm that helper functions correctly support markdown conversion:

1. **Lines 198-266**: `convertHTMLToMarkdown()` function - Verify it:
   - Creates markdown converter with base URL (line 209)
   - Converts HTML to markdown (line 210)
   - Handles conversion errors with fallback to `stripHTMLTags()` (lines 211-220)
   - Detects empty output and applies fallback if enabled (lines 232-252)
   - Logs quality metrics for troubleshooting (lines 222-263)

2. **Lines 179-193**: `stripHTMLTags()` fallback function - Verify it:
   - Removes HTML tags using regex
   - Cleans up whitespace
   - Decodes HTML entities
   - Returns plain text as fallback

3. **Lines 107-140**: `selectResultBody()` function - Verify it:
   - Prioritizes `metadata["html"]` for HTML parsing
   - Falls back to `metadata["response_body"]` for backward compatibility
   - Uses `result.Body` if it looks like HTML
   - Does NOT fall back to markdown (preserves HTML for parsers)

**Testing:**
- Test with various HTML inputs:
  - Simple HTML: `<p>Hello world</p>`
  - Complex HTML with nested tags: `<div><h1>Title</h1><p>Content</p></div>`
  - HTML with relative links: `<a href="/page">Link</a>`
  - Malformed HTML: `<p>Unclosed tag`
  - Empty HTML: `<div></div>`
- Verify markdown output is correct for each case
- Verify fallback is triggered for empty output (if enabled)
- Check logs for conversion warnings and fallback messages

**Expected Behavior:**
- HTML is converted to clean markdown format
- Relative links are resolved using base URL
- Conversion errors trigger fallback to HTML stripping
- Empty output triggers fallback if `enableEmptyOutputFallback` is true
- Quality metrics are logged for troubleshooting

### test\api\markdown_storage_test.go(NEW)

References: 

- internal\services\crawler\html_scraper.go(MODIFY)
- internal\services\atlassian\jira_transformer.go(MODIFY)
- internal\storage\sqlite\document_storage.go(MODIFY)

**RECOMMENDED: Add integration test to verify end-to-end markdown storage.**

Create a test that verifies the complete markdown pipeline:

1. **Setup**: Create a test source with base URL and filters
2. **Crawl**: Start a crawl job with seed URLs (after previous phase fixes seed URL generation)
3. **Wait**: Wait for job completion
4. **Transform**: Trigger collection event to transform results
5. **Verify**: Query documents table and verify:
   - Documents exist with `content_markdown` populated
   - Markdown content is non-empty
   - Markdown format is correct (contains markdown syntax)
   - HTML has been converted (no `<tags>` in markdown)

**Test Structure:**
```go
func TestMarkdownStoragePipeline(t *testing.T) {
    // 1. Setup test source and job
    // 2. Start crawl with seed URLs
    // 3. Wait for completion
    // 4. Trigger transformation
    // 5. Query documents
    // 6. Verify markdown is stored
    // 7. Verify markdown format is correct
}
```

**Assertions:**
- `len(documents) > 0` - Documents were created
- `doc.ContentMarkdown != ""` - Markdown is populated
- `strings.Contains(doc.ContentMarkdown, "#")` - Contains markdown headers
- `!strings.Contains(doc.ContentMarkdown, "<div>")` - HTML tags removed
- `strings.Contains(doc.ContentMarkdown, "[link]")` - Contains markdown links

Reference existing test patterns in `test/api/crawl_transform_test.go` for structure.

### docs\architecture.md(MODIFY)

References: 

- internal\services\crawler\html_scraper.go(MODIFY)
- internal\services\crawler\types.go(MODIFY)
- internal\services\crawler\service.go(MODIFY)
- internal\services\atlassian\helpers.go(MODIFY)

**RECOMMENDED: Document the markdown storage architecture.**

Add a section explaining the complete markdown pipeline:

**Markdown Storage Pipeline**

Quaero converts HTML content to markdown format for LLM consumption and search indexing. The pipeline consists of five stages:

**1. HTML Scraping** (`html_scraper.go`)
- Fetches HTML content from URLs
- Converts HTML to markdown using `github.com/JohannesKaufmann/html-to-markdown`
- Stores markdown in `ScrapeResult.Markdown` field
- Uses base URL for resolving relative links

**2. Metadata Storage** (`types.go`)
- `ToCrawlResult()` converts `ScrapeResult` to `CrawlResult`
- Stores markdown in `CrawlResult.Metadata["markdown"]`
- Also stores HTML in `metadata["html"]` for transformers
- Body contains HTML (not markdown) for backward compatibility

**3. Metadata Propagation** (`service.go`)
- Crawler service merges scrape metadata into result metadata
- Markdown flows through: `ScrapeResult` → `CrawlResult` → `item.Metadata` → final result
- Metadata is preserved across the entire request pipeline

**4. Document Transformation** (`jira_transformer.go`, `confluence_transformer.go`)
- Transformers extract HTML from `CrawlResult` via `selectResultBody()`
- Convert HTML to markdown via `convertHTMLToMarkdown()` helper
- Store markdown in `Document.ContentMarkdown` field
- Fallback to HTML stripping if conversion produces empty output

**5. Database Persistence** (`document_storage.go`)
- `SaveDocument()` persists `ContentMarkdown` to `content_markdown` column
- Smart upsert preserves full content when upserting metadata-only documents
- Markdown is retrieved via `GetDocument()` and related methods

**Configuration:**
- `config.OutputFormat` controls markdown generation (markdown/html/both)
- `enableEmptyOutputFallback` controls fallback to HTML stripping
- Base URL is used for resolving relative links in markdown

**Data Flow:**
```
HTML Page → HTMLScraper → ScrapeResult.Markdown
          ↓
          ToCrawlResult() → CrawlResult.Metadata["markdown"]
          ↓
          Metadata Merge → item.Metadata["markdown"]
          ↓
          Transformer → convertHTMLToMarkdown()
          ↓
          Document.ContentMarkdown → Database
```

**Troubleshooting:**
- Check logs for "Converting HTML to markdown" messages
- Look for "Markdown conversion produced empty output" warnings
- Verify `content_markdown` column is populated in database
- Ensure base URL is configured correctly for link resolution