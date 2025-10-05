package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

const (
	atlassianServiceName = "atlassian"
)

// AtlassianAuthService manages authentication for Atlassian services
type AtlassianAuthService struct {
	client      *http.Client
	baseURL     string
	userAgent   string
	cloudID     string
	atlToken    string
	authStorage interfaces.AuthStorage
	logger      arbor.ILogger
}

// NewAtlassianAuthService creates a new Atlassian authentication service
func NewAtlassianAuthService(authStorage interfaces.AuthStorage, logger arbor.ILogger) (*AtlassianAuthService, error) {
	service := &AtlassianAuthService{
		authStorage: authStorage,
		logger:      logger,
	}

	if err := service.loadStoredAuth(); err != nil {
		logger.Debug().Str("error", err.Error()).Msg("No stored authentication found")
	}

	return service, nil
}

func (s *AtlassianAuthService) loadStoredAuth() error {
	authData, err := s.LoadAuth()
	if err != nil {
		return err
	}

	if err := s.UpdateAuth(authData); err != nil {
		return fmt.Errorf("failed to apply stored auth: %w", err)
	}

	s.logger.Info().Msg("Successfully loaded stored authentication")
	return nil
}

// UpdateAuth updates authentication state and configures HTTP client
func (s *AtlassianAuthService) UpdateAuth(authData *interfaces.AtlassianAuthData) error {
	if err := s.configureHTTPClient(authData); err != nil {
		return fmt.Errorf("failed to configure HTTP client: %w", err)
	}

	s.extractAuthDetails(authData)

	return s.storeAuth(authData)
}

func (s *AtlassianAuthService) configureHTTPClient(authData *interfaces.AtlassianAuthData) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("failed to create cookie jar: %w", err)
	}

	s.client = &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	baseURL, err := url.Parse(authData.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	s.client.Jar.SetCookies(baseURL, authData.GetHTTPCookies())
	s.baseURL = authData.BaseURL
	s.userAgent = authData.UserAgent

	return nil
}

func (s *AtlassianAuthService) extractAuthDetails(authData *interfaces.AtlassianAuthData) {
	if cloudID, ok := authData.Tokens["cloudId"].(string); ok {
		s.cloudID = cloudID
		s.logger.Debug().Str("cloudId", cloudID).Msg("CloudID extracted")
	} else {
		s.logger.Warn().Msg("CloudID not found in auth tokens")
	}

	if atlToken, ok := authData.Tokens["atlToken"].(string); ok {
		s.atlToken = atlToken
		s.logger.Debug().Msg("atlToken extracted")
	} else {
		s.logger.Warn().Msg("atlToken not found in auth tokens")
	}
}

func (s *AtlassianAuthService) storeAuth(authData *interfaces.AtlassianAuthData) error {
	ctx := context.Background()

	cookiesJSON, err := json.Marshal(authData.Cookies)
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	tokens := make(map[string]string)
	for k, v := range authData.Tokens {
		if str, ok := v.(string); ok {
			tokens[k] = str
		}
	}

	credentials := &models.AuthCredentials{
		Service:   atlassianServiceName,
		Cookies:   cookiesJSON,
		Tokens:    tokens,
		BaseURL:   authData.BaseURL,
		UserAgent: authData.UserAgent,
		UpdatedAt: time.Now().Unix(),
	}

	return s.authStorage.StoreCredentials(ctx, credentials)
}

// IsAuthenticated checks if valid authentication exists
func (s *AtlassianAuthService) IsAuthenticated() bool {
	return s.client != nil && s.baseURL != ""
}

// LoadAuth loads authentication from database
func (s *AtlassianAuthService) LoadAuth() (*interfaces.AtlassianAuthData, error) {
	ctx := context.Background()

	credentials, err := s.authStorage.GetCredentials(ctx, atlassianServiceName)
	if err != nil {
		return nil, err
	}

	// No credentials stored yet
	if credentials == nil {
		return nil, fmt.Errorf("no credentials found")
	}

	var cookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(credentials.Cookies, &cookies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	tokens := make(map[string]interface{})
	for k, v := range credentials.Tokens {
		tokens[k] = v
	}

	authData := &interfaces.AtlassianAuthData{
		BaseURL:   credentials.BaseURL,
		UserAgent: credentials.UserAgent,
		Cookies:   cookies,
		Tokens:    tokens,
	}

	return authData, nil
}

// GetHTTPClient returns configured HTTP client with cookies
func (s *AtlassianAuthService) GetHTTPClient() *http.Client {
	return s.client
}

// GetBaseURL returns the base URL for API requests
func (s *AtlassianAuthService) GetBaseURL() string {
	return s.baseURL
}

// GetUserAgent returns the user agent string
func (s *AtlassianAuthService) GetUserAgent() string {
	return s.userAgent
}

// GetCloudID returns the Atlassian cloud ID
func (s *AtlassianAuthService) GetCloudID() string {
	return s.cloudID
}

// GetAtlToken returns the atl_token for CSRF protection
func (s *AtlassianAuthService) GetAtlToken() string {
	return s.atlToken
}
