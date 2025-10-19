# AGENTS.md

This file provides guidance to AI agents (Claude Code, GitHub Copilot, etc.) when working with code in this repository.

## CRITICAL: BUILD AND TEST

**Failure to follow these instructions will result in your removal from the project.**

### Build Instructions (Windows ONLY)

**Building, compiling, and running the application MUST be done using:**
- `.\scripts\build.ps1`
- `.\scripts\build.ps1 -Run`
- **ONLY exception:** `go build` for compile tests (no output binary)

**Build Commands:**

```powershell
# Development build
.\scripts\build.ps1

# Clean build
.\scripts\build.ps1 -Clean

# Release build (optimized)
.\scripts\build.ps1 -Release

# Build and run
.\scripts\build.ps1 -Run
```

### Testing Instructions

**CRITICAL: The test runner handles EVERYTHING automatically - do NOT run build scripts or start the service manually!**

**Test Runner** (`cmd/quaero-test-runner/`):
- Builds the application using `scripts/build.ps1`
- Starts a test server on port 3333 for browser validation
- Starts the Quaero service in a visible window
- Waits for service readiness
- Runs all test suites (API + UI)
- Captures screenshots for UI tests
- Saves results to timestamped directories
- Stops the service and cleans up

**How to Run Tests:**

```powershell
# Option 1: Use pre-built test runner (recommended)
.\scripts\build.ps1           # Builds test runner automatically
cd bin
.\quaero-test-runner.exe

# Option 2: Run from source
cd cmd/quaero-test-runner
go run .
```

**For Development/Debugging Only:**
```powershell
# Run tests directly (requires manual service start)
.\scripts\build.ps1 -Run      # Start service in separate window first
cd test
go test -v ./api              # API tests
go test -v ./ui               # UI tests
```

**See:** `cmd/quaero-test-runner/README.md` for detailed documentation, configuration, and troubleshooting.

**IMPORTANT:**
- ❌ DO NOT run `build.ps1` before the test runner
- ❌ DO NOT manually start the service before the test runner
- ✅ Let the test runner control the service lifecycle

## Project Overview

**Quaero** (Latin: "I seek, I search") - A knowledge collection system with RAG capabilities.

### Key Features

- 🔐 **Automatic Authentication** - Chrome extension captures credentials
- 📊 **Real-time Updates** - WebSocket-based live log streaming
- 💾 **SQLite Storage** - Local database with full-text search
- 🌐 **Web Interface** - Browser-based UI for collection and browsing
- 🤖 **Local LLM** - Offline inference with llama.cpp
- 🔍 **Vector Search** - 768-dimension embeddings for semantic search
- ⚡ **Fast Collection** - Efficient scraping and storage
- ⏰ **Scheduled Jobs** - Automated crawling and document summarization

### Technology Stack

- **Language:** Go 1.25+
- **Storage:** SQLite with FTS5 (full-text search)
- **Web UI:** HTML templates, Alpine.js, Bulma CSS, WebSockets
- **LLM:** llama.cpp (offline mode), Mock mode (testing)
- **Authentication:** Chrome extension → HTTP service
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2

## Architecture Overview

### Layered Architecture

Quaero follows a clean architecture pattern with clear separation of concerns:

```
┌─────────────────────────────────────────┐
│  cmd/quaero/                            │  Entry point, CLI commands
│  └─ Uses: internal/app                 │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  internal/app/                          │  Dependency injection & orchestration
│  └─ Initializes: all services          │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  internal/server/                       │  HTTP server & routing
│  └─ Uses: handlers/                    │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  internal/handlers/                     │  HTTP/WebSocket handlers
│  └─ Uses: services/                    │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  internal/services/                     │  Business logic
│  └─ Uses: storage/, interfaces/        │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  internal/storage/sqlite/               │  Data persistence
│  └─ Uses: interfaces/                  │
└─────────────────────────────────────────┘
```

### Key Architectural Patterns

**Dependency Injection:**
- Constructor-based DI throughout
- All dependencies passed explicitly via constructors
- `internal/app/app.go` is the composition root
- No global state or service locators

**Event-Driven Architecture:**
- `EventService` implements pub/sub pattern
- Services subscribe to events during initialization
- Two main events:
  - `EventCollectionTriggered` - Triggers document collection/sync
  - `EventEmbeddingTriggered` - Triggers embedding generation
- Scheduler publishes events on cron schedule (every 5 minutes)

