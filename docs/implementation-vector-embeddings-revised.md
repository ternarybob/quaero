# Implementation Plan: Vector Embeddings (Revised)

**Architecture:** Services own document transformation and insertion via DocumentService interface

---

## Revised Architecture

### Responsibility Model

```
JiraScraperService
    ↓ (transforms own data)
    Document
    ↓ (via interface)
DocumentService
    ↓ (stores)
documents table

ConfluenceScraperService
    ↓ (transforms own data)
    Document
    ↓ (via interface)
DocumentService
    ↓ (stores)
documents table
```

### Each Service is Responsible For:

1. **Fetching** source data (existing)
2. **Storing** in source-specific table (existing)
3. **Transforming** to normalized document (NEW)
4. **Inserting** via DocumentService interface (NEW)

---

## Core Interfaces

### DocumentService Interface

```go
// internal/interfaces/document_service.go
package interfaces

import (
    "context"
    "github.com/ternarybob/quaero/internal/models"
)

// DocumentService handles normalized document operations
type DocumentService interface {
    // Save a single document (will generate embedding)
    SaveDocument(ctx context.Context, doc *models.Document) error

    // Save multiple documents in batch
    SaveDocuments(ctx context.Context, docs []*models.Document) error

    // Update existing document
    UpdateDocument(ctx context.Context, doc *models.Document) error

    // Get document by ID
    GetDocument(ctx context.Context, id string) (*models.Document, error)

    // Get document by source reference
    GetBySource(ctx context.Context, sourceType, sourceID string) (*models.Document, error)

    // Delete document
    DeleteDocument(ctx context.Context, id string) error

    // Search
    Search(ctx context.Context, query *SearchQuery) ([]*models.Document, error)

    // Stats
    Count(ctx context.Context, sourceType string) (int, error)
}

// SearchQuery represents search parameters
type SearchQuery struct {
    // Text query for keyword search
    Text string

    // Query embedding for vector search
    Embedding []float32

    // Filters
    SourceType string
    SourceIDs  []string

    // Pagination
    Limit  int
    Offset int

    // Search mode
    Mode SearchMode // Keyword, Vector, or Hybrid
}

type SearchMode string

const (
    SearchModeKeyword SearchMode = "keyword"
    SearchModeVector  SearchMode = "vector"
    SearchModeHybrid  SearchMode = "hybrid"
)
```

---

## Implementation

### Step 1: Document Service Implementation

