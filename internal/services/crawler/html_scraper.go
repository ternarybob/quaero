package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

// HTMLScraper provides JavaScript-enabled HTML scraping with chromedp
type HTMLScraper struct {
	config        common.CrawlerConfig
	logger        arbor.ILogger
	httpClient    *http.Client
	cookies       []*http.Cookie
	browserCtx    context.Context // Optional: reusable browser context from pool
	browserCancel context.CancelFunc
}

// NewHTMLScraper creates a new HTML scraper with chromedp and markdown conversion
func NewHTMLScraper(config common.CrawlerConfig, logger arbor.ILogger, httpClient *http.Client, cookies []*http.Cookie) *HTMLScraper {
	return &HTMLScraper{
		config:     config,
		logger:     logger,
		httpClient: httpClient,
		cookies:    cookies,
	}
}

// NewHTMLScraperWithBrowser creates a scraper with a pre-allocated browser context
// This is more efficient for scraping multiple URLs as it reuses the same browser instance
func NewHTMLScraperWithBrowser(config common.CrawlerConfig, logger arbor.ILogger, httpClient *http.Client, cookies []*http.Cookie, browserCtx context.Context, browserCancel context.CancelFunc) *HTMLScraper {
	return &HTMLScraper{
		config:        config,
		logger:        logger,
		httpClient:    httpClient,
		cookies:       cookies,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
	}
}

