package httpclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// NewDefaultHTTPClient creates a simple HTTP client with a timeout
func NewDefaultHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

// NewHTTPClientWithAuth creates an HTTP client with cookie jar and authentication
// configured from AuthCredentials. Returns a configured client ready to make
// authenticated requests to the service.
func NewHTTPClientWithAuth(authCreds *models.AuthCredentials) (*http.Client, error) {
	if authCreds == nil {
		return NewDefaultHTTPClient(30 * time.Second), nil
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// Parse base URL for fallback
	baseURL, err := url.Parse(authCreds.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Unmarshal cookies from JSON
	var cookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(authCreds.Cookies, &cookies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	// Group cookies by domain to set them with appropriate URLs
	// This ensures cookie jar accepts cookies based on their declared domain
	cookiesByDomain := make(map[string][]*http.Cookie)
	for _, c := range cookies {
		// Calculate expiration time
		// If expiration is 0 or in the past, treat as session cookie (no expiration)
		// This prevents cookie jar from rejecting cookies with zero/invalid timestamps
		var expires time.Time
		if c.Expires > 0 {
			expires = time.Unix(c.Expires, 0)
			// If cookie expired more than a day ago, treat as session cookie
			if expires.Before(time.Now().Add(-24 * time.Hour)) {
				expires = time.Time{} // Zero value = session cookie
			}
		}
		// Zero time.Time = session cookie (no expiration)

		httpCookie := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  expires,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}

		// Use cookie's domain, removing leading dot if present
		domain := strings.TrimPrefix(c.Domain, ".")
		if domain == "" {
			domain = baseURL.Host // Fallback to base URL host
		}

		cookiesByDomain[domain] = append(cookiesByDomain[domain], httpCookie)
	}

	// Set cookies for each domain using a URL that matches that domain
	for domain, domainCookies := range cookiesByDomain {
		// Build URL for this domain (always use https for Atlassian)
		domainURL, err := url.Parse(fmt.Sprintf("https://%s/", domain))
		if err != nil {
			// Log warning and skip this domain
			continue
		}

		// Set cookies for this domain
		client.Jar.SetCookies(domainURL, domainCookies)
	}

	return client, nil
}

// NewHTTPClientFromAtlassianAuth creates an HTTP client from AtlassianAuthData
// (used by auth service which receives auth data from Chrome extension)
func NewHTTPClientFromAtlassianAuth(authData *interfaces.AtlassianAuthData) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	baseURL, err := url.Parse(authData.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	client.Jar.SetCookies(baseURL, authData.GetHTTPCookies())

	return client, nil
}
