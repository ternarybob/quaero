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

// LoadVariablesFromFiles loads variables from TOML files.
// It first checks for a variables.toml file in the given directory (like email.toml/connectors.toml).
// Then it loads any additional .toml files from a variables/ subdirectory for backward compatibility.
func (m *Manager) LoadVariablesFromFiles(ctx context.Context, dirPath string) error {
	m.logger.Debug().Str("dir", dirPath).Msg("Loading variables from files")

	loadedCount := 0
	skippedCount := 0
	errorCount := 0

	// First, check for variables.toml file directly in the given directory
	// This follows the same pattern as email.toml and connectors.toml
	variablesFile := filepath.Join(dirPath, "variables.toml")
	if _, err := os.Stat(variablesFile); err == nil {
		loaded, skipped, errors := m.loadVariablesFromFile(ctx, variablesFile)
		loadedCount += loaded
		skippedCount += skipped
		errorCount += errors
	} else {
		m.logger.Debug().Str("file", variablesFile).Msg("variables.toml not found in directory, checking subdirectory")
	}

	// Also check for variables in a variables/ subdirectory (backward compatibility)
	variablesDir := filepath.Join(dirPath, "variables")
	if info, err := os.Stat(variablesDir); err == nil && info.IsDir() {
		loaded, skipped, errors := m.loadVariablesFromDirectory(ctx, variablesDir)
		loadedCount += loaded
		skippedCount += skipped
		errorCount += errors
	}

	m.logger.Debug().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Int("errors", errorCount).
		Msg("Finished loading variables from files")

	return nil
}

// loadVariablesFromFile loads variables from a single TOML file
func (m *Manager) loadVariablesFromFile(ctx context.Context, filePath string) (loaded, skipped, errors int) {
	m.logger.Debug().Str("file", filePath).Msg("Loading variables from file")

	content, err := os.ReadFile(filePath)
	if err != nil {
		m.logger.Warn().Err(err).Str("file", filePath).Msg("Failed to read variable file")
		return 0, 0, 1
	}

	// Parse TOML file
	// Map of section name (key) to VariableFile struct
	var variables map[string]VariableFile
	if err := toml.Unmarshal(content, &variables); err != nil {
		m.logger.Warn().Err(err).Str("file", filePath).Msg("Failed to parse variable file")
		return 0, 0, 1
	}

	fileName := filepath.Base(filePath)
	for key, variable := range variables {
		if variable.Value == "" {
			m.logger.Warn().Str("file", fileName).Str("key", key).Msg("Skipping variable with empty value")
			skipped++
			continue
		}

		// Use Upsert to store the variable
		// This handles both new and existing keys
		description := variable.Description
		if description == "" {
			description = "Loaded from " + fileName
		}

		isNew, err := m.kv.Upsert(ctx, key, variable.Value, description)
		if err != nil {
			m.logger.Error().Err(err).Str("key", key).Msg("Failed to store variable")
			errors++
			continue
		}

		if isNew {
			m.logger.Debug().Str("key", key).Msg("Loaded new variable")
		} else {
			m.logger.Debug().Str("key", key).Msg("Updated existing variable")
		}
		loaded++
	}

	return loaded, skipped, errors
}

// loadVariablesFromDirectory loads variables from all TOML files in a directory
func (m *Manager) loadVariablesFromDirectory(ctx context.Context, dirPath string) (loaded, skipped, errors int) {
	m.logger.Debug().Str("dir", dirPath).Msg("Loading variables from directory")

	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		m.logger.Warn().Err(err).Str("dir", dirPath).Msg("Failed to read variables directory")
		return 0, 0, 1
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		l, s, e := m.loadVariablesFromFile(ctx, filePath)
		loaded += l
		skipped += s
		errors += e
	}

	return loaded, skipped, errors
}