// ScrapeURL scrapes a single URL with JavaScript rendering and returns results
func (s *HTMLScraper) ScrapeURL(ctx context.Context, targetURL string) (*ScrapeResult, error) {
	startTime := time.Now()

	// Create result container
	result := &ScrapeResult{
		URL:       targetURL,
		Success:   false,
		Timestamp: startTime,
		Metadata:  make(map[string]interface{}),
	}

	// Determine which browser context to use
	var browserCtx context.Context
	var browserCancel context.CancelFunc
	var allocatorCancel context.CancelFunc
	var shouldDeferCancel bool = false

	if s.browserCtx != nil {
		// Use pre-allocated browser from pool (efficient for multiple URLs)
		browserCtx = s.browserCtx
		s.logger.Debug().Str("url", targetURL).Msg("Using pooled browser context")
	} else {
		// Create new browser instance (fallback for compatibility)
		var allocatorCtx context.Context
		allocatorCtx, allocatorCancel = chromedp.NewExecAllocator(
			context.Background(),
			append(
				chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", true),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.Flag("disable-dev-shm-usage", true),
				chromedp.UserAgent(s.config.UserAgent),
			)...,
		)
		shouldDeferCancel = true

		// Create browser context from allocator
		browserCtx, browserCancel = chromedp.NewContext(allocatorCtx)
		s.logger.Debug().Str("url", targetURL).Msg("Created new browser instance (not pooled)")
	}

	// Cleanup if we created a new browser (not from pool)
	if shouldDeferCancel {
		defer func() {
			if browserCancel != nil {
				browserCancel()
			}
			if allocatorCancel != nil {
				allocatorCancel()
			}
		}()
	}

	// Determine context for chromedp operations
	// For pooled browsers, use the browser context directly to avoid cancellation issues
	// For new browsers, apply request timeout
	var chromedpCtx context.Context
	var chromedpCancel context.CancelFunc

	if s.browserCtx != nil {
		// Using pooled browser - use browser context directly without timeout wrapper
		// The pool manages the lifecycle, and individual request timeouts could interfere
		chromedpCtx = browserCtx
		chromedpCancel = nil
		s.logger.Debug().Str("url", targetURL).Msg("Using pooled browser context without timeout wrapper")
	} else {
		// Using new browser - apply request timeout as before
		chromedpCtx, chromedpCancel = context.WithTimeout(browserCtx, s.config.RequestTimeout)
		defer chromedpCancel()
		s.logger.Debug().Str("url", targetURL).Msg("Using new browser with timeout wrapper")
	}

	// Apply configured cookies to browser context
	if err := s.setupCookies(browserCtx, targetURL); err != nil {
		s.logger.Warn().Err(err).Str("url", targetURL).Msg("Failed to setup cookies")
	}

	// Navigate to URL and wait for JavaScript rendering
	var htmlContent string
	var statusCode int64
	var responseHeaders map[string]interface{}

	err := chromedp.Run(chromedpCtx,
		chromedp.Navigate(targetURL),
		chromedp.Sleep(s.config.JavaScriptWaitTime), // Wait for JavaScript to render
		chromedp.OuterHTML("html", &htmlContent),
		chromedp.Evaluate(`({
			statusCode: window.performance?.getEntriesByType?.('navigation')?.[0]?.responseStatus || 200,
			headers: {}
		})`, &responseHeaders),
	)

	if err != nil {
		result.Error = err.Error()
		result.Success = false
		s.logger.Error().Err(err).Str("url", targetURL).Msg("Failed to scrape URL with chromedp")
		return result, err
	}

	// Extract status code from response
	if sc, ok := responseHeaders["statusCode"].(float64); ok {
		statusCode = int64(sc)
	} else {
		statusCode = 200 // Default to 200 if we got HTML
	}

	result.StatusCode = int(statusCode)
	result.RawHTML = htmlContent
	result.Success = statusCode >= 200 && statusCode < 300

	s.logger.Debug().
		Str("url", targetURL).
		Int("status", int(statusCode)).
		Int("html_length", len(htmlContent)).
		Msg("Successfully scraped URL with JavaScript rendering")

	// Extract metadata
	if s.config.IncludeMetadata {
		metadata := s.extractMetadataFromHTML(htmlContent, targetURL)
		result.Metadata = metadata

		// Populate top-level fields
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

	// Convert to markdown
	if s.config.OutputFormat == OutputFormatMarkdown || s.config.OutputFormat == OutputFormatBoth {
		markdown := s.convertHTMLToMarkdown(htmlContent, targetURL)
		result.Markdown = markdown

		if markdown == "" {
			s.logger.Warn().
				Str("url", targetURL).
				Int("html_length", len(htmlContent)).
				Msg("Markdown conversion produced empty result")
		} else {
			s.logger.Info().
				Str("url", targetURL).
				Int("html_length", len(htmlContent)).
				Int("markdown_length", len(markdown)).
				Msg("Successfully converted HTML to markdown")
		}
	}

	// Store cleaned HTML if configured
	if s.config.OutputFormat == OutputFormatHTML || s.config.OutputFormat == OutputFormatBoth {
		cleanedHTML := s.extractMainContentFromHTML(htmlContent)
		result.HTML = cleanedHTML
	}

	// Extract plain text
	result.TextContent = s.extractPlainTextFromHTML(htmlContent)

	// Extract links
	if s.config.IncludeLinks {
		links := s.extractLinksFromHTML(htmlContent, targetURL)
		result.Links = links
	}

	// Calculate duration
	result.Duration = time.Since(startTime)

	return result, nil
}

// setupCookies applies cookies to chromedp browser context
func (s *HTMLScraper) setupCookies(ctx context.Context, targetURL string) error {
	if len(s.cookies) == 0 && (s.httpClient == nil || s.httpClient.Jar == nil) {
		return nil // No cookies to set
	}

	// Parse target URL to get domain
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("failed to parse URL for cookies: %w", err)
	}

	// Collect all cookies
	allCookies := make([]*http.Cookie, 0, len(s.cookies))
	allCookies = append(allCookies, s.cookies...)

	// Add cookies from httpClient jar if available
	if s.httpClient != nil && s.httpClient.Jar != nil {
		jarCookies := s.httpClient.Jar.Cookies(parsedURL)
		allCookies = append(allCookies, jarCookies...)
	}

	// Convert to chromedp cookie format and apply
	if len(allCookies) > 0 {
		// Use chromedp.ActionFunc to set all cookies
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			for _, c := range allCookies {
				// Determine domain - use cookie's domain or fall back to URL host
				domain := c.Domain
				if domain == "" {
					domain = parsedURL.Host
				} else {
					domain = strings.TrimPrefix(domain, ".") // Remove leading dot
				}

				// Set cookie using network.SetCookie
				// Note: chromedp handles cookie expiration internally, so we don't explicitly set WithExpires
				err := network.SetCookie(c.Name, c.Value).
					WithDomain(domain).
					WithPath(c.Path).
					WithHTTPOnly(c.HttpOnly).
					WithSecure(c.Secure).
					Do(ctx)

				if err != nil {
					return err
				}
			}
			return nil
		})); err != nil {
			return fmt.Errorf("failed to set cookies: %w", err)
		}

		s.logger.Debug().
			Int("cookie_count", len(allCookies)).
			Str("url", targetURL).
			Msg("Applied cookies to browser context")
	}

	return nil
}

