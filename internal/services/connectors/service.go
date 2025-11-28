package connectors

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Service implements interfaces.ConnectorService
type Service struct {
	storage interfaces.ConnectorStorage
	logger  arbor.ILogger
}

// NewService creates a new connector service
func NewService(storage interfaces.ConnectorStorage, logger arbor.ILogger) *Service {
	return &Service{
		storage: storage,
		logger:  logger,
	}
}

// CreateConnector creates a new connector
func (s *Service) CreateConnector(ctx context.Context, connector *models.Connector) error {
	if connector.ID == "" {
		connector.ID = uuid.New().String()
	}
	now := time.Now()
	connector.CreatedAt = now
	connector.UpdatedAt = now

	if err := s.storage.SaveConnector(ctx, connector); err != nil {
		return fmt.Errorf("failed to create connector: %w", err)
	}

	s.logger.Debug().Str("connector_id", connector.ID).Msg("Connector created")
	return nil
}

// GetConnector retrieves a connector by ID
func (s *Service) GetConnector(ctx context.Context, id string) (*models.Connector, error) {
	return s.storage.GetConnector(ctx, id)
}

// GetConnectorByName retrieves a connector by name
func (s *Service) GetConnectorByName(ctx context.Context, name string) (*models.Connector, error) {
	connectors, err := s.storage.ListConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list connectors: %w", err)
	}

	for _, c := range connectors {
		if c.Name == name {
			return c, nil
		}
	}

	return nil, fmt.Errorf("connector not found: %s", name)
}

// ListConnectors retrieves all connectors
func (s *Service) ListConnectors(ctx context.Context) ([]*models.Connector, error) {
	return s.storage.ListConnectors(ctx)
}

// UpdateConnector updates an existing connector
func (s *Service) UpdateConnector(ctx context.Context, connector *models.Connector) error {
	connector.UpdatedAt = time.Now()
	if err := s.storage.UpdateConnector(ctx, connector); err != nil {
		return fmt.Errorf("failed to update connector: %w", err)
	}

	s.logger.Debug().Str("connector_id", connector.ID).Msg("Connector updated")
	return nil
}

// DeleteConnector deletes a connector
func (s *Service) DeleteConnector(ctx context.Context, id string) error {
	if err := s.storage.DeleteConnector(ctx, id); err != nil {
		return fmt.Errorf("failed to delete connector: %w", err)
	}

	s.logger.Debug().Str("connector_id", id).Msg("Connector deleted")
	return nil
}
