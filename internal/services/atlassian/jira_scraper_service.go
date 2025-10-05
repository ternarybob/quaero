package atlassian

import (
	"fmt"
	"io"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// JiraScraperService scrapes Jira projects and issues
type JiraScraperService struct {
	authService interfaces.AtlassianAuthService
	jiraStorage interfaces.JiraStorage
	logger      arbor.ILogger
	uiLogger    interface{}
}

// NewJiraScraperService creates a new Jira scraper service
func NewJiraScraperService(jiraStorage interfaces.JiraStorage, authService interfaces.AtlassianAuthService, logger arbor.ILogger) *JiraScraperService {
	return &JiraScraperService{
		jiraStorage: jiraStorage,
		authService: authService,
		logger:      logger,
	}
}

// Close closes the scraper and releases resources
func (s *JiraScraperService) Close() error {
	return nil
}

// SetUILogger sets a UI logger for real-time updates
func (s *JiraScraperService) SetUILogger(logger interface{}) {
	s.uiLogger = logger
}

func (s *JiraScraperService) makeRequest(method, path string) ([]byte, error) {
	reqURL := s.authService.GetBaseURL() + path

	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.authService.GetUserAgent())
	req.Header.Set("Accept", "application/json, text/html")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.authService.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		s.logger.Error().
			Str("url", reqURL).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("HTTP request failed")

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("auth expired (status %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, readErr
}