// extractMetadataFromHTML extracts metadata from raw HTML string
func (s *HTMLScraper) extractMetadataFromHTML(htmlContent, baseURL string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Use regex to extract basic metadata (title, description, etc.)
	// This is a simplified implementation - for production, consider using a proper HTML parser

	// Extract title
	titleRegex := regexp.MustCompile(`<title[^>]*>(.*?)</title>`)
	if matches := titleRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
		metadata["title"] = strings.TrimSpace(matches[1])
	}

	// Extract meta description
	descRegex := regexp.MustCompile(`<meta\s+name=["']description["']\s+content=["']([^"']+)["']`)
	if matches := descRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
		metadata["description"] = matches[1]
	}

	// Extract language
	langRegex := regexp.MustCompile(`<html[^>]+lang=["']([^"']+)["']`)
	if matches := langRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
		metadata["language"] = matches[1]
	}

	// Extract Open Graph tags
	openGraph := make(map[string]string)
	ogRegex := regexp.MustCompile(`<meta\s+property=["'](og:[^"']+)["']\s+content=["']([^"']+)["']`)
	for _, matches := range ogRegex.FindAllStringSubmatch(htmlContent, -1) {
		if len(matches) > 2 {
			openGraph[matches[1]] = matches[2]
		}
	}
	if len(openGraph) > 0 {
		metadata["open_graph"] = openGraph
	}

	// Extract Twitter Card tags
	twitterCard := make(map[string]string)
	twitterRegex := regexp.MustCompile(`<meta\s+name=["'](twitter:[^"']+)["']\s+content=["']([^"']+)["']`)
	for _, matches := range twitterRegex.FindAllStringSubmatch(htmlContent, -1) {
		if len(matches) > 2 {
			twitterCard[matches[1]] = matches[2]
		}
	}
	if len(twitterCard) > 0 {
		metadata["twitter_card"] = twitterCard
	}

	// Extract canonical URL
	canonicalRegex := regexp.MustCompile(`<link\s+rel=["']canonical["']\s+href=["']([^"']+)["']`)
	if matches := canonicalRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
		metadata["canonical_url"] = matches[1]
	}

	// Extract JSON-LD
	jsonLDRegex := regexp.MustCompile(`<script\s+type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	jsonLDScripts := []interface{}{}
	for _, matches := range jsonLDRegex.FindAllStringSubmatch(htmlContent, -1) {
		if len(matches) > 1 {
			var data interface{}
			if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
				switch v := data.(type) {
				case []interface{}:
					jsonLDScripts = append(jsonLDScripts, v...)
				case map[string]interface{}:
					jsonLDScripts = append(jsonLDScripts, v)
				}
			}
		}
	}
	if len(jsonLDScripts) > 0 {
		metadata["json_ld"] = jsonLDScripts
	}

	return metadata
}

// convertHTMLToMarkdown converts HTML to markdown
func (s *HTMLScraper) convertHTMLToMarkdown(htmlContent, baseURL string) string {
	// Extract main content first if configured
	if s.config.OnlyMainContent {
		htmlContent = s.extractMainContentFromHTML(htmlContent)
	}

	// Create markdown converter
	mdConverter := md.NewConverter(baseURL, true, nil)

	// Convert to markdown
	markdown, err := mdConverter.ConvertString(htmlContent)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("url", baseURL).
			Msg("Failed to convert HTML to markdown")

		// Try fallback without main content extraction
		if s.config.EnableEmptyOutputFallback && s.config.OnlyMainContent {
			s.logger.Info().Str("url", baseURL).Msg("Retrying markdown conversion without main content extraction")
			markdown, err = mdConverter.ConvertString(htmlContent)
			if err != nil {
				s.logger.Error().Err(err).Str("url", baseURL).Msg("Fallback markdown conversion also failed")
				return ""
			}
		} else {
			return ""
		}
	}

	return markdown
}

// extractMainContentFromHTML extracts main content from HTML string
func (s *HTMLScraper) extractMainContentFromHTML(htmlContent string) string {
	// Try to find main content using regex
	// Look for <main>, <article>, or role=main
	// Note: Go regexp doesn't support backreferences, so we try each tag separately
	mainTagRegex := regexp.MustCompile(`<main[^>]*>(.*?)</main>`)
	if matches := mainTagRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
		s.logger.Debug().Msg("Found <main> content tag")
		return matches[1]
	}

	articleTagRegex := regexp.MustCompile(`<article[^>]*>(.*?)</article>`)
	if matches := articleTagRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
		s.logger.Debug().Msg("Found <article> content tag")
		return matches[1]
	}

	// Try role=main
	roleMainRegex := regexp.MustCompile(`<[^>]+role=["']main["'][^>]*>(.*?)</[^>]+>`)
	if matches := roleMainRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
		s.logger.Debug().Msg("Found role=main content")
		return matches[1]
	}

	// Fallback: extract body and remove boilerplate
	bodyRegex := regexp.MustCompile(`<body[^>]*>(.*?)</body>`)
	if matches := bodyRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
		body := matches[1]

		// Remove common boilerplate elements
		// Note: Go regexp doesn't support backreferences, so we remove each tag type separately
		body = regexp.MustCompile(`<nav[^>]*>.*?</nav>`).ReplaceAllString(body, "")
		body = regexp.MustCompile(`<header[^>]*>.*?</header>`).ReplaceAllString(body, "")
		body = regexp.MustCompile(`<footer[^>]*>.*?</footer>`).ReplaceAllString(body, "")
		body = regexp.MustCompile(`<aside[^>]*>.*?</aside>`).ReplaceAllString(body, "")
		body = regexp.MustCompile(`<script[^>]*>.*?</script>`).ReplaceAllString(body, "")
		body = regexp.MustCompile(`<style[^>]*>.*?</style>`).ReplaceAllString(body, "")
		body = regexp.MustCompile(`<noscript[^>]*>.*?</noscript>`).ReplaceAllString(body, "")

		// Remove ad/promo divs
		adRegex := regexp.MustCompile(`<div[^>]+(class|id)=["'][^"']*(ad|promo|sidebar)[^"']*["'][^>]*>.*?</div>`)
		body = adRegex.ReplaceAllString(body, "")

		s.logger.Debug().Msg("Using body with boilerplate removal")
		return body
	}

	// Last resort: return original content
	s.logger.Warn().Msg("No body tag found, using full HTML")
	return htmlContent
}

