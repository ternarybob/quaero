// -----------------------------------------------------------------------
// Load Variables (Key/Value Pairs) from Files - TOML configuration files
// -----------------------------------------------------------------------
//
// This file loads user-defined variables (generic key/value pairs) from TOML files
// and stores them in the KV store. This is separate from auth credentials loading
// (which handles cookie-based authentication for web scraping).
//
// Default storage location: ./variables/ directory
// File format: Any *.toml file in the variables directory
//
// TOML file format:
//   [section-name]
//   value = "your-secret-value"
//   description = "Optional description"

package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// KeyValueFile represents a key/value entry in a TOML configuration file.
// Each TOML section becomes a key in the KV store, with the section name as the key name.
//
// TOML sections: [section-name] with value (required) and description (optional) fields
// Example:
//   [google-api-key]
//   value = "AIzaSyABC123..."
//   description = "Google API key for Gemini"
type KeyValueFile struct {
	Value       string `toml:"value" json:"value"`             // Required: The secret value
	Description string `toml:"description" json:"description"` // Optional: Human-readable description
}

// LoadKeysFromFiles loads variables (key/value pairs) from TOML files in the specified directory
// and stores them in the KV store. This is called during startup to seed configuration values.
//
// This function is separate from LoadAuthCredentialsFromFiles():
// - Auth credentials: Cookie-based authentication for web scraping
// - Variables: Generic secrets and configuration values (API keys, tokens, etc.)
//
// Default storage location: ./variables/ directory
// The function is idempotent - uses Set() which has ON CONFLICT UPDATE behavior.
func (m *Manager) LoadKeysFromFiles(ctx context.Context, dirPath string) error {
	m.logger.Info().Str("path", dirPath).Msg("Loading variables from files")

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		m.logger.Debug().Str("path", dirPath).Msg("Key/value directory not found, skipping file loading")
		return nil // Not an error - directory is optional
	}

	// Read directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read key/value directory: %w", err)
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

		// Only process TOML files
		if ext != ".toml" {
			m.logger.Debug().Str("file", entry.Name()).Msg("Skipping non-TOML file")
			skippedCount++
			continue
		}

		// Load and parse TOML file (supports sections)
		kvFiles, err := m.loadKeysFromTOML(filePath)
		if err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to load key/value file")
			skippedCount++
			continue
		}

		// Process each section in the TOML file
		for sectionName, kvFile := range kvFiles {
			// Validate required fields
			if err := m.validateKeyValueFile(kvFile, sectionName); err != nil {
				m.logger.Warn().Err(err).Str("file", entry.Name()).Str("section", sectionName).Msg("Key/value validation failed")
				skippedCount++
				continue
			}

			// Use provided description or default
			description := kvFile.Description
			if description == "" {
				description = "Loaded from file"
			}

			// Save key/value to KV store (idempotent - uses ON CONFLICT to update existing)
			if err := m.kv.Set(ctx, sectionName, kvFile.Value, description); err != nil {
				m.logger.Error().Err(err).Str("file", entry.Name()).Str("section", sectionName).Msg("Failed to save key/value to KV store")
				skippedCount++
				continue
			}

			m.logger.Info().
				Str("key", sectionName).
				Str("file", entry.Name()).
				Msg("Loaded key/value pair from file")

			loadedCount++
		}
	}

	m.logger.Info().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Str("dir", dirPath).
		Msg("Finished loading key/value pairs from files")

	return nil
}

// loadKeysFromTOML loads key/value pairs from a TOML file with sections.
// Each section represents one key to be stored in the KV store.
//
// TOML format: [section-name] with value (required) and description (optional) fields
// Returns a map of section names (which become KV keys) to KeyValueFile structs.
func (m *Manager) loadKeysFromTOML(filePath string) (map[string]*KeyValueFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse as map of sections
	var sections map[string]*KeyValueFile
	if err := toml.Unmarshal(data, &sections); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("no sections found in TOML file")
	}

	return sections, nil
}

// validateKeyValueFile validates that required fields are present in a key/value configuration.
// The sectionName parameter is the TOML section name, which becomes the KV store key
// (e.g., "google-api-key", "github-token").
func (m *Manager) validateKeyValueFile(kvFile *KeyValueFile, sectionName string) error {
	if sectionName == "" {
		return fmt.Errorf("section name is required")
	}
	if kvFile.Value == "" {
		return fmt.Errorf("value is required")
	}
	// Description is optional - no validation needed
	return nil
}
