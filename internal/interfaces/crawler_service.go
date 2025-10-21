package interfaces

import (
	"context"
)

// CrawlerService defines the interface for the unified crawler service
// that handles crawling operations for all data sources (Jira, Confluence, GitHub)
// Note: Most methods use interface{} for types to avoid import cycles
type CrawlerService interface {
	// Start initializes the crawler service
	Start() error

	// StartCrawl creates and starts a new crawl job
	// sourceType: "jira", "confluence", "github"
	// entityType: "projects", "issues", "spaces", "pages", etc.
	// seedURLs: Initial URLs to begin crawling
	// config: Crawl configuration (concurrency, rate limits, filters, etc.)
	// sourceID: Optional source ID to load configuration from
	// refreshSource: If true, re-fetch latest source config and auth
	// sourceConfigSnapshot: Optional point-in-time source configuration
	// authSnapshot: Optional point-in-time authentication snapshot
	// Returns: jobID for tracking the crawl
	StartCrawl(sourceType, entityType string, seedURLs []string, config interface{}, sourceID string, refreshSource bool, sourceConfigSnapshot interface{}, authSnapshot interface{}) (string, error)

	// GetJobStatus retrieves the current status of a crawl job
	// Returns: *crawler.CrawlJob - caller must perform type assertion with proper error handling
	// Example: job, ok := result.(*crawler.CrawlJob); if !ok { handle error }
	GetJobStatus(jobID string) (interface{}, error)

	// CancelJob cancels a running crawl job
	CancelJob(jobID string) error

	// GetJobResults retrieves the results of a completed crawl job
	GetJobResults(jobID string) (interface{}, error)

	// ListJobs returns a list of crawl jobs with optional filtering
	ListJobs(ctx context.Context, opts *ListOptions) (interface{}, error)

	// RerunJob re-executes a previous job with the same or updated configuration
	// updateConfig: If nil, uses original job configuration
	RerunJob(ctx context.Context, jobID string, updateConfig interface{}) (string, error)

	// WaitForJob blocks until a job completes or context is cancelled
	WaitForJob(ctx context.Context, jobID string) (interface{}, error)

	// Close cleanly shuts down the crawler service
	Close() error
}
