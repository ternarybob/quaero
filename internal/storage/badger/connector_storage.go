package badger

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

// ConnectorStorage implements the ConnectorStorage interface for Badger
type ConnectorStorage struct {
	db     *BadgerDB
	logger arbor.ILogger
}

// NewConnectorStorage creates a new ConnectorStorage instance
func NewConnectorStorage(db *BadgerDB, logger arbor.ILogger) interfaces.ConnectorStorage {
	return &ConnectorStorage{
		db:     db,
		logger: logger,
	}
}

func (s *ConnectorStorage) SaveConnector(ctx context.Context, connector *models.Connector) error {
	if connector.ID == "" {
		return fmt.Errorf("connector ID is required")
	}
	if err := s.db.Store().Upsert(connector.ID, connector); err != nil {
		return fmt.Errorf("failed to save connector: %w", err)
	}
	return nil
}

func (s *ConnectorStorage) GetConnector(ctx context.Context, id string) (*models.Connector, error) {
	var connector models.Connector
	if err := s.db.Store().Get(id, &connector); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, fmt.Errorf("connector not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get connector: %w", err)
	}
	return &connector, nil
}

func (s *ConnectorStorage) ListConnectors(ctx context.Context) ([]*models.Connector, error) {
	var connectors []models.Connector
	// Order by CreatedAt DESC to match SQL implementation
	if err := s.db.Store().Find(&connectors, badgerhold.Where("ID").Ne("").SortBy("CreatedAt").Reverse()); err != nil {
		return nil, fmt.Errorf("failed to list connectors: %w", err)
	}

	result := make([]*models.Connector, len(connectors))
	for i := range connectors {
		result[i] = &connectors[i]
	}
	return result, nil
}

func (s *ConnectorStorage) UpdateConnector(ctx context.Context, connector *models.Connector) error {
	return s.SaveConnector(ctx, connector)
}

func (s *ConnectorStorage) DeleteConnector(ctx context.Context, id string) error {
	if err := s.db.Store().Delete(id, &models.Connector{}); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete connector: %w", err)
	}
	return nil
}

// DeleteAllConnectors removes all connectors from storage
// Used by config reload to clear all connectors before reloading from TOML files
func (s *ConnectorStorage) DeleteAllConnectors(ctx context.Context) error {
	// Find all connectors
	var connectors []models.Connector
	if err := s.db.Store().Find(&connectors, nil); err != nil {
		return fmt.Errorf("failed to list connectors for deletion: %w", err)
	}

	// Delete each connector
	for _, connector := range connectors {
		if err := s.db.Store().Delete(connector.ID, &models.Connector{}); err != nil {
			s.logger.Warn().Str("id", connector.ID).Err(err).Msg("Failed to delete connector during DeleteAll")
		}
	}

	s.logger.Info().Int("count", len(connectors)).Msg("Deleted all connectors")
	return nil
}
