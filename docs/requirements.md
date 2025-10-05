# Quaero Requirements

**quaero** (Latin: "I seek, I search") - A local knowledge base system with natural language query capabilities.

Version: 2.0
Date: 2025-10-05
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
- **Storage:** SQLite with FTS5 (full-text search) and sqlite-vec (vector embeddings)
- **LLM:** Ollama (Qwen2.5-32B for text, Llama3.2-Vision-11B for images)
- **Browser Automation:** rod (for web scraping)
- **Authentication:** Chrome extension → WebSocket → HTTP service
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Banner:** github.com/ternarybob/banner (startup display)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2

---

## Architecture

### Monorepo Structure

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
│       └── sidepanel.js             # Side panel interface
│
├── internal/
│   ├── common/                      # Stateless utilities
│   │   ├── config.go                # Configuration loading
│   │   ├── logger.go                # Logger initialization
│   │   ├── banner.go                # Startup banner
│   │   └── version.go               # Version management
│   ├── services/                    # Stateful services
│   │   ├── atlassian/               # Jira & Confluence
│   │   │   ├── confluence_service.go
│   │   │   ├── confluence_api.go
│   │   │   ├── confluence_scraper.go
│   │   │   ├── jira_service.go
│   │   │   └── jira_api.go
│   │   └── github/                  # GitHub collector
│   │       ├── github_service.go
│   │       └── github_api.go
│   ├── handlers/                    # HTTP handlers
│   │   ├── websocket.go             # WebSocket handler
│   │   ├── collector.go             # Collector endpoints
│   │   ├── ui.go                    # Web UI handler
│   │   └── api.go                   # REST API
│   ├── models/                      # Data models
│   │   ├── document.go
│   │   └── atlassian.go
│   ├── interfaces/                  # Service interfaces
│   │   ├── collector.go
│   │   └── atlassian.go
│   └── server/                      # HTTP server
│       ├── server.go
│       ├── routes.go
│       └── middleware.go
│
├── pages/                           # Web UI (NOT CLI)
│   ├── index.html                   # Main dashboard
│   ├── confluence.html              # Confluence UI
│   ├── jira.html                    # Jira UI
│   ├── partials/                    # Reusable components
│   │   ├── navbar.html
│   │   ├── footer.html
│   │   └── service-logs.html
│   └── static/                      # Static assets
│       ├── common.css
│       └── partial-loader.js
│
├── test/                            # Integration tests
│   ├── integration/
│   └── fixtures/
│
└── docs/                            # Documentation
    ├── requirements.md              # This file
    ├── api.md                       # API documentation
    └── development.md               # Developer guide
