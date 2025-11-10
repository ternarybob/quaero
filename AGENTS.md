<!-- OPENSPEC:START -->
# OpenSpec Instructions

These instructions are for AI assistants working in this project.

Always open `@/openspec/AGENTS.md` when the request:
- Mentions planning or proposals (words like proposal, spec, change, plan)
- Introduces new capabilities, breaking changes, architecture shifts, or big performance/security work
- Sounds ambiguous and you need the authoritative spec before coding

Use `@/openspec/AGENTS.md` to learn:
- How to create and apply change proposals
- Spec format and conventions
- Project structure and guidelines

Keep this managed block so 'openspec update' can refresh the instructions.

<!-- OPENSPEC:END -->

# AGENTS.md

This file provides guidance to AI agents (Claude Code, GitHub Copilot, etc.) when working with code in this repository.

## CRITICAL: BUILD AND TEST

**Failure to follow these instructions will result in your removal from the project.**

### Build Instructions (Windows ONLY)

**Building, compiling, and running the application MUST be done using:**
- `.\scripts\build.ps1`
- `.\scripts\build.ps1 -Deploy`
- `.\scripts\build.ps1 -Run`
- **ONLY exception:** `go build` for compile tests (no output binary)

**Build Commands:**

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
- **Removed parameters** - `-Clean`, `-Verbose`, `-Release`, `-ResetDatabase` removed for simplicity. See `docs/simplify-build-script/migration-guide.md` for alternatives

### Testing Instructions

**CRITICAL: The test runner handles EVERYTHING automatically - do NOT run build scripts or start the service manually!**

**IMPORTANT: Do NOT create temporary files for testing or building (e.g., run_test.ps1, test_compile.go, etc.). Always use the official build and test commands:**
- **Build**: `.\scripts\build.ps1`
- **Test**: `go test` commands directly

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

### Queue-Based Job Processing

Quaero uses a queue-based architecture for distributed job processing with goqite (SQLite-backed message queue):

**Core Components:**

1. **QueueManager** (`internal/queue/manager.go`)
   - Manages goqite-backed job queue
   - Lifecycle management (Start/Stop/Restart)
   - Message operations: Enqueue, EnqueueWithDelay, Receive, Delete, Extend
   - Queue statistics: GetQueueLength, GetQueueStats
   - Visibility timeout for worker fault tolerance

2. **WorkerPool** (`internal/queue/worker.go`)
   - Pool of worker goroutines processing queue messages
   - Configurable concurrency level
   - Registered handlers for different job types
   - Automatic retry with max_receive limit
   - Graceful shutdown support

3. **JobMessage** (`internal/queue/types.go`)
   - Message types: "parent", "crawler_url", "summarizer", "cleanup"
   - Contains job configuration, metadata, and parent/child relationships
   - Serializable to JSON for queue storage
   - Supports depth tracking for crawler jobs

4. **Job Types** (`internal/jobs/types/`)
   - **CrawlerJob** (`crawler.go`) - Fetches URLs, extracts content, spawns child jobs
   - **SummarizerJob** (`summarizer.go`) - Generates summaries, extracts keywords
   - **CleanupJob** (`cleanup.go`) - Cleans up old jobs and logs
   - **BaseJob** (`base.go`) - Shared functionality (logging, status updates, child job enqueueing)

**Job Execution Flow:**

```
1. User triggers job via UI or JobDefinition
   ↓
2. Parent job message created and enqueued to goqite queue
   ↓
3. WorkerPool receives message from queue
   ↓
4. Worker routes message to appropriate handler (CrawlerJob, SummarizerJob, etc.)
   ↓
5. Handler executes job logic:
   - CrawlerJob: Fetch URL, extract content, discover links
   - SummarizerJob: Generate summary using LLM
   - CleanupJob: Delete old jobs/logs
   ↓
6. Job spawns child jobs if needed (URL discovery creates crawler_url messages)
   ↓
7. Progress tracked in crawl_jobs table
   ↓
8. Logs stored in job_logs table (unlimited history)
   ↓
9. Worker deletes message from queue on completion/failure
```

**Key Features:**

- **Persistent Queue:** goqite uses SQLite for durable message storage
- **Worker Pool:** Configurable concurrency with polling-based processing
- **Job Spawning:** Parent jobs can spawn child jobs (URL discovery)
- **Progress Tracking:** Real-time progress updates via crawl_jobs table
- **Unlimited Logs:** job_logs table with CASCADE DELETE for automatic cleanup
- **Fault Tolerance:** Visibility timeout prevents message loss on worker crash
- **Depth Limiting:** Crawler jobs respect max_depth configuration

**Configuration:**

```toml
[queue]
queue_name = "quaero-jobs"
concurrency = 4
poll_interval = "1s"
visibility_timeout = "5m"
max_receive = 3
```

### Job Definitions vs Queue Jobs

**Important Distinction:**

- **JobExecutor** (`internal/services/jobs/executor.go`):
  - Orchestrates multi-step workflows defined by users (JobDefinitions)
  - Executes steps sequentially with retry logic and error handling
  - Polls crawl jobs asynchronously when wait_for_completion is enabled
  - Publishes progress events for UI updates
  - Supports error strategies: fail, continue, retry

