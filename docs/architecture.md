# Quaero Architecture

**Version:** 2.1
**Last Updated:** 2025-10-06
**Status:** Active Development

---

## Overview

Quaero is a knowledge collection and search system that gathers documentation from multiple sources (Confluence, Jira, GitHub) and provides semantic search capabilities using vector embeddings and local LLMs.

---

## Current Implementation Status

### ✅ Phase 1.0 - Core Infrastructure (COMPLETE)
- Web-based UI with real-time updates
- SQLite storage with FTS5 full-text search
- Chrome extension authentication
- Jira & Confluence collectors
- WebSocket for live log streaming
- RESTful API endpoints
- HTTP server with graceful shutdown
- Dependency injection architecture
- Test suite (integration & unit tests)

### ✅ Phase 1.1 - Vector Embeddings (COMPLETE)
- **Document Model:** Normalized document structure with metadata
- **Embedding Service:** Ollama integration for vector generation
- **Document Service:** High-level document management with embedding
- **Processing Service:** Background job for document extraction and vectorization
- **Scheduler:** CRON-based periodic processing
- **Document Storage:** SQLite persistence with embedding support
- **Documents UI:** Web interface for browsing vectorized documents
- **API Endpoints:**
  - `GET /api/documents/stats` - Document statistics
  - `GET /api/documents` - List documents with filtering
  - `POST /api/documents/process` - Trigger document processing

**Implementation Details:**
- Model: `nomic-embed-text` (768 dimensions)
- Storage: Binary serialization of float32 embeddings
- Processing: Automatic embedding generation on document save
- Scheduling: Configurable CRON schedule (default: every 6 hours)

### 🚧 Phase 1.2 - RAG Pipeline (IN PROGRESS)
- RAG orchestration service
- Context building from search results
- LLM integration for answer generation
- Natural language query interface (CLI & Web)

### 📋 Phase 2.0 - GitHub Integration (PLANNED)
- GitHub collector implementation
- Repository and wiki collection
- GitHub UI page

### 📋 Phase 3.0 - Advanced Search (PLANNED)
- Vector similarity search (requires sqlite-vec)
- Hybrid search (keyword + semantic)
- Image processing and OCR
- Additional data sources (Slack, Linear)

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser (Chrome)                                               │
│  ┌────────────────────────────────────────────────────┐         │
│  │  Quaero Chrome Extension                           │         │
│  │  • Captures Atlassian auth (cookies, tokens)       │         │
│  │  • Connects via WebSocket                          │         │
│  │  • Sends auth data to server                       │         │
│  └──────────────────┬─────────────────────────────────┘         │
└────────────────────┼────────────────────────────────────────────┘
                     │ WebSocket: ws://localhost:8080/ws
                     │
                     ↓
┌─────────────────────────────────────────────────────────────────┐
│  Quaero Server (Go HTTP/WebSocket)                              │
│                                                                  │
│  ┌─────────────────────────────────────────────────────┐        │
│  │  HTTP Server (internal/server/)                     │        │
│  │  • Routes (routes.go)                               │        │
│  │  • Middleware (middleware.go)                       │        │
│  │  • Graceful shutdown                                │        │
│  └──────────────────┬──────────────────────────────────┘        │
│                     │                                            │
│  ┌──────────────────▼──────────────────────────────────┐        │
│  │  Handlers (internal/handlers/)                      │        │
│  │  • WebSocketHandler - Real-time comms               │        │
│  │  • UIHandler - Serves web pages                     │        │
│  │  • CollectorHandler - Collection triggers           │        │
│  │  • DocumentHandler - Document API                   │        │
│  │  • DataHandler - Data API endpoints                 │        │
│  └──────────────────┬──────────────────────────────────┘        │
│                     │                                            │
│  ┌──────────────────▼──────────────────────────────────┐        │
│  │  Services (internal/services/)                      │        │
│  │  • atlassian/                                       │        │
│  │    - AtlassianAuthService - Auth management         │        │
│  │    - JiraScraperService - Jira collection           │        │
│  │    - ConfluenceScraperService - Confluence          │        │
│  │  • documents/                                       │        │
│  │    - DocumentService - Document management          │        │
│  │  • embeddings/                                      │        │
│  │    - EmbeddingService - Vector generation           │        │
│  │  • processing/                                      │        │
│  │    - ProcessingService - Background jobs            │        │
│  │    - Scheduler - CRON scheduling                    │        │
│  └──────────────────┬──────────────────────────────────┘        │
│                     │                                            │
│  ┌──────────────────▼──────────────────────────────────┐        │
│  │  Storage Layer (internal/storage/sqlite/)           │        │
│  │  • SQLiteDB - Connection manager                    │        │
│  │  • JiraStorage - Jira persistence                   │        │
│  │  • ConfluenceStorage - Confluence persistence       │        │
│  │  • DocumentStorage - Document persistence           │        │
│  │  • AuthStorage - Auth credentials                   │        │
│  └──────────────────┬──────────────────────────────────┘        │
└────────────────────┼────────────────────────────────────────────┘
                     │
                     ↓
