package badger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// ConnectorFile represents a connector in TOML format
// Format:
// [connector_name]
// type = "github"
// token = "ghp_xxx" or token = "{variable_name}"
type ConnectorFile struct {
	Type  string `toml:"type"`
	Token string `toml:"token"`
}

// LoadConnectorsFromFiles loads connectors from TOML files in the specified directory
// It supports variable substitution using {variable_name} syntax in the token field
func LoadConnectorsFromFiles(ctx context.Context, connectorStorage interfaces.ConnectorStorage, kvStorage interfaces.KeyValueStorage, dirPath string, logger arbor.ILogger) error {
	logger.Debug().Str("dir", dirPath).Msg("Loading connectors from files")

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logger.Debug().Str("dir", dirPath).Msg("Connectors directory does not exist, skipping")
		return nil
	}

	// Load KV map for variable substitution
	var kvMap map[string]string
	if kvStorage != nil {
		var err error
		kvMap, err = kvStorage.GetAll(ctx)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to load KV map for connector variable substitution")
			kvMap = make(map[string]string)
		}
	} else {
		kvMap = make(map[string]string)
	}

	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		logger.Warn().Err(err).Str("dir", dirPath).Msg("Failed to read connectors directory")
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
			logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to read connector file")
			errorCount++
			continue
		}

		// Parse TOML file - map of section name to connector config
		var connectors map[string]ConnectorFile
		if err := toml.Unmarshal(content, &connectors); err != nil {
			logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to parse connector file")
			errorCount++
			continue
		}

		// Process each connector
		for name, connFile := range connectors {
			// Validate connector type
			connType := models.ConnectorType(connFile.Type)
			if connType != models.ConnectorTypeGitHub && connType != models.ConnectorTypeGitLab {
				logger.Warn().
					Str("file", entry.Name()).
					Str("connector", name).
					Str("type", connFile.Type).
					Msg("Skipping connector: unknown type, valid types are: github, gitlab")
				skippedCount++
				continue
			}

			// Validate token (before substitution)
			if connFile.Token == "" {
				logger.Warn().
					Str("file", entry.Name()).
					Str("connector", name).
					Msg("Skipping connector: token is required")
				skippedCount++
				continue
			}

			// Perform variable substitution on token using {variable_name} syntax
			token := common.ReplaceKeyReferences(connFile.Token, kvMap, logger)

			// Validate token after substitution - if it still contains {var} pattern, variable wasn't found
			if strings.Contains(token, "{") && strings.Contains(token, "}") {
				logger.Warn().
					Str("file", entry.Name()).
					Str("connector", name).
					Str("token_pattern", connFile.Token).
					Msg("Token contains unresolved variable reference")
			}

			// Create config JSON with substituted token
			configJSON, err := json.Marshal(map[string]string{"token": token})
			if err != nil {
				logger.Warn().Err(err).
					Str("file", entry.Name()).
					Str("connector", name).
					Msg("Failed to marshal connector config")
				errorCount++
				continue
			}

			// Create connector model
			now := time.Now()
			connector := &models.Connector{
				ID:        name, // Use section name as ID
				Name:      name, // Use section name as Name
				Type:      connType,
				Config:    configJSON,
				CreatedAt: now,
				UpdatedAt: now,
			}

			// Check if connector already exists
			existing, err := connectorStorage.GetConnector(ctx, name)
			if err == nil && existing != nil {
				// Update existing connector
				connector.CreatedAt = existing.CreatedAt // Preserve original creation time
				if err := connectorStorage.UpdateConnector(ctx, connector); err != nil {
					logger.Warn().Err(err).
						Str("connector", name).
						Msg("Failed to update connector")
					errorCount++
					continue
				}
				logger.Debug().Str("connector", name).Str("type", string(connType)).Msg("Updated existing connector")
			} else {
				// Save new connector
				if err := connectorStorage.SaveConnector(ctx, connector); err != nil {
					logger.Warn().Err(err).
						Str("connector", name).
						Msg("Failed to save connector")
					errorCount++
					continue
				}
				logger.Debug().Str("connector", name).Str("type", string(connType)).Msg("Loaded new connector")
			}

			loadedCount++
		}
	}

	logger.Info().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Int("errors", errorCount).
		Msg("Finished loading connectors from files")

	return nil
}
