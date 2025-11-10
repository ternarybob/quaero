# Project Context

## Purpose
Quaero is a local-first knowledge management and search system with RAG (Retrieval-Augmented Generation) capabilities. It provides:
- Generic web crawling with ChromeDP-based crawler for any authenticated or public website
- Local LLM inference for embeddings and chat (offline mode)
- Vector search with semantic similarity
- MCP (Model Context Protocol) server integration for AI assistant access
- Real-time WebSocket updates for UI
- Privacy-focused design: all data and processing stays local

## Tech Stack

### Backend
- **Language**: Go 1.21+
- **Web Framework**: `net/http` with `gorilla/websocket` for WebSockets
- **CLI Framework**: `spf13/cobra`
- **Database**: SQLite with `modernc.org/sqlite` (pure Go driver)
- **LLM Integration**: llama.cpp via HTTP API (subprocess management)
- **Browser Automation**: `chromedp/chromedp` for crawler and testing
- **Logging**: `github.com/ternarybob/arbor` (structured logging)
- **Configuration**: TOML via `pelletier/go-toml/v2`
- **Scheduler**: `robfig/cron/v3`

### Frontend
- **Framework**: Vanilla JavaScript with Alpine.js
- **CSS Framework**: Bulma CSS (migrated from BeerCSS)
- **Templating**: Go's `html/template` (server-side rendering)
- **Real-time Updates**: WebSocket connections for log streaming

### AI/ML Stack
- **Embedding Model**: nomic-embed-text-v1.5 (768 dimensions)
- **Chat Model**: qwen2.5-7b-instruct-q4
- **Inference Engine**: llama.cpp (local, offline)
- **Mock Mode**: Available for testing without models

### MCP Integration
- **Protocol**: Model Context Protocol (stdio/JSON-RPC)
- **Transport**: Local stdio (no network)
- **Client Compatibility**: Claude Desktop, other MCP-compatible AI assistants

## Project Conventions

### Code Style

**Logging (CRITICAL)**:
- ALWAYS use `github.com/ternarybob/arbor` for structured logging
- NEVER use `fmt.Println()` or `log.Printf()` in production code
- Format: `logger.Info().Str("field", value).Msg("Message")`
- MCP server: Use WARN level to avoid interfering with stdio

**Error Handling**:
- Always wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- NEVER ignore errors with `_`
- Log errors before returning from handlers

**Naming Conventions**:
- Services: `{Domain}Service` (e.g., `ChatService`, `CrawlerService`)
- Handlers: `{Feature}Handler` (e.g., `AuthHandler`, `SearchHandler`)
- Interfaces: Defined in `internal/interfaces/` with descriptive names
- Constructor pattern: `New()` or `NewService()` returns initialized service

**File Organization**:
- Max file size: 500 lines
- Max function size: 80 lines (ideal: 20-40)
- Colocate tests: `service.go` and `service_test.go` together

### Architecture Patterns

**Layered Architecture**:
```
cmd/quaero → internal/app → internal/server → internal/handlers → internal/services → internal/storage
```

**Dependency Injection**:
- Constructor-based DI throughout
- All dependencies passed explicitly via constructors
- `internal/app/app.go` is the composition root
- No global state or service locators

**Interface-Based Design**:
- All service dependencies use interfaces from `internal/interfaces/`
- Enables testing with mocks
- Allows swapping implementations

**Event-Driven Architecture**:
- `EventService` implements pub/sub pattern
- Services subscribe to events during initialization
- Main events: `EventCollectionTriggered`, `EventEmbeddingTriggered`
- Scheduler publishes events on cron schedule (every 5 minutes)

**Stateless vs. Stateful Components**:
- `internal/common/` - Stateless utilities (NO receiver methods)
- `internal/services/` - Stateful services (WITH receiver methods)
- Clear separation enforced

### Testing Strategy

