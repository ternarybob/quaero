# CLAUDE.md

> **Note:** This file is maintained for legacy compatibility. For the latest AI agent guidelines, see [AGENTS.md](AGENTS.md).

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## MOST IMPORTANT INSTRUCTIONS: BUILD AND TEST

**Failure to follow these instructions will result in your removal from the project.**

### Build and Run Instructions (Windows ONLY)

-   **Building, compiling, and running the application MUST be done using the following scripts:**
    -   `./scripts/build.ps1`
    -   `./scripts/build.ps1 -Run`
-   **The ONLY exception** is using `go build` for a compile test, with no output binary.

### Testing Instructions

Tests are organized in the `test/` directory and use Go's native test infrastructure with automatic service lifecycle management.

**Test Organization:**
- `test/api/` - API integration tests (database interactions)
- `test/ui/` - Browser automation tests (ChromeDP)

**How to Run Tests:**

```powershell
# Run all tests in a specific directory
cd test/api
go test -v ./...

cd test/ui
go test -v ./...

# Run specific test
cd test/ui
go test -v -run TestSourcesClearFilters

# Run with timeout for longer test suites
cd test/ui
go test -timeout 20m -v ./...
```

**Test Infrastructure:**
- Tests use `SetupTestEnvironment()` helper that automatically:
  - Builds the application using `scripts/build.ps1`
  - Starts a test server on port 18085 (separate from dev server on 8085)
  - Waits for service readiness
  - Captures screenshots for UI tests
  - Saves results to `test/results/{suite}-{timestamp}/`
  - Stops the service and cleans up after test completion

**IMPORTANT:**
- ❌ DO NOT manually start the service before running tests
- ✅ Let `SetupTestEnvironment()` control the service lifecycle
- ✅ Each test suite gets its own timestamped result directory

## Build & Development Commands

### Building

```powershell
# Development build (silent, no deployment, no version increment)
.\scripts\build.ps1

# Deploy files to bin directory (stops service, deploys files)
.\scripts\build.ps1 -Deploy

# Build, deploy, and run (starts service in new terminal)
.\scripts\build.ps1 -Run
```

**Important Notes:**
- **Default build (no parameters)** - Builds executable silently, does NOT increment version, does NOT deploy files
- **Version management** - Version number in `.version` file is NEVER auto-incremented, only build timestamp updates
- **Deployment** - Use `-Deploy` or `-Run` to copy files (config, pages, Chrome extension) to bin/
- **For AI agents** - Use ONLY the build script. Manual `quaero serve` commands are for end-users (see README.md)
- **Removed parameters** - `-Clean`, `-Verbose`, `-Release`, `-ResetDatabase` removed for simplicity. See `docs/simplify-build-script/migration-guide.md` for alternatives

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
│  └─ Uses: storage/, interfaces/        │  ← MCP server uses search service
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  internal/storage/sqlite/               │  Data persistence
│  └─ Uses: interfaces/                  │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│  cmd/quaero-mcp/                        │  MCP Server (stdio/JSON-RPC)
│  └─ Uses: internal/services/search     │  Thin wrapper, read-only
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
7. **Auth Service** - Generic web authentication
8. **Crawler Service** - ChromeDP-based web crawler
9. **Processing Service** - Document processing
10. **Embedding Coordinator** - Auto-subscribes to embedding events
11. **Scheduler Service** - Triggers events on cron (every 5 minutes)
12. **Handlers** - HTTP/WebSocket handlers

**Important:** Services that subscribe to events must be initialized after the EventService but before any events are published.

### Data Flow: Crawling → Processing → Embedding

