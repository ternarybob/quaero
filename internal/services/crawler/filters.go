package crawler

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/ternarybob/arbor"
)

// VERIFICATION COMMENT 3: Shared link filtering logic (DRY principle)
// Consolidates duplicate filtering from CrawlerJob.shouldEnqueueURL() and other components
// Single source of truth for include/exclude patterns and source-specific validation

// FilterResult contains filtering outcome and metadata
type FilterResult struct {
	ShouldEnqueue bool
	Reason        string
	ExcludedBy    string // Pattern that excluded the URL (if applicable)
}

// LinkFilter handles URL filtering with include/exclude patterns
type LinkFilter struct {
	includeRegexes []*regexp.Regexp
	excludeRegexes []*regexp.Regexp
	sourceType     string
	logger         arbor.ILogger
}

// NewLinkFilter creates a new link filter with compiled patterns
func NewLinkFilter(includePatterns, excludePatterns []string, sourceType string, logger arbor.ILogger) *LinkFilter {
	filter := &LinkFilter{
		sourceType:     sourceType,
		logger:         logger,
		includeRegexes: make([]*regexp.Regexp, 0, len(includePatterns)),
		excludeRegexes: make([]*regexp.Regexp, 0, len(excludePatterns)),
	}

	// Compile include patterns
	for _, pattern := range includePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			filter.includeRegexes = append(filter.includeRegexes, re)
		} else {
			logger.Warn().
				Err(err).
				Str("pattern", pattern).
				Msg("Failed to compile include pattern")
		}
	}

	// Compile exclude patterns
	for _, pattern := range excludePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			filter.excludeRegexes = append(filter.excludeRegexes, re)
		} else {
			logger.Warn().
				Err(err).
				Str("pattern", pattern).
				Msg("Failed to compile exclude pattern")
		}
	}

	return filter
}

// FilterURL applies all filtering rules to a URL
func (f *LinkFilter) FilterURL(url string) FilterResult {
	// Apply exclude patterns first (fastest rejection)
	if len(f.excludeRegexes) > 0 {
		for _, re := range f.excludeRegexes {
			if re.MatchString(url) {
				return FilterResult{
					ShouldEnqueue: false,
					Reason:        "matches exclude pattern",
					ExcludedBy:    re.String(),
				}
			}
		}
	}

	// Apply include patterns (if any specified)
	if len(f.includeRegexes) > 0 {
		matched := false
		for _, re := range f.includeRegexes {
			if re.MatchString(url) {
				matched = true
				break
			}
		}
		if !matched {
			return FilterResult{
				ShouldEnqueue: false,
				Reason:        "does not match include patterns",
				ExcludedBy:    "",
			}
		}
	}

	// Apply source-specific validation
	switch f.sourceType {
	case "jira":
		if !IsValidJiraURL(url) {
			return FilterResult{
				ShouldEnqueue: false,
				Reason:        "not a valid Jira URL",
				ExcludedBy:    "",
			}
		}
	case "confluence":
		if !IsValidConfluenceURL(url) {
			return FilterResult{
				ShouldEnqueue: false,
				Reason:        "not a valid Confluence URL",
				ExcludedBy:    "",
			}
		}
	}

	// All checks passed
	return FilterResult{
		ShouldEnqueue: true,
		Reason:        "",
		ExcludedBy:    "",
	}
}

// FilterLinks applies filtering to multiple URLs and returns filtered set with statistics
func (f *LinkFilter) FilterLinks(urls []string) (filtered []string, excluded []string, notIncluded []string) {
	filtered = make([]string, 0, len(urls))
	excluded = make([]string, 0)
	notIncluded = make([]string, 0)

	for _, url := range urls {
		result := f.FilterURL(url)
		if result.ShouldEnqueue {
			filtered = append(filtered, url)
		} else {
			// Categorize rejections
			if result.ExcludedBy != "" {
				excluded = append(excluded, url)
			} else if result.Reason == "does not match include patterns" {
				notIncluded = append(notIncluded, url)
			}
		}
	}

	return filtered, excluded, notIncluded
}

// IsValidJiraURL checks if a URL is a valid Jira URL
// Valid Jira URLs typically contain /browse/, /projects/, or /issues/ paths
func IsValidJiraURL(urlStr string) bool {
	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Check path for Jira-specific patterns
	path := strings.ToLower(parsedURL.Path)

	// Valid Jira paths
	jiraPatterns := []string{
		"/browse/",   // Issue view (e.g., /browse/PROJ-123)
		"/projects/", // Project pages
		"/issues/",   // Issues list
		"/secure/",   // Various Jira operations
		"/rest/api/", // API endpoints (though these should be filtered out)
	}

	for _, pattern := range jiraPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// IsValidConfluenceURL checks if a URL is a valid Confluence URL
// Valid Confluence URLs typically contain /spaces/, /pages/, or /display/ paths
func IsValidConfluenceURL(urlStr string) bool {
	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Check path for Confluence-specific patterns
	path := strings.ToLower(parsedURL.Path)

	// Valid Confluence paths
	confluencePatterns := []string{
		"/spaces/",   // Space overview
		"/pages/",    // Page view
		"/display/",  // Legacy page view
		"/wiki/",     // Wiki namespace
		"/rest/api/", // API endpoints (though these should be filtered out)
	}

	for _, pattern := range confluencePatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}
