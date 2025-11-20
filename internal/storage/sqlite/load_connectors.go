// -----------------------------------------------------------------------
// Load Connectors from Files - TOML configuration files
// -----------------------------------------------------------------------
//
// This file loads connector configurations from TOML files at startup
// and stores them in the connectors table via ConnectorService.
//
// Default storage location: ./connectors/ directory
// File format: Any *.toml file in the connectors directory
//
// TOML file format (section name = connector name):
//   [my-github]
//   type = "github"
//   token = "ghp_xxxxx"

package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/quaero/internal/models"
)

// ConnectorFile represents a connector entry in a TOML configuration file.
// Each TOML section becomes a connector, with the section name as the connector name.
//
// TOML sections: [connector-name] with type (required) and config fields
// Example:
//
//	[my-github]
//	type = "github"
//	token = "ghp_xxxxx"
type ConnectorFile struct {
	Type  string `toml:"type" json:"type"`   // Required: Connector type (e.g., "github")
	Token string `toml:"token" json:"token"` // GitHub-specific: Personal Access Token
	// Additional connector types can add their fields here
}

// LoadConnectorsFromFiles loads connectors from TOML files in the specified directory
// and stores them via the ConnectorService. This is called during startup.
//
// Default storage location: ./connectors/ directory
// The function is idempotent - uses upsert strategy via ConnectorService.
// Duplicate names (case-insensitive) are detected and logged with warnings.
func (m *Manager) LoadConnectorsFromFiles(ctx context.Context, dirPath string) error {
	m.logger.Info().Str("path", dirPath).Msg("Loading connectors from files")

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		m.logger.Debug().Str("path", dirPath).Msg("Connectors directory not found, skipping file loading")
		return nil // Not an error - directory is optional
	}

	// We need ConnectorService to create connectors
	// Since Manager doesn't have it, we'll need to pass it in or use the storage directly
	// For now, let's access the database directly similar to how we do in other loaders

	// Read directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read connectors directory: %w", err)
	}

	loadedCount := 0
	skippedCount := 0
	duplicateCount := 0

	// Track connectors loaded so far (case-insensitive) with their source file
	seenConnectors := make(map[string]struct {
		file         string
		originalName string
	})

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
		connectorFiles, err := m.loadConnectorsFromTOML(filePath)
		if err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to load connector file")
			skippedCount++
			continue
		}

		// Process each section in the TOML file
		for connectorName, connFile := range connectorFiles {
			// Validate required fields
			if err := m.validateConnectorFile(connFile, connectorName); err != nil {
				m.logger.Warn().Err(err).Str("file", entry.Name()).Str("name", connectorName).Msg("Connector validation failed")
				skippedCount++
				continue
			}

			// Normalize name for duplicate detection (case-insensitive)
			normalizedName := strings.ToLower(strings.TrimSpace(connectorName))

			// Check for duplicate connectors across files
			if previousEntry, exists := seenConnectors[normalizedName]; exists {
				m.logger.Warn().
					Str("name", connectorName).
					Str("normalized_name", normalizedName).
					Str("current_file", entry.Name()).
					Str("previous_file", previousEntry.file).
					Str("previous_name", previousEntry.originalName).
					Msg("Duplicate connector detected (case-insensitive) - will overwrite previous value")
				duplicateCount++
			}

			// Marshal config to JSON based on connector type
			var configJSON []byte
			switch models.ConnectorType(connFile.Type) {
			case models.ConnectorTypeGitHub:
				config := models.GitHubConnectorConfig{
					Token: connFile.Token,
				}
				configJSON, err = json.Marshal(config)
				if err != nil {
					m.logger.Error().Err(err).Str("file", entry.Name()).Str("name", connectorName).Msg("Failed to marshal connector config")
					skippedCount++
					continue
				}
			default:
				m.logger.Warn().Str("type", connFile.Type).Str("file", entry.Name()).Msg("Unknown connector type")
				skippedCount++
				continue
			}

			// Check if connector exists in database
			existingConn, err := m.getConnectorByName(ctx, connectorName)
			existsInDB := err == nil && existingConn != nil

			// Upsert connector to database
			if existsInDB {
				// Update existing connector
				if err := m.updateConnector(ctx, existingConn.ID, connectorName, models.ConnectorType(connFile.Type), configJSON); err != nil {
					m.logger.Error().Err(err).Str("file", entry.Name()).Str("name", connectorName).Msg("Failed to update connector")
					skippedCount++
					continue
				}
				m.logger.Warn().
					Str("name", connectorName).
					Str("file", entry.Name()).
					Msg("Updated existing connector from file (database value overwritten)")
			} else {
				// Create new connector
				if err := m.createConnector(ctx, connectorName, models.ConnectorType(connFile.Type), configJSON); err != nil {
					m.logger.Error().Err(err).Str("file", entry.Name()).Str("name", connectorName).Msg("Failed to create connector")
					skippedCount++
					continue
				}
				m.logger.Info().
					Str("name", connectorName).
					Str("file", entry.Name()).
					Msg("Created new connector from file")
			}

			// Track this connector for duplicate detection
			seenConnectors[normalizedName] = struct {
				file         string
				originalName string
			}{
				file:         entry.Name(),
				originalName: connectorName,
			}

			loadedCount++
		}
	}

	// Log summary with duplicate warnings
	if duplicateCount > 0 {
		m.logger.Warn().
			Int("duplicates", duplicateCount).
			Msg("Duplicate connectors detected during file loading - later files override earlier files")
	}

	m.logger.Info().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Int("duplicates", duplicateCount).
		Str("dir", dirPath).
		Msg("Finished loading connectors from files")

	return nil
}

