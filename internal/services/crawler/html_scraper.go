package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

// HTMLScraper provides Firecrawl-inspired HTML scraping with Colly v2
type HTMLScraper struct {
	collector  *colly.Collector
	config     common.CrawlerConfig
	logger     arbor.ILogger
	httpClient *http.Client
	cookies    []*http.Cookie
}

// contextAwareTransport wraps an http.RoundTripper to support context cancellation
type contextAwareTransport struct {
	base http.RoundTripper
	ctx  context.Context
}

// RoundTrip implements http.RoundTripper with context cancellation support
func (t *contextAwareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check if context is already cancelled before starting request
	select {
	case <-t.ctx.Done():
		return nil, t.ctx.Err()
	default:
	}

	// Clone request with context to enable in-flight cancellation
	req = req.WithContext(t.ctx)

	// Delegate to underlying transport
	return t.base.RoundTrip(req)
}

// NewHTMLScraper creates a new HTML scraper with Colly and markdown conversion
func NewHTMLScraper(config common.CrawlerConfig, logger arbor.ILogger, httpClient *http.Client, cookies []*http.Cookie) *HTMLScraper {
	// Create Colly collector with Firecrawl-inspired options
	collectorOpts := []colly.CollectorOption{
		colly.Async(true),
		colly.MaxDepth(config.MaxDepth),
		colly.UserAgent(config.UserAgent),
	}

	// Add IgnoreRobotsTxt if configured to ignore robots.txt
	if !config.FollowRobotsTxt {
		collectorOpts = append(collectorOpts, colly.IgnoreRobotsTxt())
	}

	c := colly.NewCollector(collectorOpts...)

	// Configure MaxBodySize after collector creation (Comment 4)
	c.MaxBodySize = config.MaxBodySize

	// Configure request timeout
	c.SetRequestTimeout(config.RequestTimeout)

	// Apply rate limiting
	err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: config.MaxConcurrency,
		Delay:       config.RequestDelay,
		RandomDelay: config.RandomDelay,
	})
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to set rate limit on collector")
	}

	// Apply user agent rotation and referer if enabled
	if config.UserAgentRotation {
		extensions.RandomUserAgent(c)
		extensions.Referer(c)
	}

	// Use custom HTTP client if provided
	if httpClient != nil {
		// Attach CookieJar for full cookie lifecycle management (Comment 3)
		// This ensures cookies are persisted across redirects and subrequests.
		// We still seed initial cookies via setupCookies() for explicit auth data.
		if httpClient.Jar != nil {
			c.SetCookieJar(httpClient.Jar)
		}

		// Use custom transport if provided
		if httpClient.Transport != nil {
			c.WithTransport(httpClient.Transport)
		}
	}

	return &HTMLScraper{
		collector:  c,
		config:     config,
		logger:     logger,
		httpClient: httpClient,
		cookies:    cookies,
	}
}

