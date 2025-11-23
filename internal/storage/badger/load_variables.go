package badger

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// VariableFile represents the structure of a variable in a TOML file
// Format:
// [key_name]
// value = "some-value"
// description = "optional description"
type VariableFile struct {
	Value       string `toml:"value"`
	Description string `toml:"description"`
}

// LoadVariablesFromFiles loads variables from TOML files in the specified directory
func (m *Manager) LoadVariablesFromFiles(ctx context.Context, dirPath string) error {
	m.logger.Info().Str("dir", dirPath).Msg("Loading variables from files")

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		m.logger.Debug().Str("dir", dirPath).Msg("Variables directory does not exist, skipping")
		return nil
	}

	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		m.logger.Warn().Err(err).Str("dir", dirPath).Msg("Failed to read variables directory")
		return nil // Non-fatal
	}

	loadedCount := 0
	skippedCount := 0
	errorCount := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to read variable file")
			errorCount++
			continue
		}

		// Parse TOML file
		// Map of section name (key) to VariableFile struct
		var variables map[string]VariableFile
		if err := toml.Unmarshal(content, &variables); err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to parse variable file")
			errorCount++
			continue
		}

		// Process each variable
		for key, variable := range variables {
			if variable.Value == "" {
				m.logger.Warn().Str("file", entry.Name()).Str("key", key).Msg("Skipping variable with empty value")
				skippedCount++
				continue
			}

			// Use Upsert to store the variable
			// This handles both new and existing keys
			description := variable.Description
			if description == "" {
				description = "Loaded from " + entry.Name()
			}

			isNew, err := m.kv.Upsert(ctx, key, variable.Value, description)
			if err != nil {
				m.logger.Error().Err(err).Str("key", key).Msg("Failed to store variable")
				errorCount++
				continue
			}

			if isNew {
				m.logger.Debug().Str("key", key).Msg("Loaded new variable")
			} else {
				m.logger.Debug().Str("key", key).Msg("Updated existing variable")
			}
			loadedCount++
		}
	}

	m.logger.Info().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Int("errors", errorCount).
		Msg("Finished loading variables from files")

	return nil
}
