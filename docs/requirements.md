# Quaero Requirements

**quaero** (Latin: "I seek, I search") - A local knowledge base system with natural language query capabilities.

Version: 2.1
Date: 2025-10-06
Status: Active Development

---

## Project Overview

### Purpose

Quaero is a self-contained knowledge base system that:
- Collects documentation from approved sources (Confluence, Jira, GitHub)
- Processes and stores content with full-text and vector search
- Provides natural language query interface using local LLMs (Ollama)
- Runs completely offline on a single machine
- Uses Chrome extension for seamless authentication

### Technology Stack

- **Language:** Go 1.25+
- **Web UI:** HTML templates, vanilla JavaScript, WebSockets
- **Storage:** SQLite with FTS5 (full-text search) and vector embeddings
- **LLM:** Ollama (local models for embeddings and text generation)
- **Browser Automation:** rod (for web scraping)
- **Authentication:** Chrome extension → WebSocket → HTTP service
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Banner:** github.com/ternarybob/banner (startup display)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2

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