// loadConnectorsFromTOML loads connectors from a TOML file with sections.
// Each section represents one connector to be stored.
//
// TOML format: [section-name] with type (required) and config fields
// Returns a map of section names (connector names) to ConnectorFile structs.
func (m *Manager) loadConnectorsFromTOML(filePath string) (map[string]*ConnectorFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse as map of sections
	var sections map[string]*ConnectorFile
	if err := toml.Unmarshal(data, &sections); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("no sections found in TOML file")
	}

	return sections, nil
}

// validateConnectorFile validates that required fields are present in a connector configuration.
func (m *Manager) validateConnectorFile(connFile *ConnectorFile, connectorName string) error {
	if connectorName == "" {
		return fmt.Errorf("connector name is required")
	}
	if connFile.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Validate type-specific fields
	switch models.ConnectorType(connFile.Type) {
	case models.ConnectorTypeGitHub:
		if connFile.Token == "" {
			return fmt.Errorf("token is required for GitHub connectors")
		}
	default:
		return fmt.Errorf("unknown connector type: %s", connFile.Type)
	}

	return nil
}

// getConnectorByName retrieves a connector by name (case-sensitive)
func (m *Manager) getConnectorByName(ctx context.Context, name string) (*models.Connector, error) {
	query := `SELECT id, name, type, config, created_at, updated_at FROM connectors WHERE name = ?`
	row := m.db.DB().QueryRowContext(ctx, query, name)

	var conn models.Connector
	var typeStr string
	err := row.Scan(&conn.ID, &conn.Name, &typeStr, &conn.Config, &conn.CreatedAt, &conn.UpdatedAt)
	if err != nil {
		return nil, err
	}

	conn.Type = models.ConnectorType(typeStr)
	return &conn, nil
}

// createConnector creates a new connector in the database
func (m *Manager) createConnector(ctx context.Context, name string, connType models.ConnectorType, config json.RawMessage) error {
	now := time.Now().Unix()
	query := `INSERT INTO connectors (id, name, type, config, created_at, updated_at) 
			  VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?)`
	_, err := m.db.DB().ExecContext(ctx, query, name, string(connType), config, now, now)
	return err
}

// updateConnector updates an existing connector in the database
func (m *Manager) updateConnector(ctx context.Context, id string, name string, connType models.ConnectorType, config json.RawMessage) error {
	now := time.Now().Unix()
	query := `UPDATE connectors SET name = ?, type = ?, config = ?, updated_at = ? WHERE id = ?`
	_, err := m.db.DB().ExecContext(ctx, query, name, string(connType), config, now, id)
	return err
}
