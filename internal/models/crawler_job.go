package models

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
	// CompletionCandidateAt is the timestamp when job first became a completion candidate (PendingURLs == 0)
	// Used for grace period verification before marking complete. Reset to zero if new URLs enqueued during grace period.
	CompletionCandidateAt time.Time `json:"completion_candidate_at,omitempty"`
	Error                 string    `json:"error,omitempty"`
	// ResultCount is a snapshot of Progress.CompletedURLs at job completion
	// Synced when job reaches terminal status (completed/failed/cancelled)
	// Used for historical tracking and validation
	ResultCount int `json:"result_count"`
	// FailedCount is a snapshot of Progress.FailedURLs at job completion
	// Synced when job reaches terminal status (completed/failed/cancelled)
	// Used for historical tracking and validation
	FailedCount    int             `json:"failed_count"`
	DocumentsSaved int             `json:"documents_saved"`     // Number of documents successfully saved to storage
	SeedURLs       []string        `json:"seed_urls,omitempty"` // Initial URLs used to start the crawl (for rerun capability)
	SeenURLs       map[string]bool `json:"seen_urls,omitempty"` // Track URLs that have been enqueued to prevent duplicates
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
