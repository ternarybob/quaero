// -----------------------------------------------------------------------
// Link Extractor - Link discovery and filtering with pattern matching
// -----------------------------------------------------------------------

package crawler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ternarybob/arbor"
)

// LinkExtractor handles link discovery and filtering from HTML content
type LinkExtractor struct {
	logger arbor.ILogger
}

// NewLinkExtractor creates a new link extractor
func NewLinkExtractor(logger arbor.ILogger) *LinkExtractor {
	return &LinkExtractor{
		logger: logger,
	}
}

// LinkFilterResult represents the result of link filtering
type LinkFilterResult struct {
	OriginalLinks     []string `json:"original_links"`     // All discovered links
	FilteredLinks     []string `json:"filtered_links"`     // Links after include/exclude filtering
	Found             int      `json:"found"`              // Total links found
	Filtered          int      `json:"filtered"`           // Links that passed filtering
	Excluded          int      `json:"excluded"`           // Links that were excluded
	Reasons           []string `json:"exclusion_reasons"`  // Reasons for exclusions
	IncludeMatches    int      `json:"include_matches"`    // Links that matched include patterns
	ExcludeMatches    int      `json:"exclude_matches"`    // Links that matched exclude patterns
	InvalidURLs       int      `json:"invalid_urls"`       // Links that were invalid URLs
	DuplicatesRemoved int      `json:"duplicates_removed"` // Number of duplicate links removed
}

// LinkProcessingResult represents comprehensive link processing statistics
type LinkProcessingResult struct {
	Found    int `json:"found"`         // Total links discovered
	Filtered int `json:"filtered"`      // Links after include/exclude filtering
	Followed int `json:"followed"`      // Links that will be followed (respecting depth limits)
	Skipped  int `json:"skipped_depth"` // Links skipped due to depth limits
}

// ExtractLinks discovers all links from HTML content
func (le *LinkExtractor) ExtractLinks(html string, sourceURL string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML for link extraction: %w", err)
	}

	return le.extractLinksFromDocument(doc, sourceURL), nil
}

// extractLinksFromDocument extracts links from a goquery document
func (le *LinkExtractor) extractLinksFromDocument(doc *goquery.Document, sourceURL string) []string {
	var links []string
	linkSet := make(map[string]bool) // For deduplication

	// Parse source URL for resolving relative links
	baseURL, err := url.Parse(sourceURL)
	if err != nil {
		le.logger.Warn().Err(err).Str("source_url", sourceURL).Msg("Failed to parse source URL for link resolution")
		baseURL = nil
	}

	// Extract links from <a> tags
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		// Skip javascript:, mailto:, tel:, and fragment-only links
		if le.shouldSkipLink(href) {
			return
		}

		// Resolve relative URLs
		resolvedURL := le.resolveURL(href, baseURL)
		if resolvedURL == "" {
			return
		}

		// Deduplicate links
		if !linkSet[resolvedURL] {
			linkSet[resolvedURL] = true
			links = append(links, resolvedURL)
		}
	})

	// Extract links from other elements (img, link, script, etc.)
	le.extractAdditionalLinks(doc, baseURL, linkSet, &links)

	le.logger.Debug().
		Str("source_url", sourceURL).
		Int("links_found", len(links)).
		Msg("Links extracted from HTML content")

	return links
}

// shouldSkipLink determines if a link should be skipped during extraction
func (le *LinkExtractor) shouldSkipLink(href string) bool {
	href = strings.ToLower(strings.TrimSpace(href))

	// Skip empty links
	if href == "" {
		return true
	}

	// Skip javascript:, mailto:, tel: links
	if strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "tel:") ||
		strings.HasPrefix(href, "sms:") ||
		strings.HasPrefix(href, "ftp:") {
		return true
	}

	// Skip fragment-only links (anchors)
	if strings.HasPrefix(href, "#") {
		return true
	}

	// Skip data: URLs
	if strings.HasPrefix(href, "data:") {
		return true
	}

	return false
}

// resolveURL resolves a potentially relative URL against a base URL
func (le *LinkExtractor) resolveURL(href string, baseURL *url.URL) string {
	if baseURL == nil {
		// If we can't resolve, try to parse as absolute URL
		if parsedURL, err := url.Parse(href); err == nil && parsedURL.IsAbs() {
			return parsedURL.String()
		}
		return ""
	}

	resolvedURL, err := baseURL.Parse(href)
	if err != nil {
		le.logger.Debug().Err(err).Str("href", href).Msg("Failed to resolve URL")
		return ""
	}

	return resolvedURL.String()
}

// extractAdditionalLinks extracts links from non-anchor elements
func (le *LinkExtractor) extractAdditionalLinks(doc *goquery.Document, baseURL *url.URL, linkSet map[string]bool, links *[]string) {
	// Extract from link[rel="canonical"] and other link elements
	doc.Find("link[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists && href != "" {
			if rel, exists := s.Attr("rel"); exists {
				// Only include certain rel types
				if rel == "canonical" || rel == "alternate" || rel == "next" || rel == "prev" {
					resolvedURL := le.resolveURL(href, baseURL)
					if resolvedURL != "" && !linkSet[resolvedURL] {
						linkSet[resolvedURL] = true
						*links = append(*links, resolvedURL)
					}
				}
			}
		}
	})

	// Extract from img[src] for potential content pages
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists && src != "" {
			resolvedURL := le.resolveURL(src, baseURL)
			if resolvedURL != "" && !linkSet[resolvedURL] {
				// Only include if it looks like a content URL (not just an image)
				if le.isContentURL(resolvedURL) {
					linkSet[resolvedURL] = true
					*links = append(*links, resolvedURL)
				}
			}
		}
	})
}