// extractPlainTextFromHTML extracts plain text from HTML
func (s *HTMLScraper) extractPlainTextFromHTML(htmlContent string) string {
	// Extract main content if configured
	if s.config.OnlyMainContent {
		htmlContent = s.extractMainContentFromHTML(htmlContent)
	}

	// Remove script and style tags
	// Note: Go regexp doesn't support backreferences, so we remove each tag type separately
	text := regexp.MustCompile(`<script[^>]*>.*?</script>`).ReplaceAllString(htmlContent, "")
	text = regexp.MustCompile(`<style[^>]*>.*?</style>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`<noscript[^>]*>.*?</noscript>`).ReplaceAllString(text, "")

	// Remove HTML tags
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text = tagRegex.ReplaceAllString(text, " ")

	// Decode HTML entities
	text = s.decodeHTMLEntities(text)

	// Clean whitespace
	text = s.cleanWhitespace(text)

	return text
}

// extractLinksFromHTML extracts links from HTML string
func (s *HTMLScraper) extractLinksFromHTML(htmlContent, baseURL string) []string {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		s.logger.Warn().Err(err).Str("base_url", baseURL).Msg("Failed to parse base URL")
		return []string{}
	}

	linkMap := make(map[string]bool)
	links := []string{}

	// Extract all href attributes
	hrefRegex := regexp.MustCompile(`<a[^>]+href=["']([^"']+)["']`)
	for _, matches := range hrefRegex.FindAllStringSubmatch(htmlContent, -1) {
		if len(matches) > 1 {
			href := matches[1]

			// Skip unwanted link types
			if strings.HasPrefix(href, "javascript:") ||
				strings.HasPrefix(href, "#") ||
				strings.HasPrefix(href, "mailto:") ||
				strings.HasPrefix(href, "tel:") {
				continue
			}

			// Parse and resolve relative URLs
			parsedHref, err := url.Parse(href)
			if err != nil {
				continue
			}

			// Resolve relative URLs
			absoluteURL := parsedBase.ResolveReference(parsedHref)

			// Normalize URL
			normalizedURL := s.normalizeURL(absoluteURL)

			// Skip file downloads
			if s.isFileDownload(normalizedURL) {
				continue
			}

			// Deduplicate
			if !linkMap[normalizedURL] {
				linkMap[normalizedURL] = true
				links = append(links, normalizedURL)
			}
		}
	}

	s.logger.Debug().
		Int("total_discovered", len(links)).
		Str("url", baseURL).
		Msg("Extracted links")

	return links
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

func (s *HTMLScraper) decodeHTMLEntities(text string) string {
	// Common HTML entities
	entities := map[string]string{
		"&amp;":   "&",
		"&lt;":    "<",
		"&gt;":    ">",
		"&quot;":  "\"",
		"&apos;":  "'",
		"&nbsp;":  " ",
		"&ndash;": "–",
		"&mdash;": "—",
	}

	for entity, replacement := range entities {
		text = strings.ReplaceAll(text, entity, replacement)
	}

	return text
}

// ExtractLinks is kept for backward compatibility with existing code
// It's a wrapper around extractLinksFromHTML for chromedp implementation
func (s *HTMLScraper) ExtractLinks(htmlContent, baseURL string) []string {
	return s.extractLinksFromHTML(htmlContent, baseURL)
}

// ExtractContent is kept for backward compatibility
func (s *HTMLScraper) ExtractContent(htmlContent string) string {
	return s.extractPlainTextFromHTML(htmlContent)
}

// ExtractMetadata is kept for backward compatibility
func (s *HTMLScraper) ExtractMetadata(htmlContent, baseURL string) map[string]interface{} {
	return s.extractMetadataFromHTML(htmlContent, baseURL)
}
