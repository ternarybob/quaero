# Crawler Refactoring Documentation

This document tracks major refactoring efforts in the crawler subsystem, documenting architectural changes, rationale, and lessons learned.

## Parser Consolidation (October 2025)

### Completed Changes

1. ✅ Deleted `jira_parser.go` - Parsing logic moved inline to `jira_transformer.go`
2. ✅ Deleted `confluence_parser.go` - Parsing logic moved inline to `confluence_transformer.go`
3. ✅ Transformers now parse HTML directly using shared helpers from `crawler/helpers.go`
4. ✅ Eliminated parser layer while maintaining identical extraction quality

### Architecture After Consolidation

**Crawler Service:**
- Uses `html_scraper.go` for generic HTML scraping
- Produces `ScrapeResult` with markdown, metadata, links, and RawHTML
- Remains generic and reusable for all content sources

**Specialized Transformers:**
- Parse RawHTML directly using CSS selectors and shared helpers from `crawler/helpers.go`
- Extract source-specific metadata that cannot be inferred from generic HTML meta tags
- Self-contained: parsing + transformation logic in one place

**Shared Helpers:**
- `crawler/helpers.go` provides reusable extraction utilities:
  - `CreateDocument()` - Creates goquery.Document from HTML string
  - `ExtractTextFromDoc()` - Tries multiple selectors in priority order
  - `ExtractMultipleTextsFromDoc()` - Collects text from all matching elements
  - `ExtractCleanedHTML()` - Extracts and cleans HTML content
  - `ExtractDateFromDoc()` - Extracts and normalizes dates to RFC3339
  - `ParseJiraIssueKey()` - Regex-based Jira issue key extraction
  - `ParseConfluencePageID()` - Regex-based Confluence page ID extraction
  - `ParseSpaceKey()` - Regex-based Confluence space key extraction
  - `NormalizeStatus()` - Status value normalization

### Rationale

**Why separate parsers were removed:**
- `html_scraper` provides generic metadata extraction (title, description, Open Graph, JSON-LD) suitable for most use cases
- Jira/Confluence-specific fields (IssueKey, ProjectKey, Status, Priority, PageID, SpaceKey, etc.) require specialized CSS selector-based extraction
- Separate parser files created unnecessary indirection - parsing logic was only called from one place (the transformer)
- Moving parsing logic inline to transformers eliminates the extra layer while keeping specialized logic where it's actually used

**Why shared helpers are retained:**
- Common extraction patterns (multiple CSS selector fallbacks, date normalization, regex-based ID parsing) are reusable across transformers
- Helpers remain generic enough to be useful for future data sources (GitHub, etc.)
- Helper functions are well-tested and proven reliable

### Benefits

- **Reduced codebase size:** 2 files deleted (~460 lines removed)
- **Eliminated indirection:** Transformers parse directly instead of calling separate parsers
- **Maintained extraction quality:** Same CSS selectors, same fallbacks, same logic
- **Kept html_scraper generic:** Remains reusable for future sources without source-specific customization
- **Self-contained transformers:** Parsing + transformation in one place, easier to understand and maintain

### Trade-offs

**Transformers are longer:**
- jira_transformer.go gained ~150-200 lines
- confluence_transformer.go gained ~150-200 lines
- **Acceptable:** Each transformer is still under 500 lines, well within project standards

**Parsing logic moved packages:**
- Moved from `internal/services/crawler` to `internal/services/atlassian`
- **Acceptable:** Logic is Jira/Confluence-specific, not generic crawling logic
- Keeps crawler package focused on generic HTML scraping

### Future Considerations

**For new data sources (e.g., GitHub):**
- Follow the same pattern: parse HTML inline in transformer using shared helpers from `crawler/helpers.go`
- Only create new helpers if common patterns emerge that would be reusable across multiple transformers
- Keep `html_scraper` generic - don't add source-specific logic there

**If duplication emerges:**
- Extract common parsing patterns into additional shared helpers in `crawler/helpers.go`
- Consider helper functions for common metadata patterns (author extraction, tag extraction, etc.)
- Maintain the principle: helpers should be reusable, not source-specific

### Lessons Learned

1. **Abstraction isn't always valuable:** The parser layer seemed like good abstraction initially, but it was only called from one place and added unnecessary indirection.

2. **Inline when single-use:** If code is only called from one location, consider inlining it for clarity.

3. **Extract when reusable:** Helper functions are valuable when they abstract reusable patterns (CSS selector fallbacks, date normalization, etc.).

4. **Generic vs. Specialized:** Keep generic components (html_scraper) truly generic. Put specialized logic where it's used (transformers).

5. **Consolidation reduces complexity:** Fewer files, less jumping between layers, easier debugging and maintenance.
