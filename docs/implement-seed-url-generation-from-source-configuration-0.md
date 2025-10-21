I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Critical Bug Analysis

**The Problem:**
At line 153 in `job_helper.go`, the code passes `[]string{}` (empty array) to `crawlerService.StartCrawl()` with a comment claiming "crawler derives URLs from source config". This logic was never implemented, causing:

1. **Empty queue initialization** - No URLs are enqueued (service.go lines 444-460)
2. **Zero progress tracking** - `TotalURLs` and `PendingURLs` set to 0 (service.go line 211, 464-465)
3. **Immediate job completion** - Workers find nothing to process and exit
4. **0 results** - No pages crawled, no documents saved

**Impact:** ALL crawl jobs fail immediately regardless of configuration.

**The Solution:**
Generate seed URLs from source configuration based on source type and base URL. This aligns with the architectural principle that sources define "WHAT to connect to" while jobs define "HOW to filter/process".

## Seed URL Generation Strategy

**For Jira sources:**
- `{base_url}/browse` - Main issue browser page
- `{base_url}/projects` - Project listing page

**For Confluence sources:**
- `{base_url}/wiki/spaces` - Space listing page
- `{base_url}/wiki` - Wiki home page (fallback)

**For GitHub sources:**
- `{base_url}/repositories` - Repository listing page

**URL Construction:**
- Use `strings.TrimRight(baseURL, "/")` to remove trailing slashes
- Use `fmt.Sprintf("%s/%s", baseURL, path)` for path joining
- Validate that base URL is not empty before generation

## Implementation Considerations

**Why multiple seed URLs per source type?**
- Provides redundancy if one entry point fails
- Covers different navigation patterns (direct browse vs. project list)
- Increases crawl coverage by starting from multiple entry points

**Why not use the existing `joinPath()` helper?**
- It's unexported (lowercase) in `url_utils.go`
- Simple URL construction with `fmt.Sprintf()` is sufficient
- Avoids unnecessary dependency on internal helper

**Validation Strategy:**
- Check if generated seed URLs array is empty before calling `StartCrawl()`
- Return descriptive error if no URLs could be generated
- Log the generated URLs for debugging and transparency

**Logging Strategy:**
- Log seed URL generation at DEBUG level (shows URLs being generated)
- Log seed URL count at INFO level (confirms generation success)
- Log individual URLs if count is small (≤ 5), otherwise show first 5
- Include source type and base URL in log context

### Approach

Fix the critical bug where crawl jobs complete immediately with 0 results by implementing seed URL generation from source configuration. Add a `generateSeedURLs()` function that creates appropriate starting URLs based on source type (Jira, Confluence, GitHub) and base URL. Update `StartCrawlJob()` to use generated seed URLs instead of passing an empty array to the crawler service. Include validation and comprehensive logging.

### Reasoning

Read `job_helper.go` and identified the bug at line 153 where an empty array is passed to `StartCrawl()`. Examined `service.go` to understand how the crawler expects seed URLs - it uses them to initialize the queue (lines 444-460), track progress (line 211), and validate URLs (lines 220-237). Reviewed `source.go` to confirm available source types (Jira, Confluence, GitHub) and the SourceConfig structure. Checked `url_utils.go` for URL handling utilities and found the `joinPath()` helper function. Examined `crawler_actions.go` to understand the current flow where no seed URLs are being provided.

## Mermaid Diagram

sequenceDiagram
    participant JH as job_helper.go
    participant GSU as generateSeedURLs()
    participant CS as Crawler Service
    participant Q as URL Queue
    participant W as Workers

    Note over JH: StartCrawlJob() called
    
    JH->>GSU: generateSeedURLs(source, logger)
    
    alt Source Type: Jira
        GSU->>GSU: Generate /browse and /projects URLs
    else Source Type: Confluence
        GSU->>GSU: Generate /wiki/spaces and /wiki URLs
    else Source Type: GitHub
        GSU->>GSU: Generate /repositories URL
    end
    
    GSU->>GSU: Validate URLs not empty
    GSU->>GSU: Log generated URLs (DEBUG)
    GSU-->>JH: Return seedURLs array
    
    JH->>JH: Validate len(seedURLs) > 0
    JH->>JH: Log seed URL count (INFO)
    
    JH->>CS: StartCrawl(..., seedURLs, ...)
    
    CS->>CS: Initialize job with TotalURLs = len(seedURLs)
    
    loop For each seed URL
        CS->>Q: Push(URLQueueItem)
        Q-->>CS: URL enqueued
    end
    
    CS->>CS: Update PendingURLs = actuallyEnqueued
    CS->>W: Start workers
    
    W->>Q: Pop() - Get next URL
    Q-->>W: Return URL to crawl
    W->>W: Fetch page, discover links
    W->>Q: Enqueue discovered links
    
    Note over W,Q: Crawl continues until<br/>queue empty or max depth reached

## Proposed File Changes

### internal\services\jobs\job_helper.go(MODIFY)

References: 

- internal\models\source.go
- internal\services\crawler\service.go

Add a new function `generateSeedURLs()` after the `StartCrawlJob()` function (around line 172). The function signature should be: `func generateSeedURLs(source *models.SourceConfig, logger arbor.ILogger) ([]string, error)`. Implementation logic:

1. **Validate input**: Check if `source.BaseURL` is empty, return error if so.

2. **Clean base URL**: Use `strings.TrimRight(source.BaseURL, "/")` to remove trailing slashes and store in a variable.

3. **Generate URLs based on source type** using a switch statement on `source.Type`:
   - **Case `models.SourceTypeJira`**: Return array with two URLs:
     - `fmt.Sprintf("%s/browse", baseURL)`
     - `fmt.Sprintf("%s/projects", baseURL)`
   - **Case `models.SourceTypeConfluence`**: Return array with two URLs:
     - `fmt.Sprintf("%s/wiki/spaces", baseURL)`
     - `fmt.Sprintf("%s/wiki", baseURL)`
   - **Case `models.SourceTypeGithub`**: Return array with one URL:
     - `fmt.Sprintf("%s/repositories", baseURL)`
   - **Default case**: Return error with message indicating unsupported source type.

4. **Log generated URLs**: Use `logger.Debug()` to log the generated seed URLs with fields: `source_id`, `source_type`, `base_url`, `seed_url_count`, and `seed_urls` (array).

5. **Return the generated URLs array and nil error**.

Update `StartCrawlJob()` function at line 147-159:

1. **Replace the comment and empty array** (lines 147-153) with a call to `generateSeedURLs()`:
   - Call: `seedURLs, err := generateSeedURLs(source, logger)`
   - Handle error: If error is not nil, return empty string and wrapped error with message "failed to generate seed URLs: %w"

2. **Add validation** after seed URL generation:
   - Check if `len(seedURLs) == 0`
   - If empty, return error: "no seed URLs generated for source %s (type: %s)"

3. **Add logging** after validation:
   - Use `logger.Info()` with fields: `source_id`, `source_type`, `seed_url_count`
   - Message: "Generated seed URLs for crawl job"
   - If `len(seedURLs) <= 5`, add `Strs("seed_urls", seedURLs)` to show all URLs
   - If `len(seedURLs) > 5`, add `Strs("seed_urls_sample", seedURLs[:5])` and `Int("remaining_count", len(seedURLs)-5)`

4. **Pass generated seed URLs** to `crawlerService.StartCrawl()` (line 150-159):
   - Replace `[]string{}` with `seedURLs` variable

5. **Update the comment** above the `StartCrawl()` call to reflect the new behavior: "Start crawl with generated seed URLs based on source type and base URL"