// ScrapeURL scrapes a single URL and returns Firecrawl-style results
func (s *HTMLScraper) ScrapeURL(ctx context.Context, targetURL string) (*ScrapeResult, error) {
	startTime := time.Now()

	// Create result container
	result := &ScrapeResult{
		URL:       targetURL,
		Success:   false,
		Timestamp: startTime,
		Metadata:  make(map[string]interface{}),
	}

	// Clone collector to avoid handler accumulation (Comment 1)
	c := s.collector.Clone()

	// Apply context-aware transport for in-flight cancellation (Comment 2)
	// Get base transport from httpClient or use default
	baseTransport := http.DefaultTransport
	if s.httpClient != nil && s.httpClient.Transport != nil {
		baseTransport = s.httpClient.Transport
	}
	// Wrap transport with context-aware wrapper
	contextTransport := &contextAwareTransport{
		base: baseTransport,
		ctx:  ctx,
	}
	c.WithTransport(contextTransport)

	// Create markdown converter with base URL for relative links (Comment 4)
	mdConverter := md.NewConverter(targetURL, true, nil)

	// Error channel for async error handling
	errChan := make(chan error, 1)

	// Track context cancellation (Comment 2)
	var cancelled atomic.Bool

	// Setup cookies - check for cookie jar first (Comment 7)
	s.setupCookies(c, targetURL)

	// Register OnRequest callback with context cancellation check (Comment 2)
	c.OnRequest(func(r *colly.Request) {
		// Check if context was cancelled
		if ctx.Err() != nil {
			cancelled.Store(true)
			r.Abort()
			return
		}

		// Set additional headers
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
		r.Headers.Set("Accept-Encoding", "gzip, deflate")

		s.logger.Debug().
			Str("url", r.URL.String()).
			Int("depth", r.Depth).
			Msg("Scraping URL")
	})

	// Register OnError callback to populate result (Comment 3)
	c.OnError(func(r *colly.Response, err error) {
		result.Error = err.Error()
		result.Success = false
		if r != nil && r.StatusCode > 0 {
			result.StatusCode = r.StatusCode
		}

		s.logger.Error().
			Err(err).
			Str("url", r.Request.URL.String()).
			Int("status_code", r.StatusCode).
			Msg("Scraping error")

		// Send error to channel
		select {
		case errChan <- err:
		default:
		}
	})

	// Register OnResponse callback with Content-Type checking (Comment 5)
	c.OnResponse(func(r *colly.Response) {
		s.captureResponseHeaders(r, result)
	})

	// Register OnHTML callback to extract content (Comment 6 - using helpers)
	c.OnHTML("html", func(e *colly.HTMLElement) {
		// Extract and populate metadata
		s.extractAndPopulateMetadata(e, result)

		// Convert content to markdown
		s.convertContentToMarkdown(e, targetURL, mdConverter, result)

		// Store cleaned HTML if configured
		s.storeCleanHTML(e, result)

		// Extract plain text
		s.extractPlainText(e, result)

		// Extract links for result
		s.extractLinksForResult(e, targetURL, result)

		// Set success flag if content extraction completed without errors (Comment 5)
		if result.Error == "" {
			result.Success = (result.StatusCode >= 200 && result.StatusCode < 300)
		}
	})

	// Start context cancellation watcher goroutine (Comment 2)
	// The contextAwareTransport wrapper now handles in-flight request cancellation
	// This goroutine sets the cancelled flag for post-processing checks
	go func() {
		<-ctx.Done()
		cancelled.Store(true)
	}()

	// Visit the URL
	err := c.Visit(targetURL)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		errChan <- err
	}

	// Wait for completion
	c.Wait()

	// Calculate duration
	result.Duration = time.Since(startTime)

	// Check for cancellation
	if cancelled.Load() {
		result.Error = "context cancelled"
		result.Success = false
		return result, context.Canceled
	}

	// Check for errors
	select {
	case err := <-errChan:
		return result, err
	default:
		// Only set generic error if no specific error was captured
		if !result.Success && result.Error == "" && result.StatusCode > 0 {
			result.Error = fmt.Sprintf("HTTP status code: %d", result.StatusCode)
		}
		return result, nil
	}
}

// setupCookies handles cookie injection from both cookies slice and http.Client jar (Comment 7)
func (s *HTMLScraper) setupCookies(c *colly.Collector, targetURL string) {
	allCookies := make([]*http.Cookie, 0, len(s.cookies))

	// Add explicitly provided cookies
	allCookies = append(allCookies, s.cookies...)

	// Check if httpClient has a cookie jar and extract cookies for target URL
	if s.httpClient != nil && s.httpClient.Jar != nil {
		parsedURL, err := url.Parse(targetURL)
		if err == nil {
			jarCookies := s.httpClient.Jar.Cookies(parsedURL)
			allCookies = append(allCookies, jarCookies...)
		} else {
			s.logger.Warn().Err(err).Str("url", targetURL).Msg("Failed to parse URL for cookie jar extraction")
		}
	}

	// Set all collected cookies
	if len(allCookies) > 0 {
		c.SetCookies(targetURL, allCookies)
	}
}

// captureResponseHeaders captures response details and checks Content-Type (Comment 5)
func (s *HTMLScraper) captureResponseHeaders(r *colly.Response, result *ScrapeResult) {
	result.StatusCode = r.StatusCode
	result.RawHTML = string(r.Body)

	// Store response headers in metadata
	// Note: In Colly v2.2.0, r.Headers is *http.Header (pointer to map), requiring dereferencing
	headers := make(map[string]string)
	if r.Headers != nil && len(*r.Headers) > 0 {
		for key, values := range *r.Headers {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}
	result.Metadata["headers"] = headers

	// Check Content-Type header (Comment 5)
	// http.Header.Get() method works on pointer receivers
	contentType := ""
	if r.Headers != nil {
		contentType = r.Headers.Get("Content-Type")
	}
	result.Metadata["content_type"] = contentType

	// Only mark as success if Content-Type is HTML or empty (default to HTML)
	isHTML := contentType == "" || strings.HasPrefix(strings.ToLower(contentType), "text/html")
	if !isHTML {
		result.Success = false
		result.Error = fmt.Sprintf("unsupported content type: %s", contentType)
		s.logger.Warn().
			Str("url", result.URL).
			Str("content_type", contentType).
			Msg("Non-HTML content type detected")
	} else {
		result.Success = r.StatusCode >= 200 && r.StatusCode < 300
	}
}

// extractAndPopulateMetadata extracts metadata and populates top-level fields (Comment 6)
func (s *HTMLScraper) extractAndPopulateMetadata(e *colly.HTMLElement, result *ScrapeResult) {
	if !s.config.IncludeMetadata {
		return
	}

	metadata := s.ExtractMetadata(e)
	result.Metadata = metadata

	// Extract top-level fields from metadata
	if title, ok := metadata["title"].(string); ok {
		result.Title = title
	}
	if desc, ok := metadata["description"].(string); ok {
		result.Description = desc
	}
	if lang, ok := metadata["language"].(string); ok {
		result.Language = lang
	}
}

// convertContentToMarkdown converts HTML to markdown (Comment 6)
func (s *HTMLScraper) convertContentToMarkdown(e *colly.HTMLElement, targetURL string, mdConverter *md.Converter, result *ScrapeResult) {
	if s.config.OutputFormat != OutputFormatMarkdown && s.config.OutputFormat != OutputFormatBoth {
		return
	}

	htmlContent := s.extractMainContent(e)

	cleanedHTML, err := htmlContent.DOM.Html()
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to extract HTML for markdown conversion")
		return
	}

	markdown, err := mdConverter.ConvertString(cleanedHTML)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to convert HTML to markdown")
		return
	}

	result.Markdown = markdown
}

