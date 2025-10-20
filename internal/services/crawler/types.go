package crawler

import (
	"encoding/json"
	"time"

	"github.com/ternarybob/quaero/internal/models"
)

// JobStatus represents the state of a crawl job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Content type and output format constants for Firecrawl-style scraping
const (
	ContentTypeHTML      = "text/html"
	ContentTypeJSON      = "application/json"
	ContentTypeMarkdown  = "text/markdown"
	OutputFormatMarkdown = "markdown" // Firecrawl's primary format
	OutputFormatHTML     = "html"
	OutputFormatBoth     = "both"
)

// CrawlJob represents a crawl job inspired by Firecrawl's job model
// Configuration is snapshot at job creation time for self-contained, re-runnable jobs
type CrawlJob struct {
	ID                   string        `json:"id"`
	SourceType           string        `json:"source_type"`            // "jira", "confluence", "github"
	EntityType           string        `json:"entity_type"`            // "projects", "issues", "spaces", "pages"
	Config               CrawlConfig   `json:"config"`                 // Snapshot of configuration at job creation time
	SourceConfigSnapshot string        `json:"source_config_snapshot"` // JSON snapshot of models.SourceConfig at creation
	AuthSnapshot         string        `json:"auth_snapshot"`          // JSON snapshot of models.AuthCredentials at creation
	RefreshSource        bool          `json:"refresh_source"`         // Whether to refresh config/auth before execution
	Status               JobStatus     `json:"status"`
	Progress             CrawlProgress `json:"progress"`
	CreatedAt            time.Time     `json:"created_at"`
	StartedAt            time.Time     `json:"started_at,omitempty"`
	CompletedAt          time.Time     `json:"completed_at,omitempty"`
	Error                string        `json:"error,omitempty"`
	ResultCount          int           `json:"result_count"`
	FailedCount          int           `json:"failed_count"`
	SeedURLs             []string      `json:"seed_urls,omitempty"` // Initial URLs used to start the crawl (for rerun capability)
}

// CrawlConfig defines crawl behavior
type CrawlConfig struct {
	MaxDepth        int           `json:"max_depth"`        // Maximum depth for recursive crawling
	MaxPages        int           `json:"max_pages"`        // Maximum number of pages to crawl
	Concurrency     int           `json:"concurrency"`      // Number of concurrent workers
	RateLimit       time.Duration `json:"rate_limit"`       // Minimum delay between requests per domain
	RetryAttempts   int           `json:"retry_attempts"`   // Maximum retry attempts for failed requests
	RetryBackoff    time.Duration `json:"retry_backoff"`    // Initial backoff duration for retries
	IncludePatterns []string      `json:"include_patterns"` // URL patterns to include (regex)
	ExcludePatterns []string      `json:"exclude_patterns"` // URL patterns to exclude (regex)
	FollowLinks     bool          `json:"follow_links"`     // Whether to follow discovered links
	DetailLevel     string        `json:"detail_level"`     // "metadata" or "full" for Firecrawl-style layered crawling
}

// CrawlProgress tracks crawl job progress
type CrawlProgress struct {
	TotalURLs           int       `json:"total_urls"`
	CompletedURLs       int       `json:"completed_urls"`
	FailedURLs          int       `json:"failed_urls"`
	PendingURLs         int       `json:"pending_urls"`
	CurrentURL          string    `json:"current_url,omitempty"`
	Percentage          float64   `json:"percentage"`
	StartTime           time.Time `json:"start_time"`
	EstimatedCompletion time.Time `json:"estimated_completion,omitempty"`
}

