// -----------------------------------------------------------------------
// Load Cookie-Based Authentication from Files - TOML configuration files
// -----------------------------------------------------------------------
//
// This file loads cookie-based authentication credentials from TOML files
// and stores them in the auth_credentials table. This is separate from API
// key loading (which uses load_keys.go and stores in key_value_store table).
//
// Cookie-based auth is typically captured via Chrome extension, but file-based
// loading is useful for testing, CI/CD, or manual setup scenarios.
//
// TOML file format:
//   [section-name]
//   name = "Bob's Atlassian"
//   site_domain = "bobmcallan.atlassian.net"
//   service_type = "atlassian"
//   base_url = "https://bobmcallan.atlassian.net"
//   user_agent = "Mozilla/5.0..."
//   tokens = { "access_token" = "xyz123" }  # Optional
//   data = { "region" = "us-east-1" }       # Optional
//
// NOTE: Sections with 'api_key' field will be skipped with a warning.
// API keys belong in ./keys directory, not ./auth directory.

package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/quaero/internal/models"
)

// AuthCredentialFile represents a cookie-based auth configuration file format (TOML).
// This structure matches models.AuthCredentials for cookie-based authentication.
//
// TOML sections: [section-name] with name, site_domain, service_type, base_url, etc.
type AuthCredentialFile struct {
	// Core fields (match models.AuthCredentials)
	Name        string `toml:"name" json:"name"`                 // Required: Human-readable name
	SiteDomain  string `toml:"site_domain" json:"site_domain"`   // Required (or base_url): Domain for site grouping
	ServiceType string `toml:"service_type" json:"service_type"` // Optional: Service identifier (e.g., "atlassian", "github")
	BaseURL     string `toml:"base_url" json:"base_url"`         // Optional (or site_domain): Service base URL
	UserAgent   string `toml:"user_agent" json:"user_agent"`     // Optional: User agent string

	// Optional fields
	Tokens map[string]string      `toml:"tokens" json:"tokens"` // Optional: Auth tokens as inline table
	Data   map[string]interface{} `toml:"data" json:"data"`     // Optional: Service-specific metadata as inline table

	// Detection field (for skipping API key sections)
	APIKey string `toml:"api_key" json:"api_key"` // If present, section will be skipped with warning
}

// LoadAuthCredentialsFromFiles loads cookie-based auth credentials from TOML files
// in the specified directory and stores them in the auth_credentials table.
// This is called during startup to seed authentication credentials.
//
// This function handles cookie-based authentication ONLY:
// - Cookie-based auth: Stored in auth_credentials table
// - API keys: Handled by LoadKeysFromFiles() → key_value_store table
//
// Any sections containing 'api_key' field will be skipped with a warning.
func (m *Manager) LoadAuthCredentialsFromFiles(ctx context.Context, dirPath string) error {
	m.logger.Info().Str("path", dirPath).Msg("Loading cookie-based auth credentials from files")

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

		// Only process TOML files
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
			// Check if this is an API key section (should be skipped)
			if authFile.APIKey != "" {
				m.logger.Warn().
					Str("section", sectionName).
					Str("file", entry.Name()).
					Msg("Skipping API key section - API keys should be in ./keys directory, not ./auth")
				skippedCount++
				continue
			}

			// Validate required fields for cookie-based auth
			if err := m.validateAuthCredentialFile(authFile, sectionName); err != nil {
				m.logger.Warn().Err(err).Str("file", entry.Name()).Str("section", sectionName).Msg("Auth credentials validation failed")
				skippedCount++
				continue
			}

			// Build AuthCredentials model from TOML data
			credentials := &models.AuthCredentials{
				Name:        authFile.Name,
				SiteDomain:  authFile.SiteDomain,
				ServiceType: authFile.ServiceType,
				BaseURL:     authFile.BaseURL,
				UserAgent:   authFile.UserAgent,
				Tokens:      authFile.Tokens,
				Data:        authFile.Data,
				// ID, CreatedAt, UpdatedAt will be set by StoreCredentials()
			}

			// If BaseURL is provided but SiteDomain is not, extract domain from BaseURL
			if credentials.BaseURL != "" && credentials.SiteDomain == "" {
				// Extract domain from URL (simple approach - just the host part)
				// This will be handled by AuthStorage.StoreCredentials() which calls extractDomainFromURL()
			}

			// If SiteDomain is provided but BaseURL is not, construct BaseURL
			if credentials.SiteDomain != "" && credentials.BaseURL == "" {
				credentials.BaseURL = fmt.Sprintf("https://%s", credentials.SiteDomain)
			}

			// Store in auth_credentials table
			if err := m.auth.StoreCredentials(ctx, credentials); err != nil {
				m.logger.Error().Err(err).Str("file", entry.Name()).Str("section", sectionName).Msg("Failed to save auth credentials to database")
				skippedCount++
				continue
			}

			m.logger.Info().
				Str("name", credentials.Name).
				Str("site_domain", credentials.SiteDomain).
				Str("service_type", credentials.ServiceType).
				Str("file", entry.Name()).
				Msg("Loaded cookie-based auth credentials from file")

			loadedCount++
		}
	}

	m.logger.Info().
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Str("dir", dirPath).
		Msg("Finished loading cookie-based auth credentials from files")

	return nil
}

// loadAuthCredsFromTOML loads cookie-based auth from a TOML file with sections.
// Each section represents one set of auth credentials to be stored in auth_credentials table.
//
// TOML format: [section-name] with name, site_domain, service_type, base_url, etc.
// Returns a map of section names to AuthCredentialFile structs.
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

// validateAuthCredentialFile validates that required fields are present in a cookie-based auth configuration.
// The sectionName parameter is the TOML section name used for logging.
//
// Required fields:
// - name: Human-readable identifier
// - site_domain OR base_url: At least one must be provided
func (m *Manager) validateAuthCredentialFile(authFile *AuthCredentialFile, sectionName string) error {
	if sectionName == "" {
		return fmt.Errorf("section name is required")
	}
	if authFile.Name == "" {
		return fmt.Errorf("name is required for cookie-based auth")
	}
	if authFile.SiteDomain == "" && authFile.BaseURL == "" {
		return fmt.Errorf("either site_domain or base_url is required")
	}
	// ServiceType is optional - can be empty
	return nil
}
