// Package models defines the core document model for Quaero's knowledge collection system.
//
// ARCHITECTURE: Markdown + Metadata Canonical Format
//
// All content from sources (Jira, Confluence, GitHub) is transformed into two parts:
// 1. Generic Markdown (ContentMarkdown field) - Clean, unified text format ideal for AI processing and full-text search
// 2. Rich Metadata (Metadata field) - Structured JSON with source-specific data for efficient filtering
//
// This design enables a two-step query pattern:
// - Step 1: Filter documents using structured metadata (SQL WHERE clauses on JSON fields)
// - Step 2: Reason and synthesize answers from clean Markdown content of filtered results
//
// See docs/architecture.md for complete documentation of the transformation pipeline.
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

// Document represents a normalized document from any source.
//
// DESIGN PHILOSOPHY: Markdown-First Content + Structured Metadata
//
// ContentMarkdown is the PRIMARY CONTENT field containing clean, unified text format
// that works seamlessly with:
// - AI/LLM reasoning and synthesis
// - Full-text search (Badger)
// - Human readability and debugging
//
// Metadata is a flexible map containing source-specific structured data that enables:
// - Efficient filtering (SQL WHERE clauses on JSON fields)
// - Faceted search (group by project, status, priority, etc.)
// - Schema evolution (add fields without database migrations)
//
// Example Transformation Pipeline:
// Jira HTML → ParseJiraIssuePage() → JiraIssueData struct →
// convertHTMLToMarkdown() → ContentMarkdown + JiraMetadata.ToMap() → Document →
// SaveDocument() → Badger (content_markdown + metadata JSON)
//
// See docs/architecture.md for complete pipeline documentation.
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
	URL      string                 `json:"url"`  // Link to original
	Tags     []string               `json:"tags"` // User-defined tags from job definitions for categorization and filtering

	// Structured extracts - populated at ingestion time by extractors
	// Enables deterministic retrieval of structured data rather than LLM needle-finding
	Extracts    []Extract  `json:"extracts,omitempty"`
	ExtractedAt *time.Time `json:"extracted_at,omitempty"` // When extraction was last performed
	ContentHash string     `json:"content_hash,omitempty"` // Hash for change detection and re-extraction triggers

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

// LocalDirMetadata represents local directory file metadata
type LocalDirMetadata struct {
	BasePath     string     `json:"base_path"`     // Base directory path that was indexed
	FilePath     string     `json:"file_path"`     // Relative file path within base directory
	AbsolutePath string     `json:"absolute_path"` // Full absolute path to the file
	Folder       string     `json:"folder"`        // Parent folder within base directory
	Extension    string     `json:"extension"`     // File extension (e.g., ".go", ".ts")
	FileSize     int64      `json:"file_size"`     // File size in bytes
	ModTime      *time.Time `json:"mod_time"`      // File modification time
	FileType     string     `json:"file_type"`     // Detected file type (code, markdown, config, etc.)
}

// ToMap converts local directory metadata to map for storage
func (l *LocalDirMetadata) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// CodeMapMetadata represents hierarchical code structure metadata
// Used by CodeMapWorker for efficient large codebase analysis
type CodeMapMetadata struct {
	// Identity
	BasePath    string `json:"base_path"`    // Root directory being mapped
	NodeType    string `json:"node_type"`    // "project", "directory", "file"
	RelPath     string `json:"rel_path"`     // Relative path from base
	ParentPath  string `json:"parent_path"`  // Parent node's relative path (empty for root)
	ProjectName string `json:"project_name"` // Human-readable project name

	// Structure (for directories)
	ChildCount     int      `json:"child_count"`     // Number of direct children
	FileCount      int      `json:"file_count"`      // Total files in subtree
	DirCount       int      `json:"dir_count"`       // Total directories in subtree
	TotalSize      int64    `json:"total_size"`      // Total bytes in subtree
	TotalLOC       int      `json:"total_loc"`       // Total lines of code in subtree
	Languages      []string `json:"languages"`       // Detected languages in subtree
	MainLanguage   string   `json:"main_language"`   // Primary language by LOC
	ChildPaths     []string `json:"child_paths"`     // Direct child relative paths
	IgnoredCount   int      `json:"ignored_count"`   // Files ignored by filters
	IgnoredReasons []string `json:"ignored_reasons"` // Why files were ignored

	// Structure (for files)
	Extension  string   `json:"extension"`   // File extension
	FileSize   int64    `json:"file_size"`   // File size in bytes
	LOC        int      `json:"loc"`         // Lines of code
	Language   string   `json:"language"`    // Detected programming language
	FileType   string   `json:"file_type"`   // code, config, doc, test, etc.
	Exports    []string `json:"exports"`     // Exported functions/classes/types
	Imports    []string `json:"imports"`     // Import statements
	HasTests   bool     `json:"has_tests"`   // Contains test code
	Complexity string   `json:"complexity"`  // low, medium, high (heuristic)
	ModTime    string   `json:"mod_time"`    // Last modification time (RFC3339)
	ContentMD5 string   `json:"content_md5"` // MD5 hash for change detection

	// AI-Generated (populated by summarization step)
	Summary     string   `json:"summary"`      // AI-generated summary
	Purpose     string   `json:"purpose"`      // Inferred purpose/responsibility
	KeyConcepts []string `json:"key_concepts"` // Main concepts/patterns found
	Complexity_ string   `json:"complexity_"`  // AI-assessed complexity

	// Processing State
	Indexed        bool   `json:"indexed"`         // Structure has been indexed
	Summarized     bool   `json:"summarized"`      // AI summary has been generated
	LastIndexed    string `json:"last_indexed"`    // When structure was indexed (RFC3339)
	LastSummarized string `json:"last_summarized"` // When summary was generated (RFC3339)
}