// isContentURL determines if a URL likely points to content rather than just media
func (le *LinkExtractor) isContentURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	path := strings.ToLower(parsedURL.Path)

	// Skip common image/media extensions
	mediaExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".ico", ".css", ".js", ".pdf", ".zip", ".tar", ".gz"}
	for _, ext := range mediaExtensions {
		if strings.HasSuffix(path, ext) {
			return false
		}
	}

	return true
}

// FilterLinks filters links based on include and exclude patterns
func (le *LinkExtractor) FilterLinks(links []string, includePatterns, excludePatterns []string) *LinkFilterResult {
	result := &LinkFilterResult{
		OriginalLinks: make([]string, len(links)),
		FilteredLinks: make([]string, 0),
		Found:         len(links),
		Reasons:       make([]string, 0),
	}

	// Copy original links
	copy(result.OriginalLinks, links)

	// Compile regex patterns
	includeRegexes, includeErrors := le.compilePatterns(includePatterns)
	excludeRegexes, excludeErrors := le.compilePatterns(excludePatterns)

	// Log pattern compilation errors
	for _, err := range includeErrors {
		le.logger.Warn().Err(err).Msg("Failed to compile include pattern")
		result.Reasons = append(result.Reasons, fmt.Sprintf("Invalid include pattern: %v", err))
	}
	for _, err := range excludeErrors {
		le.logger.Warn().Err(err).Msg("Failed to compile exclude pattern")
		result.Reasons = append(result.Reasons, fmt.Sprintf("Invalid exclude pattern: %v", err))
	}

	// Process each link
	for _, link := range links {
		// Validate URL
		if _, err := url.Parse(link); err != nil {
			result.InvalidURLs++
			result.Reasons = append(result.Reasons, fmt.Sprintf("Invalid URL: %s", link))
			continue
		}

		// Apply include patterns (if any)
		includeMatch := len(includeRegexes) == 0 // If no include patterns, include by default
		for _, regex := range includeRegexes {
			if regex.MatchString(link) {
				includeMatch = true
				result.IncludeMatches++
				break
			}
		}

		if !includeMatch {
			result.Excluded++
			result.Reasons = append(result.Reasons, fmt.Sprintf("Does not match include patterns: %s", link))
			continue
		}

		// Apply exclude patterns
		excludeMatch := false
		for _, regex := range excludeRegexes {
			if regex.MatchString(link) {
				excludeMatch = true
				result.ExcludeMatches++
				result.Reasons = append(result.Reasons, fmt.Sprintf("Matches exclude pattern: %s", link))
				break
			}
		}

		if excludeMatch {
			result.Excluded++
			continue
		}

		// Link passed all filters
		result.FilteredLinks = append(result.FilteredLinks, link)
	}

	result.Filtered = len(result.FilteredLinks)

	// Log filtering results
	le.logger.Debug().
		Int("found", result.Found).
		Int("filtered", result.Filtered).
		Int("excluded", result.Excluded).
		Int("include_matches", result.IncludeMatches).
		Int("exclude_matches", result.ExcludeMatches).
		Int("invalid_urls", result.InvalidURLs).
		Msg("Link filtering completed")

	return result
}

// compilePatterns compiles regex patterns and returns compiled regexes and any errors
func (le *LinkExtractor) compilePatterns(patterns []string) ([]*regexp.Regexp, []error) {
	var regexes []*regexp.Regexp
	var errors []error

	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		regex, err := regexp.Compile(pattern)
		if err != nil {
			errors = append(errors, fmt.Errorf("pattern '%s': %w", pattern, err))
			continue
		}

		regexes = append(regexes, regex)
	}

	return regexes, errors
}

// ProcessLinks combines link extraction and filtering in one operation
func (le *LinkExtractor) ProcessLinks(html string, sourceURL string, includePatterns, excludePatterns []string) (*LinkFilterResult, error) {
	// Extract links
	links, err := le.ExtractLinks(html, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract links: %w", err)
	}

	// Filter links
	result := le.FilterLinks(links, includePatterns, excludePatterns)

	le.logger.Info().
		Str("source_url", sourceURL).
		Int("found", result.Found).
		Int("filtered", result.Filtered).
		Int("excluded", result.Excluded).
		Msg("Link processing completed")

	return result, nil
}

// GetLinkProcessingStats converts LinkFilterResult to LinkProcessingResult for compatibility
func (le *LinkExtractor) GetLinkProcessingStats(filterResult *LinkFilterResult, maxDepth, currentDepth int) *LinkProcessingResult {
	followed := 0
	skipped := 0

	// If we're at max depth, all filtered links are skipped
	if currentDepth >= maxDepth {
		skipped = filterResult.Filtered
	} else {
		followed = filterResult.Filtered
	}

	return &LinkProcessingResult{
		Found:    filterResult.Found,
		Filtered: filterResult.Filtered,
		Followed: followed,
		Skipped:  skipped,
	}
}

// LogLinkProcessingResult logs comprehensive link processing statistics
func (le *LinkExtractor) LogLinkProcessingResult(result *LinkProcessingResult, sourceURL string) {
	le.logger.Info().
		Str("source_url", sourceURL).
		Int("found", result.Found).
		Int("filtered", result.Filtered).
		Int("followed", result.Followed).
		Int("skipped_depth", result.Skipped).
		Msg(fmt.Sprintf("Links found: %d | filtered: %d | followed: %d",
			result.Found, result.Filtered, result.Followed))
}