```go
// internal/services/documents/document_service.go
package documents

import (
    "context"
    "fmt"

    "github.com/ternarybob/arbor"
    "github.com/ternarybob/quaero/internal/interfaces"
    "github.com/ternarybob/quaero/internal/models"
)

type Service struct {
    storage           interfaces.DocumentStorage
    embeddingService  interfaces.EmbeddingService
    logger            arbor.ILogger
}

func NewService(
    storage interfaces.DocumentStorage,
    embeddingService interfaces.EmbeddingService,
    logger arbor.ILogger,
) *Service {
    return &Service{
        storage:          storage,
        embeddingService: embeddingService,
        logger:           logger,
    }
}

// SaveDocument saves a document and generates its embedding
func (s *Service) SaveDocument(ctx context.Context, doc *models.Document) error {
    // Generate embedding if not present
    if doc.Embedding == nil {
        if err := s.embeddingService.EmbedDocument(ctx, doc); err != nil {
            return fmt.Errorf("failed to generate embedding: %w", err)
        }
    }

    // Save to storage
    if err := s.storage.SaveDocument(doc); err != nil {
        return fmt.Errorf("failed to save document: %w", err)
    }

    s.logger.Info().
        Str("doc_id", doc.ID).
        Str("source", doc.SourceType).
        Str("source_id", doc.SourceID).
        Msg("Document saved with embedding")

    return nil
}

// SaveDocuments saves multiple documents in batch
func (s *Service) SaveDocuments(ctx context.Context, docs []*models.Document) error {
    // Generate embeddings for all documents
    for _, doc := range docs {
        if doc.Embedding == nil {
            if err := s.embeddingService.EmbedDocument(ctx, doc); err != nil {
                s.logger.Error().
                    Err(err).
                    Str("doc_id", doc.ID).
                    Msg("Failed to generate embedding")
                return err
            }
        }
    }

    // Save all documents
    if err := s.storage.SaveDocuments(docs); err != nil {
        return fmt.Errorf("failed to save documents: %w", err)
    }

    s.logger.Info().
        Int("count", len(docs)).
        Msg("Documents saved with embeddings")

    return nil
}

// Other methods...
func (s *Service) GetDocument(ctx context.Context, id string) (*models.Document, error) {
    return s.storage.GetDocument(id)
}

func (s *Service) GetBySource(ctx context.Context, sourceType, sourceID string) (*models.Document, error) {
    return s.storage.GetDocumentBySource(sourceType, sourceID)
}

func (s *Service) Search(ctx context.Context, query *interfaces.SearchQuery) ([]*models.Document, error) {
    switch query.Mode {
    case interfaces.SearchModeKeyword:
        return s.storage.FullTextSearch(query.Text, query.Limit)
    case interfaces.SearchModeVector:
        return s.storage.VectorSearch(query.Embedding, query.Limit)
    case interfaces.SearchModeHybrid:
        return s.storage.HybridSearch(query.Text, query.Embedding, query.Limit)
    default:
        return nil, fmt.Errorf("invalid search mode: %s", query.Mode)
    }
}

func (s *Service) Count(ctx context.Context, sourceType string) (int, error) {
    if sourceType == "" {
        return s.storage.CountDocuments()
    }
    return s.storage.CountDocumentsBySource(sourceType)
}
```

### Step 2: EmbeddingService Interface

```go
// internal/interfaces/embedding_service.go
package interfaces

import (
    "context"
    "github.com/ternarybob/quaero/internal/models"
)

// EmbeddingService generates vector embeddings
type EmbeddingService interface {
    // Generate embedding for raw text
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

    // Generate and set embedding for a document
    EmbedDocument(ctx context.Context, doc *models.Document) error

    // Get model information
    ModelName() string
    Dimension() int
}
```

### Step 3: Update JiraScraperService

```go
// internal/services/atlassian/jira_scraper_service.go
package atlassian

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/google/uuid"
    "github.com/ternarybob/arbor"
    "github.com/ternarybob/quaero/internal/interfaces"
    "github.com/ternarybob/quaero/internal/models"
)

type JiraScraperService struct {
    storage         interfaces.JiraStorage
    documentService interfaces.DocumentService  // NEW
    authService     *AtlassianAuthService
    logger          arbor.ILogger
}

func NewJiraScraperService(
    storage interfaces.JiraStorage,
    documentService interfaces.DocumentService,  // NEW
    authService *AtlassianAuthService,
    logger arbor.ILogger,
) *JiraScraperService {
    return &JiraScraperService{
        storage:         storage,
        documentService: documentService,  // NEW
        authService:     authService,
        logger:          logger,
    }
}

// FetchAndStoreIssues fetches issues and stores in both tables
func (j *JiraScraperService) FetchAndStoreIssues(projectKey string) error {
    ctx := context.Background()

    // 1. Fetch issues (existing)
    issues, err := j.fetchIssues(projectKey)
    if err != nil {
        return fmt.Errorf("failed to fetch issues: %w", err)
    }

    // 2. Store in source-specific table (existing)
    for _, issue := range issues {
        if err := j.storage.SaveIssue(issue); err != nil {
            j.logger.Error().
                Err(err).
                Str("issue", issue.Key).
                Msg("Failed to save issue")
            continue
        }
    }

    // 3. Transform to documents (NEW - service owns transformation)
    documents := make([]*models.Document, 0, len(issues))
    for _, issue := range issues {
        doc, err := j.transformToDocument(issue)
        if err != nil {
            j.logger.Error().
                Err(err).
                Str("issue", issue.Key).
                Msg("Failed to transform issue")
            continue
        }
        documents = append(documents, doc)
    }

    // 4. Save via DocumentService (NEW)
    if err := j.documentService.SaveDocuments(ctx, documents); err != nil {
        return fmt.Errorf("failed to save documents: %w", err)
    }

    j.logger.Info().
        Str("project", projectKey).
        Int("issues", len(issues)).
        Int("documents", len(documents)).
        Msg("Stored issues and documents")

    return nil
}

// transformToDocument converts Jira issue to normalized document
// Service owns this transformation logic
func (j *JiraScraperService) transformToDocument(issue *models.JiraIssue) (*models.Document, error) {
    docID := fmt.Sprintf("doc_%s", uuid.New().String())

    // Build content
    content := j.buildContent(issue)
    contentMD := j.buildMarkdown(issue)

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

func (j *JiraScraperService) buildContent(issue *models.JiraIssue) string {
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

func (j *JiraScraperService) buildMarkdown(issue *models.JiraIssue) string {
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
```