**Interface-Based Design:**
- All service dependencies use interfaces from `internal/interfaces/`
- Enables testing with mocks
- Allows swapping implementations

### Service Initialization Flow

The app initialization sequence in `internal/app/app.go` is critical:

1. **Storage Layer** - SQLite
2. **LLM Service** - Required for embeddings (offline/mock mode)
3. **Embedding Service** - Uses LLM service
4. **Document Service** - Uses embedding service
5. **Chat Service** - RAG-enabled chat with LLM
6. **Event Service** - Pub/sub for system events
7. **Auth Service** - Atlassian authentication
8. **Jira/Confluence Services** - Auto-subscribe to collection events
9. **Processing Service** - Document processing
10. **Embedding Coordinator** - Auto-subscribes to embedding events
11. **Scheduler Service** - Triggers events on cron (every 5 minutes)
12. **Handlers** - HTTP/WebSocket handlers

**Important:** Services that subscribe to events must be initialized after the EventService but before any events are published.

### Data Flow: Collection → Processing → Embedding

```
1. User clicks "Collect" in UI
   ↓
2. Handler triggers Jira/Confluence scraper
   ↓
3. Scraper stores raw data (jira_issues, confluence_pages)
   ↓
4. Scheduler publishes EventCollectionTriggered (every 5 minutes)
   ↓
5. Jira/Confluence services transform raw data → documents table
   ↓
6. Scheduler publishes EventEmbeddingTriggered
   ↓
7. EmbeddingCoordinator processes unembedded documents
   ↓
8. Documents ready for search/RAG
```

### LLM Service Architecture

The LLM service provides a unified interface for embeddings and chat:

**Modes:**
- **Offline** - Local llama.cpp inference (production default)
- **Mock** - Fake responses for testing (no models required)
- **Cloud** - Future: OpenAI/Anthropic APIs

**Current Implementation:**
- `internal/services/llm/offline/llama.go` - llama.cpp integration
- Uses `llama-server` subprocess with HTTP API
- Embedding model: nomic-embed-text-v1.5 (768 dimensions)
- Chat model: qwen2.5-7b-instruct-q4

**Configuration:**
```toml
[llm]
mode = "offline"  # or "mock"

[llm.offline]
llama_dir = "./llama.cpp"
model_dir = "./models"
embed_model = "nomic-embed-text-v1.5-q8.gguf"
chat_model = "qwen2.5-7b-instruct-q4.gguf"
context_size = 2048
thread_count = 4
gpu_layers = 0
mock_mode = false  # Set to true for testing
```

### Storage Schema

**Documents Table** (`documents`):
- Central unified storage for all source types
- Fields: id, source_id, source_type, title, content, embedding, embedding_model, last_synced, created_at, updated_at
- FTS5 index: documents_fts (title + content)
- Force sync flags: force_sync_pending, force_embed_pending

**Source Tables:**
- `jira_projects`, `jira_issues` - Raw Jira data
- `confluence_spaces`, `confluence_pages` - Raw Confluence data
- Scrapers populate these, then transform to documents

**Auth Table:**
- `auth_credentials` - Atlassian authentication tokens

### Chrome Extension & Authentication Flow

**Chrome Extension** (`cmd/quaero-chrome-extension/`):
- Captures authentication cookies and tokens from Atlassian sites (Jira/Confluence)
- Automatically deployed to `bin/` during build
- Uses Chrome Side Panel API for modern UI
- WebSocket connection for real-time server status

**Authentication Flow:**
1. User navigates to Jira/Confluence and logs in
2. User clicks Quaero extension icon
3. Extension captures cookies, cloudId, and atlToken
4. Extension sends auth data to `POST /api/auth`
5. AuthHandler (`internal/handlers/auth_handler.go`) receives data
6. AuthService (`internal/services/auth/service.go`) stores credentials
7. AuthService configures HTTP client with cookies
8. Crawler service can now access Jira/Confluence APIs

**Auth API Endpoints:**
- `POST /api/auth` - Capture authentication from Chrome extension
- `GET /api/auth/status` - Check if authenticated
- `GET /api/version` - Server version info
- `WS /ws` - WebSocket for real-time updates

**Key Files:**
- `cmd/quaero-chrome-extension/background.js` - Auth capture logic
- `cmd/quaero-chrome-extension/sidepanel.js` - Side panel UI with status
- `internal/handlers/auth_handler.go` - HTTP handler for auth endpoints
- `internal/services/auth/service.go` - Auth service with HTTP client config
- `internal/interfaces/atlassian.go` - Auth data types

