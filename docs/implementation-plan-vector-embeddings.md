# Implementation Plan: Vector Embeddings (Phase 1.1)

**Goal:** Create a normalized document store with vector embeddings for unified search across all data sources.

---

## Architecture Overview

### Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│ COLLECTION PHASE                                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Jira Collector          Confluence Collector              │
│        ↓                         ↓                          │
│  jira_issues             confluence_pages                   │
│  (source-specific)       (source-specific)                  │
│        │                         │                          │
│        └─────────┬───────────────┘                          │
│                  ↓                                          │
│          Document Transformer                               │
│                  ↓                                          │
│          documents (normalized)                             │
│                  ↓                                          │
│          Embedding Generator                                │
│                  ↓                                          │
│          documents.embedding (vector)                       │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ QUERY PHASE                                                 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  User Query → Query Embedding                               │
│                  ↓                                          │
│          Vector Similarity Search                           │
│          (on documents.embedding)                           │
│                  ↓                                          │
│          Top K Documents                                    │
│                  ↓                                          │
│          Context Builder                                    │
│          (can fetch from source tables for full context)   │
│                  ↓                                          │
│          LLM (with context)                                 │
│                  ↓                                          │
│          Answer                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Database Schema

### 1. New Normalized Documents Table

```sql
-- Unified document store
CREATE TABLE documents (
    -- Identity
    id TEXT PRIMARY KEY,                    -- doc_{uuid}
    source_type TEXT NOT NULL,              -- 'jira' | 'confluence' | 'github'
    source_id TEXT NOT NULL,                -- Original ID from source

    -- Content
    title TEXT NOT NULL,                    -- Normalized title
    content TEXT NOT NULL,                  -- Plain text content
    content_markdown TEXT,                  -- Markdown format (optional)

    -- Vector embedding
    embedding BLOB,                         -- Vector representation (float32[])
    embedding_model TEXT,                   -- Model used (e.g., 'nomic-embed-text')

    -- Metadata
    metadata TEXT,                          -- JSON with source-specific fields
    url TEXT,                               -- Link back to original

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Indexing
    UNIQUE(source_type, source_id)          -- Prevent duplicates
);

-- Full-text search index
CREATE VIRTUAL TABLE documents_fts USING fts5(
    title,
    content,
    content=documents,
    content_rowid=rowid
);

-- Vector search index (using sqlite-vec)
CREATE VIRTUAL TABLE vec_documents USING vec0(
    embedding float[768]                    -- Adjust dimension based on model
);

-- Indexes for filtering
CREATE INDEX idx_documents_source ON documents(source_type);
CREATE INDEX idx_documents_updated ON documents(updated_at);
```

### 2. Document Chunks (for large documents)

```sql
-- For documents too large to fit in context window
CREATE TABLE document_chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding BLOB,
    token_count INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    UNIQUE(document_id, chunk_index)
);

CREATE INDEX idx_chunks_document ON document_chunks(document_id);
```

---

## Document Model

### Normalized Document Structure

```go
// internal/models/document.go
package models

import "time"

// Document represents a normalized document from any source
type Document struct {
    // Identity
    ID         string    `json:"id"`          // doc_{uuid}
    SourceType string    `json:"source_type"` // jira, confluence, github
    SourceID   string    `json:"source_id"`   // Original ID

    // Content
    Title           string  `json:"title"`
    Content         string  `json:"content"`           // Plain text
    ContentMarkdown string  `json:"content_markdown"`  // Markdown format

    // Vector
    Embedding      []float32 `json:"-"`               // Don't serialize in JSON
    EmbeddingModel string    `json:"embedding_model"` // Model name

    // Metadata
    Metadata map[string]interface{} `json:"metadata"` // Source-specific data
    URL      string                 `json:"url"`      // Link to original

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

// Source-specific metadata structures
type JiraMetadata struct {
    IssueKey    string   `json:"issue_key"`
    ProjectKey  string   `json:"project_key"`
    IssueType   string   `json:"issue_type"`
    Status      string   `json:"status"`
    Priority    string   `json:"priority"`
    Assignee    string   `json:"assignee"`
    Reporter    string   `json:"reporter"`
    Labels      []string `json:"labels"`
    Components  []string `json:"components"`
}

type ConfluenceMetadata struct {
    PageID      string `json:"page_id"`
    SpaceKey    string `json:"space_key"`
    SpaceName   string `json:"space_name"`
    Author      string `json:"author"`
    Version     int    `json:"version"`
    ContentType string `json:"content_type"` // page, blogpost
}
```

