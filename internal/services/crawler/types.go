package crawler

import (
	"encoding/json"
	"time"
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

// CrawlJob represents a crawl job inspired by Firecrawl's job model
// Configuration is snapshot at job creation time for self-contained, re-runnable jobs
type CrawlJob struct {
	ID          string        `json:"id"`
	SourceType  string        `json:"source_type"` // "jira", "confluence", "github"
	EntityType  string        `json:"entity_type"` // "projects", "issues", "spaces", "pages"
	Config      CrawlConfig   `json:"config"`      // Snapshot of configuration at job creation time
	Status      JobStatus     `json:"status"`
	Progress    CrawlProgress `json:"progress"`
	CreatedAt   time.Time     `json:"created_at"`
	StartedAt   time.Time     `json:"started_at,omitempty"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
	Error       string        `json:"error,omitempty"`
	ResultCount int           `json:"result_count"`
	FailedCount int           `json:"failed_count"`
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
