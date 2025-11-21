package badger

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

// AuthStorage implements the AuthStorage interface for Badger
type AuthStorage struct {
	db     *BadgerDB
	logger arbor.ILogger
}

// NewAuthStorage creates a new AuthStorage instance
func NewAuthStorage(db *BadgerDB, logger arbor.ILogger) interfaces.AuthStorage {
	return &AuthStorage{
		db:     db,
		logger: logger,
	}
}

func (s *AuthStorage) StoreCredentials(ctx context.Context, credentials *models.AuthCredentials) error {
	if credentials.ID == "" {
		return fmt.Errorf("credentials ID is required")
	}

	now := time.Now().Unix()
	if credentials.CreatedAt == 0 {
		credentials.CreatedAt = now
	}
	credentials.UpdatedAt = now

	if err := s.db.Store().Upsert(credentials.ID, credentials); err != nil {
		return fmt.Errorf("failed to store credentials: %w", err)
	}
	return nil
}

func (s *AuthStorage) GetCredentialsByID(ctx context.Context, id string) (*models.AuthCredentials, error) {
	var creds models.AuthCredentials
	if err := s.db.Store().Get(id, &creds); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, fmt.Errorf("credentials not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}
	return &creds, nil
}

func (s *AuthStorage) GetCredentialsBySiteDomain(ctx context.Context, siteDomain string) (*models.AuthCredentials, error) {
	var creds []models.AuthCredentials
	if err := s.db.Store().Find(&creds, badgerhold.Where("SiteDomain").Eq(siteDomain)); err != nil {
		return nil, fmt.Errorf("failed to find credentials: %w", err)
	}
	if len(creds) == 0 {
		return nil, fmt.Errorf("credentials not found for domain: %s", siteDomain)
	}
	return &creds[0], nil
}

func (s *AuthStorage) DeleteCredentials(ctx context.Context, id string) error {
	if err := s.db.Store().Delete(id, &models.AuthCredentials{}); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete credentials: %w", err)
	}
	return nil
}

func (s *AuthStorage) ListCredentials(ctx context.Context) ([]*models.AuthCredentials, error) {
	var creds []models.AuthCredentials
	if err := s.db.Store().Find(&creds, nil); err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	result := make([]*models.AuthCredentials, len(creds))
	for i := range creds {
		result[i] = &creds[i]
	}
	return result, nil
}

// Deprecated: Use GetCredentialsBySiteDomain instead
func (s *AuthStorage) GetCredentials(ctx context.Context, service string) (*models.AuthCredentials, error) {
	return s.GetCredentialsBySiteDomain(ctx, service)
}

// Deprecated: Use ListCredentials instead
func (s *AuthStorage) ListServices(ctx context.Context) ([]string, error) {
	creds, err := s.ListCredentials(ctx)
	if err != nil {
		return nil, err
	}
	services := make([]string, len(creds))
	for i, c := range creds {
		services[i] = c.SiteDomain
	}
	return services, nil
}