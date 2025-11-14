package models

// AuthCredentials represents stored cookie-based authentication data for web services
type AuthCredentials struct {
	ID          string                 `json:"id"`           // Unique identifier
	Name        string                 `json:"name"`         // Human-readable name (e.g., "Bob's Atlassian")
	SiteDomain  string                 `json:"site_domain"`  // Domain for site grouping (e.g., "bobmcallan.atlassian.net")
	ServiceType string                 `json:"service_type"` // "atlassian", "github", etc.
	Data        map[string]interface{} `json:"data"`         // Service-specific auth data
	Cookies     []byte                 `json:"cookies"`      // Serialized cookies
	Tokens      map[string]string      `json:"tokens"`       // Auth tokens
	BaseURL     string                 `json:"base_url"`     // Service base URL
	UserAgent   string                 `json:"user_agent"`   // User agent string
	CreatedAt   int64                  `json:"created_at"`
	UpdatedAt   int64                  `json:"updated_at"`
}
