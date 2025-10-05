package models

// JiraProject represents a Jira project
type JiraProject struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	IssueCount int    `json:"issueCount"`
	ID         string `json:"id"`
}

// JiraIssue represents a Jira issue
type JiraIssue struct {
	Key    string                 `json:"key"`
	Fields map[string]interface{} `json:"fields"`
	ID     string                 `json:"id"`
}

// ConfluenceSpace represents a Confluence space
type ConfluenceSpace struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	PageCount int    `json:"pageCount"`
	ID        string `json:"id"`
}

// ConfluencePage represents a Confluence page
type ConfluencePage struct {
	ID      string                 `json:"id"`
	Title   string                 `json:"title"`
	SpaceID string                 `json:"spaceId"`
	Body    map[string]interface{} `json:"body"`
}

// AuthCredentials represents stored authentication data
type AuthCredentials struct {
	Service   string                 `json:"service"`    // "jira", "confluence", "github"
	Data      map[string]interface{} `json:"data"`       // Service-specific auth data
	Cookies   []byte                 `json:"cookies"`    // Serialized cookies
	Tokens    map[string]string      `json:"tokens"`     // Auth tokens
	BaseURL   string                 `json:"base_url"`   // Service base URL
	UserAgent string                 `json:"user_agent"` // User agent string
	UpdatedAt int64                  `json:"updated_at"`
}
