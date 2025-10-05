# Quaero Requirements

**quaero** (Latin: "I seek, I search") - A local knowledge base system with natural language query capabilities.

Version: 2.2
Date: 2025-10-06
Status: Active Development

---

## Acknowledgments

Quaero draws inspiration from [Agent Zero](https://github.com/agent0ai/agent-zero), a sophisticated AI framework featuring advanced memory management and multi-provider LLM integration. While Agent Zero is a general-purpose AI assistant focused on task execution, Quaero is purpose-built for local knowledge base management with a simpler, Docker-free deployment model.

**Key concepts adopted from Agent Zero:**
- Memory area categorization for organizing different knowledge types
- Similarity threshold filtering for better search relevance
- Embedding caching to improve performance
- Tool-based architecture for modular RAG components

**Key differences from Agent Zero:**
- **Deployment**: Native Go binary (no Docker) vs Docker-required
- **Scope**: Focused knowledge base vs general AI assistant
- **Storage**: SQLite with FTS5 vs FAISS vector database
- **LLM**: Ollama-only (local-first) vs multi-provider via LiteLLM
- **UI**: WebSocket streaming vs HTTP polling

---

## Project Overview

### Purpose

Quaero is a self-contained knowledge base system that:
- Collects documentation from approved sources (Confluence, Jira, GitHub)
- Processes and stores content with full-text and vector search
- Provides natural language query interface using local LLMs (Ollama)
- **Runs completely offline on a single machine (NO Docker required)**
- Uses Chrome extension for seamless authentication
- Organizes knowledge into memory areas for better retrieval

### Technology Stack

- **Language:** Go 1.25+
- **Web UI:** HTML templates, vanilla JavaScript, WebSockets
- **Storage:** SQLite with FTS5 (full-text search) and vector embeddings
- **LLM:** Ollama (local models for embeddings and text generation) - **NO Docker required**
- **Browser Automation:** rod (for web scraping)
- **Authentication:** Chrome extension → WebSocket → HTTP service
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Banner:** github.com/ternarybob/banner (startup display)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2

### Deployment Model

**Zero Dependencies Deployment:**
- Quaero: Native Go binary (Windows, macOS, Linux)
- Ollama: Native service (no Docker)
- SQLite: Embedded database (no server)
- **Total Docker containers required: 0**

**Installation:**
```bash
# Install Ollama (native service)
# Windows: Download from ollama.com
# macOS: brew install ollama
# Linux: curl -fsSL https://ollama.com/install.sh | sh

# Pull models
ollama pull nomic-embed-text
ollama pull qwen2.5:32b

# Run Quaero
./bin/quaero serve --config deployments/local/quaero.toml
```

---

## Approved Data Sources

**ONLY these collectors are approved:**

### 1. Confluence
- **Location:** `internal/services/atlassian/confluence_*`
- **Features:** Spaces, pages, attachments, images, browser scraping
- **API:** Confluence REST API v2
- **Authentication:** Cookies + token from Chrome extension

### 2. Jira
- **Location:** `internal/services/atlassian/jira_*`
- **Features:** Projects, issues, comments, attachments
- **API:** Jira REST API v3
- **Authentication:** Cookies + token from Chrome extension

### 3. GitHub
- **Location:** `internal/services/github/*`
- **Features:** Repositories, README files, wiki pages, issues (optional), PRs (optional)
- **API:** GitHub REST API v3
- **Authentication:** Personal access token

---

## Configuration System

### Priority Order

1. **CLI Flags** (highest priority)
2. **Environment Variables**
3. **Config File** (`config.toml`)
4. **Defaults** (lowest priority)

### Configuration File Format

**Location:** `config.toml`

```toml
[server]
host = "localhost"
port = 8080

[logging]
level = "info"
format = "json"

[confluence]
base_url = "https://yourcompany.atlassian.net"

[jira]
base_url = "https://yourcompany.atlassian.net"

[github]
base_url = "https://api.github.com"
token = ""  # Set via environment variable

[storage]
type = "sqlite"
path = "./data/quaero.db"

[storage.sqlite]
enable_fts5 = true
enable_vector = true
embedding_dimension = 768
cache_size_mb = 100
wal_mode = true

[embeddings]
ollama_url = "http://localhost:11434"
model = "nomic-embed-text"
dimension = 768

[processing]
schedule = "0 0 */6 * * *"  # Every 6 hours
enabled = true
```

### Environment Variables

```bash
QUAERO_PORT=8080
QUAERO_HOST=localhost
QUAERO_LOG_LEVEL=info
QUAERO_GITHUB_TOKEN=ghp_xxx
QUAERO_OLLAMA_URL=http://localhost:11434
```

---

## Memory System & RAG Design

### Memory Areas (Inspired by Agent Zero)

Documents are categorized into memory areas for better organization and retrieval:

**Memory Area Types:**
- **main** - Primary knowledge (Confluence pages, Jira issues, GitHub docs)
- **fragments** - Conversation history and interactions
- **solutions** - Resolved queries and answers
- **facts** - Extracted facts and summaries

**Benefits:**
- Targeted retrieval based on query type
- Better context selection for RAG
- Cleaner separation of knowledge types

**Implementation:**
```go
type MemoryArea string

const (
    MemoryAreaMain       MemoryArea = "main"
    MemoryAreaFragments  MemoryArea = "fragments"
    MemoryAreaSolutions  MemoryArea = "solutions"
    MemoryAreaFacts      MemoryArea = "facts"
)

type Document struct {
    // ... existing fields
    MemoryArea MemoryArea `json:"memory_area"`
}
```

### Search & Retrieval

**Search Modes:**
- **Keyword Search** - FTS5 full-text search
- **Vector Search** - Cosine similarity with embeddings
- **Hybrid Search** - Combined keyword + vector with weighted ranking

**Search Options:**
```go
type SearchOptions struct {
    Query               string
    Limit               int
    Mode                SearchMode  // keyword, vector, hybrid
    SimilarityThreshold float32     // 0.0-1.0 (filter by relevance)
    MinScore            float32     // FTS5 minimum score
    MemoryAreas         []MemoryArea // Filter by area
}
```

**Similarity Threshold Filtering:**
- Inspired by Agent Zero's filtering mechanism
- Only returns results above configurable threshold (default: 0.7)
- Reduces noise and improves relevance
- Configurable per-query

### Embedding Caching

**Purpose:** Avoid redundant embedding generation for duplicate content

**Strategy:**
- Cache embeddings by content hash (SHA-256)
- In-memory cache with LRU eviction
- TTL-based expiration (default: 24 hours)
- Survives restarts via database storage

**Implementation:**
```go
type EmbeddingCache struct {
    cache   map[string]CachedEmbedding
    maxSize int           // LRU limit
    ttl     time.Duration // Expiration
}

func (s *EmbeddingService) GenerateEmbedding(text string) ([]float32, error) {
    // Check cache first
    if cached := s.cache.Get(hash(text)); cached != nil {
        return cached, nil
    }
    // Generate and cache
    embedding := s.ollama.Embed(text)
    s.cache.Set(hash(text), embedding)
    return embedding
}
```

### RAG (Retrieval-Augmented Generation)

**Architecture:** Tool-based modular design

**RAG Pipeline:**
1. **Query Analysis** - Understand user intent
2. **Retrieval** - Search knowledge base (hybrid search)
3. **Context Building** - Assemble relevant documents
4. **Generation** - LLM generates answer with context
5. **Citation** - Include source links

**Tool Interface:**
```go
type RAGTool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input map[string]interface{}) (interface{}, error)
}

// Tools:
// - SearchTool: Knowledge base search
// - SummarizeTool: Document summarization
// - ExtractTool: Fact extraction
```

**LLM Integration:**
- Primary: Ollama (local models)
- Future: Multi-provider via LiteLLM (if needed)
- Model roles: chat (text generation), embedding (vectors)

---

## Required Libraries

### Mandatory Dependencies

**MUST USE:**
- `github.com/ternarybob/arbor` - All logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML configuration

**FORBIDDEN:**
- `fmt.Println` / `log.Println` for logging
- Any other logging library
- Any other config format (JSON, YAML)

---

## Startup Sequence

**REQUIRED ORDER in `main.go`:**

1. **Configuration Loading**
   ```go
   config, err := common.LoadFromFile(configPath)
   ```

2. **CLI Overrides**
   ```go
   common.ApplyCLIOverrides(config, serverPort, serverHost)
   ```

3. **Logger Initialization**
   ```go
   logger := common.InitLogger(config)
   ```

4. **Banner Display** (MANDATORY)
   ```go
   common.PrintBanner(config, logger)
   ```

5. **Version Logging**
   ```go
   version := common.GetVersion()
   logger.Info().Str("version", version).Msg("Quaero starting")
   ```

6. **Service Initialization**
   ```go
   // Storage
   db := sqlite.NewSQLiteDB(config, logger)

   // Services
   embeddingService := embeddings.NewService(ollamaURL, modelName, dimension, logger)
   documentService := documents.NewService(documentStorage, embeddingService, logger)
   processingService := processing.NewService(documentService, jiraStorage, confluenceStorage, logger)

   // Scheduler
   scheduler := processing.NewScheduler(processingService, logger)
   scheduler.Start(config.Processing.Schedule)
   ```

7. **Handler Initialization**
   ```go
   wsHandler := handlers.NewWebSocketHandler()
   collectorHandler := handlers.NewCollectorHandler(logger, confluenceService, jiraService)
   documentHandler := handlers.NewDocumentHandler(documentService, processingService)
   uiHandler := handlers.NewUIHandler(logger)
   ```

8. **Server Start**
   ```go
   server := server.New(logger, config, handlers)
   server.Start()
   ```

---

## Logging Standards

### Required Patterns

**Structured Logging:**
```go
logger.Info().
    Str("service", "confluence").
    Int("pages", count).
    Dur("duration", elapsed).
    Msg("Collection completed")
```

**Error Logging:**
```go
logger.Error().
    Err(err).
    Str("space", spaceKey).
    Msg("Failed to collect space")
```

**Debug Logging:**
```go
logger.Debug().
    Str("url", apiURL).
    Int("status", resp.StatusCode).
    Msg("API response received")
```

**Logger Injection:**
```go
type Service struct {
    logger arbor.ILogger
}

func NewService(logger arbor.ILogger) *Service {
    return &Service{logger: logger}
}
```

---

## Banner Requirement

### MANDATORY Display

**MUST use:** `github.com/ternarybob/banner`

**Implementation:**
```go
import "github.com/ternarybob/banner"

func PrintBanner(cfg *Config, logger arbor.ILogger) {
    b := banner.New()
    b.SetTitle("Quaero")
    b.SetSubtitle("Knowledge Search System")
    b.AddLine("Version", common.GetVersion())
    b.AddLine("Server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
    b.AddLine("Config", cfg.LoadedFrom)
    b.Print()
}
```

**Display Requirements:**
- Show version number
- Show server host and port
- Show configuration source (file path or "defaults")
- MUST be called after logger initialization
- MUST be called before services start

---

## Directory Structure Standards

### internal/common/ - Stateless Utilities

**Rules:**
- ✅ Pure functions only
- ✅ No state
- ❌ NO receiver methods

**Example:**
```go
// ✅ CORRECT
func LoadFromFile(path string) (*Config, error) {
    // Pure function
}

// ❌ WRONG
func (c *Config) Load() error {
    // Belongs in services/
}
```

### internal/services/ - Stateful Services

**Rules:**
- ✅ MUST use receiver methods
- ✅ State management
- ✅ Implement interfaces

**Example:**
```go
// ✅ CORRECT
type DocumentService struct {
    storage          interfaces.DocumentStorage
    embeddingService interfaces.EmbeddingService
    logger           arbor.ILogger
}

func (s *DocumentService) SaveDocument(ctx context.Context, doc *models.Document) error {
    // Use s.storage, s.embeddingService, s.logger
}
```

### internal/handlers/ - HTTP Handlers

**Rules:**
- ✅ Constructor-based dependency injection
- ✅ Interface-based (where applicable)
- ✅ Thin layer - delegates to services

**Example:**
```go
// ✅ CORRECT
type DocumentHandler struct {
    documentService   interfaces.DocumentService
    processingService *processing.Service
    logger            arbor.ILogger
}

func NewDocumentHandler(
    documentService interfaces.DocumentService,
    processingService *processing.Service,
) *DocumentHandler {
    return &DocumentHandler{
        documentService:   documentService,
        processingService: processingService,
        logger:            common.GetLogger(),
    }
}
```

---

## Error Handling

### Required Patterns

**No Ignored Errors:**
```go
// ✅ CORRECT
data, err := loadData()
if err != nil {
    return fmt.Errorf("failed to load data: %w", err)
}

// ❌ FORBIDDEN
data, _ := loadData()
```

**Error Wrapping:**
```go
// ✅ CORRECT
return fmt.Errorf("failed to collect pages: %w", err)

// ❌ AVOID
return err
```

**Error Logging:**
```go
if err := service.Collect(); err != nil {
    logger.Error().Err(err).Msg("Collection failed")
    return err
}
```

---

## Code Quality Standards

### Function Structure
- **Max Lines:** 80 (ideal: 20-40)
- **Single Responsibility:** One purpose per function
- **Error Handling:** Comprehensive validation
- **Naming:** Descriptive, intention-revealing

### File Structure
- **Max Lines:** 500
- **Modular Design:** Extract utilities to shared files
- **Clear Organization:** Logical grouping of related functions

### Forbidden Patterns
- `TODO:` comments
- `FIXME:` comments
- Hardcoded credentials
- Unused imports
- Dead code
- Ignored errors (`_ = err`)

---

## Testing Standards

### Test Coverage Goals

- **Critical paths:** 100%
- **Services:** 80%+
- **Handlers:** 80%+
- **Utilities:** 90%+

### Testing Commands

**ALWAYS use the test script:**
```bash
./test/run-tests.ps1 -Type all
./test/run-tests.ps1 -Type unit
./test/run-tests.ps1 -Type integration
```

**NEVER use:**
```bash
cd test && go test      # ❌ WRONG
go test ./...           # ❌ WRONG
```

---

## Build Standards

**ALWAYS use the build script:**
```bash
./scripts/build.ps1
./scripts/build.ps1 -Clean -Release
```

**NEVER use:**
```bash
go build                # ❌ WRONG
```

---

## Chrome Extension

### Purpose

Captures authentication credentials from Atlassian sites (Confluence, Jira) and sends to Quaero server.

### Integration Flow

1. User navigates to Confluence/Jira in Chrome
2. User clicks extension icon
3. Extension captures cookies and tokens
4. Extension connects to `ws://localhost:8080/ws`
5. Extension sends `AuthData` message
6. Server receives and stores credentials
7. Collectors use credentials for API calls

### Installation

1. Open Chrome Extensions (`chrome://extensions/`)
2. Enable "Developer mode"
3. Click "Load unpacked"
4. Select `cmd/quaero-chrome-extension/`

---

## Deployment

### Build

```bash
# Development build
go build -o bin/quaero ./cmd/quaero

# Production build (with version)
./scripts/build.ps1 -Release
```

### Run

```bash
# With config file
./bin/quaero serve --config config.toml

# With CLI flags
./bin/quaero serve --port 8080 --host localhost

# With environment variables
export QUAERO_PORT=8080
./bin/quaero serve
```

---

## Compliance Rules

### Required Libraries

✅ **MUST USE:**
- `github.com/ternarybob/arbor` - Logging
- `github.com/ternarybob/banner` - Banners
- `github.com/pelletier/go-toml/v2` - TOML config

❌ **FORBIDDEN:**
- `fmt.Println` / `log.Println` for logging
- Any other logging library
- Any other config format (JSON, YAML)

### Architecture Rules

✅ **REQUIRED:**
- Stateless functions in `internal/common/`
- Receiver methods in `internal/services/`
- Interface injection in `internal/handlers/`
- Banner on startup
- Structured logging

❌ **FORBIDDEN:**
- Receiver methods in `internal/common/`
- Direct service instantiation in handlers
- Ignored errors
- TODO/FIXME in committed code

---

**Last Updated:** 2025-10-06
**Status:** Active Development
**Version:** 2.1