┌─────────────────────────────────────────────────────────────────┐
│  SQLite Database (./quaero.db)                                  │
│  • jira_projects, jira_issues                                   │
│  • confluence_spaces, confluence_pages                          │
│  • documents (normalized with embeddings)                       │
│  • document_chunks (for large documents)                        │
│  • documents_fts (FTS5 full-text search index)                  │
│  • auth_credentials                                             │
└─────────────────────────────────────────────────────────────────┘
                     │
                     ↓
┌─────────────────────────────────────────────────────────────────┐
│  Ollama (Local LLM Server)                                      │
│  • nomic-embed-text - Embedding generation (768d)               │
│  • qwen2.5:32b - Text generation (future)                       │
│  • llama3.2-vision:11b - Vision tasks (future)                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
quaero/
├── cmd/
│   ├── quaero/                      # Main application
│   │   ├── main.go                  # Entry point, startup sequence
│   │   ├── serve.go                 # HTTP server command
│   │   └── version.go               # Version command
│   └── quaero-chrome-extension/     # Chrome extension
│       ├── manifest.json            # Extension configuration
│       ├── background.js            # Service worker
│       ├── popup.js                 # Extension popup
│       ├── sidepanel.js             # Side panel interface
│       └── content.js               # Page content interaction
│
├── internal/
│   ├── common/                      # Stateless utilities (NO receiver methods)
│   │   ├── config.go                # Configuration loading (TOML)
│   │   ├── logger.go                # Logger initialization (arbor)
│   │   ├── banner.go                # Startup banner (ternarybob/banner)
│   │   └── version.go               # Version management
│   │
│   ├── app/                         # Application orchestration
│   │   └── app.go                   # Manual dependency wiring
│   │
│   ├── services/                    # Stateful services (WITH receiver methods)
│   │   ├── atlassian/               # Jira & Confluence
│   │   │   ├── auth_service.go      # Authentication management
│   │   │   ├── jira_scraper_service.go
│   │   │   ├── jira_projects.go
│   │   │   ├── jira_issues.go
│   │   │   ├── jira_data.go
│   │   │   ├── confluence_scraper_service.go
│   │   │   ├── confluence_spaces.go
│   │   │   ├── confluence_pages.go
│   │   │   └── confluence_data.go
│   │   │
│   │   ├── documents/               # Document management
│   │   │   └── document_service.go  # High-level document operations
│   │   │
│   │   ├── embeddings/              # Vector embedding generation
│   │   │   └── embedding_service.go # Ollama integration
│   │   │
│   │   └── processing/              # Background processing
│   │       ├── processing_service.go # Document extraction & vectorization
│   │       └── scheduler.go         # CRON scheduler
│   │
│   ├── handlers/                    # HTTP handlers (constructor injection)
│   │   ├── websocket.go             # WebSocket handler
│   │   ├── ui.go                    # Web UI handler
│   │   ├── collector.go             # Collection endpoints
│   │   ├── document_handler.go      # Document API endpoints
│   │   ├── data.go                  # Data API endpoints
│   │   └── scraper.go               # Scraping endpoints
│   │
│   ├── storage/                     # Storage layer
│   │   ├── factory.go               # Storage factory
│   │   └── sqlite/                  # SQLite implementation
│   │       ├── manager.go           # Storage manager
│   │       ├── connection.go        # DB connection
│   │       ├── migrations.go        # Schema migrations
│   │       ├── jira_storage.go      # Jira persistence
│   │       ├── confluence_storage.go # Confluence persistence
│   │       ├── document_storage.go  # Document persistence
│   │       └── auth_storage.go      # Auth persistence
│   │
│   ├── interfaces/                  # Service interfaces
│   │   ├── storage.go               # Storage interfaces
│   │   ├── atlassian.go             # Atlassian interfaces
│   │   ├── document_service.go      # Document service interface
│   │   └── embedding_service.go     # Embedding service interface
│   │
│   └── models/                      # Data models
│       ├── atlassian.go             # Atlassian data structures
│       └── document.go              # Document model with embeddings
│
├── pages/                           # Web UI (NOT CLI)
│   ├── index.html                   # Main dashboard
│   ├── confluence.html              # Confluence UI
│   ├── jira.html                    # Jira UI
│   ├── documents.html               # Documents UI (NEW)
│   ├── partials/                    # Reusable components
│   │   ├── navbar.html
│   │   └── service-logs.html
│   └── static/                      # Static assets
│       ├── common.css
│       └── partial-loader.js
│
├── test/                            # Testing
│   ├── integration/                 # Integration tests
│   ├── ui/                          # UI tests
│   ├── run-tests.ps1                # Test runner script
│   └── README.md
│
├── scripts/                         # Build scripts
│   └── build.ps1                    # Build script
│
└── docs/                            # Documentation
    ├── architecture.md              # This file
    └── requirements.md              # Requirements doc
