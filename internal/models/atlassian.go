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