### Step 4: Update ConfluenceScraperService

```go
// internal/services/atlassian/confluence_scraper_service.go
package atlassian

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/google/uuid"
    "github.com/ternarybob/arbor"
    "github.com/ternarybob/quaero/internal/interfaces"
    "github.com/ternarybob/quaero/internal/models"
)

type ConfluenceScraperService struct {
    storage         interfaces.ConfluenceStorage
    documentService interfaces.DocumentService  // NEW
    authService     *AtlassianAuthService
    logger          arbor.ILogger
}

func NewConfluenceScraperService(
    storage interfaces.ConfluenceStorage,
    documentService interfaces.DocumentService,  // NEW
    authService *AtlassianAuthService,
    logger arbor.ILogger,
) *ConfluenceScraperService {
    return &ConfluenceScraperService{
        storage:         storage,
        documentService: documentService,  // NEW
        authService:     authService,
        logger:          logger,
    }
}

// FetchAndStorePages fetches pages and stores in both tables
func (c *ConfluenceScraperService) FetchAndStorePages(spaceKey string) error {
    ctx := context.Background()

    // 1. Fetch pages
    pages, err := c.fetchPages(spaceKey)
    if err != nil {
        return fmt.Errorf("failed to fetch pages: %w", err)
    }

    // 2. Store in source-specific table
    for _, page := range pages {
        if err := c.storage.SavePage(page); err != nil {
            c.logger.Error().
                Err(err).
                Str("page_id", page.PageID).
                Msg("Failed to save page")
            continue
        }
    }

    // 3. Transform to documents (service owns transformation)
    documents := make([]*models.Document, 0, len(pages))
    for _, page := range pages {
        doc, err := c.transformToDocument(page)
        if err != nil {
            c.logger.Error().
                Err(err).
                Str("page_id", page.PageID).
                Msg("Failed to transform page")
            continue
        }
        documents = append(documents, doc)
    }

    // 4. Save via DocumentService
    if err := c.documentService.SaveDocuments(ctx, documents); err != nil {
        return fmt.Errorf("failed to save documents: %w", err)
    }

    c.logger.Info().
        Str("space", spaceKey).
        Int("pages", len(pages)).
        Int("documents", len(documents)).
        Msg("Stored pages and documents")

    return nil
}

// transformToDocument converts Confluence page to normalized document
func (c *ConfluenceScraperService) transformToDocument(page *models.ConfluencePage) (*models.Document, error) {
    docID := fmt.Sprintf("doc_%s", uuid.New().String())

    // Build content
    content := c.buildContent(page)
    contentMD := c.buildMarkdown(page)

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

func (c *ConfluenceScraperService) buildContent(page *models.ConfluencePage) string {
    return fmt.Sprintf("%s\n\n%s", page.Title, page.Content)
}

func (c *ConfluenceScraperService) buildMarkdown(page *models.ConfluencePage) string {
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

---

## Dependency Injection Updates

### App Initialization

```go
// internal/app/app.go

