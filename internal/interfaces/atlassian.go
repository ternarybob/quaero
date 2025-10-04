package interfaces

import (
	"net/http"
	"time"
)

// AtlassianAuthService manages authentication state for Atlassian services
type AtlassianAuthService interface {
	UpdateAuth(authData *AtlassianAuthData) error
	IsAuthenticated() bool
	LoadAuth() (*AtlassianAuthData, error)
	GetHTTPClient() *http.Client
	GetBaseURL() string
	GetUserAgent() string
	GetCloudID() string
	GetAtlToken() string
}

// JiraScraperService defines operations for Jira data scraping
type JiraScraperService interface {
	ScrapeProjects() error
	GetProjectIssues(projectKey string) error
	GetProjectIssueCount(projectKey string) (int, error)
	DeleteProjectIssues(projectKey string) error
	ClearProjectsCache() error
	GetJiraData() (map[string]interface{}, error)
	GetProjectCount() int
	GetIssueCount() int
	ClearAllData() error
	Close() error
}

// ConfluenceScraperService defines operations for Confluence data scraping
type ConfluenceScraperService interface {
	ScrapeSpaces() error
	GetSpacePages(spaceKey string) error
	GetSpacePageCount(spaceKey string) (int, error)
	ClearSpacesCache() error
	GetConfluenceData() (map[string]interface{}, error)
	GetSpaceCount() int
	GetPageCount() int
	ClearAllData() error
	Close() error
}

// AtlassianExtensionCookie represents a cookie from browser extension
type AtlassianExtensionCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
	SameSite string `json:"sameSite"`
}

// ToHTTPCookie converts extension cookie to standard HTTP cookie
func (c *AtlassianExtensionCookie) ToHTTPCookie() *http.Cookie {
	cookie := &http.Cookie{
		Name:     c.Name,
		Value:    c.Value,
		Domain:   c.Domain,
		Path:     c.Path,
		Secure:   c.Secure,
		HttpOnly: c.HTTPOnly,
	}

	if c.Expires > 0 {
		cookie.Expires = time.Unix(c.Expires, 0)
	}

	switch c.SameSite {
	case "Strict", "strict":
		cookie.SameSite = http.SameSiteStrictMode
	case "Lax", "lax":
		cookie.SameSite = http.SameSiteLaxMode
	case "None", "none":
		cookie.SameSite = http.SameSiteNoneMode
	default:
		cookie.SameSite = http.SameSiteDefaultMode
	}

	return cookie
}

// AtlassianAuthData represents authentication data from browser extension
type AtlassianAuthData struct {
	Cookies   []*AtlassianExtensionCookie `json:"cookies"`
	Tokens    map[string]interface{}      `json:"tokens"`
	UserAgent string                      `json:"userAgent"`
	BaseURL   string                      `json:"baseUrl"`
	Timestamp int64                       `json:"timestamp"`
}

// GetHTTPCookies converts all extension cookies to HTTP cookie format
func (a *AtlassianAuthData) GetHTTPCookies() []*http.Cookie {
	cookies := make([]*http.Cookie, len(a.Cookies))
	for i, ec := range a.Cookies {
		cookies[i] = ec.ToHTTPCookie()
	}
	return cookies
}
