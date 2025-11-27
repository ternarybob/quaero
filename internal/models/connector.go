package models

import (
	"encoding/json"
	"errors"
	"time"
)

// ConnectorType defines the type of connector
type ConnectorType string

const (
	ConnectorTypeGitHub ConnectorType = "github"
	ConnectorTypeGitLab ConnectorType = "gitlab"
)

// Connector represents an external service connection
type Connector struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Type      ConnectorType   `json:"type"`
	Config    json.RawMessage `json:"config"` // Stored as JSON in DB
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ConnectorConfig is a marker interface for connector configurations
type ConnectorConfig interface {
	Validate() error
}

// GitHubConnectorConfig defines configuration for GitHub connectors
type GitHubConnectorConfig struct {
	Token string `json:"token"`
}

func (c *GitHubConnectorConfig) Validate() error {
	if c.Token == "" {
		return errors.New("token is required")
	}
	return nil
}

// GitLabConnectorConfig defines configuration for GitLab connectors
type GitLabConnectorConfig struct {
	Token string `json:"token"`
}

func (c *GitLabConnectorConfig) Validate() error {
	if c.Token == "" {
		return errors.New("token is required")
	}
	return nil
}