```
1. User triggers crawler job via UI or scheduled job
   ↓
2. Crawler job executes with seed URLs and patterns
   ↓
3. Crawler stores documents in documents table (markdown format)
   ↓
4. Scheduler publishes EventEmbeddingTriggered (every 5 minutes)
   ↓
5. EmbeddingCoordinator processes unembedded documents
   ↓
6. Documents ready for search/RAG
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

**Auth Table:**
- `auth_credentials` - Generic web authentication tokens and cookies

### Chrome Extension & Authentication Flow

**Chrome Extension** (`cmd/quaero-chrome-extension/`):
- Captures authentication cookies and tokens from authenticated websites
- Generic auth capability - works with any site (not limited to specific platforms)
- Examples: Jira, Confluence, GitHub, or any authenticated web service
- Automatically deployed to `bin/` during build
- Uses Chrome Side Panel API for modern UI
- WebSocket connection for real-time server status

**Authentication Flow:**
1. User navigates to an authenticated website (e.g., Jira, Confluence, GitHub)
2. User clicks Quaero extension icon
3. Extension captures cookies and authentication tokens from the active site
4. Extension sends auth data to `POST /api/auth`
5. AuthHandler (`internal/handlers/auth_handler.go`) receives data
6. AuthService (`internal/services/auth/service.go`) stores credentials
7. AuthService configures HTTP client with cookies
8. Crawler service can now access authenticated content on that site

**Auth API Endpoints:**
- `POST /api/auth` - Capture authentication from Chrome extension
- `GET /api/auth/status` - Check if authenticated
- `GET /api/version` - Server version info
- `WS /ws` - WebSocket for real-time updates

**Key Files:**
- `cmd/quaero-chrome-extension/background.js` - Generic auth capture logic
- `cmd/quaero-chrome-extension/sidepanel.js` - Side panel UI with status
- `internal/handlers/auth_handler.go` - HTTP handler for auth endpoints
- `internal/services/auth/service.go` - Auth service with HTTP client config
- `internal/interfaces/auth.go` - Auth data types (generic, not platform-specific)

**Configuration:**
- Default server URL: `http://localhost:8085`
- Configurable in extension settings
- Supports WebSocket (WS) and secure WebSocket (WSS)

### MCP Server Architecture

**MCP Server** (`cmd/quaero-mcp/`):
- Exposes Quaero's search functionality to AI assistants via Model Context Protocol (MCP)
- Thin wrapper around existing `internal/services/search` package
- Uses stdio/JSON-RPC transport for local-only communication
- Integrates with Claude Desktop and other MCP-compatible clients
- Built automatically with main application via `scripts/build.ps1`

**Architecture Pattern:**
- **Minimal wrapper (< 200 lines main.go)** - No business logic duplication
- **Interface-based** - Uses existing SearchService interface
- **Read-only** - MCP tools only query data, never modify
- **Local-only** - No network exposure, all data stays on machine

**MCP Tools (4 Total):**

1. **search_documents** - Full-text search using SQLite FTS5
   - Parameters: `query` (FTS5 syntax), `limit`, `source_types`
   - Maps to: `SearchService.Search(query, opts)`
   - Use case: Finding documents by keywords

2. **get_document** - Retrieve single document by ID
   - Parameters: `document_id` (format: doc_{uuid})
   - Maps to: `SearchService.GetByID(id)`
   - Use case: Getting complete document content

3. **list_recent_documents** - List recently updated documents
   - Parameters: `limit`, `source_type` (optional filter)
   - Maps to: `SearchService.Search("", opts)` with ORDER BY updated_at DESC
   - Use case: Seeing recent activity

4. **get_related_documents** - Find documents by cross-reference
   - Parameters: `reference` (e.g., BUG-123, PROJ-456)
   - Maps to: `SearchService.SearchByReference(ref, opts)`
   - Use case: Tracking issue relationships

**Response Format:**
- All tools return **Markdown** formatted results
- Includes metadata (source type, URL, timestamps)
- Content previews or full content depending on tool
- Suitable for direct display in AI assistant interfaces

**File Organization:**
```
cmd/quaero-mcp/
├── main.go           # MCP server initialization (~70 lines)
├── handlers.go       # Tool handler implementations (~163 lines)
├── formatters.go     # Markdown response formatters (~127 lines)
└── tools.go          # MCP tool definitions (~58 lines)
```

**Integration with Search Service:**
```go
// MCP server uses existing search service
searchService := search.NewService(storage, logger)

// Each tool handler calls appropriate method
func handleSearchDocuments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    docs, err := searchService.Search(ctx, query, opts)
    return formatSearchResults(docs), nil
}
```

**Logging for MCP:**
- **CRITICAL:** MCP uses stdio for JSON-RPC protocol
- **NEVER** log to stdout (reserved for protocol)
- **Use WARN level** by default (minimal stderr output)
- Logging configuration in main.go:
  ```go
  logger := arbor.NewLogger().WithConsoleWriter(arbor_models.WriterConfiguration{
      Type:             arbor_models.LogWriterTypeConsole,
      TimeFormat:       "15:04:05",
      TextOutput:       true,
      DisableTimestamp: false,
  }).WithLevelFromString("warn") // Minimal logging to avoid interfering with stdio
  ```

**Configuration:**
- Uses standard `quaero.toml` config file
- Environment variable: `QUAERO_CONFIG` points to config path
- Same database as main Quaero application
- No separate configuration needed

**Testing:**
- API tests in `test/api/mcp_server_test.go`
- Tests verify handler logic and search integration
- No stdio tests (complexity vs. value trade-off)
- Manual testing via Claude Desktop recommended

