package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces/jobtypes"
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

// JobType represents the type of a crawl job in the hierarchy
type JobType string

const (
	JobTypeParent         JobType = "parent"          // Parent job that spawns child jobs
	JobTypePreValidation  JobType = "pre_validation"  // Pre-flight validation job
	JobTypeCrawlerURL     JobType = "crawler_url"     // Individual URL crawling job
	JobTypePostSummary    JobType = "post_summary"    // Post-processing summarization job
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

// CrawlJob represents a crawl job inspired by Firecrawl's job model.
// Configuration is snapshot at job creation time for self-contained, re-runnable jobs.
//
// Job Types:
//   - parent: Orchestrator job that spawns child jobs
//   - pre_validation: Pre-flight validation before crawling
//   - crawler_url: Individual URL crawling job
//   - post_summary: Post-processing summarization job
//
// Parent-Child Hierarchy:
//   - Parent jobs have empty ParentID
//   - Child jobs reference their root parent via ParentID (flat hierarchy)
//   - All children of a parent share the same ParentID (not nested)
//   - See manager.go lines 395-416 for hierarchy design rationale
//
// Configuration Snapshots:
//   - Config: Crawl behavior (max_depth, concurrency, etc.)
//   - SourceConfigSnapshot: Source configuration at creation time
//   - AuthSnapshot: Authentication credentials at creation time
//   - RefreshSource: Whether to refresh config/auth before execution
//
// Snapshots enable:
//   - Re-running jobs with original configuration
//   - Auditing what configuration was used
//   - Isolating jobs from config changes
//
// Usage Example - Creating a Parent Job:
//   job := &CrawlJob{
//       ID:         uuid.New().String(),
//       ParentID:   "", // Empty for parent jobs
//       JobType:    JobTypeParent,
//       Name:       "Crawl Jira Issues",
//       SourceType: "jira",
//       EntityType: "issues",
//       Config: CrawlConfig{
//           MaxDepth:    3,
//           MaxPages:    100,
//           Concurrency: 4,
//           FollowLinks: true,
//       },
//       Status:    JobStatusPending,
//       SeedURLs:  []string{"https://jira.example.com/browse/PROJ-1"},
//   }
//   jobStorage.SaveJob(ctx, job)
//
// Usage Example - Creating a Child Job:
//   childJob := &CrawlJob{
//       ID:         uuid.New().String(),
//       ParentID:   "parent-job-id", // Reference root parent
//       JobType:    JobTypeCrawlerURL,
//       Name:       "URL: https://example.com/page1",
//       SourceType: "jira",
//       EntityType: "issues",
//       Config:     parentJob.Config, // Inherit parent config
//       Status:     JobStatusPending,
//       Progress: CrawlProgress{
//           TotalURLs:   1,
//           PendingURLs: 1,
//       },
//   }
//   jobStorage.SaveJob(ctx, childJob)
type CrawlJob struct {
	ID string `json:"id"`
	// ParentID is the parent job ID for child jobs (empty for parent jobs)
	// Child crawler_url jobs reference their root parent job ID
	// Used for hierarchical display in Queue Management UI
	ParentID             string        `json:"parent_id,omitempty"`
	JobType              JobType       `json:"job_type"`               // Type of job in hierarchy (parent, pre_validation, crawler_url, post_summary)
	Name                 string        `json:"name"`                   // User-friendly name for the job
	Description          string        `json:"description"`            // User-provided description
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
	LastHeartbeat        time.Time     `json:"last_heartbeat,omitempty"` // Timestamp of last URL processed (for idle detection)
	// Error contains a concise, user-friendly description of why the job failed.
	// Format: "Category: Brief description" (e.g., "HTTP 404: Not Found", "Timeout: No activity for 10m").
	// Only populated when job status is 'failed' or when individual operations fail.
	// This field is displayed in the UI and should be actionable for users.
	Error string `json:"error,omitempty"`
	// ResultCount is a snapshot of Progress.CompletedURLs at job completion
	// Synced when job reaches terminal status (completed/failed/cancelled)
	// Used for historical tracking and validation
	ResultCount int `json:"result_count"`
	// FailedCount is a snapshot of Progress.FailedURLs at job completion
	// Synced when job reaches terminal status (completed/failed/cancelled)
	// Used for historical tracking and validation
	FailedCount    int                    `json:"failed_count"`
	DocumentsSaved int                    `json:"documents_saved"`     // Number of documents successfully saved to storage
	SeedURLs       []string               `json:"seed_urls,omitempty"` // Initial URLs used to start the crawl (for rerun capability)
	// SeenURLs is an in-memory cache of URLs that have been enqueued to prevent duplicates.
	// NOTE: This field is NOT persisted to the database (omitempty tag).
	// Persistent URL deduplication is handled by the job_seen_urls table via JobStorage.MarkURLSeen().
	// This in-memory map is used for fast lookups during job execution to avoid database queries.
	// The map is populated from the database when the job is loaded.
	// See JobStorage.MarkURLSeen() for the authoritative deduplication mechanism.
	SeenURLs       map[string]bool        `json:"seen_urls,omitempty"`
	// Metadata stores custom key-value data for the job.
	// Common use cases:
	//   - corpus_summary: Generated summary of all documents in the job
	//   - corpus_keywords: Extracted keywords from all documents
	//   - custom_tags: User-defined tags for categorization
	//   - execution_context: Additional context for job execution
	// NOTE: This field is NOT indexed. Use for small amounts of metadata only.
	// For large data, store in separate tables and reference by job ID.
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
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

// ToJSON serializes CrawlConfig to JSON string for database storage
func (c *CrawlConfig) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSONCrawlConfig deserializes CrawlConfig from JSON string
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
func (j *CrawlJob) SetSourceConfigSnapshot(config *SourceConfig) error {
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
func (j *CrawlJob) GetSourceConfigSnapshot() (*SourceConfig, error) {
	if j.SourceConfigSnapshot == "" {
		return nil, nil
	}
	var config SourceConfig
	if err := json.Unmarshal([]byte(j.SourceConfigSnapshot), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SetAuthSnapshot marshals and stores auth credentials as JSON
func (j *CrawlJob) SetAuthSnapshot(auth *AuthCredentials) error {
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
func (j *CrawlJob) GetAuthSnapshot() (*AuthCredentials, error) {
	if j.AuthSnapshot == "" {
		return nil, nil
	}
	var auth AuthCredentials
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

// GetStatusReport returns a standardized status report for this job
// This method encapsulates status calculation logic and provides consistent reporting
// for both parent and child jobs. Accepts childStats which may be nil for jobs without children.
func (j *CrawlJob) GetStatusReport(childStats *jobtypes.JobChildStats) *jobtypes.JobStatusReport {
	report := &jobtypes.JobStatusReport{
		Status:   string(j.Status),
		Errors:   []string{},
		Warnings: []string{},
	}

	// Calculate child job statistics
	if childStats != nil {
		report.ChildCount = childStats.ChildCount
		report.CompletedChildren = childStats.CompletedChildren
		report.FailedChildren = childStats.FailedChildren
		report.RunningChildren = childStats.ChildCount - childStats.CompletedChildren - childStats.FailedChildren
		if report.RunningChildren < 0 {
			report.RunningChildren = 0
		}
	} else {
		report.ChildCount = 0
		report.CompletedChildren = 0
		report.FailedChildren = 0
		report.RunningChildren = 0
	}

	// Generate progress text based on job type and available data
	if j.ParentID == "" {
		// This is a parent job
		if report.ChildCount == 0 {
			report.ProgressText = "No child jobs spawned yet"
		} else {
			report.ProgressText = fmt.Sprintf("Completed: %d | Failed: %d | Running: %d | Total: %d",
				report.CompletedChildren,
				report.FailedChildren,
				report.RunningChildren,
				report.ChildCount)
		}
	} else {
		// This is a child job - use job's own progress
		if j.Progress.TotalURLs > 0 {
			report.ProgressText = fmt.Sprintf("%d URLs (%d completed, %d failed, %d running)",
				j.Progress.TotalURLs,
				j.Progress.CompletedURLs,
				j.Progress.FailedURLs,
				j.Progress.PendingURLs)
		} else {
			report.ProgressText = fmt.Sprintf("Status: %s", j.Status)
		}
	}

	// Extract errors from job.Error field
	if j.Error != "" {
		report.Errors = append(report.Errors, j.Error)
	}

	return report
}