// ToMap converts code map metadata to map for storage
func (c *CodeMapMetadata) ToMap() (map[string]interface{}, error) {
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

// CrossSourceMetadata contains cross-reference information extracted from content.
//
// NOTE: Currently unpopulated by transformers. Future enhancement to extract cross-references
// from content using identifiers/extractor.go service.
//
// Planned Implementation:
//   - After markdown conversion in transformers, call identifierExtractor.ExtractCrossReferences()
//   - Populate ReferencedIssues (Jira keys like "BUG-123"), ReferencedPRs (GitHub "#123"),
//     and ReferencedPages (Confluence page IDs)
//   - Merge into document metadata for relationship tracking and impact analysis
//
// See docs/metadata_gaps_analysis.md for detailed implementation plan.
type CrossSourceMetadata struct {
	ReferencedIssues []string `json:"referenced_issues"` // Jira keys found in content (e.g., ["BUG-123", "STORY-456"])
	ReferencedPRs    []string `json:"referenced_prs"`    // GitHub PR numbers (e.g., ["#123", "#456"])
	ReferencedPages  []string `json:"referenced_pages"`  // Confluence page IDs mentioned
}

// Extract represents structured data extracted from document content at ingestion time.
// This enables deterministic retrieval of structured information rather than relying
// on LLM "needle-finding" at query time.
//
// Extracts are populated during document crawling/ingestion by rule-based or LLM extractors.
// Tools return these structured extracts, not raw markdown, for reliable AI synthesis.
type Extract struct {
	Type       string          `json:"type"`                  // Extract type: "financial", "meeting", "technical", "api", etc.
	Schema     string          `json:"schema"`                // Schema version used for extraction (e.g., "v1", "v2")
	Data       json.RawMessage `json:"data"`                  // Structured data matching the schema
	Confidence float64         `json:"confidence,omitempty"`  // Extraction confidence (1.0 for rule-based, 0.0-1.0 for LLM)
	Span       *TextSpan       `json:"span,omitempty"`        // Location in raw content (for reference back to source)
	ExtractedAt string         `json:"extracted_at,omitempty"` // When extraction was performed (RFC3339)
}

// TextSpan represents a location range in the source document
type TextSpan struct {
	Start int `json:"start"` // Start character offset
	End   int `json:"end"`   // End character offset
}

// FinancialExtract represents financial data extracted from documents
type FinancialExtract struct {
	Ticker        string   `json:"ticker"`
	ReportDate    string   `json:"report_date,omitempty"`
	ReportType    string   `json:"report_type,omitempty"`    // quarterly, annual
	Revenue       *float64 `json:"revenue,omitempty"`
	NetIncome     *float64 `json:"net_income,omitempty"`
	EPS           *float64 `json:"eps,omitempty"`
	DividendYield *float64 `json:"dividend_yield,omitempty"`
	PERatio       *float64 `json:"pe_ratio,omitempty"`
	MarketCap     *float64 `json:"market_cap,omitempty"`
	Notes         []string `json:"notes,omitempty"`          // Qualitative observations
}

// MeetingExtract represents meeting notes/decisions extracted from documents
type MeetingExtract struct {
	Date        string   `json:"date"`
	Attendees   []string `json:"attendees,omitempty"`
	Topics      []string `json:"topics"`
	Decisions   []string `json:"decisions,omitempty"`
	ActionItems []Action `json:"action_items,omitempty"`
}

// Action represents an action item from a meeting
type Action struct {
	Description string `json:"description"`
	Owner       string `json:"owner,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
}

// APIExtract represents API/technical documentation extracted from documents
type APIExtract struct {
	ServiceName string     `json:"service_name"`
	Endpoints   []Endpoint `json:"endpoints,omitempty"`
	AuthMethod  string     `json:"auth_method,omitempty"`
	BaseURL     string     `json:"base_url,omitempty"`
}

// Endpoint represents an API endpoint
type Endpoint struct {
	Method      string `json:"method"`       // GET, POST, PUT, DELETE
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	RequestBody string `json:"request_body,omitempty"`  // JSON schema or example
	Response    string `json:"response,omitempty"`      // JSON schema or example
}

// ToMap converts Extract to map for storage
func (e *Extract) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ToMap converts FinancialExtract to map for storage
func (f *FinancialExtract) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ToMap converts MeetingExtract to map for storage
func (m *MeetingExtract) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ToMap converts APIExtract to map for storage
func (a *APIExtract) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
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
