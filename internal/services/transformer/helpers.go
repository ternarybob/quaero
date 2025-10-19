package transformer

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// generateDocumentID generates a unique document ID
func generateDocumentID() string {
	return "doc_" + uuid.New().String()
}

// extractSourceID attempts to find an ID in the JSON response or URL
func extractSourceID(data map[string]interface{}, urlStr string) string {
	// Common ID field names to search for
	idFields := []string{"id", "key", "uuid", "identifier", "number", "issueId", "pageId"}

	if data != nil {
		// Try common ID fields at root level
		for _, field := range idFields {
			if val, ok := data[field]; ok {
				// Handle string IDs
				if str, ok := val.(string); ok && str != "" {
					return str
				}
				// Handle numeric IDs (int, float)
				if num, ok := val.(float64); ok {
					return fmt.Sprintf("%.0f", num)
				}
				if num, ok := val.(int); ok {
					return fmt.Sprintf("%d", num)
				}
				if num, ok := val.(int64); ok {
					return fmt.Sprintf("%d", num)
				}
			}
		}

		// Search nested maps for ID fields (one level deep)
		for _, value := range data {
			if nested, ok := value.(map[string]interface{}); ok {
				for _, field := range idFields {
					if val, ok := nested[field]; ok {
						// Handle string IDs
						if str, ok := val.(string); ok && str != "" {
							return str
						}
						// Handle numeric IDs
						if num, ok := val.(float64); ok {
							return fmt.Sprintf("%.0f", num)
						}
						if num, ok := val.(int); ok {
							return fmt.Sprintf("%d", num)
						}
					}
				}
			}
		}
	}

	// Fallback to stable hash of full absolute URL (host + path + query)
	// This prevents collisions from generic URL segments like "index.html"
	if urlStr != "" {
		// Parse URL to get host, path, and query
		parsedURL, err := url.Parse(urlStr)
		if err == nil && parsedURL.Host != "" {
			// Construct normalized URL: scheme://host/path?query
			normalizedURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)
			if parsedURL.RawQuery != "" {
				normalizedURL += "?" + parsedURL.RawQuery
			}
			return fmt.Sprintf("url_%x", hashString(normalizedURL))
		}
		// Fallback to hashing the raw URL string if parsing failed
		return fmt.Sprintf("url_%x", hashString(urlStr))
	}

	// Final fallback for empty URL
	return "url_unknown"
}

// constructURL builds the full URL from base URL and response data
func constructURL(sourceConfig *models.SourceConfig, requestURL string, data map[string]interface{}) string {
	// Check for URL in response data
	urlFields := []string{"url", "self", "web_url", "webui", "link", "href"}
	for _, field := range urlFields {
		if val, ok := data[field]; ok {
			if str, ok := val.(string); ok && str != "" {
				// If absolute URL, use it
				if strings.HasPrefix(str, "http") {
					return str
				}
				// If relative URL and we have base URL, combine them
				if sourceConfig != nil && sourceConfig.BaseURL != "" {
					return joinURL(sourceConfig.BaseURL, str)
				}
			}
		}
	}

	// Check nested _links field (common in REST APIs)
	if links, ok := data["_links"].(map[string]interface{}); ok {
		for _, field := range urlFields {
			if val, ok := links[field]; ok {
				if str, ok := val.(string); ok && str != "" {
					if strings.HasPrefix(str, "http") {
						return str
					}
					if sourceConfig != nil && sourceConfig.BaseURL != "" {
						return joinURL(sourceConfig.BaseURL, str)
					}
				}
			}
		}
	}

	// Fallback to request URL
	if requestURL != "" {
		if strings.HasPrefix(requestURL, "http") {
			return requestURL
		}
		if sourceConfig != nil && sourceConfig.BaseURL != "" {
			return joinURL(sourceConfig.BaseURL, requestURL)
		}
	}

	return requestURL
}

// joinURL safely joins base URL and path
func joinURL(base, path string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return base + path
	}
	pathURL, err := url.Parse(path)
	if err != nil {
		return base + path
	}
	return baseURL.ResolveReference(pathURL).String()
}

// stripHTML removes HTML tags from text
func stripHTML(content string) string {
	if content == "" {
		return ""
	}

	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	cleaned := re.ReplaceAllString(content, " ")

	return cleaned
}

