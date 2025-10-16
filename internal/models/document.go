package models

import (
	"encoding/json"
	"time"
)

const (
	// DetailLevelMetadata indicates document contains only metadata (Firecrawl-style incremental crawling)
	DetailLevelMetadata = "metadata"
	// DetailLevelFull indicates document contains full content
	DetailLevelFull = "full"
)

// Document represents a normalized document from any source
// PRIMARY CONTENT FORMAT: Markdown (ContentMarkdown field)
type Document struct {
	// Identity
	ID         string `json:"id"`          // doc_{uuid}
	SourceType string `json:"source_type"` // jira, confluence, github
	SourceID   string `json:"source_id"`   // Original ID from source

	// Content (markdown-first)
	Title           string `json:"title"`
	ContentMarkdown string `json:"content_markdown"` // PRIMARY CONTENT: Markdown format
	DetailLevel     string `json:"detail_level"`     // "metadata" or "full" for Firecrawl-style layered crawling

	// NOTE: Phase 5 - Embedding fields removed (using FTS5 search only)

	// Metadata (source-specific data + extracted keywords stored as JSON)
	// Example: {"project": "PROJ-123", "assignee": "alice", "keywords": ["bug", "urgent"]}
	Metadata map[string]interface{} `json:"metadata"`
	URL      string                 `json:"url"` // Link to original

	// Sync tracking
	LastSynced       *time.Time `json:"last_synced,omitempty"`        // When document was last synced from source
	SourceVersion    string     `json:"source_version,omitempty"`     // Version/etag from source to detect changes
	ForceSyncPending bool       `json:"force_sync_pending,omitempty"` // Flag for manual force sync
	// NOTE: Phase 5 - ForceEmbedPending removed (no longer using embeddings)

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NOTE: Phase 5 - DocumentChunk struct removed (no longer using chunking for embeddings)

// JiraMetadata represents Jira-specific metadata
type JiraMetadata struct {
	IssueKey       string     `json:"issue_key"`
	ProjectKey     string     `json:"project_key"`
	IssueType      string     `json:"issue_type"` // Bug, Story, Task, Epic
	Status         string     `json:"status"`     // Open, In Progress, Resolved, Closed
	Priority       string     `json:"priority"`
	Assignee       string     `json:"assignee"`
	Reporter       string     `json:"reporter"`
	Labels         []string   `json:"labels"`
	Components     []string   `json:"components"`
	Summary        string     `json:"summary"`         // Issue summary/title
	ResolutionDate *time.Time `json:"resolution_date"` // When issue was resolved
	CreatedDate    *time.Time `json:"created_date"`    // When issue was created
	UpdatedDate    *time.Time `json:"updated_date"`    // Last update timestamp
}

// ConfluenceMetadata represents Confluence-specific metadata
type ConfluenceMetadata struct {
	PageID       string     `json:"page_id"`
	PageTitle    string     `json:"page_title"`    // Page title
	SpaceKey     string     `json:"space_key"`     // Space identifier (e.g., "TEAM", "DOCS")
	SpaceName    string     `json:"space_name"`    // Human-readable space name
	Author       string     `json:"author"`        // Page author
	Version      int        `json:"version"`       // Page version number
	ContentType  string     `json:"content_type"`  // page, blogpost
	LastModified *time.Time `json:"last_modified"` // When page was last modified
	CreatedDate  *time.Time `json:"created_date"`  // When page was created
}

// GitHubMetadata represents GitHub-specific metadata
type GitHubMetadata struct {
	RepoName     string     `json:"repo_name"`     // Repository name (e.g., "org/repo")
	FilePath     string     `json:"file_path"`     // File path within repository
	CommitSHA    string     `json:"commit_sha"`    // Commit SHA
	Branch       string     `json:"branch"`        // Branch name
	FunctionName string     `json:"function_name"` // Auto-extracted function/class name
	Author       string     `json:"author"`        // Commit author
	CommitDate   *time.Time `json:"commit_date"`   // Commit timestamp
	PullRequest  string     `json:"pull_request"`  // Associated PR number (if any)
}

// CrossSourceMetadata contains cross-reference information extracted from content
type CrossSourceMetadata struct {
	ReferencedIssues []string `json:"referenced_issues"` // Jira keys found in content (e.g., ["BUG-123", "STORY-456"])
	ReferencedPRs    []string `json:"referenced_prs"`    // GitHub PR numbers (e.g., ["#123", "#456"])
	ReferencedPages  []string `json:"referenced_pages"`  // Confluence page IDs mentioned
}

// DocumentStats represents statistics about documents
type DocumentStats struct {
	TotalDocuments      int            `json:"total_documents"`
	DocumentsBySource   map[string]int `json:"documents_by_source"`
	JiraDocuments       int            `json:"jira_documents"`
	ConfluenceDocuments int            `json:"confluence_documents"`
	LastUpdated         time.Time      `json:"last_updated"`
	AverageContentSize  int            `json:"average_content_size"`
	// NOTE: Phase 5 - Embedding-related fields removed: VectorizedCount, VectorizedDocuments, PendingVectorize, EmbeddingModel
}

// MetadataToMap converts typed metadata to map for storage
func (j *JiraMetadata) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// MetadataToMap converts typed metadata to map for storage
func (c *ConfluenceMetadata) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ToMap converts GitHub metadata to map for storage
func (g *GitHubMetadata) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ToMap converts cross-source metadata to map for storage
func (c *CrossSourceMetadata) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
