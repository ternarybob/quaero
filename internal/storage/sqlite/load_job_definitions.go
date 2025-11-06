// -----------------------------------------------------------------------
// Load Job Definitions from Files - TOML/JSON crawler job definitions
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/quaero/internal/models"
)

// CrawlerJobDefinitionFile represents a simplified crawler job definition file format
// This is a user-friendly format for defining crawler jobs in TOML/JSON files
type CrawlerJobDefinitionFile struct {
	ID          string   `toml:"id" json:"id"`                     // Unique identifier
	Name        string   `toml:"name" json:"name"`                 // Human-readable name
	Description string   `toml:"description" json:"description"`   // Job description
	StartURLs   []string `toml:"start_urls" json:"start_urls"`     // Initial URLs to crawl
	Schedule    string   `toml:"schedule" json:"schedule"`         // Cron expression (empty = manual only)
	Timeout     string   `toml:"timeout" json:"timeout"`           // Duration string (e.g., "30m", "1h")
	Enabled     bool     `toml:"enabled" json:"enabled"`           // Whether job is enabled
	AutoStart   bool     `toml:"auto_start" json:"auto_start"`     // Whether to auto-start on scheduler init
	
	// Crawler configuration
	IncludePatterns []string `toml:"include_patterns" json:"include_patterns"` // URL patterns to include (regex)
	ExcludePatterns []string `toml:"exclude_patterns" json:"exclude_patterns"` // URL patterns to exclude (regex)
	MaxDepth        int      `toml:"max_depth" json:"max_depth"`               // Maximum crawl depth
	MaxPages        int      `toml:"max_pages" json:"max_pages"`               // Maximum pages to crawl
	Concurrency     int      `toml:"concurrency" json:"concurrency"`           // Number of concurrent workers
	FollowLinks     bool     `toml:"follow_links" json:"follow_links"`         // Whether to follow discovered links
}

// ToJobDefinition converts the simplified file format to a full JobDefinition model
func (c *CrawlerJobDefinitionFile) ToJobDefinition() *models.JobDefinition {
	return &models.JobDefinition{
		ID:          c.ID,
		Name:        c.Name,
		Type:        models.JobDefinitionTypeCrawler,
		Description: c.Description,
		Sources:     []string{}, // Crawler jobs don't use pre-configured sources
		Steps: []models.JobStep{
			{
				Name:   "crawl",
				Action: "crawl",
				Config: map[string]interface{}{
					"start_urls":       c.StartURLs,
					"include_patterns": c.IncludePatterns,
					"exclude_patterns": c.ExcludePatterns,
					"max_depth":        c.MaxDepth,
					"max_pages":        c.MaxPages,
					"concurrency":      c.Concurrency,
					"follow_links":     c.FollowLinks,
				},
				OnError: models.ErrorStrategyContinue,
			},
		},
		Schedule:  c.Schedule,
		Timeout:   c.Timeout,
		Enabled:   c.Enabled,
		AutoStart: c.AutoStart,
		Config:    map[string]interface{}{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Validate validates the crawler job definition file
func (c *CrawlerJobDefinitionFile) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("id is required")
	}
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(c.StartURLs) == 0 {
		return fmt.Errorf("start_urls must contain at least one URL")
	}
	if c.MaxDepth < 0 {
		return fmt.Errorf("max_depth must be >= 0")
	}
	if c.MaxPages < 0 {
		return fmt.Errorf("max_pages must be >= 0")
	}
	if c.Concurrency < 1 {
		return fmt.Errorf("concurrency must be >= 1")
	}
	
	// Validate timeout format if provided
	if c.Timeout != "" {
		if _, err := time.ParseDuration(c.Timeout); err != nil {
			return fmt.Errorf("invalid timeout duration '%s': %w", c.Timeout, err)
		}
	}
	
	return nil
}

// LoadJobDefinitionsFromFiles loads crawler job definitions from TOML/JSON files
// in the specified directory. This is called during startup to seed user-defined jobs.
func (m *Manager) LoadJobDefinitionsFromFiles(ctx context.Context, dirPath string) error {
	m.logger.Info().Str("path", dirPath).Msg("Loading job definitions from files")
	
	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		m.logger.Debug().Str("path", dirPath).Msg("Job definitions directory not found, skipping file loading")
		return nil // Not an error - directory is optional
	}
	
	// Read directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read job definitions directory: %w", err)
	}
	
	loadedCount := 0
	skippedCount := 0
	
	// Process each file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		filePath := filepath.Join(dirPath, entry.Name())
		ext := filepath.Ext(entry.Name())
		
		var crawlerJobFile *CrawlerJobDefinitionFile
		
		switch ext {
		case ".toml":
			crawlerJobFile, err = m.loadCrawlerJobFromTOML(filePath)
		case ".json":
			crawlerJobFile, err = m.loadCrawlerJobFromJSON(filePath)
		default:
			m.logger.Debug().Str("file", entry.Name()).Msg("Skipping non-TOML/JSON file")
			skippedCount++
			continue
		}
		
		if err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to load job definition file")
			skippedCount++
			continue
		}
		
		// Validate crawler job file
		if err := crawlerJobFile.Validate(); err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Invalid job definition in file")
			skippedCount++
			continue
		}
		
		// Convert to full JobDefinition model
		jobDef := crawlerJobFile.ToJobDefinition()
		
		// Validate full job definition
		if err := jobDef.Validate(); err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Job definition validation failed")
			skippedCount++
			continue
		}
		
		// Upsert job definition (idempotent - won't overwrite existing)
		if err := m.upsertJobDefinition(ctx, jobDef); err != nil {
			m.logger.Error().Err(err).Str("file", entry.Name()).Msg("Failed to upsert job definition")
			skippedCount++
			continue
		}
		
		m.logger.Info().
			Str("job_def_id", jobDef.ID).
			Str("file", entry.Name()).
			Msg("Loaded crawler job definition from file")
		
		loadedCount++
	}
	
	m.logger.Info().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Msg("Finished loading job definitions from files")
	
	return nil
}

// loadCrawlerJobFromTOML loads a crawler job definition from a TOML file
func (m *Manager) loadCrawlerJobFromTOML(filePath string) (*CrawlerJobDefinitionFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	var crawlerJob CrawlerJobDefinitionFile
	if err := toml.Unmarshal(data, &crawlerJob); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}
	
	return &crawlerJob, nil
}

// loadCrawlerJobFromJSON loads a crawler job definition from a JSON file
func (m *Manager) loadCrawlerJobFromJSON(filePath string) (*CrawlerJobDefinitionFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	var crawlerJob CrawlerJobDefinitionFile
	if err := json.Unmarshal(data, &crawlerJob); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	return &crawlerJob, nil
}