- **Queue Jobs** (`internal/jobs/types/`):
  - Handle individual task execution (CrawlerJob, SummarizerJob, CleanupJob)
  - Process URLs, generate summaries, clean up old jobs
  - Provide persistent queue with worker pool
  - Enable job spawning and depth tracking

**Both systems coexist and complement each other:**
- JobDefinitions can trigger crawl jobs via the "crawl" action
- JobExecutor polls those crawl jobs until completion
- Crawl jobs are executed by the queue-based CrawlerJob type
- JobExecutor is NOT replaced by the queue system - it serves a different purpose

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

**Binary Search Order:**

The service searches for `llama-server` in this order:
1. `{llamaDir}/llama-server` (or `.exe` on Windows) - llamaDir defaults to `./llama`
2. `./bin/llama-server` (or `.exe`)
3. `./llama-server` (or `.exe`)
4. `llama-server` in system PATH

You can override the llama directory via:
- Configuration file: `config.Server.LlamaDir`
- Environment variable: `QUAERO_SERVER_LLAMA_DIR`

**Example configuration:**
```toml
[llm]
mode = "offline"  # or "mock"

[server]
llama_dir = "./llama"  # Default: searches ./llama, ./bin, and PATH
# Or override with environment variable
# QUAERO_SERVER_LLAMA_DIR="C:/llama.cpp"  # Windows example
# QUAERO_SERVER_LLAMA_DIR="/usr/local/llama"  # Unix example

[llm.offline]
model_dir = "./models"
embed_model = "nomic-embed-text-v1.5-q8.gguf"
chat_model = "qwen2.5-7b-instruct-q4.gguf"
context_size = 2048
thread_count = 4
gpu_layers = 0
mock_mode = false  # Set to true for testing
```

**See main README.md 'LLM Setup' section for quick start guide and `internal/services/llm/offline/README.md` for detailed technical documentation.**

### Storage Schema

**Documents Table** (`documents`):
- Central unified storage for all source types
- Fields: id, source_id, source_type, title, content, embedding, embedding_model, last_synced, created_at, updated_at
- FTS5 index: documents_fts (title + content)
- Force sync flags: force_sync_pending, force_embed_pending

**Auth Table:**
- `auth_credentials` - Generic web authentication tokens and cookies

**Job Tables:**
- `crawl_jobs` - Persistent job state and progress tracking
  - **Note:** `logs` column was removed - logs now in separate table
- `job_logs` - Unlimited job log history
  - Foreign key with `ON DELETE CASCADE` for automatic cleanup
  - Indexed by job_id and level for efficient queries
- `job_seen_urls` - URL deduplication for crawler jobs

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
0. Check startup logs for 'LLM service initialized in offline mode' vs 'falling back to MOCK mode'
1. `llama-server` binary exists in configured llama_dir:
   - Windows: `where llama-server` or `Test-Path .\llama\llama-server.exe`
   - Unix: `which llama-server` or `ls -la ./llama/llama-server`
2. Binary has execute permissions (Unix/macOS only): `chmod +x ./llama/llama-server`
3. Model files exist:
   - `ls -lh ./models/nomic-embed-text-v1.5-q8.gguf`
   - `ls -lh ./models/qwen2.5-7b-instruct-q4.gguf`
4. Sufficient RAM available (8-16GB)
5. Check llama-server version compatibility: `./llama/llama-server --version`
6. Check logs for subprocess errors
7. Try mock_mode=true for testing without models

**See `internal/services/llm/offline/README.md` for detailed installation and troubleshooting.**

### Installing llama-server Binary

**Quick Reference for Installation Methods:**

**Prebuilt Binaries:**
- Download from https://github.com/ggml-org/llama.cpp/releases
- Windows: `llama-b6922-bin-win-cpu-x64.zip` (CPU) or CUDA/ROCm variants
- macOS: `llama-b6922-bin-macos-arm64.zip` (Apple Silicon) or x64 (Intel)
- Linux: `llama-b6922-bin-ubuntu-x64.zip` (CPU) or Vulkan variant
- Extract to `./llama/llama-server.exe` (Windows) or `./llama/llama-server` (Unix)

**Package Managers:**
- Homebrew (macOS/Linux): `brew install llama.cpp`
- winget (Windows): `winget install llama.cpp`
- MacPorts (macOS): `sudo port install llama.cpp`
- Nix (macOS/Linux): `nix profile install nixpkgs#llama-cpp`

**Build from Source:**
- See detailed instructions in `internal/services/llm/offline/README.md`

**Recommended placement:** `./llama/llama-server.exe` (Windows) or `./llama/llama-server` (Unix)

**Note:** If installed via package manager, llama-server will be in PATH and automatically found.

### Verifying Offline Mode

**Expected startup log output:**

✅ **Success:**
```
LLM service initialized in offline mode
```

❌ **Failure:**
```
Failed to create offline LLM service, falling back to MOCK mode
```

**Explanation:** Mock mode provides fake responses for testing. Embeddings and chat will not work properly.

**Health check endpoint:**
```bash
curl http://localhost:8085/api/health
```

**Note:** Adjust port if configured differently in your quaero.toml.

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
