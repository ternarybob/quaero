package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// ConnectorService defines operations for managing connectors
type ConnectorService interface {
	CreateConnector(ctx context.Context, connector *models.Connector) error
	GetConnector(ctx context.Context, id string) (*models.Connector, error)
	ListConnectors(ctx context.Context) ([]*models.Connector, error)
	UpdateConnector(ctx context.Context, connector *models.Connector) error
	DeleteConnector(ctx context.Context, id string) error
}

// Connector defines the common interface for all connector implementations
type Connector interface {
	// TestConnection verifies if the connector configuration is valid and working
	TestConnection(ctx context.Context) error
	// Type returns the connector type
	Type() models.ConnectorType
}

// GitHubConnector defines specific operations for GitHub
type GitHubConnector interface {
	Connector
	GetJobLog(ctx context.Context, owner, repo string, jobID int64) (string, error)
}
