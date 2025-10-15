package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Service manages source configurations
type Service struct {
	storage      interfaces.SourceStorage
	authStorage  interfaces.AuthStorage
	eventService interfaces.EventService
	logger       arbor.ILogger
}

// NewService creates a new SourceService
func NewService(storage interfaces.SourceStorage, authStorage interfaces.AuthStorage, eventService interfaces.EventService, logger arbor.ILogger) *Service {
	return &Service{
		storage:      storage,
		authStorage:  authStorage,
		eventService: eventService,
		logger:       logger,
	}
}

// extractSiteDomain extracts the site domain from a URL
func extractSiteDomain(baseURL string) string {
	// Parse the URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	// Get the hostname
	host := u.Hostname()

	// Remove www. prefix if present
	host = strings.TrimPrefix(host, "www.")

	return host
}

// CreateSource validates and creates a new source
func (s *Service) CreateSource(ctx context.Context, source *models.SourceConfig) error {
	// Generate UUID if not provided
	if source.ID == "" {
		source.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	source.CreatedAt = now
	source.UpdatedAt = now

	// Validate source configuration
	if err := source.Validate(); err != nil {
		return fmt.Errorf("source validation failed: %w", err)
	}

	// Extract site domain from base URL
	siteDomain := extractSiteDomain(source.BaseURL)

	// Validate authentication if provided
	if source.AuthID != "" {
		authCreds, err := s.authStorage.GetCredentialsByID(ctx, source.AuthID)
		if err != nil {
			return fmt.Errorf("authentication not found: %w", err)
		}

		// Verify that the auth domain matches the source domain
		if authCreds.SiteDomain != siteDomain {
			s.logger.Warn().
				Str("auth_domain", authCreds.SiteDomain).
				Str("source_domain", siteDomain).
				Msg("Authentication domain does not match source URL domain")
			// This is a warning, not an error - allow mismatched domains
			// as user may have specific reasons for this setup
		}
	}

	// Save to storage
	if err := s.storage.SaveSource(ctx, source); err != nil {
		return fmt.Errorf("failed to save source: %w", err)
	}

	s.logger.Info().
		Str("id", source.ID).
		Str("name", source.Name).
		Str("type", source.Type).
		Str("site_domain", siteDomain).
		Str("has_auth", fmt.Sprintf("%v", source.AuthID != "")).
		Msg("Source created successfully")

	// Publish event
	event := interfaces.Event{
		Type: interfaces.EventSourceCreated,
		Payload: map[string]interface{}{
			"source_id":   source.ID,
			"source_type": source.Type,
			"source_name": source.Name,
			"site_domain": siteDomain,
			"has_auth":    source.AuthID != "",
			"timestamp":   time.Now(),
		},
	}
	s.eventService.Publish(ctx, event)

	return nil
}

// UpdateSource validates and updates an existing source
func (s *Service) UpdateSource(ctx context.Context, source *models.SourceConfig) error {
	// Validate source configuration
	if err := source.Validate(); err != nil {
		return fmt.Errorf("source validation failed: %w", err)
	}

	// Check if source exists
	existing, err := s.storage.GetSource(ctx, source.ID)
	if err != nil {
		return fmt.Errorf("source not found: %w", err)
	}

	// Preserve created_at timestamp
	source.CreatedAt = existing.CreatedAt
	source.UpdatedAt = time.Now()

	// Extract site domain from base URL
	siteDomain := extractSiteDomain(source.BaseURL)

	// Validate authentication if provided
	if source.AuthID != "" {
		authCreds, err := s.authStorage.GetCredentialsByID(ctx, source.AuthID)
		if err != nil {
			return fmt.Errorf("authentication not found: %w", err)
		}

		// Verify that the auth domain matches the source domain
		if authCreds.SiteDomain != siteDomain {
			s.logger.Warn().
				Str("auth_domain", authCreds.SiteDomain).
				Str("source_domain", siteDomain).
				Msg("Authentication domain does not match source URL domain")
			// This is a warning, not an error - allow mismatched domains
			// as user may have specific reasons for this setup
		}
	}

	// Save to storage
	if err := s.storage.SaveSource(ctx, source); err != nil {
		return fmt.Errorf("failed to update source: %w", err)
	}

	s.logger.Info().
		Str("id", source.ID).
		Str("name", source.Name).
		Str("type", source.Type).
		Str("site_domain", siteDomain).
		Str("has_auth", fmt.Sprintf("%v", source.AuthID != "")).
		Msg("Source updated successfully")

	// Publish event
	event := interfaces.Event{
		Type: interfaces.EventSourceUpdated,
		Payload: map[string]interface{}{
			"source_id":   source.ID,
			"source_type": source.Type,
			"source_name": source.Name,
			"site_domain": siteDomain,
			"has_auth":    source.AuthID != "",
			"timestamp":   time.Now(),
		},
	}
	s.eventService.Publish(ctx, event)

	return nil
}

// GetSource retrieves a source by ID
func (s *Service) GetSource(ctx context.Context, id string) (*models.SourceConfig, error) {
	source, err := s.storage.GetSource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}
	return source, nil
}

// ListSources retrieves all sources
func (s *Service) ListSources(ctx context.Context) ([]*models.SourceConfig, error) {
	sources, err := s.storage.ListSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	return sources, nil
}

// DeleteSource deletes a source by ID
func (s *Service) DeleteSource(ctx context.Context, id string) error {
	// Get source info before deletion for event
	source, err := s.storage.GetSource(ctx, id)
	if err != nil {
		return fmt.Errorf("source not found: %w", err)
	}

	// Delete from storage
	if err := s.storage.DeleteSource(ctx, id); err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	s.logger.Info().
		Str("id", id).
		Str("name", source.Name).
		Str("type", source.Type).
		Msg("Source deleted successfully")

	// Publish event
	event := interfaces.Event{
		Type: interfaces.EventSourceDeleted,
		Payload: map[string]interface{}{
			"source_id":   id,
			"source_type": source.Type,
			"source_name": source.Name,
			"timestamp":   time.Now(),
		},
	}
	s.eventService.Publish(ctx, event)

	return nil
}

// GetEnabledSources retrieves only enabled sources
func (s *Service) GetEnabledSources(ctx context.Context) ([]*models.SourceConfig, error) {
	sources, err := s.storage.GetEnabledSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled sources: %w", err)
	}
	return sources, nil
}