// extractHTMLTitle extracts <title> tag from HTML content
func extractHTMLTitle(html string) string {
	// Match <title>...</title>
	re := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// deriveTitleFromURL creates a title from the URL path
func deriveTitleFromURL(urlStr string) string {
	if urlStr == "" {
		return "Untitled Document"
	}

	// Extract the last segment of the path
	parts := strings.Split(strings.TrimRight(urlStr, "/"), "/")
	if len(parts) > 0 {
		lastSegment := parts[len(parts)-1]
		// Remove query parameters
		if idx := strings.Index(lastSegment, "?"); idx != -1 {
			lastSegment = lastSegment[:idx]
		}
		// Clean up and capitalize
		title := strings.ReplaceAll(lastSegment, "-", " ")
		title = strings.ReplaceAll(title, "_", " ")
		if title != "" {
			return strings.Title(title)
		}
	}

	return "Untitled Document"
}

// extractAllText recursively extracts all text content from JSON
func extractAllText(data interface{}) string {
	var textParts []string

	switch v := data.(type) {
	case string:
		if v != "" {
			textParts = append(textParts, v)
		}
	case map[string]interface{}:
		for _, value := range v {
			text := extractAllText(value)
			if text != "" {
				textParts = append(textParts, text)
			}
		}
	case []interface{}:
		for _, item := range v {
			text := extractAllText(item)
			if text != "" {
				textParts = append(textParts, text)
			}
		}
	}

	return strings.Join(textParts, " ")
}

// cleanText normalizes whitespace and removes excessive newlines
func cleanText(text string) string {
	if text == "" {
		return ""
	}

	// Replace multiple spaces with single space
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Replace excessive newlines (3+) with double newline
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	// Trim whitespace
	text = strings.TrimSpace(text)

	return text
}

// hashString creates a simple hash of a string (for fallback IDs)
func hashString(s string) uint32 {
	var hash uint32
	for _, c := range s {
		hash = hash*31 + uint32(c)
	}
	return hash
}

// getJobResults retrieves results from a completed job via the crawler service
func getJobResults(job *crawler.CrawlJob, crawlerService interfaces.CrawlerService, logger arbor.ILogger) ([]*crawler.CrawlResult, error) {
	// Get results from crawler service (in-memory storage)
	// NOTE: Results are lost after service restart - future enhancement could persist to DB
	results, err := crawlerService.GetJobResults(job.ID)
	if err != nil {
		logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to get job results from crawler service")
		return nil, err
	}

	// Type assert the interface{} result to []*crawler.CrawlResult
	crawlResults, ok := results.([]*crawler.CrawlResult)
	if !ok {
		logger.Error().Str("job_id", job.ID).Msg("Unexpected result type from GetJobResults")
		return nil, fmt.Errorf("unexpected result type from GetJobResults: expected []*crawler.CrawlResult")
	}

	return crawlResults, nil
}

// flattenJSON converts nested JSON to dot-notation map (utility for future use)
func flattenJSON(data map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			// Recursively flatten nested objects
			nested := flattenJSON(v, fullKey)
			for k, val := range nested {
				result[k] = val
			}
		case []interface{}:
			// Store arrays as-is
			result[fullKey] = v
		default:
			// Store primitive values
			result[fullKey] = v
		}
	}

	return result
}

// aggregateErrors combines multiple errors into a single error message
func aggregateErrors(errors []error) error {
	if len(errors) == 0 {
		return nil
	}

	var messages []string
	for _, err := range errors {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}

	if len(messages) == 0 {
		return nil
	}

	return fmt.Errorf("multiple errors: %s", strings.Join(messages, "; "))
}

// extractBodyFromMetadata safely extracts []byte body from result.Metadata["response_body"]
// Returns the body and a boolean indicating if extraction was successful
func extractBodyFromMetadata(metadata map[string]interface{}) ([]byte, bool) {
	if metadata == nil {
		return nil, false
	}

	bodyRaw, ok := metadata["response_body"]
	if !ok {
		return nil, false
	}

	// Try []byte first (most common case)
	if body, ok := bodyRaw.([]byte); ok {
		return body, true
	}

	// Fallback to string
	if bodyStr, ok := bodyRaw.(string); ok {
		return []byte(bodyStr), true
	}

	return nil, false
}
