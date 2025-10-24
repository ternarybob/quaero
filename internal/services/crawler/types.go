package crawler

import (
	"time"

	"github.com/ternarybob/quaero/internal/models"
)

// Type aliases for backward compatibility within crawler package
// All job-related types have been moved to internal/models to break import cycles
type (
	JobStatus     = models.JobStatus
	CrawlJob      = models.CrawlJob
	CrawlConfig   = models.CrawlConfig
	CrawlProgress = models.CrawlProgress
)

// Re-export constants from models
const (
	JobStatusPending   = models.JobStatusPending
	JobStatusRunning   = models.JobStatusRunning
	JobStatusCompleted = models.JobStatusCompleted
	JobStatusFailed    = models.JobStatusFailed
	JobStatusCancelled = models.JobStatusCancelled

	ContentTypeHTML      = models.ContentTypeHTML
	ContentTypeJSON      = models.ContentTypeJSON
	ContentTypeMarkdown  = models.ContentTypeMarkdown
	OutputFormatMarkdown = models.OutputFormatMarkdown
	OutputFormatHTML     = models.OutputFormatHTML
	OutputFormatBoth     = models.OutputFormatBoth
)

// Re-export helper functions from models
var (
	FromJSONCrawlConfig   = models.FromJSONCrawlConfig
	FromJSONCrawlProgress = models.FromJSONCrawlProgress
)

// VERIFICATION COMMENT 2: URLQueueItem removed - legacy type from old worker-based architecture
// The new queue system uses queue.JobMessage instead (see internal/queue/types.go)

// CrawlResult represents the result of crawling a single URL
type CrawlResult struct {
	URL        string                 `json:"url"`
	StatusCode int                    `json:"status_code"`
	Body       []byte                 `json:"body,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ScrapeResult represents Firecrawl-style HTML scraping results with markdown output
type ScrapeResult struct {
	URL         string                 `json:"url"`          // The scraped URL
	StatusCode  int                    `json:"status_code"`  // HTTP status code
	Success     bool                   `json:"success"`      // Whether scraping succeeded
	Markdown    string                 `json:"markdown"`     // Converted markdown content (primary output for LLM consumption)
	HTML        string                 `json:"html"`         // Cleaned HTML content (optional, based on config)
	RawHTML     string                 `json:"raw_html"`     // Original raw HTML (for debugging/archival)
	Title       string                 `json:"title"`        // Page title from <title> tag or Open Graph
	Description string                 `json:"description"`  // Meta description or Open Graph description
	Language    string                 `json:"language"`     // Page language (from <html lang> or meta tags)
	Links       []string               `json:"links"`        // Discovered links (absolute URLs) for crawling
	Metadata    map[string]interface{} `json:"metadata"`     // Extracted metadata (Open Graph, Twitter Cards, JSON-LD, etc.)
	TextContent string                 `json:"text_content"` // Plain text content (cleaned, for search indexing)
	Duration    time.Duration          `json:"duration"`     // Time taken to scrape
	Error       string                 `json:"error"`        // Error message if scraping failed
	Timestamp   time.Time              `json:"timestamp"`    // When the scrape was performed
}

// PageMetadata represents structured page metadata for type safety
type PageMetadata struct {
	Title        string                   `json:"title"`
	Description  string                   `json:"description"`
	Keywords     []string                 `json:"keywords"`
	Author       string                   `json:"author"`
	Language     string                   `json:"language"`
	CanonicalURL string                   `json:"canonical_url"`
	OpenGraph    map[string]string        `json:"open_graph"`   // og:title, og:description, og:image, etc.
	TwitterCard  map[string]string        `json:"twitter_card"` // twitter:title, twitter:description, etc.
	JSONLD       []map[string]interface{} `json:"json_ld"`      // Structured data from JSON-LD scripts
}

// ToCrawlResult converts ScrapeResult to CrawlResult for compatibility with existing code
// Body contains HTML/RawHTML for HTML parsers; markdown is available in metadata["markdown"]
func (s *ScrapeResult) ToCrawlResult() *CrawlResult {
	// Prefer HTML/RawHTML for Body to support HTML parsers
	// Markdown is stored in metadata for consumers that need it
	content := s.HTML
	if content == "" {
		content = s.RawHTML
	}

	// Create metadata map with additional fields
	metadata := make(map[string]interface{})
	if s.Metadata != nil {
		for k, v := range s.Metadata {
			metadata[k] = v
		}
	}
	metadata["title"] = s.Title
	metadata["description"] = s.Description
	metadata["language"] = s.Language
	metadata["links"] = s.Links
	metadata["markdown"] = s.Markdown
	metadata["html"] = s.HTML
	metadata["text_content"] = s.TextContent

	// Extract headers from metadata if available (Comment 6)
	// Handle both map[string]string and map[string][]string with type switches
	headers := make(map[string]string)
	if s.Metadata != nil {
		if headersRaw, exists := s.Metadata["headers"]; exists {
			switch h := headersRaw.(type) {
			case map[string]string:
				// Direct map[string]string
				headers = h
			case map[string][]string:
				// Normalize map[string][]string to map[string]string (take first value)
				for key, values := range h {
					if len(values) > 0 {
						headers[key] = values[0]
					}
				}
			default:
				// Fall back gracefully - headers remain empty
			}
		}
	}

	return &CrawlResult{
		URL:        s.URL,
		StatusCode: s.StatusCode,
		Body:       []byte(content),
		Headers:    headers,
		Duration:   s.Duration,
		Error:      s.Error,
		Metadata:   metadata,
	}
}

// GetContent returns content in priority order: Markdown > HTML > TextContent > RawHTML
func (s *ScrapeResult) GetContent() string {
	if s.Markdown != "" {
		return s.Markdown
	}
	if s.HTML != "" {
		return s.HTML
	}
	if s.TextContent != "" {
		return s.TextContent
	}
	return s.RawHTML
}
