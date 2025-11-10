package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/httpclient"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

const (
	// ServiceNameAtlassian represents Atlassian services (Jira, Confluence)
	// The auth service is generic and can support any authenticated site via cookie capture
	ServiceNameAtlassian = "atlassian"
)

// Service manages generic authentication for web services via cookie/token capture
type Service struct {
	serviceName string
	client      *http.Client
	baseURL     string
	userAgent   string

	// Atlassian-specific fields
	cloudID  string
	atlToken string

	authStorage interfaces.AuthStorage
	logger      arbor.ILogger
}

// NewAtlassianAuthService creates a new authentication service for Atlassian
func NewAtlassianAuthService(authStorage interfaces.AuthStorage, logger arbor.ILogger) (*Service, error) {
	service := &Service{
		serviceName: ServiceNameAtlassian,
		authStorage: authStorage,
		logger:      logger,
	}

	if err := service.loadStoredAuth(); err != nil {
		logger.Debug().Str("error", err.Error()).Msg("No stored authentication found")
	}

	return service, nil
}

// NewService creates a new generic authentication service
func NewService(serviceName string, authStorage interfaces.AuthStorage, logger arbor.ILogger) (*Service, error) {
	service := &Service{
		serviceName: serviceName,
		authStorage: authStorage,
		logger:      logger,
	}

	if err := service.loadStoredAuth(); err != nil {
		logger.Debug().Str("error", err.Error()).Msg("No stored authentication found")
	}

	return service, nil
}

func (s *Service) loadStoredAuth() error {
	authData, err := s.LoadAuth()
	if err != nil {
		return err
	}

	// Service-specific loading
	switch s.serviceName {
	case ServiceNameAtlassian:
		return s.UpdateAuth(authData)
	default:
		return fmt.Errorf("unsupported service: %s", s.serviceName)
	}
}

// UpdateAuth updates authentication state (implements AtlassianAuthService interface)
func (s *Service) UpdateAuth(authData *interfaces.AtlassianAuthData) error {
	if err := s.configureHTTPClient(authData); err != nil {
		return fmt.Errorf("failed to configure HTTP client: %w", err)
	}

	s.extractAtlassianDetails(authData)

	return s.storeAtlassianAuth(authData)
}

func (s *Service) configureHTTPClient(authData *interfaces.AtlassianAuthData) error {
	client, err := httpclient.NewHTTPClientFromAtlassianAuth(authData)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	s.client = client
	s.baseURL = authData.BaseURL
	s.userAgent = authData.UserAgent

	return nil
}

func (s *Service) extractAtlassianDetails(authData *interfaces.AtlassianAuthData) {
	if cloudID, ok := authData.Tokens["cloudId"].(string); ok {
		s.cloudID = cloudID
		s.logger.Debug().Str("cloudId", cloudID).Msg("CloudID extracted")
	} else {
		s.logger.Debug().Msg("CloudID not found in auth tokens")
	}

	if atlToken, ok := authData.Tokens["atlToken"].(string); ok {
		s.atlToken = atlToken
		s.logger.Debug().Msg("atlToken extracted")
	} else {
		s.logger.Debug().Msg("atlToken not found in auth tokens")
	}
}

func (s *Service) storeAtlassianAuth(authData *interfaces.AtlassianAuthData) error {
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

	// Extract site domain from base URL
	baseURL, err := url.Parse(authData.BaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse base URL: %w", err)
	}
	siteDomain := baseURL.Host

	credentials := &models.AuthCredentials{
		ServiceType: s.serviceName,
		SiteDomain:  siteDomain,
		Cookies:     cookiesJSON,
		Tokens:      tokens,
		BaseURL:     authData.BaseURL,
		UserAgent:   authData.UserAgent,
	}

	return s.authStorage.StoreCredentials(ctx, credentials)
}

// IsAuthenticated checks if valid authentication exists
func (s *Service) IsAuthenticated() bool {
	return s.client != nil && s.baseURL != ""
}

// LoadAuth loads authentication from database (implements AtlassianAuthService interface)
func (s *Service) LoadAuth() (*interfaces.AtlassianAuthData, error) {
	ctx := context.Background()

	credentials, err := s.authStorage.GetCredentials(ctx, s.serviceName)
	if err != nil {
		return nil, err
	}

	// No credentials stored yet
	if credentials == nil {
		return nil, fmt.Errorf("no credentials found")
	}

	// Unmarshal cookies
	var cookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(credentials.Cookies, &cookies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	// Convert tokens map
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
func (s *Service) GetHTTPClient() *http.Client {
	return s.client
}

// GetBaseURL returns the base URL for API requests
func (s *Service) GetBaseURL() string {
	return s.baseURL
}

// GetUserAgent returns the user agent string
func (s *Service) GetUserAgent() string {
	return s.userAgent
}

// GetCloudID returns the Atlassian cloud ID (Atlassian-specific)
func (s *Service) GetCloudID() string {
	return s.cloudID
}

// GetAtlToken returns the atl_token for CSRF protection (Atlassian-specific)
func (s *Service) GetAtlToken() string {
	return s.atlToken
}
