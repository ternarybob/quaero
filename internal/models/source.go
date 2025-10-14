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
	MaxDepth        int      `json:"max_depth"`
	FollowLinks     bool     `json:"follow_links"`
	Concurrency     int      `json:"concurrency"`
	RateLimit       int      `json:"rate_limit"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	MaxPages        int      `json:"max_pages"`
}

// SourceConfig represents a data source configuration
type SourceConfig struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	BaseURL     string                 `json:"base_url"`
	Enabled     bool                   `json:"enabled"`
	AuthDomain  string                 `json:"auth_domain"`
	CrawlConfig CrawlConfig            `json:"crawl_config"`
	Filters     map[string]interface{} `json:"filters"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
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

	return nil
}