**Build Process:**
```powershell
# MCP server built automatically with main application
.\scripts\build.ps1

# Creates both binaries:
# - bin/quaero.exe (main application)
# - bin/quaero-mcp.exe (MCP server)
```

**Claude Desktop Integration:**
Add to `%APPDATA%\Claude\claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "quaero": {
      "command": "C:\\development\\quaero\\bin\\quaero-mcp.exe",
      "args": [],
      "env": {
        "QUAERO_CONFIG": "C:\\development\\quaero\\bin\\quaero.toml"
      }
    }
  }
}
```

**Security Model:**
- **Local-only execution** - No network communication
- **Read-only access** - MCP tools never modify data
- **Same security as main app** - Uses same database and permissions
- **No cloud API calls** - All data processing local

**Documentation:**
- Setup guide: `docs/implement-mcp-server/mcp-configuration.md`
- Usage examples: `docs/implement-mcp-server/usage-examples.md`
- Implementation plan: `docs/implement-mcp-server/plan.md`

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

**Data Collection:**
- **Generic Crawler** - ChromeDP-based web crawler for all data sources
- Configured via crawler job definitions in `job-definitions/` directory
- Supports URL patterns, extractors, and authentication
- Examples available for Jira, Confluence, GitHub patterns
- **DO NOT** create source-specific API integrations
- **DO NOT** create direct database scrapers for specific platforms

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
- Component definitions in `pages/static/alpine-components.js`
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

**Use the Generic Crawler Approach:**

1. **Create a Crawler Job Definition** in `job-definitions/` directory:
   - Define seed URLs (starting points for crawling)
   - Specify URL patterns to match (regex or glob patterns)
   - Configure crawl depth and concurrency
   - Set authentication requirements (if needed)

2. **Add URL Pattern Extractors** (optional):
   - Create extractor in `internal/services/identifiers/` for page-specific identifier extraction
   - Create extractor in `internal/services/metadata/` for page-specific metadata extraction
   - Follow existing patterns for Jira/Confluence as examples

3. **Configure Authentication** (if required):
   - Use Chrome extension to capture authentication cookies
   - Extension works generically with any authenticated site
   - No code changes required for new authentication sources

4. **Test the Crawler Job**:
   - Trigger job via UI or API
   - Monitor job progress via WebSocket events
   - Verify documents are stored in documents table
   - Check that metadata extraction works correctly

**DO NOT:**
- Create source-specific API integration code
- Add new scraper services in `internal/services/`
- Create direct database access for specific platforms
- Build custom HTTP clients for specific APIs

**The crawler is intentionally generic** - it works with any website, authenticated or not. Configure behavior through job definitions, not code.

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

### Working with MCP Server

**DO:**
- Keep main.go under 200 lines (thin wrapper pattern)
- Use existing SearchService interface (no duplication)
- Return Markdown-formatted responses
- Log at WARN level (avoid interfering with stdio)
- Test handlers via API tests in `test/api/`
- Document new tools in usage-examples.md

**DON'T:**
- Add business logic to MCP handlers (use services/)
- Log to stdout (breaks JSON-RPC protocol)
- Duplicate search functionality from SearchService
- Add write/modify operations (MCP is read-only)
- Skip testing (API tests verify integration)

**Adding a New MCP Tool:**
1. Define tool in `cmd/quaero-mcp/tools.go` (MCP schema)
2. Add handler in `cmd/quaero-mcp/handlers.go` (calls SearchService)
3. Add formatter in `cmd/quaero-mcp/formatters.go` (Markdown output)
4. Register tool in `cmd/quaero-mcp/main.go` (server.AddTool)
5. Add API test in `test/api/mcp_server_test.go`
6. Document in `docs/implement-mcp-server/usage-examples.md`

**MCP Server Constraints:**
- Must use existing SearchService methods (no new queries)
- Read-only operations only (no data modification)
- Minimal logging (WARN level to stderr)
- Markdown response format (for AI assistants)
- File size: main.go < 200 lines, total < 500 lines

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
1. **Crawled** - Fetched by crawler and converted to markdown
2. **Stored** - Saved directly to documents table with metadata
3. **Embedded** - Vector embedding generated
4. **Searchable** - Available for RAG queries

Use `force_embed_pending` flag to manually trigger embedding generation.

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

## Task Master AI Instructions
**Import Task Master's development workflow commands and guidelines, treat as if import is in the main CLAUDE.md file.**
@./.taskmaster/CLAUDE.md
