// -----------------------------------------------------------------------
// Package validation provides TOML validation services for job definitions
// -----------------------------------------------------------------------

package validation

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// TOMLValidationService provides TOML validation functionality
type TOMLValidationService struct {
	logger arbor.ILogger
}

// ValidationResult contains the result of TOML validation
type ValidationResult struct {
	Valid   bool      `json:"valid"`
	Error   string    `json:"error,omitempty"`
	Message string    `json:"message"`
	JobDef  *models.JobDefinition `json:"job_definition,omitempty"`
}

// NewTOMLValidationService creates a new TOML validation service
func NewTOMLValidationService(logger arbor.ILogger) *TOMLValidationService {
	return &TOMLValidationService{
		logger: logger,
	}
}

// CrawlerJobDefinitionFile represents the simplified crawler job file format
// This is duplicated here to avoid circular dependency with storage/sqlite
type CrawlerJobDefinitionFile struct {
	ID             string   `toml:"id" json:"id"`
	Name           string   `toml:"name" json:"name"`
	JobType        string   `toml:"job_type" json:"job_type"`
	Description    string   `toml:"description" json:"description"`
	StartURLs      []string `toml:"start_urls" json:"start_urls"`
	Schedule       string   `toml:"schedule" json:"schedule"`
	Timeout        string   `toml:"timeout" json:"timeout"`
	Enabled        bool     `toml:"enabled" json:"enabled"`
	AutoStart      bool     `toml:"auto_start" json:"auto_start"`
	Authentication string   `toml:"authentication" json:"authentication"`
	IncludePatterns []string `toml:"include_patterns" json:"include_patterns"`
	ExcludePatterns []string `toml:"exclude_patterns" json:"exclude_patterns"`
	MaxDepth        int      `toml:"max_depth" json:"max_depth"`
	MaxPages        int      `toml:"max_pages" json:"max_pages"`
	Concurrency     int      `toml:"concurrency" json:"concurrency"`
	FollowLinks     bool     `toml:"follow_links" json:"follow_links"`
}

// ValidateTOML validates TOML content and attempts to parse it as a crawler job definition
func (s *TOMLValidationService) ValidateTOML(ctx context.Context, tomlContent string) ValidationResult {
	// Step 1: Parse TOML syntax
	var rawConfig map[string]interface{}
	if err := toml.Unmarshal([]byte(tomlContent), &rawConfig); err != nil {
		return ValidationResult{
			Valid:   false,
			Error:   err.Error(),
			Message: fmt.Sprintf("TOML syntax error: %v", err),
		}
	}

	// Step 2: Try to parse as CrawlerJobDefinitionFile (simplified format)
	var crawlerJob CrawlerJobDefinitionFile
	if err := toml.Unmarshal([]byte(tomlContent), &crawlerJob); err != nil {
		return ValidationResult{
			Valid:   false,
			Error:   err.Error(),
			Message: fmt.Sprintf("Failed to parse crawler job: %v", err),
		}
	}

	// Step 3: Validate basic crawler job fields
	if crawlerJob.ID == "" {
		return ValidationResult{
			Valid:   false,
			Error:   "id field is required",
			Message: "Job definition validation failed: id field is required",
		}
	}
	if crawlerJob.Name == "" {
		return ValidationResult{
			Valid:   false,
			Error:   "name field is required",
			Message: "Job definition validation failed: name field is required",
		}
	}
	if len(crawlerJob.StartURLs) == 0 {
		return ValidationResult{
			Valid:   false,
			Error:   "start_urls must contain at least one URL",
			Message: "Job definition validation failed: start_urls must contain at least one URL",
		}
	}

	// Step 4: Convert to full JobDefinition for complete validation
	jobDef := s.crawlerJobToJobDefinition(&crawlerJob)

	// Step 5: Validate complete job definition business rules
	if err := jobDef.Validate(); err != nil {
		return ValidationResult{
			Valid:   false,
			Error:   err.Error(),
			Message: fmt.Sprintf("Job definition validation failed: %v", err),
			JobDef:  jobDef,
		}
	}

	// Step 6: Success
	return ValidationResult{
		Valid:   true,
		Message: "TOML is valid",
		JobDef:  jobDef,
	}
}

// crawlerJobToJobDefinition converts simplified crawler job to full JobDefinition
func (s *TOMLValidationService) crawlerJobToJobDefinition(c *CrawlerJobDefinitionFile) *models.JobDefinition {
	jobType := models.JobOwnerTypeUser
	if c.JobType != "" {
		jobType = models.JobOwnerType(c.JobType)
	}

	return &models.JobDefinition{
		ID:          c.ID,
		Name:        c.Name,
		Type:        models.JobDefinitionTypeCrawler,
		JobType:     jobType,
		Description: c.Description,
		SourceType:  "web",
		BaseURL:     "",
		AuthID:      c.Authentication,
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

// UpdateValidationStatus updates the validation status in the database
func (s *TOMLValidationService) UpdateValidationStatus(
	ctx context.Context,
	db *sql.DB,
	jobDefID string,
	result ValidationResult,
) error {
	now := time.Now()

	status := "unknown"
	errorMsg := ""

	if result.Valid {
		status = "valid"
	} else {
		status = "invalid"
		errorMsg = result.Error
	}

	query := `
		UPDATE job_definitions
		SET validation_status = ?,
		    validation_error = ?,
		    validated_at = ?,
		    updated_at = ?
		WHERE id = ?
	`

	_, err := db.ExecContext(
		ctx,
		query,
		status,
		errorMsg,
		now.Unix(),
		now.Unix(),
		jobDefID,
	)

	if err != nil {
		s.logger.Error().
			Err(err).
			Str("job_definition_id", jobDefID).
			Msg("Failed to update validation status")
		return fmt.Errorf("failed to update validation status: %w", err)
	}

	s.logger.Debug().
		Str("job_definition_id", jobDefID).
		Str("status", status).
		Msg("Updated validation status")

	return nil
}
