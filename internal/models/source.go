package models

import (
	"fmt"
	"strings"
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
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	BaseURL     string      `json:"base_url"`
	SeedURLs    []string    `json:"seed_urls,omitempty"` // Optional seed URLs for crawling (one per line). Leave empty for auto-discovery from base URL.
	Enabled     bool        `json:"enabled"`
	AuthID      string      `json:"auth_id"`     // Reference to auth_credentials.id
	AuthDomain  string      `json:"auth_domain"` // Deprecated: kept for backward compatibility
	CrawlConfig CrawlConfig `json:"crawl_config"`
	// Filters contains URL pattern filtering criteria for link crawling.
	// Supported filter keys:
	//   - include_patterns: ["browse", "project"] - only follow links containing these patterns
	//   - exclude_patterns: ["logout", "admin"] - ignore links containing these patterns
	// During crawling: load page → scan for links → filter by patterns → follow filtered links
	// Empty map means no filtering (follow all links within max_depth).
	// Examples: {"include_patterns": ["browse"]} for Jira project pages
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

	// Validate filters structure
	if err := s.validateFilters(); err != nil {
		return err
	}

	return nil
}

// validateFilters performs basic filter structure validation
func (s *SourceConfig) validateFilters() error {
	if s.Filters == nil {
		s.Filters = make(map[string]interface{}) // Initialize empty filters
		return nil
	}

	// Validate URL pattern filters (generic for all source types)
	return s.validateURLPatternFilters()
}

// validateURLPatternFilters validates generic URL pattern filters
func (s *SourceConfig) validateURLPatternFilters() error {
	for key, value := range s.Filters {
		switch key {
		case "include_patterns":
			if err := s.validateArrayOrStringFilter(value, "include patterns"); err != nil {
				return err
			}
		case "exclude_patterns":
			if err := s.validateArrayOrStringFilter(value, "exclude patterns"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported filter key: %s (supported: include_patterns, exclude_patterns)", key)
		}
	}
	return nil
}

// validateStringFilter validates that a filter value is a non-empty string
func (s *SourceConfig) validateStringFilter(value interface{}, filterName string) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s filter must be a string", filterName)
	}
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("%s filter cannot be empty", filterName)
	}
	return nil
}

// validateArrayOrStringFilter validates that a filter value is either a string or array of strings
func (s *SourceConfig) validateArrayOrStringFilter(value interface{}, filterName string) error {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s filter cannot be empty string", filterName)
		}
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("%s filter cannot be empty array", filterName)
		}
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("%s filter item %d must be a string", filterName, i)
			}
			if strings.TrimSpace(str) == "" {
				return fmt.Errorf("%s filter item %d cannot be empty", filterName, i)
			}
		}
	case []string:
		if len(v) == 0 {
			return fmt.Errorf("%s filter cannot be empty array", filterName)
		}
		for i, str := range v {
			if strings.TrimSpace(str) == "" {
				return fmt.Errorf("%s filter item %d cannot be empty", filterName, i)
			}
		}
	default:
		return fmt.Errorf("%s filter must be a string or array of strings", filterName)
	}
	return nil
}
