package connectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// Service implements interfaces.ConnectorService
type Service struct {
	db     *sql.DB
	logger arbor.ILogger
}

// NewService creates a new connector service
func NewService(db *sql.DB, logger arbor.ILogger) *Service {
	return &Service{
		db:     db,
		logger: logger,
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

	configJSON, err := json.Marshal(connector.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO connectors (id, name, type, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err = s.db.ExecContext(ctx, query,
		connector.ID,
		connector.Name,
		connector.Type,
		string(configJSON),
		connector.CreatedAt.Unix(),
		connector.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert connector: %w", err)
	}

	s.logger.Info().Str("connector_id", connector.ID).Msg("Connector created")
	return nil
}

// GetConnector retrieves a connector by ID
func (s *Service) GetConnector(ctx context.Context, id string) (*models.Connector, error) {
	query := `
		SELECT id, name, type, config, created_at, updated_at
		FROM connectors
		WHERE id = ?
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var c models.Connector
	var configStr string
	var createdAt, updatedAt int64

	err := row.Scan(&c.ID, &c.Name, &c.Type, &configStr, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("connector not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan connector: %w", err)
	}

	c.Config = json.RawMessage(configStr)
	c.CreatedAt = time.Unix(createdAt, 0)
	c.UpdatedAt = time.Unix(updatedAt, 0)

	return &c, nil
}

// ListConnectors retrieves all connectors
func (s *Service) ListConnectors(ctx context.Context) ([]*models.Connector, error) {
	query := `
		SELECT id, name, type, config, created_at, updated_at
		FROM connectors
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query connectors: %w", err)
	}
	defer rows.Close()

	var connectors []*models.Connector
	for rows.Next() {
		var c models.Connector
		var configStr string
		var createdAt, updatedAt int64

		err := rows.Scan(&c.ID, &c.Name, &c.Type, &configStr, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan connector: %w", err)
		}

		c.Config = json.RawMessage(configStr)
		c.CreatedAt = time.Unix(createdAt, 0)
		c.UpdatedAt = time.Unix(updatedAt, 0)
		connectors = append(connectors, &c)
	}

	return connectors, nil
}

// UpdateConnector updates an existing connector
func (s *Service) UpdateConnector(ctx context.Context, connector *models.Connector) error {
	connector.UpdatedAt = time.Now()

	configJSON, err := json.Marshal(connector.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		UPDATE connectors
		SET name = ?, type = ?, config = ?, updated_at = ?
		WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, query,
		connector.Name,
		connector.Type,
		string(configJSON),
		connector.UpdatedAt.Unix(),
		connector.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update connector: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("connector not found: %s", connector.ID)
	}

	s.logger.Info().Str("connector_id", connector.ID).Msg("Connector updated")
	return nil
}

// DeleteConnector deletes a connector
func (s *Service) DeleteConnector(ctx context.Context, id string) error {
	query := `DELETE FROM connectors WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete connector: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("connector not found: %s", id)
	}

	s.logger.Info().Str("connector_id", id).Msg("Connector deleted")
	return nil
}
