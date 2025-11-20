package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v57/github"
	"github.com/ternarybob/quaero/internal/githublogs"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"golang.org/x/oauth2"
)

// Connector implements interfaces.GitHubConnector
type Connector struct {
	client *github.Client
}

// NewConnector creates a new GitHub connector from a generic connector model
func NewConnector(c *models.Connector) (*Connector, error) {
	if c.Type != models.ConnectorTypeGitHub {
		return nil, fmt.Errorf("invalid connector type: %s", c.Type)
	}

	var config models.GitHubConnectorConfig
	if err := json.Unmarshal(c.Config, &config); err != nil {
		return nil, fmt.Errorf("invalid github config: %w", err)
	}

	if config.Token == "" {
		return nil, fmt.Errorf("github token is required")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &Connector{client: client}, nil
}

// TestConnection verifies the token works by getting the authenticated user
func (c *Connector) TestConnection(ctx context.Context) error {
	_, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("github connection test failed: %w", err)
	}
	return nil
}

// Type returns the connector type
func (c *Connector) Type() models.ConnectorType {
	return models.ConnectorTypeGitHub
}

// GetJobLog fetches and sanitizes the job log
func (c *Connector) GetJobLog(ctx context.Context, owner, repo string, jobID int64) (string, error) {
	rawLog, err := githublogs.GetJobLog(ctx, c.client, owner, repo, jobID)
	if err != nil {
		return "", err
	}
	return githublogs.SanitizeLogForAI(rawLog), nil
}

// Ensure interface compliance
var _ interfaces.GitHubConnector = (*Connector)(nil)