type App struct {
    Config            *common.Config
    Logger            arbor.ILogger
    StorageManager    interfaces.StorageManager

    // NEW Services
    EmbeddingService  interfaces.EmbeddingService
    DocumentService   interfaces.DocumentService

    // Existing Services
    AuthService       *atlassian.AtlassianAuthService
    JiraService       *atlassian.JiraScraperService
    ConfluenceService *atlassian.ConfluenceScraperService

    // Handlers...
}

func (a *App) initServices() error {
    var err error

    // 1. Initialize embedding service
    a.EmbeddingService = embeddings.NewService(
        a.Config.Embeddings.OllamaURL,
        a.Config.Embeddings.Model,
        a.Logger,
    )

    // 2. Initialize document service
    a.DocumentService = documents.NewService(
        a.StorageManager.DocumentStorage(),  // NEW storage interface
        a.EmbeddingService,
        a.Logger,
    )

    // 3. Initialize auth service
    a.AuthService, err = atlassian.NewAtlassianAuthService(
        a.StorageManager.AuthStorage(),
        a.Logger,
    )
    if err != nil {
        return fmt.Errorf("failed to initialize auth service: %w", err)
    }

    // 4. Initialize Jira service (NOW with DocumentService)
    a.JiraService = atlassian.NewJiraScraperService(
        a.StorageManager.JiraStorage(),
        a.DocumentService,  // NEW
        a.AuthService,
        a.Logger,
    )

    // 5. Initialize Confluence service (NOW with DocumentService)
    a.ConfluenceService = atlassian.NewConfluenceScraperService(
        a.StorageManager.ConfluenceStorage(),
        a.DocumentService,  // NEW
        a.AuthService,
        a.Logger,
    )

    return nil
}
```

---

## Benefits of This Architecture

### 1. **Service Ownership**
Each service owns its domain transformation:
```go
// Jira service knows how to transform Jira data
func (j *JiraScraperService) transformToDocument(issue *JiraIssue) (*Document, error)

// Confluence service knows how to transform Confluence data
func (c *ConfluenceScraperService) transformToDocument(page *ConfluencePage) (*Document, error)
```

### 2. **Single Responsibility**
- **JiraScraperService**: Fetch Jira data, transform to document
- **DocumentService**: Handle embeddings, storage, search
- **EmbeddingService**: Generate vectors

### 3. **Testability**
```go
// Test Jira transformation in isolation
func TestJiraService_TransformToDocument(t *testing.T) {
    mockDocService := &MockDocumentService{}
    service := NewJiraScraperService(storage, mockDocService, auth, logger)

    // Service can transform without needing real DocumentService
}
```

### 4. **Interface-Based**
Services depend on interfaces, not concrete types:
```go
type JiraScraperService struct {
    documentService interfaces.DocumentService  // Interface
}
```

### 5. **Clear Flow**
```
Collector → Transform → DocumentService → [Embed → Store]
```

---

## Implementation Checklist

- [ ] Define `DocumentService` interface
- [ ] Define `EmbeddingService` interface
- [ ] Implement `DocumentService` in `internal/services/documents/`
- [ ] Implement `EmbeddingService` in `internal/services/embeddings/`
- [ ] Add `DocumentStorage` to `StorageManager` interface
- [ ] Implement `DocumentStorage` in SQLite storage layer
- [ ] Update `JiraScraperService` constructor to accept `DocumentService`
- [ ] Add `transformToDocument()` method to `JiraScraperService`
- [ ] Update `ConfluenceScraperService` constructor to accept `DocumentService`
- [ ] Add `transformToDocument()` method to `ConfluenceScraperService`
- [ ] Update `App.initServices()` to wire dependencies
- [ ] Add configuration for embeddings (ollama URL, model)
- [ ] Create database migrations for documents table
- [ ] Write unit tests for transformations
- [ ] Write integration tests for end-to-end flow

---

This architecture makes each service responsible for its own domain while using shared services (DocumentService, EmbeddingService) for common functionality.
