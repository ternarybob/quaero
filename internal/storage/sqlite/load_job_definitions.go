// -----------------------------------------------------------------------
// Load Job Definitions from Files - TOML/JSON job definitions
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
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// JobDefinitionFile represents a generic job definition file format (TOML/JSON)
// This structure matches the models.JobDefinition closely for direct unmarshaling
type JobDefinitionFile struct {
	ID             string                 `toml:"id" json:"id"`
	Name           string                 `toml:"name" json:"name"`
	Type           string                 `toml:"type" json:"type"`                     // Job type: crawler, summarizer, custom, places
	JobType        string                 `toml:"job_type" json:"job_type"`             // Owner type: system, user
	Description    string                 `toml:"description" json:"description"`
	SourceType     string                 `toml:"source_type" json:"source_type"`       // Optional for some job types
	BaseURL        string                 `toml:"base_url" json:"base_url"`             // Optional
	AuthID         string                 `toml:"auth_id" json:"auth_id"`               // Optional
	Steps          []models.JobStep       `toml:"steps" json:"steps"`                   // Required
	Schedule       string                 `toml:"schedule" json:"schedule"`             // Cron or empty
	Timeout        string                 `toml:"timeout" json:"timeout"`               // Duration string
	Enabled        bool                   `toml:"enabled" json:"enabled"`
	AutoStart      bool                   `toml:"auto_start" json:"auto_start"`
	Config         map[string]interface{} `toml:"config" json:"config"`                 // Optional job-level config
	PreJobs        []string               `toml:"pre_jobs" json:"pre_jobs"`             // Optional
	PostJobs       []string               `toml:"post_jobs" json:"post_jobs"`           // Optional
	ErrorTolerance *models.ErrorTolerance `toml:"error_tolerance" json:"error_tolerance"` // Optional
	Tags           []string               `toml:"tags" json:"tags"`                     // Tags to apply to documents created by this job
}

// ToJobDefinition converts the file format to a full JobDefinition model
// Performs {key-name} replacement using the provided KV storage
func (j *JobDefinitionFile) ToJobDefinition(kvStorage interfaces.KeyValueStorage, logger arbor.ILogger) *models.JobDefinition {
	// Default to 'user' if job_type is not specified
	jobType := models.JobOwnerTypeUser
	if j.JobType != "" {
		jobType = models.JobOwnerType(j.JobType)
	}

	jobDef := &models.JobDefinition{
		ID:             j.ID,
		Name:           j.Name,
		Type:           models.JobDefinitionType(j.Type),
		JobType:        jobType,
		Description:    j.Description,
		SourceType:     j.SourceType,
		BaseURL:        j.BaseURL,
		AuthID:         j.AuthID,
		Steps:          j.Steps,
		Schedule:       j.Schedule,
		Timeout:        j.Timeout,
		Enabled:        j.Enabled,
		AutoStart:      j.AutoStart,
		Config:         j.Config,
		PreJobs:        j.PreJobs,
		PostJobs:       j.PostJobs,
		ErrorTolerance: j.ErrorTolerance,
		Tags:           j.Tags,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Initialize empty maps if nil
	if jobDef.Config == nil {
		jobDef.Config = make(map[string]interface{})
	}

	// Perform {key-name} replacement if KV storage is available
	if kvStorage != nil {
		ctx := context.Background()
		kvMap, err := kvStorage.GetAll(ctx)
		if err != nil {
			logger.Warn().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to fetch KV map for replacement, skipping replacement (graceful degradation)")
		} else {
			// Replace in job-level config map
			if err := common.ReplaceInMap(jobDef.Config, kvMap, logger); err != nil {
				logger.Warn().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to replace in job config")
			}

			// Replace in each step's config map
			for i := range jobDef.Steps {
				if jobDef.Steps[i].Config != nil {
					if err := common.ReplaceInMap(jobDef.Steps[i].Config, kvMap, logger); err != nil {
						logger.Warn().Err(err).Str("job_def_id", jobDef.ID).Int("step_index", i).Msg("Failed to replace in step config")
					}
				}
			}

			// Replace in string fields
			jobDef.BaseURL = common.ReplaceKeyReferences(jobDef.BaseURL, kvMap, logger)
			jobDef.AuthID = common.ReplaceKeyReferences(jobDef.AuthID, kvMap, logger)
			jobDef.SourceType = common.ReplaceKeyReferences(jobDef.SourceType, kvMap, logger)
		}
	}

	return jobDef
}

// LoadJobDefinitionsFromFiles loads job definitions from TOML/JSON files
// in the specified directory. This is called during startup to seed user-defined jobs.
// Supports all job types (crawler, places, summarizer, custom) via generic loading.
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

		var jobFile *JobDefinitionFile

		switch ext {
		case ".toml":
			jobFile, err = m.loadJobDefFromTOML(filePath)
		case ".json":
			jobFile, err = m.loadJobDefFromJSON(filePath)
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

		// Convert to full JobDefinition model
		jobDef := jobFile.ToJobDefinition(m.kv, m.logger)

		// Validate full job definition
		if err := jobDef.Validate(); err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Job definition validation failed")
			skippedCount++
			continue
		}

		// Save raw TOML/JSON content for reference
		rawContent, _ := os.ReadFile(filePath)
		jobDef.TOML = string(rawContent)

		// Save job definition (idempotent - uses ON CONFLICT to update existing)
		if err := m.jobDefinition.SaveJobDefinition(ctx, jobDef); err != nil {
			m.logger.Error().Err(err).Str("file", entry.Name()).Msg("Failed to save job definition")
			skippedCount++
			continue
		}

		m.logger.Info().
			Str("job_def_id", jobDef.ID).
			Str("type", string(jobDef.Type)).
			Str("file", entry.Name()).
			Msg("Loaded job definition from file")

		loadedCount++
	}

	m.logger.Info().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Msg("Finished loading job definitions from files")

	return nil
}

// loadJobDefFromTOML loads a generic job definition from a TOML file
func (m *Manager) loadJobDefFromTOML(filePath string) (*JobDefinitionFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var jobFile JobDefinitionFile
	if err := toml.Unmarshal(data, &jobFile); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	return &jobFile, nil
}

// loadJobDefFromJSON loads a generic job definition from a JSON file
func (m *Manager) loadJobDefFromJSON(filePath string) (*JobDefinitionFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var jobFile JobDefinitionFile
	if err := json.Unmarshal(data, &jobFile); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &jobFile, nil
}