// storeCleanHTML stores cleaned HTML if configured (Comment 6)
func (s *HTMLScraper) storeCleanHTML(e *colly.HTMLElement, result *ScrapeResult) {
	if s.config.OutputFormat != OutputFormatHTML && s.config.OutputFormat != OutputFormatBoth {
		return
	}

	htmlContent := s.extractMainContent(e)

	cleanedHTML, err := htmlContent.DOM.Html()
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to extract cleaned HTML")
		return
	}

	result.HTML = cleanedHTML
}

// extractPlainText extracts plain text content (Comment 6)
func (s *HTMLScraper) extractPlainText(e *colly.HTMLElement, result *ScrapeResult) {
	result.TextContent = s.ExtractContent(e)
}

// extractLinksForResult extracts links if configured (Comment 6)
func (s *HTMLScraper) extractLinksForResult(e *colly.HTMLElement, targetURL string, result *ScrapeResult) {
	if !s.config.IncludeLinks {
		return
	}

	result.Links = s.ExtractLinks(e, targetURL)
}

// extractMainContent extracts and cleans main content from HTML element
func (s *HTMLScraper) extractMainContent(e *colly.HTMLElement) *colly.HTMLElement {
	htmlContent := e.DOM

	// Extract main content if configured
	if s.config.OnlyMainContent {
		// Try to find main content container
		mainContent := htmlContent.Find("main, article, [role=main]").First()
		if mainContent.Length() > 0 {
			// Create new HTMLElement with main content
			return &colly.HTMLElement{
				Name:     "main",
				Text:     "",
				Request:  e.Request,
				Response: e.Response,
				DOM:      mainContent,
				Index:    0,
			}
		}

		// Remove boilerplate elements from full DOM
		htmlContent.Find("nav, header, footer, aside, script, style, noscript").Remove()
		htmlContent.Find("[class*=ad], [id*=ad], [class*=promo], [class*=sidebar]").Remove()
	}

	return e
}

// ExtractLinks discovers and extracts links from HTML
func (s *HTMLScraper) ExtractLinks(htmlElement *colly.HTMLElement, baseURL string) []string {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		s.logger.Warn().Err(err).Str("base_url", baseURL).Msg("Failed to parse base URL")
		return []string{}
	}

	linkMap := make(map[string]bool)
	links := []string{}

	// Iterate over all anchor tags
	htmlElement.ForEach("a[href]", func(_ int, el *colly.HTMLElement) {
		href := el.Attr("href")
		if href == "" {
			return
		}

		// Skip unwanted link types
		if strings.HasPrefix(href, "javascript:") ||
			strings.HasPrefix(href, "#") ||
			strings.HasPrefix(href, "mailto:") ||
			strings.HasPrefix(href, "tel:") {
			return
		}

		// Parse and resolve relative URLs
		parsedHref, err := url.Parse(href)
		if err != nil {
			return
		}

		// Resolve relative URLs
		absoluteURL := parsedBase.ResolveReference(parsedHref)

		// Normalize URL
		normalizedURL := s.normalizeURL(absoluteURL)

		// Skip file downloads (configurable list)
		if s.isFileDownload(normalizedURL) {
			return
		}

		// Deduplicate
		if !linkMap[normalizedURL] {
			linkMap[normalizedURL] = true
			links = append(links, normalizedURL)
		}
	})

	s.logger.Debug().
		Int("total_discovered", len(links)).
		Str("url", baseURL).
		Msg("Extracted links")

	return links
}

