package crawler

import (
	"encoding/json"
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

// ============================================================================
// CONVERSION HELPERS: CrawlJob ↔ models.Job
// ============================================================================
//
// These helpers convert between the legacy CrawlJob structure and the new
// executor-agnostic models.Job structure. The new Job model uses flexible
// Config and Metadata maps instead of fixed struct fields.
//
// CrawlJob → Job Mapping:
//   - CrawlJob.Config → Job.Config["crawl_config"]
//   - CrawlJob.SourceConfigSnapshot → Job.Config["source_config_snapshot"]
//   - CrawlJob.AuthSnapshot → Job.Config["auth_snapshot"]
//   - CrawlJob.RefreshSource → Job.Config["refresh_source"]
//   - CrawlJob.SeedURLs → Job.Config["seed_urls"]
//   - CrawlJob.SourceType → Job.Config["source_type"]
//   - CrawlJob.EntityType → Job.Config["entity_type"]
//   - CrawlJob.Metadata → Job.Metadata (direct copy)
//   - CrawlJob.Progress → Job.Progress (converted to JobProgress)
//   - CrawlJob.SeenURLs → NOT stored (in-memory only, reconstructed from DB)
//
// ============================================================================

// CrawlJobToJob converts a legacy CrawlJob to the new models.Job structure
func CrawlJobToJob(crawlJob *CrawlJob) *models.Job {
	// Build config map from CrawlJob fields
	config := make(map[string]interface{})
	config["crawl_config"] = crawlJob.Config
	config["source_config_snapshot"] = crawlJob.SourceConfigSnapshot
	config["auth_snapshot"] = crawlJob.AuthSnapshot
	config["refresh_source"] = crawlJob.RefreshSource
	config["seed_urls"] = crawlJob.SeedURLs
	config["source_type"] = crawlJob.SourceType
	config["entity_type"] = crawlJob.EntityType

	// Convert CrawlProgress to JobProgress
	var jobProgress *models.JobProgress
	if crawlJob.Progress.TotalURLs > 0 || crawlJob.Progress.CompletedURLs > 0 {
		jobProgress = &models.JobProgress{
			TotalURLs:     crawlJob.Progress.TotalURLs,
			CompletedURLs: crawlJob.Progress.CompletedURLs,
			FailedURLs:    crawlJob.Progress.FailedURLs,
			PendingURLs:   crawlJob.Progress.PendingURLs,
			CurrentURL:    crawlJob.Progress.CurrentURL,
			Percentage:    crawlJob.Progress.Percentage,
		}
	}

	// Handle ParentID pointer
	var parentID *string
	if crawlJob.ParentID != "" {
		parentID = &crawlJob.ParentID
	}

	// Convert timestamps to pointers
	var startedAt, completedAt, finishedAt, lastHeartbeat *time.Time
	if !crawlJob.StartedAt.IsZero() {
		startedAt = &crawlJob.StartedAt
	}
	if !crawlJob.CompletedAt.IsZero() {
		completedAt = &crawlJob.CompletedAt
	}
	if !crawlJob.FinishedAt.IsZero() {
		finishedAt = &crawlJob.FinishedAt
	}
	if !crawlJob.LastHeartbeat.IsZero() {
		lastHeartbeat = &crawlJob.LastHeartbeat
	}

	// Determine job type string from JobType enum
	jobType := string(crawlJob.JobType)
	if jobType == "" {
		jobType = "crawler" // Default fallback
	}

	// Create JobModel
	jobModel := &models.JobModel{
		ID:        crawlJob.ID,
		ParentID:  parentID,
		Type:      jobType,
		Name:      crawlJob.Name,
		Config:    config,
		Metadata:  crawlJob.Metadata,
		CreatedAt: crawlJob.CreatedAt,
		Depth:     0, // Not tracked in CrawlJob
	}

	// Create Job with runtime state
	job := &models.Job{
		JobModel:      jobModel,
		Status:        crawlJob.Status,
		Progress:      jobProgress,
		StartedAt:     startedAt,
		CompletedAt:   completedAt,
		FinishedAt:    finishedAt,
		LastHeartbeat: lastHeartbeat,
		Error:         crawlJob.Error,
		ResultCount:   crawlJob.ResultCount,
		FailedCount:   crawlJob.FailedCount,
	}

	return job
}

// JobToCrawlJob converts a models.Job back to a legacy CrawlJob structure
// This is used for backward compatibility with existing code that expects CrawlJob
func JobToCrawlJob(job *models.Job) (*CrawlJob, error) {
	// Extract config fields
	crawlConfig, err := extractCrawlConfig(job.Config)
	if err != nil {
		return nil, err
	}

	sourceConfigSnapshot, _ := job.Config["source_config_snapshot"].(string)
	authSnapshot, _ := job.Config["auth_snapshot"].(string)
	refreshSource, _ := job.Config["refresh_source"].(bool)
	sourceType, _ := job.Config["source_type"].(string)
	entityType, _ := job.Config["entity_type"].(string)

	// Extract seed URLs
	var seedURLs []string
	if seedURLsRaw, ok := job.Config["seed_urls"]; ok {
		if seedURLsSlice, ok := seedURLsRaw.([]interface{}); ok {
			for _, url := range seedURLsSlice {
				if urlStr, ok := url.(string); ok {
					seedURLs = append(seedURLs, urlStr)
				}
			}
		} else if seedURLsStrSlice, ok := seedURLsRaw.([]string); ok {
			seedURLs = seedURLsStrSlice
		}
	}

	// Convert JobProgress to CrawlProgress
	crawlProgress := CrawlProgress{}
	if job.Progress != nil {
		crawlProgress.TotalURLs = job.Progress.TotalURLs
		crawlProgress.CompletedURLs = job.Progress.CompletedURLs
		crawlProgress.FailedURLs = job.Progress.FailedURLs
		crawlProgress.PendingURLs = job.Progress.PendingURLs
		crawlProgress.CurrentURL = job.Progress.CurrentURL
		crawlProgress.Percentage = job.Progress.Percentage
	}

	// Handle ParentID pointer
	parentID := ""
	if job.ParentID != nil {
		parentID = *job.ParentID
	}

	// Convert timestamps from pointers
	var startedAt, completedAt, finishedAt, lastHeartbeat time.Time
	if job.StartedAt != nil {
		startedAt = *job.StartedAt
	}
	if job.CompletedAt != nil {
		completedAt = *job.CompletedAt
	}
	if job.FinishedAt != nil {
		finishedAt = *job.FinishedAt
	}
	if job.LastHeartbeat != nil {
		lastHeartbeat = *job.LastHeartbeat
	}

	// Parse JobType from string
	jobType := models.JobType(job.Type)

	crawlJob := &CrawlJob{
		ID:                   job.ID,
		ParentID:             parentID,
		JobType:              jobType,
		Name:                 job.Name,
		Description:          "", // Not stored in Job model
		SourceType:           sourceType,
		EntityType:           entityType,
		Config:               crawlConfig,
		SourceConfigSnapshot: sourceConfigSnapshot,
		AuthSnapshot:         authSnapshot,
		RefreshSource:        refreshSource,
		Status:               job.Status,
		Progress:             crawlProgress,
		CreatedAt:            job.CreatedAt,
		StartedAt:            startedAt,
		CompletedAt:          completedAt,
		FinishedAt:           finishedAt,
		LastHeartbeat:        lastHeartbeat,
		Error:                job.Error,
		ResultCount:          job.ResultCount,
		FailedCount:          job.FailedCount,
		DocumentsSaved:       0, // Not tracked in Job model
		SeedURLs:             seedURLs,
		SeenURLs:             nil, // In-memory only, not persisted
		Metadata:             job.Metadata,
	}

	return crawlJob, nil
}

// extractCrawlConfig extracts CrawlConfig from Job.Config map
func extractCrawlConfig(config map[string]interface{}) (CrawlConfig, error) {
	crawlConfigRaw, ok := config["crawl_config"]
	if !ok {
		return CrawlConfig{}, nil // Return empty config if not found
	}

	// Try direct type assertion first
	if crawlConfig, ok := crawlConfigRaw.(CrawlConfig); ok {
		return crawlConfig, nil
	}

	// Try map[string]interface{} conversion
	if crawlConfigMap, ok := crawlConfigRaw.(map[string]interface{}); ok {
		// Marshal to JSON and unmarshal to CrawlConfig
		jsonBytes, err := json.Marshal(crawlConfigMap)
		if err != nil {
			return CrawlConfig{}, err
		}
		var crawlConfig CrawlConfig
		if err := json.Unmarshal(jsonBytes, &crawlConfig); err != nil {
			return CrawlConfig{}, err
		}
		return crawlConfig, nil
	}

	return CrawlConfig{}, nil
}

// URLQueueItem was removed during queue refactoring (replaced by queue.JobMessage)
// The custom URLQueue with priority heap and deduplication has been replaced by
// goqite-backed queue manager with persistent storage and worker pool.
// See internal/queue/types.go for the new message types.

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