**Test Organization**:
```
test/
├── unit/              # Fast unit tests with mocks
├── api/               # API integration tests (database)
└── ui/                # Browser automation tests (ChromeDP)
```

**Test Infrastructure**:
- Use `SetupTestEnvironment()` helper for automatic service lifecycle
- Test server runs on port 18085 (separate from dev server on 8085)
- DO NOT manually start service before tests
- Screenshots and results saved to `test/results/{suite}-{timestamp}/`
- Use table-driven tests for multiple test cases

**Running Tests**:
```powershell
# API tests
cd test/api
go test -v ./...

# UI tests
cd test/ui
go test -v ./...

# Specific test
go test -v -run TestSourcesClearFilters
```

**Coverage Requirements**:
- Use `-coverprofile` for coverage reports
- Focus on business logic coverage
- Mock external dependencies

### Git Workflow

**Branch Strategy**:
- Main branch: `main`
- Feature branches: `feature/{description}`
- Bug fixes: `fix/{description}`

**Commit Conventions**:
- Use conventional commit format
- Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`
- Include co-author footer for AI-assisted commits

**OpenSpec Integration**:
- Use `/openspec:proposal` for planning new features
- Use `/openspec:apply` for implementing approved changes
- Specs stored in `openspec/changes/`

## Domain Context

**Knowledge Management**:
- Document-centric architecture (unified documents table)
- Source types: Generic web content (via crawler)
- Metadata extraction: Configurable per source type
- Cross-reference tracking: Links between documents

**RAG Pipeline**:
1. Crawl web content (ChromeDP)
2. Convert to markdown and store
3. Generate embeddings (vector representations)
4. Search by semantic similarity
5. Inject context into LLM prompts
6. Generate responses with citations

**Authentication Model**:
- Generic web authentication via Chrome extension
- Captures cookies and tokens from any authenticated site
- Examples: Jira, Confluence, GitHub, or any web service
- Stored in `auth_credentials` table

**MCP Server Model**:
- Read-only access to search functionality
- Four tools: search, get, list recent, get related
- Markdown-formatted responses for AI assistants
- Local-only communication (stdio, no network)

## Important Constraints

**Privacy & Security**:
- All data processing MUST stay local (offline mode)
- Network isolation verifiable for government/healthcare use
- No external API calls in offline mode
- Audit logging for compliance

**Windows-Only Development**:
- Build scripts use PowerShell (`.ps1`)
- Paths use Windows conventions
- Testing on Windows platform

**LLM Model Requirements**:
- Embedding model: nomic-embed-text-v1.5 (768 dimensions)
- Chat model: qwen2.5-7b-instruct-q4
- Requires 8-16GB RAM for local inference
- Mock mode available for testing without models

**MCP Server Constraints**:
- main.go must stay under 200 lines (thin wrapper)
- Read-only operations only
- NEVER log to stdout (breaks JSON-RPC)
- Use existing SearchService (no duplication)

**Build Requirements**:
- MUST use `./scripts/build.ps1` for building
- ONLY exception: `go build` for compile tests (no binary output)
- Version tracked in `.version` file
- Auto-deployment to `bin/` with `-Deploy` or `-Run` flags

## External Dependencies

**Required Libraries** (do not replace):
- `github.com/ternarybob/arbor` - Structured logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML config parsing

**Core Dependencies**:
- `github.com/spf13/cobra` - CLI framework
- `github.com/gorilla/websocket` - WebSocket support
- `modernc.org/sqlite` - Pure Go SQLite driver
- `github.com/robfig/cron/v3` - Cron scheduler
- `github.com/chromedp/chromedp` - Browser automation

**LLM Dependencies**:
- llama.cpp - Local inference engine (external subprocess)
- Model files: GGUF format models in `./models/` directory

**MCP Protocol**:
- Model Context Protocol specification
- stdio/JSON-RPC transport
- Claude Desktop integration

**Chrome Extension**:
- Chrome Side Panel API
- Generic auth capture (works with any site)
- WebSocket client for server status