// ExtractContent extracts plain text content from HTML
func (s *HTMLScraper) ExtractContent(htmlElement *colly.HTMLElement) string {
	// Get the body element
	body := htmlElement.DOM.Find("body")
	if body.Length() == 0 {
		s.logger.Warn().Msg("No body tag found in HTML")
		return ""
	}

	// If only main content is configured, find it
	if s.config.OnlyMainContent {
		mainContent := body.Find("main, article, [role=main]").First()
		if mainContent.Length() > 0 {
			body = mainContent
		}
	}

	// Remove unwanted elements
	body.Find("script, style, noscript").Remove()
	body.Find("nav, header, footer, aside").Remove()
	body.Find("[class*=ad], [id*=ad], [class*=promo]").Remove()

	// Extract text
	text := body.Text()

	// Clean up whitespace
	text = s.cleanWhitespace(text)

	return text
}

// ExtractMetadata extracts page metadata including Open Graph and JSON-LD
func (s *HTMLScraper) ExtractMetadata(htmlElement *colly.HTMLElement) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Extract title
	title := htmlElement.DOM.Find("title").First().Text()
	if title != "" {
		metadata["title"] = strings.TrimSpace(title)
	}

	// Extract standard meta tags
	htmlElement.ForEach("meta[name]", func(_ int, el *colly.HTMLElement) {
		name := el.Attr("name")
		content := el.Attr("content")
		if name != "" && content != "" {
			switch strings.ToLower(name) {
			case "description":
				metadata["description"] = content
			case "keywords":
				metadata["keywords"] = strings.Split(content, ",")
			case "author":
				metadata["author"] = content
			}
		}
	})

	// Extract language
	lang := htmlElement.DOM.Find("html").AttrOr("lang", "")
	if lang == "" {
		htmlElement.ForEach("meta[http-equiv='content-language']", func(_ int, el *colly.HTMLElement) {
			lang = el.Attr("content")
		})
	}
	if lang != "" {
		metadata["language"] = lang
	}

	// Extract Open Graph tags
	openGraph := make(map[string]string)
	htmlElement.ForEach("meta[property^='og:']", func(_ int, el *colly.HTMLElement) {
		property := el.Attr("property")
		content := el.Attr("content")
		if property != "" && content != "" {
			openGraph[property] = content
		}
	})
	if len(openGraph) > 0 {
		metadata["open_graph"] = openGraph
	}

	// Extract Twitter Card tags
	twitterCard := make(map[string]string)
	htmlElement.ForEach("meta[name^='twitter:']", func(_ int, el *colly.HTMLElement) {
		name := el.Attr("name")
		content := el.Attr("content")
		if name != "" && content != "" {
			twitterCard[name] = content
		}
	})
	if len(twitterCard) > 0 {
		metadata["twitter_card"] = twitterCard
	}

	// Extract canonical URL
	canonical := htmlElement.DOM.Find("link[rel='canonical']").AttrOr("href", "")
	if canonical != "" {
		metadata["canonical_url"] = canonical
	}

	// Extract JSON-LD structured data (handles both objects and arrays)
	jsonLDScripts := []interface{}{}
	htmlElement.ForEach("script[type='application/ld+json']", func(_ int, el *colly.HTMLElement) {
		jsonText := el.Text
		if jsonText == "" {
			return
		}

		// Try to unmarshal as generic interface{} to detect type
		var data interface{}
		if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to parse JSON-LD script")
			return
		}

		// Handle both array and object formats
		switch v := data.(type) {
		case []interface{}:
			// JSON-LD is an array - append all entries
			jsonLDScripts = append(jsonLDScripts, v...)
		case map[string]interface{}:
			// JSON-LD is a single object - append it
			jsonLDScripts = append(jsonLDScripts, v)
		default:
			s.logger.Warn().Msg("Unexpected JSON-LD format (not object or array)")
		}
	})
	if len(jsonLDScripts) > 0 {
		metadata["json_ld"] = jsonLDScripts
	}

	return metadata
}

// Close cleans up scraper resources
func (s *HTMLScraper) Close() {
	s.logger.Debug().Msg("Closing HTML scraper")
}

// Helper methods

func (s *HTMLScraper) normalizeURL(u *url.URL) string {
	// Remove fragment
	u.Fragment = ""

	// Lowercase scheme and host
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	return u.String()
}

func (s *HTMLScraper) isFileDownload(urlStr string) bool {
	downloadExtensions := []string{
		".pdf", ".zip", ".tar", ".gz", ".exe", ".dmg",
		".pkg", ".deb", ".rpm", ".iso", ".rar", ".7z",
		".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	}

	lowercaseURL := strings.ToLower(urlStr)
	for _, ext := range downloadExtensions {
		if strings.HasSuffix(lowercaseURL, ext) {
			return true
		}
	}
	return false
}

func (s *HTMLScraper) cleanWhitespace(text string) string {
	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`[ \t]+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	// Replace multiple newlines with double newline (paragraph separation)
	newlineRegex := regexp.MustCompile(`\n{3,}`)
	text = newlineRegex.ReplaceAllString(text, "\n\n")

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}