---

## Implementation Steps

### Step 1: Database Migrations

```go
// internal/storage/sqlite/migrations.go

const migrationDocuments = `
-- Create documents table
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    content_markdown TEXT,
    embedding BLOB,
    embedding_model TEXT,
    metadata TEXT,
    url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_type, source_id)
);

CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source_type);
CREATE INDEX IF NOT EXISTS idx_documents_updated ON documents(updated_at);

-- FTS5 index
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
    title,
    content,
    content=documents,
    content_rowid=rowid
);

-- Triggers to keep FTS5 in sync
CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
    INSERT INTO documents_fts(rowid, title, content)
    VALUES (new.rowid, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
    DELETE FROM documents_fts WHERE rowid = old.rowid;
END;

CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
    UPDATE documents_fts SET title = new.title, content = new.content
    WHERE rowid = new.rowid;
END;

-- Document chunks
CREATE TABLE IF NOT EXISTS document_chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding BLOB,
    token_count INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    UNIQUE(document_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_chunks_document ON document_chunks(document_id);
`
```

### Step 2: Document Transformer Service

```go
// internal/services/documents/transformer.go
package documents

import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/ternarybob/quaero/internal/models"
)

// Transformer converts source-specific data to normalized documents
type Transformer struct{}

func NewTransformer() *Transformer {
    return &Transformer{}
}

// TransformJiraIssue converts a Jira issue to a normalized document
func (t *Transformer) TransformJiraIssue(issue *models.JiraIssue) (*models.Document, error) {
    // Generate document ID
    docID := fmt.Sprintf("doc_%s", uuid.New().String())

    // Build plain text content
    content := t.buildJiraContent(issue)

    // Build markdown content
    contentMD := t.buildJiraMarkdown(issue)

    // Build metadata
    metadata := models.JiraMetadata{
        IssueKey:   issue.Key,
        ProjectKey: issue.ProjectKey,
        IssueType:  issue.IssueType,
        Status:     issue.Status,
        Priority:   issue.Priority,
        Assignee:   issue.Assignee,
        Reporter:   issue.Reporter,
        Labels:     issue.Labels,
        Components: issue.Components,
    }

    metadataJSON, err := json.Marshal(metadata)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal metadata: %w", err)
    }

    var metadataMap map[string]interface{}
    json.Unmarshal(metadataJSON, &metadataMap)

    return &models.Document{
        ID:              docID,
        SourceType:      "jira",
        SourceID:        issue.Key,
        Title:           fmt.Sprintf("[%s] %s", issue.Key, issue.Summary),
        Content:         content,
        ContentMarkdown: contentMD,
        Metadata:        metadataMap,
        URL:             issue.URL,
        CreatedAt:       issue.CreatedAt,
        UpdatedAt:       issue.UpdatedAt,
    }, nil
}

func (t *Transformer) buildJiraContent(issue *models.JiraIssue) string {
    return fmt.Sprintf(
        "%s\n\n%s\n\nStatus: %s\nPriority: %s\nAssignee: %s\nReporter: %s",
        issue.Summary,
        issue.Description,
        issue.Status,
        issue.Priority,
        issue.Assignee,
        issue.Reporter,
    )
}

func (t *Transformer) buildJiraMarkdown(issue *models.JiraIssue) string {
    return fmt.Sprintf(`# %s

**Issue Key:** %s
**Status:** %s
**Priority:** %s
**Assignee:** %s
**Reporter:** %s

## Description

%s

## Metadata

- **Project:** %s
- **Type:** %s
- **Labels:** %v
- **Components:** %v
`,
        issue.Summary,
        issue.Key,
        issue.Status,
        issue.Priority,
        issue.Assignee,
        issue.Reporter,
        issue.Description,
        issue.ProjectKey,
        issue.IssueType,
        issue.Labels,
        issue.Components,
    )
}

// TransformConfluencePage converts a Confluence page to a normalized document
func (t *Transformer) TransformConfluencePage(page *models.ConfluencePage) (*models.Document, error) {
    docID := fmt.Sprintf("doc_%s", uuid.New().String())

    // Build plain text content
    content := t.buildConfluenceContent(page)

    // Build markdown content
    contentMD := t.buildConfluenceMarkdown(page)

    // Build metadata
    metadata := models.ConfluenceMetadata{
        PageID:      page.PageID,
        SpaceKey:    page.SpaceKey,
        SpaceName:   page.SpaceName,
        Author:      page.Author,
        Version:     page.Version,
        ContentType: page.Type,
    }

    metadataJSON, err := json.Marshal(metadata)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal metadata: %w", err)
    }

    var metadataMap map[string]interface{}
    json.Unmarshal(metadataJSON, &metadataMap)

    return &models.Document{
        ID:              docID,
        SourceType:      "confluence",
        SourceID:        page.PageID,
        Title:           page.Title,
        Content:         content,
        ContentMarkdown: contentMD,
        Metadata:        metadataMap,
        URL:             page.URL,
        CreatedAt:       page.CreatedAt,
        UpdatedAt:       page.UpdatedAt,
    }, nil
}

func (t *Transformer) buildConfluenceContent(page *models.ConfluencePage) string {
    return fmt.Sprintf("%s\n\n%s", page.Title, page.Content)
}

func (t *Transformer) buildConfluenceMarkdown(page *models.ConfluencePage) string {
    return fmt.Sprintf(`# %s

**Space:** %s (%s)
**Author:** %s
**Version:** %d

---

%s
`,
        page.Title,
        page.SpaceName,
        page.SpaceKey,
        page.Author,
        page.Version,
        page.Content,
    )
}
```

### Step 3: Document Storage Interface

```go
// internal/interfaces/document_storage.go
package interfaces

import "github.com/ternarybob/quaero/internal/models"

type DocumentStorage interface {
    // CRUD operations
    SaveDocument(doc *models.Document) error
    GetDocument(id string) (*models.Document, error)
    GetDocumentBySource(sourceType, sourceID string) (*models.Document, error)
    UpdateDocument(doc *models.Document) error
    DeleteDocument(id string) error

    // Batch operations
    SaveDocuments(docs []*models.Document) error
    GetDocumentsBySource(sourceType string) ([]*models.Document, error)

    // Search
    FullTextSearch(query string, limit int) ([]*models.Document, error)
    VectorSearch(embedding []float32, limit int) ([]*models.Document, error)
    HybridSearch(query string, embedding []float32, limit int) ([]*models.Document, error)

    // Chunks
    SaveChunk(chunk *models.DocumentChunk) error
    GetChunks(documentID string) ([]*models.DocumentChunk, error)

    // Stats
    CountDocuments() (int, error)
    CountDocumentsBySource(sourceType string) (int, error)
}
```

### Step 4: Embedding Service

```go
// internal/services/embeddings/service.go
package embeddings

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/ternarybob/arbor"
    "github.com/ternarybob/quaero/internal/models"
)

// Service handles embedding generation via Ollama
type Service struct {
    ollamaURL string
    modelName string
    logger    arbor.ILogger
}

func NewService(ollamaURL, modelName string, logger arbor.ILogger) *Service {
    return &Service{
        ollamaURL: ollamaURL,
        modelName: modelName,
        logger:    logger,
    }
}

// GenerateEmbedding creates a vector embedding for text
func (s *Service) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    reqBody := map[string]interface{}{
        "model":  s.modelName,
        "prompt": text,
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(
        ctx,
        "POST",
        fmt.Sprintf("%s/api/embeddings", s.ollamaURL),
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to call ollama: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
    }

    var result struct {
        Embedding []float32 `json:"embedding"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return result.Embedding, nil
}

// EmbedDocument generates and sets embedding for a document
func (s *Service) EmbedDocument(ctx context.Context, doc *models.Document) error {
    // Combine title and content for embedding
    text := fmt.Sprintf("%s\n\n%s", doc.Title, doc.Content)

    embedding, err := s.GenerateEmbedding(ctx, text)
    if err != nil {
        return fmt.Errorf("failed to generate embedding: %w", err)
    }

    doc.Embedding = embedding
    doc.EmbeddingModel = s.modelName

    s.logger.Debug().
        Str("doc_id", doc.ID).
        Int("embedding_dim", len(embedding)).
        Msg("Generated embedding")

    return nil
}

// EmbedDocuments generates embeddings for multiple documents
func (s *Service) EmbedDocuments(ctx context.Context, docs []*models.Document) error {
    for i, doc := range docs {
        if err := s.EmbedDocument(ctx, doc); err != nil {
            s.logger.Error().
                Err(err).
                Str("doc_id", doc.ID).
                Int("index", i).
                Msg("Failed to embed document")
            return err
        }
    }

    return nil
}
```

---

## Configuration Updates

```toml
# quaero.toml

[embeddings]
enabled = true
ollama_url = "http://localhost:11434"
model = "nomic-embed-text"
dimension = 768
batch_size = 10

[embeddings.auto]
# Automatically embed new documents
enabled = true
# Re-embed documents older than this (days)
refresh_after_days = 30
```

---

## Integration with Collectors

### Modified Collection Flow

```go
// internal/services/atlassian/jira_scraper_service.go

func (j *JiraScraperService) FetchAndStoreIssues(projectKey string) error {
    // 1. Fetch issues (existing code)
    issues, err := j.fetchIssues(projectKey)
    if err != nil {
        return err
    }

    // 2. Store in source-specific table (existing)
    for _, issue := range issues {
        if err := j.storage.SaveIssue(issue); err != nil {
            return err
        }
    }

    // 3. NEW: Transform to documents
    docs := make([]*models.Document, 0, len(issues))
    transformer := documents.NewTransformer()

    for _, issue := range issues {
        doc, err := transformer.TransformJiraIssue(issue)
        if err != nil {
            j.logger.Error().Err(err).Str("issue", issue.Key).Msg("Failed to transform")
            continue
        }
        docs = append(docs, doc)
    }

    // 4. NEW: Generate embeddings
    if err := j.embeddingService.EmbedDocuments(context.Background(), docs); err != nil {
        return fmt.Errorf("failed to embed documents: %w", err)
    }

    // 5. NEW: Store in documents table
    if err := j.documentStorage.SaveDocuments(docs); err != nil {
        return fmt.Errorf("failed to save documents: %w", err)
    }

    j.logger.Info().
        Int("issues", len(issues)).
        Int("documents", len(docs)).
        Msg("Stored and embedded documents")

    return nil
}
```

---

## Testing Strategy

### Unit Tests

```go
func TestTransformer_TransformJiraIssue(t *testing.T) {
    transformer := documents.NewTransformer()

    issue := &models.JiraIssue{
        Key:         "TEST-123",
        Summary:     "Test issue",
        Description: "Test description",
        Status:      "Open",
        Priority:    "High",
    }

    doc, err := transformer.TransformJiraIssue(issue)
    assert.NoError(t, err)
    assert.Equal(t, "jira", doc.SourceType)
    assert.Equal(t, "TEST-123", doc.SourceID)
    assert.Contains(t, doc.Content, "Test issue")
}

func TestEmbeddingService_GenerateEmbedding(t *testing.T) {
    // Mock Ollama server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        response := map[string]interface{}{
            "embedding": make([]float32, 768),
        }
        json.NewEncoder(w).Encode(response)
    }))
    defer server.Close()

    service := embeddings.NewService(server.URL, "test-model", logger)

    embedding, err := service.GenerateEmbedding(context.Background(), "test text")
    assert.NoError(t, err)
    assert.Len(t, embedding, 768)
}
```

---

## Success Criteria

- [ ] Documents table created with proper indexes
- [ ] Transformer converts Jira issues to documents
- [ ] Transformer converts Confluence pages to documents
- [ ] Embedding service generates vectors via Ollama
- [ ] Documents stored with embeddings
- [ ] Full-text search works on documents table
- [ ] Vector search works (next phase: query interface)
- [ ] Source context preserved (can link back to jira_issues/confluence_pages)
- [ ] Performance: < 1 second to embed a document
- [ ] Performance: < 100ms for vector search on 10,000 documents

---

This implementation creates a solid foundation for Phase 1.2 (RAG Pipeline) and Phase 1.3 (Query Interface).