**Configuration:**
- Default server URL: `http://localhost:8085`
- Configurable in extension settings
- Supports WebSocket (WS) and secure WebSocket (WSS)

## Go Structure Standards

### Directory Structure & Rules

**Critical Distinction:**

#### `internal/common/` - Stateless Utilities (NO Receiver Methods)
```go
// ✅ CORRECT: Stateless pure function
package common

func LoadFromFile(path string) (*Config, error) {
    // No receiver, no state
    return loadConfig(path)
}

func InitLogger(config *Config) arbor.ILogger {
    // Pure function, no state
    return arbor.NewLogger()
}
```

**❌ BLOCKED: Receiver methods in common/**
```go
// internal/common/config.go
func (c *Config) Load() error {  // ❌ ERROR - Move to services/
    return nil
}
```

#### `internal/services/` - Stateful Services (WITH Receiver Methods)
```go
// ✅ CORRECT: Service with receiver methods
package atlassian

type JiraScraperService struct {
    db     *sql.DB
    logger arbor.ILogger
}

func (s *JiraScraperService) ScrapeProjects(ctx context.Context) error {
    s.logger.Info().Msg("Scraping projects")
    return s.db.Query(...)
}
```

**⚠️ WARNING: Stateless function in services/**
```go
// internal/services/jira_service.go
func ScrapeProjects(db *sql.DB) error {  // Should use receiver
    return nil
}
```

### Startup Sequence (main.go)

**REQUIRED ORDER:**
1. Configuration loading (`common.LoadFromFile`)
2. Logger initialization (`common.InitLogger`)
3. Banner display (`common.PrintBanner`)
4. Version logging
5. Service initialization
6. Handler initialization
7. Server start

**Example:**
```go
// cmd/quaero/main.go
func main() {
    // 1. Load config
    config, err := common.LoadFromFile(configPath)

    // 2. Init logger
    logger := common.InitLogger(config)

    // 3. Display banner
    common.PrintBanner(config, logger)

    // 4. Initialize app
    app, err := app.New(config, logger)

    // 5. Start server
    server.Start(app)
}
```

### Quaero-Specific Requirements

**Collectors (ONLY These):**
1. **Jira** (`internal/services/atlassian/jira_*`)
2. **Confluence** (`internal/services/atlassian/confluence_*`)
3. **GitHub** (`internal/services/github/*`) - Future

**DO NOT create:**
- Generic document collectors
- File system crawlers
- Other data sources without explicit requirement

**Web UI (NOT CLI):**
- Server-side rendering with Go templates
- Alpine.js for client-side interactivity
- NO CLI commands for collection
- WebSocket for real-time updates

## Code Conventions

### Logging

**REQUIRED:** Use `github.com/ternarybob/arbor` for all logging

```go
logger.Info().Str("field", value).Msg("Message")
logger.Error().Err(err).Msg("Error occurred")
logger.Debug().Int("count", n).Msg("Debug info")
```

**Never:**
- `fmt.Println()` in production code
- `log.Printf()` from standard library
- Unstructured logging

**❌ BLOCKED Examples:**
```go
fmt.Println("Starting service")     // ❌ Use logger.Info()
log.Printf("Error: %v", err)        // ❌ Use logger.Error().Err(err)
```

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process document: %w", err)
}

// Log and return errors in handlers
if err != nil {
    logger.Error().Err(err).Msg("Failed to save document")
    http.Error(w, "Internal server error", http.StatusInternalServerError)
    return
}
```

**❌ NEVER ignore errors:**
```go
_ = someFunction()  // ❌ BLOCKED - All errors must be handled
```

**✅ CORRECT:**
```go
if err := someFunction(); err != nil {
    logger.Warn().Err(err).Msg("Non-critical error")
    // Or handle appropriately
}
```

### Configuration

**Use:** `github.com/pelletier/go-toml/v2` for TOML config

**Priority order:**
1. CLI flags (highest)
2. Environment variables
3. Config file (quaero.toml)
4. Defaults (lowest)

Configuration loading happens in `internal/common/config.go`

### Required Libraries

**REQUIRED (do not replace):**
- `github.com/ternarybob/arbor` - Structured logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML config parsing

**Core dependencies:**
- `github.com/spf13/cobra` - CLI framework
- `github.com/gorilla/websocket` - WebSocket support
- `modernc.org/sqlite` - Pure Go SQLite driver
- `github.com/robfig/cron/v3` - Cron scheduler
- `github.com/chromedp/chromedp` - UI testing

## Frontend Architecture

**Framework:** Vanilla JavaScript with Alpine.js and Bulma CSS

**Important:** The project has migrated from HTMX to Alpine.js and from BeerCSS to Bulma CSS framework.

**Structure:**
```
pages/
├── *.html              # Page templates
├── partials/           # Reusable components
│   ├── navbar.html
│   ├── footer.html
│   └── service-*.html
└── static/
    ├── quaero.css      # Global styles (Bulma customization)
    └── common.js       # Common JavaScript utilities
```

**Alpine.js Usage:**
- Use Alpine.js for interactive UI components
- Data binding and reactivity via Alpine directives

**Bulma CSS:**
- Use Bulma CSS classes for styling
- Component-based styling approach
- Responsive design patterns

**WebSocket Integration:**
- Real-time log streaming via `/ws`
- Status updates broadcast to all connected clients
- Used for live collection progress

**Server-Side Rendering:**
- Go's `html/template` package for all page rendering
- Templates in `pages/*.html`
- Template composition with `{{template "name" .}}`
- Server renders complete HTML pages

**NO:**
- Client-side routing
- SPA frameworks (React, Vue, etc.)
- HTMX (removed from architecture)

## Code Quality Rules

### File & Function Limits

- **Max file size:** 500 lines
- **Max function size:** 80 lines (ideal: 20-40)
- **Single Responsibility:** One purpose per function
- **Descriptive naming:** Intention-revealing names

### Design Principles

- **DRY:** Don't Repeat Yourself - consolidate duplicate code
- **Dependency Injection:** Constructor-based DI only
- **Interface-Based Design:** All service dependencies use interfaces
- **No Global State:** No service locators or global variables
- **Table-Driven Tests:** Use test tables for multiple test cases

### Forbidden Patterns

**❌ BLOCKED:**
```go
// TODO comments without immediate action
// TODO: fix this later

// FIXME comments
// FIXME: this is broken

// Ignored errors
_ = service.DoSomething()

// fmt/log instead of arbor logger
fmt.Println("message")
log.Printf("message")

// Receiver methods in internal/common/
func (c *Config) Load() error { }

// Wrong startup sequence
logger := common.InitLogger()  // Before config load
config := common.LoadConfig()
```

## Testing Guidelines

### Test Organization

```
test/
├── unit/              # Fast unit tests with mocks
├── api/               # API integration tests (database interactions)
└── ui/                # Browser automation tests (ChromeDP)
```

### Writing Tests

**Unit Tests:**
```go
// Colocate with implementation
internal/services/chat/
├── chat_service.go
└── chat_service_test.go
```

**API Tests:**
```go
package api

func TestAPIEndpoint(t *testing.T) {
    // Test HTTP endpoints with actual database
    // Verify request/response handling
}
```

**UI Tests:**
```go
package ui

func TestUIWorkflow(t *testing.T) {
    config, _ := LoadTestConfig()

    // Use ChromeDP for browser automation
    // Use takeScreenshot() helper for visual verification
    // Results saved to test/results/{type}-{timestamp}/
}
```

### Test Runner Features

The Go-native test infrastructure (`test/run_tests.go` and `test/main_test.go`):
- **TestMain fixture** handles server lifecycle automatically
- Starts test server on port 18085 (separate from dev server)
- Waits for server readiness before running tests
- Manages timestamped test result directories
- Captures screenshots in UI tests (saved to results/)
- Provides coverage reports with `-coverprofile`
- Automatic cleanup on test completion or failure

## Common Development Tasks

### Adding a New Data Source

1. Create storage interface in `internal/interfaces/`
2. Implement SQLite storage in `internal/storage/sqlite/`
3. Create scraper service in `internal/services/`
4. Subscribe to `EventCollectionTriggered` in service constructor
5. Initialize in `internal/app/app.go` (after EventService)
6. Add handler in `internal/handlers/`
7. Register routes in `internal/server/routes.go`
8. Add UI page in `pages/`

### Adding a New API Endpoint

1. Add handler method in appropriate handler file
2. Register route in `internal/server/routes.go`
3. Test with API integration test in `test/api/`
4. Document in README.md API section

### Modifying LLM Behavior

**Important:** LLM service is abstracted via `internal/interfaces/llm_service.go`

To change embedding/chat behavior:
1. Modify implementation in `internal/services/llm/offline/`
2. Ensure interface compliance
3. Update tests in `test/unit/`
4. Consider mock mode for testing

## Important Implementation Notes

### WebSocket Log Streaming

The WebSocket handler (`internal/handlers/websocket.go`) maintains:
- Connected clients registry
- Status broadcaster goroutine
- Log streamer goroutine

Services call `WSHandler.StreamLog()` to send real-time updates to UI.

### Event-Driven Processing

The scheduler service runs every 5 minutes and publishes:
1. `EventCollectionTriggered` - Transforms scraped data to documents
2. `EventEmbeddingTriggered` - Generates embeddings for new documents

**Note:** Scraping (downloading from APIs) is user-triggered via UI, not automatic.

### Document Processing Workflow

Documents go through stages:
1. **Raw** - Stored in source tables (jira_issues, confluence_pages)
2. **Document** - Transformed to documents table
3. **Embedded** - Vector embedding generated
4. **Searchable** - Available for RAG queries

Use `force_sync_pending` and `force_embed_pending` flags to manually trigger processing.

### RAG Implementation

Chat service (`internal/services/chat/chat_service.go`) implements RAG:
1. User sends message
2. Generate query embedding
3. Search documents by vector similarity
4. Inject top-k documents into prompt context
5. Generate response with LLM
6. Return response with document citations

**Configuration:**
```go
RAGConfig{
    Enabled:       true,
    MaxDocuments:  5,
    MinSimilarity: 0.7,  // 0-1 range
    SearchMode:    "vector",
}
```

## Security & Data Privacy

**Critical:** Quaero is designed for local-only operation:
- All data stored locally in SQLite
- LLM inference runs locally (offline mode)
- No external API calls in offline mode
- Audit logging for compliance

**Offline Mode Guarantees:**
- Data never leaves the machine
- Network isolation verifiable
- Suitable for government/healthcare/confidential data

**Future Cloud Mode:**
- Explicit warnings required
- Risk acknowledgment in config
- API call audit logging
- NOT for sensitive data

## Version Management

Version tracked in `.version` file:
```
version: 0.1.0
build: 10-04-16-30-15
```

Updated automatically by build scripts.

## Troubleshooting

### Server Won't Start

Check:
1. Port availability: `netstat -an | findstr :8085`
2. Config file exists and is valid
3. Database path is writable
4. Logs in console output

### UI Tests Fail

Check:
1. Server started correctly (automatic via TestMain fixture)
2. Test server port 18085 is available (not in use)
3. ChromeDP/Chrome browser installed
4. Test results in `test/results/run-{datetime}/` for screenshots
5. Run with `-v` flag for verbose output

### Embeddings Not Generated

Check:
1. LLM service mode (offline/mock)
2. Model files exist if offline mode
3. Scheduler is running (logs every 5 minutes)
4. Documents have `force_embed_pending=true` flag
5. Embedding coordinator started successfully

### llama-server Issues

Check:
1. `llama-server` binary exists in configured llama_dir
2. Model files exist and are valid GGUF format
3. Sufficient RAM available (8-16GB)
4. Check logs for subprocess errors
5. Try mock_mode=true for testing without models

## API Endpoints Reference

### Core Endpoints

**Authentication:**
- `POST /api/auth` - Capture authentication from Chrome extension
- `GET /api/auth/status` - Check if authenticated

**Collection (UI-triggered):**
- `POST /api/scrape` - Trigger collection
- `POST /api/scrape/projects` - Scrape Jira projects
- `POST /api/scrape/spaces` - Scrape Confluence spaces

**Documents:**
- `GET /api/documents/stats` - Document statistics
- `GET /api/documents` - List documents
- `POST /api/documents/process` - Process documents
- `POST /api/documents/force-sync` - Force sync document
- `POST /api/documents/force-embed` - Force embed document

**Scheduler:**
- `POST /api/scheduler/trigger-collection` - Trigger collection event
- `POST /api/scheduler/trigger-embedding` - Trigger embedding event

**System:**
- `GET /api/version` - API version
- `GET /api/health` - Health check
- `WS /ws` - WebSocket for real-time updates

See README.md for complete API documentation.

## Task Master AI Instructions

**Import Task Master's development workflow commands and guidelines, treat as if import is in the main AGENTS.md file.**
@./.taskmaster/CLAUDE.md
