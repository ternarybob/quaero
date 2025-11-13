// -----------------------------------------------------------------------
// Load Auth Credentials from Files - TOML auth credentials files
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/quaero/internal/models"
)

// AuthCredentialFile represents a generic auth credential file format (TOML)
// This structure matches models.AuthCredentials for API key authentication
// Supports TOML sections: [section-name] with api_key, service_type, description fields
type AuthCredentialFile struct {
	APIKey      string `toml:"api_key" json:"api_key"`           // Required: The API key value
	ServiceType string `toml:"service_type" json:"service_type"` // Required: Service identifier
	Description string `toml:"description" json:"description"`   // Optional: Human-readable description
}

// ToAuthCredentials converts the file format to a full AuthCredentials model
// The name parameter comes from the TOML section name (e.g., [google-places-key])
func (a *AuthCredentialFile) ToAuthCredentials(name string) *models.AuthCredentials {
	return &models.AuthCredentials{
		ID:          uuid.New().String(),
		Name:        name,
		SiteDomain:  "", // Empty for API keys
		ServiceType: a.ServiceType,
		Data:        map[string]interface{}{"description": a.Description},
		Cookies:     nil,                     // Not used for API keys
		Tokens:      make(map[string]string), // Not used for API keys
		APIKey:      a.APIKey,
		AuthType:    "api_key", // Always API key for file-based credentials
		BaseURL:     "",        // Not used for API keys
		UserAgent:   "",        // Not used for API keys
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}
}

// LoadAuthCredentialsFromFiles loads auth credentials from TOML files
// in the specified directory. This is called during startup to seed API keys.
// Supports API key storage for services like Google Gemini, Google Places, etc.
func (m *Manager) LoadAuthCredentialsFromFiles(ctx context.Context, dirPath string) error {
	m.logger.Info().Str("path", dirPath).Msg("Loading auth credentials from files")

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		m.logger.Debug().Str("path", dirPath).Msg("Auth credentials directory not found, skipping file loading")
		return nil // Not an error - directory is optional
	}

	// Read directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read auth credentials directory: %w", err)
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

		// Only process TOML files (skip JSON for simplicity)
		if ext != ".toml" {
			m.logger.Debug().Str("file", entry.Name()).Msg("Skipping non-TOML file")
			skippedCount++
			continue
		}

		// Load and parse TOML file (supports sections)
		authFiles, err := m.loadAuthCredsFromTOML(filePath)
		if err != nil {
			m.logger.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to load auth credentials file")
			skippedCount++
			continue
		}

		// Process each section in the TOML file
		for sectionName, authFile := range authFiles {
			// Validate required fields
			if err := m.validateAuthCredentialFile(authFile, sectionName); err != nil {
				m.logger.Warn().Err(err).Str("file", entry.Name()).Str("section", sectionName).Msg("Auth credentials validation failed")
				skippedCount++
				continue
			}

			// Convert to full AuthCredentials model (section name becomes the credential name)
			authCreds := authFile.ToAuthCredentials(sectionName)

			// Save auth credentials (idempotent - uses ON CONFLICT to update existing)
			if err := m.auth.StoreCredentials(ctx, authCreds); err != nil {
				m.logger.Error().Err(err).Str("file", entry.Name()).Str("section", sectionName).Msg("Failed to save auth credentials")
				skippedCount++
				continue
			}

			// Mask the API key for logging (show first 4 + last 4 chars)
			maskedKey := m.maskAPIKeyForLogging(authCreds.APIKey)
			m.logger.Info().
				Str("name", authCreds.Name).
				Str("service_type", authCreds.ServiceType).
				Str("api_key", maskedKey).
				Str("file", entry.Name()).
				Msg("Loaded API key from file")

			loadedCount++
		}
	}

	m.logger.Info().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Str("dir", dirPath).
		Msg("Finished loading auth credentials from files")

	return nil
}

// loadAuthCredsFromTOML loads auth credentials from a TOML file with sections
// Supports format: [section-name] with api_key, service_type, description fields
// Returns a map of section names to AuthCredentialFile structs
func (m *Manager) loadAuthCredsFromTOML(filePath string) (map[string]*AuthCredentialFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse as map of sections
	var sections map[string]*AuthCredentialFile
	if err := toml.Unmarshal(data, &sections); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("no sections found in TOML file")
	}

	return sections, nil
}

// validateAuthCredentialFile validates that required fields are present
// The sectionName parameter is the TOML section name (e.g., "google-places-key")
func (m *Manager) validateAuthCredentialFile(authFile *AuthCredentialFile, sectionName string) error {
	if sectionName == "" {
		return fmt.Errorf("section name is required")
	}
	if authFile.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	if authFile.ServiceType == "" {
		return fmt.Errorf("service_type is required")
	}
	return nil
}

// maskAPIKeyForLogging masks an API key for safe logging (first 4 + last 4 chars)
func (m *Manager) maskAPIKeyForLogging(apiKey string) string {
	if len(apiKey) <= 8 {
		return "••••••••" // Mask short keys completely
	}
	return apiKey[:4] + "•••" + apiKey[len(apiKey)-4:]
}
