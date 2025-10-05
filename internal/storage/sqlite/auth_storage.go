package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
func (s *AuthStorage) StoreCredentials(ctx context.Context, credentials *models.AuthCredentials) error {
	data, err := json.Marshal(credentials.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	tokens, err := json.Marshal(credentials.Tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	query := `
	INSERT INTO auth_credentials (service, data, cookies, tokens, base_url, user_agent, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(service) DO UPDATE SET
		data = excluded.data,
		cookies = excluded.cookies,
		tokens = excluded.tokens,
		base_url = excluded.base_url,
		user_agent = excluded.user_agent,
		updated_at = excluded.updated_at`

	_, err = s.db.DB().ExecContext(ctx, query,
		credentials.Service, data, credentials.Cookies, tokens,
		credentials.BaseURL, credentials.UserAgent, credentials.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to store credentials: %w", err)
	}

	s.logger.Debug().Str("service", credentials.Service).Msg("Stored auth credentials")
	return nil
}

// GetCredentials retrieves authentication credentials for a service
func (s *AuthStorage) GetCredentials(ctx context.Context, service string) (*models.AuthCredentials, error) {
	var data, tokens []byte
	var cookies []byte
	var baseURL, userAgent string
	var updatedAt int64

	query := `SELECT data, cookies, tokens, base_url, user_agent, updated_at 
	          FROM auth_credentials WHERE service = ?`

	err := s.db.DB().QueryRowContext(ctx, query, service).Scan(
		&data, &cookies, &tokens, &baseURL, &userAgent, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	creds := &models.AuthCredentials{
		Service:   service,
		Cookies:   cookies,
		BaseURL:   baseURL,
		UserAgent: userAgent,
		UpdatedAt: updatedAt,
	}

	if err := json.Unmarshal(data, &creds.Data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if err := json.Unmarshal(tokens, &creds.Tokens); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tokens: %w", err)
	}

	return creds, nil
}

// DeleteCredentials deletes authentication credentials for a service
func (s *AuthStorage) DeleteCredentials(ctx context.Context, service string) error {
	query := `DELETE FROM auth_credentials WHERE service = ?`
	_, err := s.db.DB().ExecContext(ctx, query, service)
	if err != nil {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	s.logger.Debug().Str("service", service).Msg("Deleted auth credentials")
	return nil
}

// ListServices returns a list of all services with stored credentials
func (s *AuthStorage) ListServices(ctx context.Context) ([]string, error) {
	query := `SELECT service FROM auth_credentials ORDER BY service`

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
		services = append(services, service)
	}

	return services, rows.Err()
}
