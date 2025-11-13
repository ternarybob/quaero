package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// AuthStorage implements the AuthStorage interface for SQLite
type AuthStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
}

// NewAuthStorage creates a new AuthStorage instance
func NewAuthStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.AuthStorage {
	return &AuthStorage{
		db:     db,
		logger: logger,
	}
}

// StoreCredentials stores authentication credentials
// If credentials with the same site_domain exist, they will be updated (override)
// If it's a new site, a new entry will be created
func (s *AuthStorage) StoreCredentials(ctx context.Context, credentials *models.AuthCredentials) error {
	// Generate ID if not provided
	if credentials.ID == "" {
		credentials.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now().Unix()
	if credentials.CreatedAt == 0 {
		credentials.CreatedAt = now
	}
	credentials.UpdatedAt = now

	// Extract site domain from base URL if not provided
	if credentials.SiteDomain == "" && credentials.BaseURL != "" {
		parsedURL, err := url.Parse(credentials.BaseURL)
		if err != nil {
			return fmt.Errorf("failed to parse base URL: %w", err)
		}
		credentials.SiteDomain = parsedURL.Host
	}

	// Parse BaseURL from SiteDomain if BaseURL is empty (backward compatibility)
	if credentials.BaseURL == "" {
		// Check if SiteDomain looks like a domain (not an empty string)
		if credentials.SiteDomain != "" && !strings.Contains(credentials.SiteDomain, "://") {
			// Assume https if no protocol specified
			credentials.BaseURL = "https://" + credentials.SiteDomain
		}
	}

	// Generate default name if not provided
	if credentials.Name == "" {
		credentials.Name = fmt.Sprintf("%s (%s)", credentials.ServiceType, credentials.SiteDomain)
	}

	// Marshal JSON fields
	data, err := json.Marshal(credentials.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	tokens, err := json.Marshal(credentials.Tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	// Check if credentials exist for this site_domain
	// For credentials with empty site_domain (API keys), we need to check by name and auth_type
	var existing *models.AuthCredentials
	if credentials.SiteDomain != "" {
		// Check by site_domain for credentials with a domain
		var err error
		existing, err = s.GetCredentialsBySiteDomain(ctx, credentials.SiteDomain)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to check existing credentials: %w", err)
		}
	} else {
		// For API keys with empty site_domain, check by name and auth_type
		// to avoid conflicts when multiple API keys are stored
		query := `SELECT id, name, site_domain, service_type, data, cookies, tokens, api_key, auth_type, base_url, user_agent, created_at, updated_at
		          FROM auth_credentials WHERE name = ? AND auth_type = ? LIMIT 1`

		var data, tokens []byte
		var cookies []byte
		existing = &models.AuthCredentials{}

		err := s.db.DB().QueryRowContext(ctx, query, credentials.Name, credentials.AuthType).Scan(
			&existing.ID, &existing.Name, &existing.SiteDomain, &existing.ServiceType,
			&data, &cookies, &tokens, &existing.APIKey, &existing.AuthType,
			&existing.BaseURL, &existing.UserAgent, &existing.CreatedAt, &existing.UpdatedAt)

		if err != sql.ErrNoRows && err != nil {
			return fmt.Errorf("failed to check existing credentials: %w", err)
		}

		if err == sql.ErrNoRows {
			existing = nil
		} else {
			// Unmarshal data and tokens
			if len(data) > 0 {
				if err := json.Unmarshal(data, &existing.Data); err != nil {
					return fmt.Errorf("failed to unmarshal existing data: %w", err)
				}
			}
			if len(tokens) > 0 {
				if err := json.Unmarshal(tokens, &existing.Tokens); err != nil {
					return fmt.Errorf("failed to unmarshal existing tokens: %w", err)
				}
			}
			existing.Cookies = cookies
		}
	}

	if existing != nil {
		// Override existing credentials for this site
		credentials.ID = existing.ID
		credentials.CreatedAt = existing.CreatedAt

		// Ensure auth_type is set (default to 'cookie' for backward compatibility)
		if credentials.AuthType == "" {
			credentials.AuthType = "cookie"
		}

		query := `
		UPDATE auth_credentials SET
			name = ?,
			site_domain = ?,
			service_type = ?,
			data = ?,
			cookies = ?,
			tokens = ?,
			api_key = ?,
			auth_type = ?,
			base_url = ?,
			user_agent = ?,
			updated_at = ?
		WHERE id = ?`

		_, err = s.db.DB().ExecContext(ctx, query,
			credentials.Name, credentials.SiteDomain, credentials.ServiceType,
			data, credentials.Cookies, tokens, credentials.APIKey, credentials.AuthType,
			credentials.BaseURL, credentials.UserAgent, credentials.UpdatedAt,
			credentials.ID)

		if err != nil {
			return fmt.Errorf("failed to update credentials: %w", err)
		}

		s.logger.Info().
			Str("id", credentials.ID).
			Str("site_domain", credentials.SiteDomain).
			Msg("Updated existing auth credentials")
	} else {
		// Create new credentials for new site
		// Ensure auth_type is set (default to 'cookie' for backward compatibility)
		if credentials.AuthType == "" {
			credentials.AuthType = "cookie"
		}

		query := `
		INSERT INTO auth_credentials (id, name, site_domain, service_type, data, cookies, tokens, api_key, auth_type, base_url, user_agent, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

		_, err = s.db.DB().ExecContext(ctx, query,
			credentials.ID, credentials.Name, credentials.SiteDomain, credentials.ServiceType,
			data, credentials.Cookies, tokens, credentials.APIKey, credentials.AuthType,
			credentials.BaseURL, credentials.UserAgent, credentials.CreatedAt, credentials.UpdatedAt)

		if err != nil {
			return fmt.Errorf("failed to insert credentials: %w", err)
		}

		s.logger.Info().
			Str("id", credentials.ID).
			Str("site_domain", credentials.SiteDomain).
			Msg("Created new auth credentials")
	}

	return nil
}

// GetCredentialsByID retrieves authentication credentials by ID
func (s *AuthStorage) GetCredentialsByID(ctx context.Context, id string) (*models.AuthCredentials, error) {
	var creds models.AuthCredentials
	var data, tokens []byte
	var cookies []byte

	query := `SELECT id, name, site_domain, service_type, data, cookies, tokens, api_key, auth_type, base_url, user_agent, created_at, updated_at
	          FROM auth_credentials WHERE id = ?`

	err := s.db.DB().QueryRowContext(ctx, query, id).Scan(
		&creds.ID, &creds.Name, &creds.SiteDomain, &creds.ServiceType,
		&data, &cookies, &tokens, &creds.APIKey, &creds.AuthType,
		&creds.BaseURL, &creds.UserAgent, &creds.CreatedAt, &creds.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	creds.Cookies = cookies

	if len(data) > 0 {
		if err := json.Unmarshal(data, &creds.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	if len(tokens) > 0 {
		if err := json.Unmarshal(tokens, &creds.Tokens); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tokens: %w", err)
		}
	}

	return &creds, nil
}

// GetCredentialsBySiteDomain retrieves authentication credentials by site domain
func (s *AuthStorage) GetCredentialsBySiteDomain(ctx context.Context, siteDomain string) (*models.AuthCredentials, error) {
	var creds models.AuthCredentials
	var data, tokens []byte
	var cookies []byte

	query := `SELECT id, name, site_domain, service_type, data, cookies, tokens, api_key, auth_type, base_url, user_agent, created_at, updated_at
	          FROM auth_credentials WHERE site_domain = ?`

	err := s.db.DB().QueryRowContext(ctx, query, siteDomain).Scan(
		&creds.ID, &creds.Name, &creds.SiteDomain, &creds.ServiceType,
		&data, &cookies, &tokens, &creds.APIKey, &creds.AuthType,
		&creds.BaseURL, &creds.UserAgent, &creds.CreatedAt, &creds.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	creds.Cookies = cookies

	if len(data) > 0 {
		if err := json.Unmarshal(data, &creds.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	if len(tokens) > 0 {
		if err := json.Unmarshal(tokens, &creds.Tokens); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tokens: %w", err)
		}
	}

	return &creds, nil
}

// DeleteCredentials deletes authentication credentials by ID
func (s *AuthStorage) DeleteCredentials(ctx context.Context, id string) error {
	query := `DELETE FROM auth_credentials WHERE id = ?`
	result, err := s.db.DB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no credentials found with id: %s", id)
	}

	s.logger.Info().Str("id", id).Msg("Deleted auth credentials")
	return nil
}

// ListCredentials returns all stored credentials
func (s *AuthStorage) ListCredentials(ctx context.Context) ([]*models.AuthCredentials, error) {
	query := `SELECT id, name, site_domain, service_type, data, cookies, tokens, api_key, auth_type, base_url, user_agent, created_at, updated_at
	          FROM auth_credentials ORDER BY name, auth_type, created_at DESC`

	rows, err := s.db.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query credentials: %w", err)
	}
	defer rows.Close()

	var credentialsList []*models.AuthCredentials
	for rows.Next() {
		var creds models.AuthCredentials
		var data, tokens []byte
		var cookies []byte

		if err := rows.Scan(&creds.ID, &creds.Name, &creds.SiteDomain, &creds.ServiceType,
			&data, &cookies, &tokens, &creds.APIKey, &creds.AuthType,
			&creds.BaseURL, &creds.UserAgent, &creds.CreatedAt, &creds.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan credentials: %w", err)
		}

		creds.Cookies = cookies

		if len(data) > 0 {
			if err := json.Unmarshal(data, &creds.Data); err != nil {
				s.logger.Warn().Str("id", creds.ID).Err(err).Msg("Failed to unmarshal data")
			}
		}

		if len(tokens) > 0 {
			if err := json.Unmarshal(tokens, &creds.Tokens); err != nil {
				s.logger.Warn().Str("id", creds.ID).Err(err).Msg("Failed to unmarshal tokens")
			}
		}

		credentialsList = append(credentialsList, &creds)
	}

	return credentialsList, rows.Err()
}

// Deprecated: Use GetCredentialsBySiteDomain instead
// GetCredentials retrieves authentication credentials for a service (legacy)
// Attempts to find credentials by service_type or site_domain matching the service name
func (s *AuthStorage) GetCredentials(ctx context.Context, service string) (*models.AuthCredentials, error) {
	// Try to find by service_type first
	query := `SELECT id, name, site_domain, service_type, data, cookies, tokens, api_key, auth_type, base_url, user_agent, created_at, updated_at
	          FROM auth_credentials WHERE service_type = ? OR site_domain LIKE ? LIMIT 1`

	var creds models.AuthCredentials
	var data, tokens []byte
	var cookies []byte

	searchPattern := "%" + service + "%"
	err := s.db.DB().QueryRowContext(ctx, query, service, searchPattern).Scan(
		&creds.ID, &creds.Name, &creds.SiteDomain, &creds.ServiceType,
		&data, &cookies, &tokens, &creds.APIKey, &creds.AuthType,
		&creds.BaseURL, &creds.UserAgent, &creds.CreatedAt, &creds.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	creds.Cookies = cookies

	if len(data) > 0 {
		if err := json.Unmarshal(data, &creds.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	if len(tokens) > 0 {
		if err := json.Unmarshal(tokens, &creds.Tokens); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tokens: %w", err)
		}
	}

	return &creds, nil
}

// Deprecated: Use ListCredentials instead
// ListServices returns a list of all service types with stored credentials
func (s *AuthStorage) ListServices(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT service_type FROM auth_credentials ORDER BY service_type`

	rows, err := s.db.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query services: %w", err)
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var service string
		if err := rows.Scan(&service); err != nil {
			return nil, fmt.Errorf("failed to scan service: %w", err)
		}
		// Convert to lowercase for backwards compatibility
		services = append(services, strings.ToLower(service))
	}

	return services, rows.Err()
}

// GetCredentialsByName retrieves authentication credentials by name
// Returns first match (name is unique per auth_type)
// Handles both API key and cookie credentials - tries exact name match first, then falls back to type-specific lookup
func (s *AuthStorage) GetCredentialsByName(ctx context.Context, name string) (*models.AuthCredentials, error) {
	var creds models.AuthCredentials
	var data, tokens []byte
	var cookies []byte

	// Handle "api:" prefix for explicit API key lookups
	lookupName := name
	if strings.HasPrefix(name, "api:") {
		lookupName = strings.TrimPrefix(name, "api:")
	}

	// Try to find by exact name match first (regardless of auth_type)
	// This allows both api_key and cookie credentials to coexist with the same name
	query := `SELECT id, name, site_domain, service_type, data, cookies, tokens, api_key, auth_type, base_url, user_agent, created_at, updated_at
	          FROM auth_credentials WHERE name = ? LIMIT 1`

	err := s.db.DB().QueryRowContext(ctx, query, lookupName).Scan(
		&creds.ID, &creds.Name, &creds.SiteDomain, &creds.ServiceType,
		&data, &cookies, &tokens, &creds.APIKey, &creds.AuthType,
		&creds.BaseURL, &creds.UserAgent, &creds.CreatedAt, &creds.UpdatedAt)

	if err == sql.ErrNoRows {
		// If no exact match found and this is an API key lookup (with or without "api:" prefix),
		// try looking specifically for api_key entries as fallback
		if strings.HasPrefix(name, "api:") || isLikelyAPIKeyName(name) {
			query = `SELECT id, name, site_domain, service_type, data, cookies, tokens, api_key, auth_type, base_url, user_agent, created_at, updated_at
			          FROM auth_credentials WHERE name = ? AND auth_type = 'api_key' LIMIT 1`
			err = s.db.DB().QueryRowContext(ctx, query, lookupName).Scan(
				&creds.ID, &creds.Name, &creds.SiteDomain, &creds.ServiceType,
				&data, &cookies, &tokens, &creds.APIKey, &creds.AuthType,
				&creds.BaseURL, &creds.UserAgent, &creds.CreatedAt, &creds.UpdatedAt)
		}
	}

	if err != nil {
		return nil, s.handleGetCredentialsError(err, name)
	}

	creds.Cookies = cookies

	if len(data) > 0 {
		if err := json.Unmarshal(data, &creds.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	if len(tokens) > 0 {
		if err := json.Unmarshal(tokens, &creds.Tokens); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tokens: %w", err)
		}
	}

	return &creds, nil
}

// isLikelyAPIKeyName checks if a credential name is likely to be an API key based on naming patterns
func isLikelyAPIKeyName(name string) bool {
	// Check for common API key name patterns
	lowerName := strings.ToLower(name)

	// Names containing "key" are likely API keys
	if strings.Contains(lowerName, "key") {
		return true
	}

	// Names for common API-based services
	apiServices := []string{"google", "gemini", "places", "llm", "openai", "anthropic"}
	for _, service := range apiServices {
		if strings.Contains(lowerName, service) {
			return true
		}
	}

	return false
}

// handleGetCredentialsError handles errors from GetCredentialsByName
func (s *AuthStorage) handleGetCredentialsError(err error, name string) error {
	if err == sql.ErrNoRows {
		return nil
	}
	return fmt.Errorf("failed to get credentials by name: %w", err)
}

// GetAPIKeyByName retrieves an API key by name
// Returns error if not found or if auth_type is not 'api_key'
func (s *AuthStorage) GetAPIKeyByName(ctx context.Context, name string) (string, error) {
	var apiKey string
	var authType string

	query := `SELECT api_key, auth_type FROM auth_credentials WHERE name = ? AND auth_type = 'api_key' LIMIT 1`

	err := s.db.DB().QueryRowContext(ctx, query, name).Scan(&apiKey, &authType)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("API key '%s' not found in auth storage", name)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get API key by name: %w", err)
	}

	// Validate API key is not empty
	if apiKey == "" {
		s.logger.Warn().Str("name", name).Msg("API key entry found but API key value is empty")
		return "", fmt.Errorf("API key '%s' exists but value is empty", name)
	}

	s.logger.Debug().Str("name", name).Msg("Resolved API key from auth storage")
	return apiKey, nil
}
