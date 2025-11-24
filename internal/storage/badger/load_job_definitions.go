package badger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// LoadJobDefinitionsFromFiles loads job definitions from TOML files in the specified directory
// Similar to LoadVariablesFromFiles, this scans the directory and loads all .toml files as job definitions
func LoadJobDefinitionsFromFiles(ctx context.Context, jobDefStorage interfaces.JobDefinitionStorage, kvStorage interfaces.KeyValueStorage, definitionsDir string, logger arbor.ILogger) error {
	// Check if directory exists
	if _, err := os.Stat(definitionsDir); os.IsNotExist(err) {
		logger.Debug().Str("dir", definitionsDir).Msg("Job definitions directory does not exist, skipping")
		return nil
	}

	logger.Info().Str("dir", definitionsDir).Msg("Loading job definitions from files")

	// Read all files in the directory
	entries, err := os.ReadDir(definitionsDir)
	if err != nil {
		return fmt.Errorf("failed to read job definitions directory: %w", err)
	}

	loadedCount := 0
	for _, entry := range entries {
		// Skip directories and non-TOML files
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}

		filePath := filepath.Join(definitionsDir, entry.Name())

		// Read file contents
		tomlBytes, err := os.ReadFile(filePath)
		if err != nil {
			logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to read job definition file")
			continue
		}

		// Parse TOML into JobDefinitionFile struct
		var jobFile handlers.JobDefinitionFile
		if err := toml.Unmarshal(tomlBytes, &jobFile); err != nil {
			logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to parse job definition TOML")
			continue
		}

		// Convert to JobDefinition model
		jobDef := jobFile.ToJobDefinition(kvStorage, logger)

		// Store raw TOML content
		jobDef.TOML = string(tomlBytes)

		// Validate job definition (for logging only - don't skip saving if validation fails)
		// This allows jobs with missing variables to be loaded and displayed in the UI
		validationErr := jobDef.Validate()
		if validationErr != nil {
			logger.Warn().Err(validationErr).Str("file", entry.Name()).Str("job_id", jobDef.ID).Msg("Job definition validation failed - saving anyway for UI display")
		}

		// Check if job definition already exists
		existingJobDef, err := jobDefStorage.GetJobDefinition(ctx, jobDef.ID)
		if err == nil && existingJobDef != nil {
			// Job exists - check if it's a system job
			if existingJobDef.IsSystemJob() {
				logger.Warn().Str("job_def_id", jobDef.ID).Str("file", entry.Name()).Msg("Cannot update system job via file loading")
				continue
			}

			// Update existing job definition
			if err := jobDefStorage.UpdateJobDefinition(ctx, jobDef); err != nil {
				logger.Warn().Err(err).Str("file", entry.Name()).Str("job_id", jobDef.ID).Msg("Failed to update job definition")
				continue
			}
			if validationErr != nil {
				logger.Info().Str("file", entry.Name()).Str("job_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition updated from file (with validation warnings)")
			} else {
				logger.Info().Str("file", entry.Name()).Str("job_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition updated from file")
			}
		} else {
			// Save new job definition
			if err := jobDefStorage.SaveJobDefinition(ctx, jobDef); err != nil {
				logger.Warn().Err(err).Str("file", entry.Name()).Str("job_id", jobDef.ID).Msg("Failed to save job definition")
				continue
			}
			if validationErr != nil {
				logger.Info().Str("file", entry.Name()).Str("job_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition loaded from file (with validation warnings)")
			} else {
				logger.Info().Str("file", entry.Name()).Str("job_id", jobDef.ID).Str("name", jobDef.Name).Msg("Job definition loaded from file")
			}
		}

		loadedCount++
	}

	if loadedCount > 0 {
		logger.Info().Int("count", loadedCount).Msg("Job definitions loaded from files")
	} else {
		logger.Debug().Msg("No job definitions loaded from files")
	}

	return nil
}
