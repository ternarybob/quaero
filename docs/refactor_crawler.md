I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The current crawler service uses standard \`net/http\` with custom HTTP clients for API-based scraping. The task requires migrating to **actual HTML web scraping** using Colly v2, following Firecrawl's architecture and processes.

**Key findings:**
- Current \`makeRequest()\` expects JSON API responses with \`Accept: application/json\` header
- Auth is handled via HTTP clients with cookies from \`authService.GetHTTPClient()\`
- Per-job HTTP clients are maintained in \`jobClients\` map with auth snapshots
- Rate limiting is already implemented in \`rate_limiter.go\` (per-domain token bucket)
- The service needs to transition from REST API calls to HTML page scraping

**Firecrawl architecture insights:**
- **Scrape mode**: Single page extraction with markdown conversion (core feature)
- **Crawl mode**: Multi-page recursive crawling with link discovery (already exists in Quaero)
- **Job-based architecture**: Async jobs with status tracking (already exists in Quaero)
- **Markdown output**: Convert HTML to clean markdown format for LLM consumption
- **Metadata extraction**: Extract title, description, Open Graph tags, structured data
- **Content cleaning**: Remove scripts, styles, navigation, ads - extract main content only
- **Multiple output formats**: Markdown, HTML, raw HTML, links, screenshots (future)

**Critical considerations:**
- Colly has its own rate limiting - should use Colly's built-in for HTML scraping
- Colly manages its own HTTP client - need to inject authenticated cookies
- Markdown conversion is essential for Firecrawl-style output (use \`github.com/JohannesKaufmann/html-to-markdown/v2\`)
- Metadata extraction should include Open Graph, Twitter Cards, JSON-LD structured data
- Content should be cleaned and normalized before markdown conversion


### Approach

Create a Firecrawl-inspired HTML scraper using Colly v2 with markdown conversion capabilities. Add crawler-specific configuration to support user agent rotation, rate limiting, and concurrency control. Implement three core methods following Firecrawl's patterns: \`ScrapeURL()\` for fetching and converting HTML to markdown, \`ExtractLinks()\` for discovering links from HTML, and \`ExtractContent()\` for extracting clean text and markdown content. Integrate the \`html-to-markdown\` library for LLM-ready markdown output. The scraper will accept authenticated HTTP clients and cookies to maintain compatibility with the existing auth system.


### Reasoning

Explored the repository structure to understand the crawler service architecture. Read \`go.mod\` to identify current dependencies, \`service.go\` to understand HTTP request execution and link discovery, \`types.go\` for data structures, and \`config.go\` for configuration patterns. Examined \`rate_limiter.go\` and \`queue.go\` to understand existing infrastructure. Searched for auth-related code to understand cookie and HTTP client management. Researched Colly v2 best practices and Firecrawl's architecture to understand the target implementation pattern. Identified \`html-to-markdown\` as the best Go library for markdown conversion.


## Mermaid Diagram

sequenceDiagram
    participant Config as Config System
    participant Service as Crawler Service
    participant Scraper as HTMLScraper (Colly)
    participant MDConverter as Markdown Converter
    participant Auth as Auth Service
    participant Target as Target Website

    Note over Config: Phase 1: Setup & Configuration
    Config->>Config: Add CrawlerConfig with Firecrawl settings
    Config->>Config: Update quaero.toml with [crawler] section
    
    Note over Service,Scraper: Phase 1: HTML Scraper Creation (Firecrawl-style)
    Service->>Auth: GetHTTPClient() & cookies
    Auth-->>Service: HTTP client with auth cookies
    Service->>Scraper: NewHTMLScraper(config, logger, client, cookies)
    Scraper->>Scraper: Initialize Colly collector
    Scraper->>Scraper: Configure rate limiting (Parallelism, Delay)
    Scraper->>Scraper: Apply RandomUserAgent extension
    Scraper->>MDConverter: Initialize html-to-markdown converter
    MDConverter-->>Scraper: Converter with GFM plugins
    Scraper->>Scraper: Inject auth cookies via OnRequest
    Scraper-->>Service: Initialized HTMLScraper

    Note over Service,Target: Phase 1: Firecrawl-style Scraping Flow
    Service->>Scraper: ScrapeURL(ctx, url)
    Scraper->>Scraper: Register OnHTML callbacks
    Scraper->>Target: Visit(url) with auth cookies
    Target-->>Scraper: HTML response
    
    Note over Scraper: Content Processing (Firecrawl pattern)
    Scraper->>Scraper: ExtractMetadata(og tags, twitter cards, JSON-LD)
    Scraper->>Scraper: Remove boilerplate (nav, footer, ads)
    Scraper->>Scraper: Identify main content (<main>, <article>)
    Scraper->>MDConverter: ConvertString(cleanedHTML)
    MDConverter-->>Scraper: Clean markdown (LLM-ready)
    Scraper->>Scraper: ExtractLinks(htmlElement, baseURL)
    Scraper->>Scraper: ExtractContent(plainText)
    
    Scraper->>Scraper: Build ScrapeResult{Markdown, Metadata, Links}
    Scraper-->>Service: ScrapeResult (Firecrawl-compatible)
    Service->>Service: Convert to CrawlResult (backward compat)
    Service->>Service: Store markdown in documents table
    Service->>Service: Enqueue discovered links for crawling

## Proposed File Changes

### go.mod(MODIFY)

Add dependencies for HTML scraping and markdown conversion to the \`require\` section:

- Add \`github.com/gocolly/colly/v2 v2.1.0\` (or latest stable version) for HTML scraping with concurrent crawling, rate limiting, and HTML parsing capabilities
- Add \`github.com/JohannesKaufmann/html-to-markdown/v2 v2.0.0\` (or latest) for converting HTML to clean markdown format optimized for LLM consumption (Firecrawl-style output)
- Run \`go mod tidy\` to resolve transitive dependencies

These libraries enable Firecrawl-style web scraping: Colly handles the HTTP/HTML layer with politeness controls, while html-to-markdown converts scraped content to LLM-ready markdown format.

### internal\\common\\config.go(MODIFY)

References: 

- deployments\\local\\quaero.toml(MODIFY)

Add a new \`CrawlerConfig\` section to the \`Config\` struct to support Colly-specific settings and Firecrawl-style scraping:

**In the \`Config\` struct (around line 24):**
- Add field \`Crawler CrawlerConfig\` with TOML tag \`toml:\"crawler\"\`

**Create new \`CrawlerConfig\` struct (after \`JobsConfig\` around line 150):**
- \`UserAgent\` (string) - Default user agent string (e.g., \"Quaero/1.0 (Web Crawler)\")
- \`UserAgentRotation\` (bool) - Enable random user agent rotation using Colly's extension
- \`MaxConcurrency\` (int) - Maximum concurrent requests per domain (default: 3)
- \`RequestDelay\` (time.Duration) - Minimum delay between requests to same domain (default: 1 second)
- \`RandomDelay\` (time.Duration) - Random delay jitter to add (default: 500ms)
- \`RequestTimeout\` (time.Duration) - HTTP request timeout (default: 30 seconds)
- \`MaxBodySize\` (int) - Maximum response body size in bytes (default: 10MB)
- \`MaxDepth\` (int) - Maximum crawl depth (default: 5)
- \`FollowRobotsTxt\` (bool) - Respect robots.txt rules (default: true)
- \`OutputFormat\` (string) - Default output format: \"markdown\", \"html\", or \"both\" (default: \"markdown\")
- \`OnlyMainContent\` (bool) - Extract only main content, removing nav/footer/ads (default: true)
- \`IncludeLinks\` (bool) - Include discovered links in scrape results (default: true)
- \`IncludeMetadata\` (bool) - Extract and include page metadata (title, description, og tags) (default: true)

**In \`NewDefaultConfig()\` function (around line 234):**
- Initialize \`Crawler\` field with Firecrawl-inspired defaults:
  - Conservative concurrency (3) and delays (1s + 500ms jitter) for politeness
  - Markdown output by default for LLM consumption
  - Main content extraction enabled to remove boilerplate
  - Metadata extraction enabled for context

**In \`applyEnvOverrides()\` function (around line 366):**
- Add environment variable overrides for crawler settings using \`QUAERO_CRAWLER_*\` prefix
- Support \`QUAERO_CRAWLER_USER_AGENT\`, \`QUAERO_CRAWLER_MAX_CONCURRENCY\`, \`QUAERO_CRAWLER_REQUEST_DELAY\`, \`QUAERO_CRAWLER_OUTPUT_FORMAT\`, etc.

This configuration enables Firecrawl-style scraping with markdown conversion, metadata extraction, and content cleaning while maintaining politeness through rate limiting.

### deployments\\local\\quaero.toml(MODIFY)

Add a new \`[crawler]\` section to expose user-facing crawler configuration with Firecrawl-inspired settings:

**Add after the \`[jobs.scan_and_summarize]\` section (around line 107):**

\`\`\`toml
# =============================================================================
# Crawler Configuration (Firecrawl-style HTML Scraping)
# =============================================================================
# Configure HTML scraping behavior for web page crawling.
# Follows Firecrawl architecture: scrape pages, convert to markdown, extract metadata.
# These settings control politeness, concurrency, and output format.

[crawler]
user_agent = \"Quaero/1.0 (Web Crawler)\"  # User agent string for requests
user_agent_rotation = true                # Rotate user agents for realism
max_concurrency = 3                       # Concurrent requests per domain
request_delay = \"1s\"                      # Minimum delay between requests
random_delay = \"500ms\"                    # Random jitter to add
request_timeout = \"30s\"                   # HTTP request timeout
max_body_size = 10485760                  # Max response size (10MB)
max_depth = 5                             # Maximum crawl depth
follow_robots_txt = true                  # Respect robots.txt

# Firecrawl-style output configuration
output_format = \"markdown\"                # Output format: \"markdown\", \"html\", or \"both\"
only_main_content = true                  # Extract only main content (remove nav/footer/ads)
include_links = true                      # Include discovered links in results
include_metadata = true                   # Extract page metadata (title, description, og tags)
\`\`\`

Include comments explaining:
- These settings affect web scraping politeness and should be adjusted based on target site requirements
- Lower delays and higher concurrency may result in rate limiting or blocking
- Markdown output is optimized for LLM consumption (Firecrawl-style)
- Main content extraction removes boilerplate HTML (navigation, footers, ads)
- Metadata extraction provides context for RAG systems

### internal\\services\\crawler\\html_scraper.go(NEW)

References: 

- internal\\services\\crawler\\service.go
- internal\\services\\crawler\\types.go(MODIFY)
- internal\\common\\config.go(MODIFY)

Create a Firecrawl-inspired HTML scraper service using Colly v2 with markdown conversion:

**Package and imports:**
- Package \`crawler\`
- Import Colly v2: \`github.com/gocolly/colly/v2\` and \`github.com/gocolly/colly/v2/extensions\`
- Import markdown converter: \`github.com/JohannesKaufmann/html-to-markdown/v2\`
- Import standard libraries: \`context\`, \`fmt\`, \`net/http\`, \`net/url\`, \`strings\`, \`time\`, \`regexp\`
- Import internal: \`github.com/ternarybob/arbor\` for logging, \`github.com/ternarybob/quaero/internal/common\` for config

**HTMLScraper struct:**
- \`collector\` (*colly.Collector) - Colly collector instance
- \`mdConverter\` (*md.Converter) - HTML to markdown converter
- \`config\` (common.CrawlerConfig) - Crawler configuration
- \`logger\` (arbor.ILogger) - Structured logger
- \`httpClient\` (*http.Client) - Optional authenticated HTTP client
- \`cookies\` ([]*http.Cookie) - Authentication cookies to inject

**Constructor \`NewHTMLScraper(config common.CrawlerConfig, logger arbor.ILogger, httpClient *http.Client, cookies []*http.Cookie) *HTMLScraper\`:**
- Create Colly collector with \`colly.NewCollector()\` and Firecrawl-inspired options:
  - \`colly.Async(true)\` for concurrent crawling
  - \`colly.MaxDepth(config.MaxDepth)\` to limit recursion
  - \`colly.UserAgent(config.UserAgent)\` for default UA
  - \`colly.MaxBodySize(config.MaxBodySize)\` to prevent memory issues
- Configure collector with \`SetRequestTimeout(config.RequestTimeout)\`
- Apply rate limiting with \`collector.Limit(&colly.LimitRule{...})\` using:
  - \`DomainGlob: \"*\"\` to apply to all domains
  - \`Parallelism: config.MaxConcurrency\`
  - \`Delay: config.RequestDelay\`
  - \`RandomDelay: config.RandomDelay\`
- If \`config.UserAgentRotation\` is true, apply \`extensions.RandomUserAgent(collector)\` and \`extensions.Referer(collector)\`
- If \`httpClient\` is provided, set collector's transport: \`collector.WithTransport(httpClient.Transport)\`
- Initialize markdown converter with \`md.NewConverter()\` and configure:
  - Enable GitHub Flavored Markdown (GFM) plugin for tables, task lists, strikethrough
  - Set options for clean output: remove empty paragraphs, normalize whitespace
  - Configure link handling: preserve absolute URLs, handle relative links
- Register \`OnRequest\` callback to:
  - Inject cookies if provided (for authenticated scraping)
  - Log request with URL and depth
  - Set additional headers (Accept-Language, Accept-Encoding)
- Register \`OnError\` callback to log errors with structured logging (URL, status code, error message)
- Return initialized \`HTMLScraper\` instance

**Method \`ScrapeURL(ctx context.Context, targetURL string) (*ScrapeResult, error)\` - Firecrawl's /scrape endpoint equivalent:**
- Create result variable to capture scraped data
- Create error channel for async error handling
- Register \`OnHTML(\"html\")\` callback to extract:
  - **Metadata extraction** (if \`config.IncludeMetadata\`):
    - Page title from \`<title>\` tag
    - Meta description from \`<meta name=\"description\">\`
    - Open Graph tags: \`og:title\`, \`og:description\`, \`og:image\`, \`og:url\`
    - Twitter Card tags: \`twitter:title\`, \`twitter:description\`, \`twitter:image\`
    - Canonical URL from \`<link rel=\"canonical\">\`
    - Language from \`<html lang>\` attribute
  - **Main content extraction** (if \`config.OnlyMainContent\`):
    - Identify main content using semantic HTML5 tags: \`<main>\`, \`<article>\`, \`<section>\`
    - Remove boilerplate: \`<nav>\`, \`<header>\`, \`<footer>\`, \`<aside>\`, \`<script>\`, \`<style>\`, \`<noscript>\`
    - Remove common ad containers by class/id patterns: \"ad\", \"advertisement\", \"sidebar\", \"promo\"
  - **Link extraction** (if \`config.IncludeLinks\`):
    - Call \`ExtractLinks()\` to get all discovered links
  - **Content conversion**:
    - If \`config.OutputFormat\` is \"markdown\" or \"both\": convert cleaned HTML to markdown using \`mdConverter.ConvertString()\`
    - If \`config.OutputFormat\` is \"html\" or \"both\": store cleaned HTML
    - Extract plain text using \`ExtractContent()\` for fallback/search indexing
  - Store raw HTML using \`e.Response.Body\` for debugging/archival
- Register \`OnResponse\` callback to capture:
  - Status code
  - Response headers (Content-Type, Content-Length, Last-Modified)
  - Response time/duration
- Call \`collector.Visit(targetURL)\` to start scraping
- Call \`collector.Wait()\` to block until completion
- Handle context cancellation by checking \`ctx.Done()\` in a goroutine and calling \`collector.Abort()\`
- Return populated \`ScrapeResult\` with markdown, HTML, metadata, and links
- Return error if scraping failed (timeout, network error, 4xx/5xx status)

**Method \`ExtractLinks(htmlElement *colly.HTMLElement, baseURL string) []string\` - Link discovery for crawling:**
- Use Colly's \`htmlElement.ForEach(\"a[href]\", ...)\` to iterate over all anchor tags
- Extract \`href\` attribute from each link
- Parse relative URLs and convert to absolute using \`url.Parse()\` and \`baseURL\`
- Filter out invalid/unwanted links:
  - JavaScript links (\`javascript:\`, \`#\`, \`#!\`)
  - Mailto links (\`mailto:\`)
  - Telephone links (\`tel:\`)
  - File downloads (\`.pdf\`, \`.zip\`, \`.exe\`, \`.dmg\` - configurable)
  - External domains (optional - keep only same-domain links)
- Normalize URLs:
  - Remove fragments (\`#section\`)
  - Sort query parameters for deduplication
  - Lowercase scheme and domain
- Deduplicate using map
- Return slice of absolute URL strings
- Log debug information about discovered links count and filtered count

**Method \`ExtractContent(htmlElement *colly.HTMLElement) string\` - Plain text extraction:**
- Use Colly's \`htmlElement.DOM.Find(\"body\")\` to get body element
- If \`config.OnlyMainContent\`, find main content container first:
  - Try semantic tags: \`main\`, \`article\`, \`[role=main]\`
  - Fallback to body if not found
- Remove unwanted elements:
  - Scripts and styles: \`Find(\"script, style, noscript\").Remove()\`
  - Navigation: \`Find(\"nav, header, footer, aside\").Remove()\`
  - Ads and promos: \`Find(\"[class*=ad], [id*=ad], [class*=promo]\").Remove()\`
- Extract text content using \`.Text()\` method
- Clean up whitespace:
  - Replace multiple spaces with single space using \`regexp.MustCompile(\\s+)\`
  - Replace multiple newlines with double newline (paragraph separation)
  - Trim leading/trailing whitespace
- Return cleaned text content string
- Handle edge cases where body tag is missing (return empty string with warning log)

**Helper method \`ExtractMetadata(htmlElement *colly.HTMLElement) map[string]interface{}\` - Firecrawl-style metadata:**
- Create metadata map
- Extract standard meta tags:
  - \`title\` from \`<title>\` tag
  - \`description\` from \`<meta name=\"description\">\`
  - \`keywords\` from \`<meta name=\"keywords\">\`
  - \`author\` from \`<meta name=\"author\">\`
  - \`language\` from \`<html lang>\` or \`<meta http-equiv=\"content-language\">\`
- Extract Open Graph tags (iterate over \`meta[property^=\"og:\"]\`)
- Extract Twitter Card tags (iterate over \`meta[name^=\"twitter:\"]\`)
- Extract JSON-LD structured data from \`<script type=\"application/ld+json\">\` (parse and store as map)
- Extract canonical URL from \`<link rel=\"canonical\">\`
- Return metadata map

**Helper method \`Close()\`:**
- Clean up collector resources
- Log shutdown message with scraper statistics (total requests, errors, etc.)

**Integration notes:**
- The scraper should be instantiated per-job to maintain auth context and configuration
- Cookies from \`authService.GetHTTPClient()\` should be extracted and passed to constructor
- The scraper works independently of the existing \`RateLimiter\` - Colly handles rate limiting internally
- Results should be compatible with existing \`CrawlResult\` struct via \`ToCrawlResult()\` method
- Markdown output is the primary format for LLM consumption (Firecrawl pattern)
- Metadata extraction provides context for RAG systems and document indexing

### internal\\services\\crawler\\types.go(MODIFY)

References: 

- internal\\services\\crawler\\service.go

Add new types to support Firecrawl-style HTML scraping results with markdown output:

**Add new struct \`ScrapeResult\` (after \`CrawlResult\` around line 89) - Firecrawl's scrape response equivalent:**
- \`URL\` (string) - The scraped URL
- \`StatusCode\` (int) - HTTP status code
- \`Success\` (bool) - Whether scraping succeeded
- \`Markdown\` (string) - Converted markdown content (primary output for LLM consumption)
- \`HTML\` (string) - Cleaned HTML content (optional, based on config)
- \`RawHTML\` (string) - Original raw HTML (for debugging/archival)
- \`Title\` (string) - Page title from \`<title>\` tag or Open Graph
- \`Description\` (string) - Meta description or Open Graph description
- \`Language\` (string) - Page language (from \`<html lang>\` or meta tags)
- \`Links\` ([]string) - Discovered links (absolute URLs) for crawling
- \`Metadata\` (map[string]interface{}) - Extracted metadata (Open Graph, Twitter Cards, JSON-LD, etc.)
- \`TextContent\` (string) - Plain text content (cleaned, for search indexing)
- \`Duration\` (time.Duration) - Time taken to scrape
- \`Error\` (string) - Error message if scraping failed
- \`Timestamp\` (time.Time) - When the scrape was performed

**Add helper method \`ToCrawlResult() *CrawlResult\`:**
- Convert \`ScrapeResult\` to \`CrawlResult\` for compatibility with existing code
- Map \`Markdown\` to \`Body\` field (convert string to []byte) - prefer markdown over HTML
- If markdown is empty, fallback to \`HTML\` or \`RawHTML\`
- Copy \`URL\`, \`StatusCode\`, \`Duration\`, \`Error\` fields
- Merge \`Metadata\` map with additional fields: \`title\`, \`description\`, \`links\`, \`language\`
- Store \`markdown\`, \`html\`, \`text_content\` in metadata for multi-format access
- This enables gradual migration from API-based to HTML-based scraping

**Add helper method \`GetContent() string\`:**
- Return content in priority order: Markdown > HTML > TextContent > RawHTML
- Useful for consumers that don't care about format

**Add constants for content types and output formats (around line 20):**
- \`ContentTypeHTML = \"text/html\"\`
- \`ContentTypeJSON = \"application/json\"\`
- \`ContentTypeMarkdown = \"text/markdown\"\`
- \`OutputFormatMarkdown = \"markdown\"\` - Firecrawl's primary format
- \`OutputFormatHTML = \"html\"\`
- \`OutputFormatBoth = \"both\"\`
- These will be used to determine scraper behavior and output format

**Add struct \`PageMetadata\` for structured metadata (optional, for type safety):**
- \`Title\` (string)
- \`Description\` (string)
- \`Keywords\` ([]string)
- \`Author\` (string)
- \`Language\` (string)
- \`CanonicalURL\` (string)
- \`OpenGraph\` (map[string]string) - og:title, og:description, og:image, etc.
- \`TwitterCard\` (map[string]string) - twitter:title, twitter:description, etc.
- \`JSONLD\` ([]map[string]interface{}) - Structured data from JSON-LD scripts

These types provide a Firecrawl-compatible interface for HTML scraping results with markdown as the primary output format, while maintaining backward compatibility with the existing \`CrawlResult\` structure. The markdown output is optimized for LLM consumption and RAG systems.