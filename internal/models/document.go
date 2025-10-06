package models

import (
	"encoding/json"
	"time"
)

// Document represents a normalized document from any source
type Document struct {
	// Identity
	ID         string `json:"id"`          // doc_{uuid}
	SourceType string `json:"source_type"` // jira, confluence, github
	SourceID   string `json:"source_id"`   // Original ID from source

	// Content
	Title           string `json:"title"`
	Content         string `json:"content"`          // Plain text
	ContentMarkdown string `json:"content_markdown"` // Markdown format

	// Vector embedding
	Embedding      []float32 `json:"-"`               // Don't serialize in JSON
	EmbeddingModel string    `json:"embedding_model"` // Model name (e.g., nomic-embed-text)

	// Metadata (source-specific data stored as JSON)
	Metadata map[string]interface{} `json:"metadata"`
	URL      string                 `json:"url"` // Link to original

	// Sync tracking
	LastSynced        *time.Time `json:"last_synced,omitempty"`         // When document was last synced from source
	SourceVersion     string     `json:"source_version,omitempty"`      // Version/etag from source to detect changes
	ForceSyncPending  bool       `json:"force_sync_pending,omitempty"`  // Flag for manual force sync
	ForceEmbedPending bool       `json:"force_embed_pending,omitempty"` // Flag for re-vectorization

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DocumentChunk represents a chunk of a large document
type DocumentChunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	ChunkIndex int       `json:"chunk_index"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"-"`
	TokenCount int       `json:"token_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// JiraMetadata represents Jira-specific metadata
type JiraMetadata struct {
	IssueKey   string   `json:"issue_key"`
	ProjectKey string   `json:"project_key"`
	IssueType  string   `json:"issue_type"`
	Status     string   `json:"status"`
	Priority   string   `json:"priority"`
	Assignee   string   `json:"assignee"`
	Reporter   string   `json:"reporter"`
	Labels     []string `json:"labels"`
	Components []string `json:"components"`
}

// ConfluenceMetadata represents Confluence-specific metadata
type ConfluenceMetadata struct {
	PageID      string `json:"page_id"`
	SpaceKey    string `json:"space_key"`
	SpaceName   string `json:"space_name"`
	Author      string `json:"author"`
	Version     int    `json:"version"`
	ContentType string `json:"content_type"` // page, blogpost
}

// DocumentStats represents statistics about documents
type DocumentStats struct {
	TotalDocuments      int            `json:"total_documents"`
	DocumentsBySource   map[string]int `json:"documents_by_source"`
	VectorizedCount     int            `json:"vectorized_count"`
	VectorizedDocuments int            `json:"vectorized_documents"` // Alias for VectorizedCount
	JiraDocuments       int            `json:"jira_documents"`
	ConfluenceDocuments int            `json:"confluence_documents"`
	PendingVectorize    int            `json:"pending_vectorize"`
	LastUpdated         time.Time      `json:"last_updated"`
	EmbeddingModel      string         `json:"embedding_model"`
	AverageContentSize  int            `json:"average_content_size"`
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
