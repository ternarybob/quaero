package atlassian

import (
	"fmt"
	"io"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	bolt "go.etcd.io/bbolt"
)

const (
	spacesBucket = "confluence_spaces"
	pagesBucket  = "confluence_pages"
)

// ConfluenceScraperService scrapes Confluence spaces and pages
type ConfluenceScraperService struct {
	authService interfaces.AtlassianAuthService
	db          *bolt.DB
	logger      arbor.ILogger
	uiLogger    interface{}
}

// NewConfluenceScraperService creates a new Confluence scraper service
func NewConfluenceScraperService(db *bolt.DB, authService interfaces.AtlassianAuthService, logger arbor.ILogger) (*ConfluenceScraperService, error) {
	if err := createConfluenceBuckets(db); err != nil {
		return nil, err
	}

	return &ConfluenceScraperService{
		db:          db,
		authService: authService,
		logger:      logger,
	}, nil
}

func createConfluenceBuckets(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(spacesBucket)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(pagesBucket)); err != nil {
			return err
		}
		return nil
	})
}

// Close closes the scraper and releases resources
func (s *ConfluenceScraperService) Close() error {
	return nil
}

// SetUILogger sets a UI logger for real-time updates
func (s *ConfluenceScraperService) SetUILogger(logger interface{}) {
	s.uiLogger = logger
}

// ScrapeConfluence is an alias for ScrapeSpaces for compatibility
func (s *ConfluenceScraperService) ScrapeConfluence() error {
	return s.ScrapeSpaces()
}

func (s *ConfluenceScraperService) makeRequest(method, path string) ([]byte, error) {
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