```

---

## Core Components

### 1. Document Model

**Location:** `internal/models/document.go`

**Structure:**
```go
type Document struct {
    // Identity
    ID         string // doc_{uuid}
    SourceType string // jira, confluence, github
    SourceID   string // Original ID from source

    // Content
    Title           string
    Content         string // Plain text
    ContentMarkdown string // Markdown format

    // Vector embedding
    Embedding      []float32 // 768 dimensions (nomic-embed-text)
    EmbeddingModel string    // Model name

    // Metadata (source-specific data stored as JSON)
    Metadata map[string]interface{}
    URL      string // Link to original

    // Timestamps
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

**Source-Specific Metadata:**
- **Jira:** IssueKey, ProjectKey, IssueType, Status, Priority, Assignee, Reporter, Labels, Components
- **Confluence:** PageID, SpaceKey, SpaceName, Author, Version, ContentType

### 2. Embedding Service

**Location:** `internal/services/embeddings/embedding_service.go`

**Responsibilities:**
- Connect to Ollama API
- Generate embeddings for text
- Embed documents (title + content)
- Generate query embeddings for search
- Check Ollama availability

**Key Methods:**
```go
func (s *Service) GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
func (s *Service) EmbedDocument(ctx context.Context, doc *models.Document) error
func (s *Service) EmbedDocuments(ctx context.Context, docs []*models.Document) error
func (s *Service) GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error)
func (s *Service) IsAvailable(ctx context.Context) bool
```

**Configuration:**
- Ollama URL: `http://localhost:11434`
- Model: `nomic-embed-text`
- Dimension: 768
- Timeout: 30 seconds

### 3. Document Service

**Location:** `internal/services/documents/document_service.go`

**Responsibilities:**
- Save documents with automatic embedding generation
- Update documents with re-embedding on content change
- Retrieve documents by ID or source reference
- Delete documents and chunks
- Search (keyword, vector, hybrid)
- Get statistics

**Key Methods:**
```go
func (s *Service) SaveDocument(ctx context.Context, doc *models.Document) error
func (s *Service) SaveDocuments(ctx context.Context, docs []*models.Document) error
func (s *Service) UpdateDocument(ctx context.Context, doc *models.Document) error
func (s *Service) GetDocument(ctx context.Context, id string) (*models.Document, error)
func (s *Service) GetBySource(ctx context.Context, sourceType, sourceID string) (*models.Document, error)
func (s *Service) DeleteDocument(ctx context.Context, id string) error
func (s *Service) Search(ctx context.Context, query *SearchQuery) ([]*models.Document, error)
func (s *Service) GetStats(ctx context.Context) (*models.DocumentStats, error)
func (s *Service) Count(ctx context.Context, sourceType string) (int, error)
func (s *Service) List(ctx context.Context, opts *ListOptions) ([]*models.Document, error)
```

**Search Modes:**
- **Keyword:** FTS5 full-text search
- **Vector:** Similarity search (requires sqlite-vec)
- **Hybrid:** Combined keyword + vector (future)

### 4. Processing Service

**Location:** `internal/services/processing/processing_service.go`

**Responsibilities:**
- Extract documents from source tables (Jira, Confluence)
- Transform to normalized document format
- Generate embeddings via DocumentService
- Track processing statistics
- Support incremental updates

**Key Methods:**
```go
func (s *Service) ProcessAll(ctx context.Context) (*ProcessingStats, error)
func (s *Service) ProcessJira(ctx context.Context) (*SourceStats, error)
func (s *Service) ProcessConfluence(ctx context.Context) (*SourceStats, error)
func (s *Service) VectorizeExisting(ctx context.Context) error
```

**Processing Flow:**
1. Get all items from source storage (Jira/Confluence)
2. For each item, check if document exists
3. If new, create document (will be done by collector)
4. If exists, check for updates
5. Track statistics (new, updated, errors)

### 5. Scheduler

**Location:** `internal/services/processing/scheduler.go`

**Responsibilities:**
- Schedule periodic document processing
- Support configurable CRON schedules
- Provide manual trigger capability
- Log processing results

**Key Methods:**
```go
func (s *Scheduler) Start(schedule string) error
func (s *Scheduler) Stop()
func (s *Scheduler) RunNow()
```

**Default Schedule:** `0 0 */6 * * *` (every 6 hours)

### 6. Document Storage

**Location:** `internal/storage/sqlite/document_storage.go`

**Responsibilities:**
- Persist documents with embeddings
- Binary serialization of float32 embeddings
- Full-text search using FTS5
- Vector search (future with sqlite-vec)
- Document statistics and counts

**Schema:**
```sql
CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT,
    content_markdown TEXT,
    embedding BLOB,
    embedding_model TEXT,
    metadata TEXT,
    url TEXT,
    created_at INTEGER,
    updated_at INTEGER,
    UNIQUE(source_type, source_id)
);

CREATE VIRTUAL TABLE documents_fts USING fts5(
    title,
    content,
    content=documents,
    content_rowid=rowid
);

CREATE TABLE document_chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content TEXT,
    embedding BLOB,
    token_count INTEGER,
    created_at INTEGER,
    UNIQUE(document_id, chunk_index),
    FOREIGN KEY(document_id) REFERENCES documents(id)
);
```

**Embedding Serialization:**
- Format: Little-endian binary (4 bytes per float32)
- Storage: BLOB column
- Deserialization: On-demand when needed

### 7. Document Handler

**Location:** `internal/handlers/document_handler.go`

**Endpoints:**
- `GET /api/documents/stats` - Document statistics
- `GET /api/documents` - List documents with filtering
- `POST /api/documents/process` - Trigger document processing

**Statistics Response:**
```json
{
    "total_documents": 150,
    "documents_by_source": {
        "jira": 75,
        "confluence": 75
    },
    "vectorized_count": 140,
    "vectorized_documents": 140,
    "jira_documents": 75,
    "confluence_documents": 75,
    "pending_vectorize": 10,
    "last_updated": "2025-10-06T12:00:00Z",
    "embedding_model": "nomic-embed-text",
    "average_content_size": 2500
}
```

### 8. Documents UI

**Location:** `pages/documents.html`

**Features:**
- Document statistics dashboard
- Searchable document table
- Source type filtering (Jira, Confluence)
- Vectorization status filtering
- Document detail viewer with JSON highlighting
- Real-time log streaming
- Manual processing trigger
- Responsive design

**Filters:**
- Text search (title, content, source ID)
- Source type (all, jira, confluence)
- Vectorization status (all, vectorized, not vectorized)

---

## Data Flow Diagrams

### Document Collection Flow

```
1. User triggers collection via Web UI
   ↓
2. CollectorHandler receives request
   ↓
3. JiraScraperService/ConfluenceScraperService
   ↓
4. Fetches data from Atlassian API
   ↓
5. Stores in source-specific tables
   ↓
6. Processing Service extracts from source tables
   ↓
7. Transforms to Document model
   ↓
8. DocumentService.SaveDocument()
   ↓
9. EmbeddingService.EmbedDocument()
   ↓
10. Generates vector embedding via Ollama
    ↓
11. DocumentStorage.SaveDocument()
    ↓
12. Persists to SQLite with embedding
    ↓
13. Updates FTS5 index
    ↓
14. Returns success
```

### Document Processing Flow

```
1. Scheduler triggers (CRON or manual)
   ↓
2. ProcessingService.ProcessAll()
   ↓
3. For Jira:
   ├─ Get all projects
   ├─ Get all issues per project
   ├─ Check if document exists
   ├─ Track new/updated/errors
   └─ Return statistics
   ↓
4. For Confluence:
   ├─ Get all spaces
   ├─ Get all pages per space
   ├─ Check if document exists
   ├─ Track new/updated/errors
   └─ Return statistics
   ↓
5. Log final statistics
   ↓
6. WebSocket broadcast to UI
```

### Search Flow (Current - FTS5 only)

```
1. User enters search query
   ↓
2. DocumentService.Search()
   ↓
3. Mode: Keyword
   ↓
4. DocumentStorage.FullTextSearch()
   ↓
5. FTS5 MATCH query
   ↓
6. Return ranked results
   ↓
7. Display in UI
```

### Search Flow (Future - Vector + Hybrid)

```
1. User enters search query
   ↓
2. DocumentService.Search()
   ↓
3. Mode: Vector or Hybrid
   ↓
4. EmbeddingService.GenerateQueryEmbedding()
   ↓
5. Get embedding from Ollama
   ↓
6a. Vector Mode:
    └─ DocumentStorage.VectorSearch()
       └─ sqlite-vec similarity search
       └─ Return top-k results
   ↓
6b. Hybrid Mode:
    ├─ DocumentStorage.FullTextSearch()
    ├─ DocumentStorage.VectorSearch()
    ├─ Merge and rank results
    └─ Return combined results
   ↓
7. Display in UI with relevance scores
```

---

## Authentication Flow

```
1. User logs into Atlassian (handles 2FA, SSO automatically)
   ↓
2. Extension extracts auth state:
   • Cookies (.atlassian.net domain)
   • Local storage tokens
   • Session tokens
   • Cloud ID, ATL tokens
   ↓
3. Extension connects to WebSocket:
   ws://localhost:8080/ws
   ↓
4. Extension sends AuthData message:
   {
     "type": "auth",
     "payload": {
       "cookies": [...],
       "tokens": {...},
       "baseUrl": "https://company.atlassian.net"
     }
   }
   ↓
5. Server stores in auth_credentials table
   ↓
6. Collectors use stored auth for API calls
   ↓
7. Extension refreshes auth periodically
```

---

## API Endpoints

### HTTP Endpoints

```
GET  /                              - Dashboard UI
GET  /confluence                    - Confluence UI
GET  /jira                          - Jira UI
GET  /documents                     - Documents UI (NEW)

POST /api/collect/jira              - Trigger Jira collection
POST /api/collect/confluence        - Trigger Confluence collection

GET  /api/data/jira/projects        - Get Jira projects
GET  /api/data/jira/issues          - Get Jira issues
GET  /api/data/confluence/spaces    - Get Confluence spaces
GET  /api/data/confluence/pages     - Get Confluence pages

GET  /api/documents/stats           - Get document statistics (NEW)
GET  /api/documents                 - List documents with filtering (NEW)
POST /api/documents/process         - Trigger document processing (NEW)

GET  /health                        - Health check
```

### WebSocket Endpoint

```
WS   /ws                            - WebSocket connection
```

**Messages:**
- **Client → Server:** Auth data from extension
- **Server → Client:** Log messages, status updates

---

## Technology Stack

**Language:** Go 1.25+

**Core Libraries:**
- `github.com/ternarybob/arbor` - Logging (REQUIRED)
- `github.com/ternarybob/banner` - Banners (REQUIRED)
- `github.com/pelletier/go-toml/v2` - TOML config (REQUIRED)
- `github.com/spf13/cobra` - CLI framework
- `github.com/gorilla/websocket` - WebSocket
- `modernc.org/sqlite` - SQLite driver
- `github.com/robfig/cron/v3` - CRON scheduling

**Storage:** SQLite with FTS5

**Frontend:** Vanilla HTML/CSS/JavaScript

**Browser:** Chrome Extension (Manifest V3)

**LLM:** Ollama (local)

---

## Remaining Work

### Phase 1.2 - RAG Pipeline
- RAG orchestration service
- Context building from search results
- LLM integration for answer generation
- Natural language query interface (CLI & Web)
- Answer formatting with citations

### Phase 2.0 - GitHub Integration
- GitHub service implementation
- Repository and wiki collection
- GitHub storage schema
- GitHub UI page
- API endpoints for GitHub data

### Phase 3.0 - Advanced Search
- **Vector Search:** Integrate sqlite-vec extension
- **Hybrid Search:** Combine FTS5 + vector similarity
- **Image Processing:** OCR and vision model integration
- **Search Ranking:** Advanced ranking algorithms
- **Faceted Search:** Multiple filter dimensions

### Phase 4.0 - Additional Features
- **Incremental Updates:** Only process changed documents
- **Document Versioning:** Track changes over time
- **Scheduled Collections:** Automated periodic collection
- **Multi-User Support:** User authentication and preferences
- **Additional Sources:** Slack, Linear, Notion

---

## Performance Considerations

### Current Performance

**Document Processing:**
- Embedding generation: ~100-200ms per document (depends on Ollama)
- Batch processing: Processes documents sequentially
- Storage: SQLite handles thousands of documents efficiently

**Search Performance:**
- FTS5 keyword search: Sub-second for 10k+ documents
- Vector search: Not yet implemented (requires sqlite-vec)

### Future Optimizations

**Embedding Generation:**
- Batch embedding requests to Ollama
- Parallel processing for multiple documents
- Caching for duplicate content

**Vector Search:**
- Approximate nearest neighbor (ANN) with sqlite-vec
- Index optimization for large datasets
- Result caching for common queries

**Storage:**
- WAL mode for better concurrency
- Periodic VACUUM for database maintenance
- Connection pooling for handlers

---

## Security Considerations

**Authentication:**
- Credentials stored in SQLite (encrypted at rest recommended)
- WebSocket origin validation
- HTTPS for production deployments

**Input Validation:**
- SQL injection prevention (parameterized queries)
- XSS prevention (HTML escaping in UI)
- CSRF protection for state-changing operations

**Dependencies:**
- Regular dependency updates
- Vulnerability scanning
- Minimal dependency surface

---

## Testing Strategy

**Unit Tests:**
- Service logic
- Data transformations
- Utility functions

**Integration Tests:**
- End-to-end collection flows
- Database operations
- API endpoints

**Performance Tests:**
- Embedding generation benchmarks
- Search performance
- Large dataset handling

---

**Last Updated:** 2025-10-06
**Status:** Active Development
**Version:** 2.1