// URLQueueItem represents a URL in the crawl queue
type URLQueueItem struct {
	URL       string                 `json:"url"`
	Depth     int                    `json:"depth"`
	ParentURL string                 `json:"parent_url,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Attempts  int                    `json:"attempts"`
	Priority  int                    `json:"priority"` // Lower number = higher priority
	AddedAt   time.Time              `json:"added_at"`
}

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

// ToJSON serializes CrawlConfig to JSON string for database storage
func (c *CrawlConfig) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON deserializes CrawlConfig from JSON string
func FromJSONCrawlConfig(data string) (*CrawlConfig, error) {
	var config CrawlConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// ToJSON serializes CrawlProgress to JSON string for database storage
func (p *CrawlProgress) ToJSON() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSONCrawlProgress deserializes CrawlProgress from JSON string
func FromJSONCrawlProgress(data string) (*CrawlProgress, error) {
	var progress CrawlProgress
	if err := json.Unmarshal([]byte(data), &progress); err != nil {
		return nil, err
	}
	return &progress, nil
}

// SetSourceConfigSnapshot marshals and stores source config as JSON
func (j *CrawlJob) SetSourceConfigSnapshot(config *models.SourceConfig) error {
	if config == nil {
		j.SourceConfigSnapshot = ""
		return nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	j.SourceConfigSnapshot = string(data)
	return nil
}

// GetSourceConfigSnapshot unmarshals source config from JSON
func (j *CrawlJob) GetSourceConfigSnapshot() (*models.SourceConfig, error) {
	if j.SourceConfigSnapshot == "" {
		return nil, nil
	}
	var config models.SourceConfig
	if err := json.Unmarshal([]byte(j.SourceConfigSnapshot), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SetAuthSnapshot marshals and stores auth credentials as JSON
func (j *CrawlJob) SetAuthSnapshot(auth *models.AuthCredentials) error {
	if auth == nil {
		j.AuthSnapshot = ""
		return nil
	}
	data, err := json.Marshal(auth)
	if err != nil {
		return err
	}
	j.AuthSnapshot = string(data)
	return nil
}

// GetAuthSnapshot unmarshals auth credentials from JSON
func (j *CrawlJob) GetAuthSnapshot() (*models.AuthCredentials, error) {
	if j.AuthSnapshot == "" {
		return nil, nil
	}
	var auth models.AuthCredentials
	if err := json.Unmarshal([]byte(j.AuthSnapshot), &auth); err != nil {
		return nil, err
	}
	return &auth, nil
}

// MaskSensitiveData returns a copy of the job with sensitive snapshot data masked
// This should be used before returning jobs in API responses to prevent credential leakage
func (j *CrawlJob) MaskSensitiveData() *CrawlJob {
	masked := *j // Shallow copy

	// Mask auth snapshot if present
	if masked.AuthSnapshot != "" {
		masked.AuthSnapshot = "[REDACTED]"
	}

	// Mask sensitive fields in source config snapshot
	if masked.SourceConfigSnapshot != "" {
		var configData map[string]interface{}
		if err := json.Unmarshal([]byte(masked.SourceConfigSnapshot), &configData); err == nil {
			// Mask AuthID field
			if _, exists := configData["auth_id"]; exists {
				configData["auth_id"] = "[REDACTED]"
			}

			// Mask sensitive keys in Filters map
			if filters, ok := configData["filters"].(map[string]interface{}); ok {
				maskedFilters := maskSensitiveKeys(filters)
				configData["filters"] = maskedFilters
			}

			// Re-marshal the masked config
			if maskedJSON, err := json.Marshal(configData); err == nil {
				masked.SourceConfigSnapshot = string(maskedJSON)
			}
		}
	}

	return &masked
}

// maskSensitiveKeys recursively masks sensitive keys in a map
// Sensitive keys include: api_key, token, secret, password, credential, auth, bearer, key, etc.
func maskSensitiveKeys(data map[string]interface{}) map[string]interface{} {
	sensitivePatterns := []string{
		"api_key", "apikey", "api-key",
		"token", "bearer",
		"secret", "password", "pwd", "pass",
		"credential", "cred",
		"auth", "authorization",
		"key", "private", "public",
	}

	masked := make(map[string]interface{})
	for k, v := range data {
		// Check if key contains any sensitive pattern
		isSensitive := false
		for _, pattern := range sensitivePatterns {
			if containsCaseInsensitive(k, pattern) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			masked[k] = "[REDACTED]"
		} else if nestedMap, ok := v.(map[string]interface{}); ok {
			// Recursively mask nested maps
			masked[k] = maskSensitiveKeys(nestedMap)
		} else {
			masked[k] = v
		}
	}
	return masked
}

// containsCaseInsensitive checks if a string contains a substring (case-insensitive)
func containsCaseInsensitive(s, substr string) bool {
	// Simple case-insensitive substring check
	sLen := len(s)
	subLen := len(substr)

	if subLen > sLen {
		return false
	}

	// Convert both to lowercase and check if substring exists
	sLower := make([]byte, sLen)
	subLower := make([]byte, subLen)

	for i := 0; i < sLen; i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		sLower[i] = c
	}

	for i := 0; i < subLen; i++ {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		subLower[i] = c
	}

	// Search for substring
	for i := 0; i <= sLen-subLen; i++ {
		match := true
		for j := 0; j < subLen; j++ {
			if sLower[i+j] != subLower[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}

	return false
}
