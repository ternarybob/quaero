package models

import (
	"fmt"
	"time"
)

// SourceType constants
const (
	SourceTypeJira       = "jira"
	SourceTypeConfluence = "confluence"
	SourceTypeGithub     = "github"
)

// CrawlConfig contains configuration for crawler behavior
type CrawlConfig struct {
	MaxDepth    int  `json:"max_depth"`
	FollowLinks bool `json:"follow_links"`
	Concurrency int  `json:"concurrency"`
	RateLimit   int  `json:"rate_limit"`
	MaxPages    int  `json:"max_pages"`
}

// SourceConfig represents a data source configuration
// Sources define connection details only: base URL, authentication, and source type.
// Crawling behavior (start URLs, filtering, depth) is specified at the job level.
type SourceConfig struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	BaseURL     string      `json:"base_url"`
	Enabled     bool        `json:"enabled"`
	AuthID      string      `json:"auth_id"` // Reference to auth_credentials.id
	CrawlConfig CrawlConfig `json:"crawl_config"`
	// Filters contains include_patterns and exclude_patterns as comma-delimited strings
	// Example: {"include_patterns": "browse,projects", "exclude_patterns": "admin,logout"}
	Filters   map[string]interface{} `json:"filters"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Validate validates the source configuration
func (s *SourceConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("source name is required")
	}

	if s.Type == "" {
		return fmt.Errorf("source type is required")
	}

	// Validate source type
	validTypes := map[string]bool{
		SourceTypeJira:       true,
		SourceTypeConfluence: true,
		SourceTypeGithub:     true,
	}
	if !validTypes[s.Type] {
		return fmt.Errorf("invalid source type: %s", s.Type)
	}

	if s.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	// Validate crawl config
	if s.CrawlConfig.MaxDepth < 0 {
		return fmt.Errorf("max depth must be non-negative")
	}

	if s.CrawlConfig.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}

	if s.CrawlConfig.MaxPages < 0 {
		return fmt.Errorf("max pages must be non-negative")
	}

	// Validate filters
	if err := s.validateFilters(); err != nil {
		return err
	}

	return nil
}

// validateFilters validates the Filters map structure
func (s *SourceConfig) validateFilters() error {
	if s.Filters == nil {
		return nil // Filters are optional
	}

	// Check include_patterns if present
	if val, ok := s.Filters["include_patterns"]; ok && val != nil {
		if _, isString := val.(string); !isString {
			return fmt.Errorf("include_patterns must be a string")
		}
	}

	// Check exclude_patterns if present
	if val, ok := s.Filters["exclude_patterns"]; ok && val != nil {
		if _, isString := val.(string); !isString {
			return fmt.Errorf("exclude_patterns must be a string")
		}
	}

	return nil
}