```

### Clean Architecture Patterns

**`internal/common/` - Stateless Utilities:**
- NO receiver methods
- Pure functions only
- Configuration loading
- Logger initialization
- Banner display
- Version management

**`internal/services/` - Stateful Services:**
- MUST use receiver methods
- State management
- Implement interfaces
- Business logic

**`internal/handlers/` - HTTP Handlers:**
- Dependency injection (interfaces)
- Thin layer
- Delegate to services
- HTTP request/response handling

---

## Collectors

### Approved Collectors

**ONLY these collectors are approved:**

#### 1. Confluence
- **Location:** `internal/services/atlassian/confluence_*`
- **Features:**
  - Fetch spaces
  - Fetch pages in space
  - Fetch page content (HTML storage format)
  - Fetch attachments
  - Extract images
  - Browser scraping for JavaScript-rendered content
  - Screenshot capture
- **API:** Confluence REST API v2
- **Authentication:** Cookies + token from Chrome extension

#### 2. Jira
- **Location:** `internal/services/atlassian/jira_*`
- **Features:**
  - Fetch projects
  - Fetch issues in project
  - Fetch issue details
  - Fetch comments
  - Fetch attachments
- **API:** Jira REST API v3
- **Authentication:** Cookies + token from Chrome extension

#### 3. GitHub
- **Location:** `internal/services/github/*`
- **Features:**
  - Fetch repositories
  - Fetch README files
  - Fetch wiki pages
  - Fetch issues (optional)
  - Fetch pull requests (optional)
- **API:** GitHub REST API v3
- **Authentication:** Personal access token

### Collector Interface

```go
type Collector interface {
    Collect(ctx context.Context) ([]models.Document, error)
    Name() string
    SupportsImages() bool
}
```

---

## Web UI

### Pages

**Dashboard** (`pages/index.html`)
- System status
- Active collectors
- Recent collections
- Real-time logs

**Confluence** (`pages/confluence.html`)
- Space selector
- Collection trigger
- Progress display
- Status updates

**Jira** (`pages/jira.html`)
- Project selector
- Collection trigger
- Progress display
- Status updates

### WebSocket Integration

**Endpoint:** `ws://localhost:8080/ws`

**Message Types:**

From Server to Client:
```json
{
  "type": "log",
  "payload": {
    "timestamp": "15:04:05",
    "level": "info",
    "message": "Collection started"
  }
}

{
  "type": "status",
  "payload": {
    "service": "confluence",
    "status": "running",
    "pagesCount": 42,
    "lastScrape": "2025-10-05T10:30:00Z"
  }
}
```

From Client to Server:
```json
{
  "type": "auth",
  "payload": {
    "cookies": ["session=abc123"],
    "token": "bearer-token"
  }
}
```

### Real-Time Features

- **Log Streaming:** Backend logs streamed to browser
- **Status Updates:** Collection progress and counts
- **Connection Management:** Multiple concurrent clients
- **Automatic Reconnection:** Client reconnects on disconnect

---

## Chrome Extension

### Purpose

Captures authentication credentials from Atlassian sites (Confluence, Jira) and sends to Quaero server.

### Structure

```
cmd/quaero-chrome-extension/
├── manifest.json        # Extension configuration (Manifest V3)
├── background.js        # Service worker (authentication capture)
├── popup.html           # Extension popup UI
├── popup.js             # Popup logic
├── sidepanel.html       # Side panel UI
├── sidepanel.js         # Side panel logic
├── content.js           # Page content interaction
└── icons/               # Extension icons
```

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

## Configuration

### Priority Order

1. **CLI Flags** (highest priority)
2. **Environment Variables**
3. **Config File** (`config.toml`)
4. **Defaults** (lowest priority)

### Configuration File

**Location:** `config.toml`

**Format:**
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
embedding_dimension = 1536
cache_size_mb = 100
wal_mode = true
```

### Environment Variables

```bash
QUAERO_PORT=8080
QUAERO_HOST=localhost
QUAERO_LOG_LEVEL=info
QUAERO_GITHUB_TOKEN=ghp_xxx
```

### CLI Flags

```bash
quaero serve --port 8080 --host localhost --config /path/to/config.toml
```

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
   confluenceService := services.NewConfluenceService(logger, config)
   jiraService := services.NewJiraService(logger, config)
   githubService := services.NewGitHubService(logger, config)
   ```

7. **Handler Initialization**
   ```go
   wsHandler := handlers.NewWebSocketHandler()
   collectorHandler := handlers.NewCollectorHandler(logger, confluenceService, jiraService)
   uiHandler := handlers.NewUIHandler(logger)
   ```

8. **Server Start**
   ```go
   server := server.New(logger, config, wsHandler, collectorHandler, uiHandler)
   server.Start()
   ```

---

## Logging Standards

### Required Library

**MUST USE:** `github.com/ternarybob/arbor`

**FORBIDDEN:**
- `fmt.Println`
- `log.Println`
- Any other logging library

### Logging Patterns

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
- Max 80 lines per function (ideal: 20-40)
- Single responsibility principle
- Comprehensive error handling
- Descriptive names

### File Structure
- Max 500 lines per file
- Modular design
- Extract utilities to shared files

### Naming Conventions
- Private functions: `_helperFunction` (underscore prefix)
- Public functions: `CollectPages` (exported)
- Constants: `MAX_RETRIES`
- Interfaces: `Collector` (no "I" prefix in Go)

### Forbidden Patterns
- `TODO:` comments
- `FIXME:` comments
- Hardcoded credentials
- Unused imports
- Dead code
- Ignored errors

---

## Testing

### Test Structure

**Integration Tests:** `test/integration/`
- Confluence collector tests
- Jira collector tests
- GitHub collector tests
- WebSocket tests
- API endpoint tests

**Unit Tests:** Next to code files
```
internal/services/confluence_service.go
internal/services/confluence_service_test.go
```

### Test Coverage Goals

- Critical paths: 100%
- Services: 80%+
- Handlers: 80%+
- Utilities: 90%+

### Testing Commands

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Integration tests only
go test -tags=integration ./test/integration

# Specific package
go test ./internal/services/atlassian
```

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

## Development Workflow

### Agent-Based Development

See `CLAUDE.md` for agent architecture details.

**Agents:**
- `overwatch` - Reviews all changes, enforces standards
- `go-refactor` - Consolidates duplicates, optimizes structure
- `go-compliance` - Enforces Go standards
- `test-engineer` - Writes tests, ensures coverage
- `collector-impl` - Implements collectors
- `doc-writer` - Maintains documentation

**Usage:**
```bash
# Automatic review
# Overwatch reviews all Write/Edit operations

# Explicit invocation
> Use go-refactor to consolidate duplicates
> Have test-engineer write integration tests
```

---

## Future Enhancements

See [remaining-requirements.md](remaining-requirements.md) for detailed roadmap.

### Near Term (v2.1)

- [ ] Vector embeddings with sqlite-vec
- [ ] RAG pipeline with Ollama
- [ ] Natural language query interface (CLI & Web)
- [ ] Image processing and OCR

### Medium Term (v2.2 - v3.0)

- [ ] GitHub collector
- [ ] Slack collector (optional)
- [ ] Linear collector (optional)
- [ ] Hybrid search (keyword + semantic)
- [ ] Document versioning
- [ ] Incremental updates

### Long Term (v3.1+)

- [ ] Multi-user support
- [ ] Cloud deployment options
- [ ] API key management
- [ ] Scheduled collections
- [ ] Notifications system

### Not Planned

- CLI-based collection (replaced by Web UI)
- Multiple database backends (SQLite only)
- Built-in authentication (extension handles auth)

---

## Compliance

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

**Last Updated:** 2025-10-05
**Status:** Active Development
**Version:** 2.0
