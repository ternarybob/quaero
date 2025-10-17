# Quaero

**Quaero** (Latin: "I seek, I search") - A knowledge collection system with web-based interface.

## Overview

Quaero collects documentation from Atlassian (Confluence, Jira) using browser extension authentication and provides a web-based interface for browsing and searching the data.

### Key Features

- 🔐 **Automatic Authentication** - Chrome extension captures credentials
- 📊 **Real-time Updates** - WebSocket-based live log streaming
- 💾 **SQLite Storage** - Local database with full-text search
- 🌐 **Web Interface** - Browser-based UI for collection and browsing
- ⚡ **Fast Collection** - Efficient scraping and storage
- ⏰ **Scheduled Jobs** - Automated crawling and document summarization

## Technology Stack

- **Language:** Go 1.25+
- **Storage:** SQLite with FTS5 (full-text search)
- **Web UI:** HTML templates, Alpine.js, Spectre CSS, WebSockets
- **Authentication:** Chrome extension → WebSocket → HTTP service
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2

## Quick Start

### Prerequisites

- Go 1.25+
- Chrome browser
- SQLite support

### Installation

```bash
# Clone the repository
git clone https://github.com/ternarybob/quaero.git
cd quaero

# Build
./scripts/build.ps1

# Or use Go directly
go build -o bin/quaero ./cmd/quaero
```

### Configuration

Create `quaero.toml` in your project directory:

```toml
[server]
host = "localhost"
port = 8080

[logging]
level = "info"
format = "json"

[storage]
type = "sqlite"

[storage.sqlite]
path = "./quaero.db"
enable_fts5 = true
enable_wal = true
```

### Running the Server

```bash
# Start the server
./bin/quaero serve

# Or with custom config
./bin/quaero serve --config /path/to/quaero.toml --port 8080
```

### Installing Chrome Extension

1. Open Chrome and navigate to `chrome://extensions/`
2. Enable "Developer mode" (top right)
3. Click "Load unpacked"
4. Select the `cmd/quaero-chrome-extension/` directory

### Using Quaero

1. **Start the server:**
   ```bash
   ./bin/quaero serve
   ```

2. **Navigate to Atlassian:**
   - Go to your Confluence or Jira instance
   - Log in normally (handles 2FA, SSO, etc.)

3. **Capture Authentication:**
   - Click the Quaero extension icon
   - Click "Send to Quaero"
   - Extension sends credentials to server

4. **Access Web UI:**
   - Open http://localhost:8080
   - Click "Confluence" or "Jira"
   - Click "Collect" to start gathering data

5. **Browse Data:**
   - View collected spaces/projects
   - Browse pages/issues
   - Real-time log updates

## Build and Test Instructions

**IMPORTANT:** The following instructions are critical for maintaining a stable development environment.

### Build and Run Instructions (Windows ONLY)

-   **Building, compiling, and running the application MUST be done using the following scripts:**
    -   `./scripts/build.ps1`
    -   `./scripts/build.ps1 -Run`
-   **The ONLY exception** is using `go build` for a compile test, with no output binary.

### Testing Instructions

**CRITICAL: The test runner handles EVERYTHING automatically!**

The test runner (`cmd/quaero-test-runner/`) builds the application, starts the service, runs all tests, and cleans up automatically.

```powershell
# Option 1: Use pre-built test runner (recommended)
.\scripts\build.ps1           # Builds test runner automatically
cd bin
.\quaero-test-runner.exe

# Option 2: Run from source
cd cmd/quaero-test-runner
go run .
```

**See:** `cmd/quaero-test-runner/README.md` for detailed documentation, configuration, and troubleshooting.

**IMPORTANT:**
- ❌ DO NOT run `build.ps1` before the test runner
- ❌ DO NOT manually start the service before the test runner
- ✅ Let the test runner control the service lifecycle

**For Development/Debugging Only:**
```powershell
# Run tests directly (requires manual service start)
.\scripts\build.ps1 -Run      # Start service in separate window first
cd test
go test -v ./api              # API tests
go test -v ./ui               # UI tests
```

### UI Framework Migration

**Note:** Quaero migrated from Metro UI v5 to Spectre CSS for improved maintainability and modern design patterns.

**Git Checkpoint (Before Major Changes):**
```bash
# Create migration branch
git checkout -b refactor-spectre-css

# Create checkpoint
git commit -m "Checkpoint before Spectre CSS migration"
```

**Full Migration Checklist:** See [docs/MIGRATION_TESTING.md](docs/MIGRATION_TESTING.md) for comprehensive testing checklist and rollback procedures.

## Project Structure

```
quaero/
├── cmd/
│   ├── quaero/                      # Main application entry point
│   └── quaero-chrome-extension/     # Chrome extension for auth
├── internal/
│   ├── app/                         # Application orchestration & DI
│   ├── common/                      # Stateless utilities (config, logging, banner)
│   ├── server/                      # HTTP server & routing
│   ├── handlers/                    # HTTP & WebSocket handlers
│   │   ├── api.go                   # System API (version, health)
│   │   ├── ui.go                    # UI page handlers
│   │   ├── websocket.go             # WebSocket & log streaming
│   │   ├── scraper.go               # Scraper triggers
│   │   ├── collector.go             # Paginated data endpoints
│   │   ├── collection.go            # Manual sync endpoints
│   │   ├── document.go              # Document management
│   │   ├── scheduler.go             # Event triggers
│   │   └── embedding_handler.go     # Embedding API (testing)
│   ├── services/                    # Stateful business services
│   │   ├── atlassian/               # Jira & Confluence collectors
│   │   │   ├── jira_*.go            # Jira scraping services
│   │   │   └── confluence_*.go      # Confluence scraping services
│   │   ├── collection/              # Collection coordinator
│   │   │   └── coordinator_service.go
│   │   ├── embeddings/              # Embedding services
│   │   │   ├── embedding_service.go   # Core embedding logic
│   │   │   └── coordinator_service.go # Embedding coordinator
│   │   ├── events/                  # Pub/sub event service
│   │   │   └── event_service.go
│   │   ├── scheduler/               # Cron scheduler
│   │   │   └── scheduler_service.go
│   │   ├── llm/                     # LLM abstraction layer
│   │   │   ├── factory.go           # LLM service factory
│   │   │   ├── audit.go             # Audit logging
│   │   │   └── offline/             # Offline llama.cpp implementation
│   │   ├── documents/               # Document service
│   │   ├── processing/              # Processing service
│   │   └── workers/                 # Worker pool pattern
│   ├── storage/                     # Data persistence layer
│   │   └── sqlite/                  # SQLite implementation

│   │       ├── document_storage.go  # Document CRUD
│   │       ├── jira_storage.go      # Jira data storage
│   │       └── confluence_storage.go # Confluence data storage
│   ├── interfaces/                  # Service interfaces
│   │   ├── llm_service.go           # LLM abstraction
│   │   ├── event_service.go         # Event pub/sub
│   │   ├── embedding_service.go     # Embedding generation
│   │   └── ...                      # Other interfaces
│   └── models/                      # Data models
│       ├── document.go              # Document model
│       ├── jira.go                  # Jira models
│       └── confluence.go            # Confluence models
├── pages/                           # Web UI templates
│   ├── index.html                   # Dashboard
│   ├── jira.html                    # Jira UI
│   ├── confluence.html              # Confluence UI
│   ├── documents.html               # Documents browser
│   ├── embeddings.html              # Embeddings test UI
│   ├── partials/                    # Reusable components
│   └── static/                      # CSS, JS
├── test/                            # Go-native test infrastructure
│   ├── main_test.go                 # TestMain fixture (setup/teardown)
│   ├── helpers.go                   # Common test utilities
│   ├── run_tests.go                 # Go-native test runner
│   ├── api/                         # API integration tests
│   │   ├── sources_api_test.go
│   │   └── chat_api_test.go
│   ├── ui/                          # UI tests (chromedp)
│   │   ├── homepage_test.go
│   │   └── chat_test.go
│   └── results/                     # Test results (timestamped)
├── scripts/                         # Build & deployment
│   └── build.ps1                    # Build script
├── docs/                            # Documentation
│   ├── architecture.md
│   ├── requirements.md
│   └── remaining-requirements.md
├── bin/                             # Build output
│   ├── quaero.exe                   # Compiled binary
│   ├── quaero.toml                  # Runtime config
│   └── data/                        # SQLite database
└── CLAUDE.md                        # Development standards
```

## Commands

### Server

```bash
# Start server
quaero serve

# With custom port
quaero serve --port 8080

# With custom config
quaero serve --config /path/to/quaero.toml
```

### Version

```bash
# Show version
quaero version
```

## Architecture

### Core Services

#### 1. Collectors (Jira & Confluence)
The collector services (`internal/services/atlassian/`) scrape data from Atlassian APIs:

**Jira Collector** (`jira_*.go`):
- Scrapes projects, issues, and metadata
- Uses Atlassian REST API v3
- Stores data as documents with `source_type=jira`
- Auto-subscribes to collection events

**Confluence Collector** (`confluence_*.go`):
- Scrapes spaces, pages, and content
- Uses Confluence REST API
- Stores data as documents with `source_type=confluence`
- Auto-subscribes to collection events

Both collectors:
- Load authentication from database
- Support pagination for large datasets
- Stream real-time logs via WebSocket
- Create document records for vector search

#### 2. Collection Coordinator
The collection coordinator (`internal/services/collection/coordinator_service.go`) orchestrates data synchronization:

**Responsibilities:**
- Subscribes to `EventCollectionTriggered` events
- Queries documents marked with `force_sync_pending=true`
- Dispatches sync jobs to worker pool (max 10 concurrent)
- Delegates to appropriate collector (Jira/Confluence) based on `source_type`
- Updates `last_synced` timestamp on completion
- Clears `force_sync_pending` flag

**Worker Pool:**
- Bounded concurrency (10 workers)
- Parallel processing of sync jobs
- Error collection and aggregation
- Panic recovery per worker

#### 3. Embedding Services
The embedding system generates vector embeddings for semantic search:

**Embedding Service** (`internal/services/embeddings/embedding_service.go`):
- Generates 768-dimension embeddings via LLM service
- Supports offline mode (local models) and cloud mode (APIs)
- Combines document title + content for embedding
- Logs operations to audit trail
- Currently using mock mode for testing

**Embedding Coordinator** (`internal/services/embeddings/coordinator_service.go`):
- Subscribes to `EventEmbeddingTriggered` events
- Processes documents with `force_embed_pending=true` (forced)
- Processes unvectorized documents (missing embeddings)
- Uses worker pool for parallel embedding generation
- Updates document with embedding vector and model name

**LLM Service Modes:**
- **Offline:** Local llama.cpp models (nomic-embed-text, qwen2.5)
- **Cloud:** OpenAI/Anthropic APIs (planned)
- **Mock:** Fake embeddings for testing (current default)

#### 4. Event-Driven Architecture
The event service (`internal/services/events/event_service.go`) implements pub/sub pattern:

**Event Types:**
- `EventCollectionTriggered` - Triggers data collection from sources
- `EventEmbeddingTriggered` - Triggers embedding generation

**Features:**
- Asynchronous publishing (`Publish`) - fire-and-forget
- Synchronous publishing (`PublishSync`) - wait for all handlers
- Multiple subscribers per event type
- Panic recovery in event handlers
- Error aggregation and reporting

**Flow:**
```
Scheduler (cron) → Publish Event → Event Service → All Subscribers
                                                    ├─ Jira Collector
                                                    ├─ Confluence Collector
                                                    ├─ Collection Coordinator
                                                    └─ Embedding Coordinator
```

#### 5. Scheduler Service
The scheduler (`internal/services/scheduler/scheduler_service.go`) triggers automated workflows:

**Capabilities:**
- Cron-based scheduling (default: every 1 minute)
- Publishes `EventCollectionTriggered` events
- Publishes `EventEmbeddingTriggered` events
- Manual trigger support via API
- Prevents concurrent execution with mutex

**Workflow Cascade:**
```
Scheduler → Collection Event → Collectors scrape data → Documents created
         → Embedding Event → Coordinator generates embeddings → Vectors stored
```

### Authentication Flow

```
1. User logs into Atlassian
   ↓
2. Extension captures cookies/tokens
   ↓
3. Extension connects to ws://localhost:8080/ws
   ↓
4. Extension sends auth data
   ↓
5. Server stores credentials in SQLite
   ↓
6. Collectors use credentials for API calls
```

### Collection Flow

```
1. Scheduler triggers collection event (cron: */1 * * * *)
   ↓
2. Event service publishes to all subscribers
   ↓
3. Jira/Confluence collectors scrape their sources
   ↓
4. Collectors create document records in SQLite
   ↓
5. Collection coordinator processes force_sync documents
   ↓
6. Worker pool executes sync jobs in parallel (10 workers)
   ↓
7. WebSocket streams real-time logs to UI
```

### Embedding Flow

```
1. Scheduler triggers embedding event
   ↓
2. Embedding coordinator queries documents
   ├─ Documents with force_embed_pending=true
   └─ Unvectorized documents (no embedding)
   ↓
3. Worker pool generates embeddings in parallel
   ↓
4. LLM service (offline/cloud/mock) creates 768-dim vectors
   ↓
5. Documents updated with embedding + model name
   ↓
6. Vectors ready for semantic search (future: sqlite-vec)
```

## Web UI

### Dashboard (/)
- System status
- Authentication status
- Quick links

### Confluence (/confluence)
- Space browser
- Collection trigger
- Real-time logs

### Jira (/jira)
- Project browser
- Collection trigger
- Real-time logs

## API Endpoints

### HTTP Endpoints

#### UI Routes
```
GET  /                           - Dashboard
GET  /jira                       - Jira UI
GET  /confluence                 - Confluence UI
GET  /documents                  - Documents browser
GET  /embeddings                 - Embeddings test page
GET  /settings                   - Settings page
GET  /static/common.css          - Shared styles
GET  /ui/status                  - UI status endpoint
GET  /ui/parser-status           - Parser status
```

#### Authentication
```
POST /api/auth                   - Update authentication credentials
```

#### Collection (UI-triggered)
```
POST /api/scrape                 - Trigger collection
POST /api/scrape/projects        - Scrape Jira projects
POST /api/scrape/spaces          - Scrape Confluence spaces
```

#### Cache Management
```
POST /api/projects/refresh-cache     - Refresh Jira projects cache
POST /api/projects/get-issues        - Get project issues
POST /api/spaces/refresh-cache       - Refresh Confluence spaces cache
POST /api/spaces/get-pages           - Get space pages
```

#### Data Access
```
POST /api/data/clear-all             - Clear all data
POST /api/data/jira/clear            - Clear Jira data
POST /api/data/confluence/clear      - Clear Confluence data
GET  /api/data/jira                  - Get Jira data summary
GET  /api/data/jira/issues           - Get Jira issues
GET  /api/data/confluence            - Get Confluence data summary
GET  /api/data/confluence/pages      - Get Confluence pages
```

#### Collector (Paginated Data)
```
GET  /api/collector/projects         - Get projects (paginated)
GET  /api/collector/spaces           - Get spaces (paginated)
GET  /api/collector/issues           - Get issues (paginated)
GET  /api/collector/pages            - Get pages (paginated)
```

#### Collection Sync (Manual)
```
POST /api/collection/jira/sync       - Manual Jira sync
POST /api/collection/confluence/sync - Manual Confluence sync
POST /api/collection/sync-all        - Sync all sources
```

#### Documents
```
GET  /api/documents/stats            - Document statistics
GET  /api/documents                  - List documents
POST /api/documents/process          - Process documents
POST /api/documents/force-sync       - Force sync document
POST /api/documents/force-embed      - Force embed document
```

#### Embeddings
```
POST /api/embeddings/generate        - Generate embedding (testing)
```

#### Processing
```
GET  /api/processing/status          - Processing status
```

#### Scheduler
```
POST /api/scheduler/trigger-collection - Trigger collection event
POST /api/scheduler/trigger-embedding  - Trigger embedding event
```

#### Default Jobs
```
GET  /api/jobs/default                      - List all default jobs with status
POST /api/jobs/default/{name}/enable        - Enable a default job
POST /api/jobs/default/{name}/disable       - Disable a default job
PUT  /api/jobs/default/{name}/schedule      - Update job schedule (JSON: {"schedule": "* * * * *"})
```

#### System
```
GET  /api/version                    - API version
GET  /api/health                     - Health check
```

### WebSocket

```
WS   /ws                             - Real-time updates & log streaming
```

## Development

### Building

```bash
# Development build
./scripts/build.ps1

# Production build
./scripts/build.ps1 -Release

# Clean build
./scripts/build.ps1 -Clean
```

### Testing

**Test Runner** (`cmd/quaero-test-runner/`):

The test runner handles the complete test lifecycle automatically:
- Builds the application
- Starts the service in a visible window
- Runs API and UI tests
- Captures screenshots for UI tests
- Saves results to timestamped directories
- Stops the service and cleans up

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
```bash
# Run tests directly (requires manual service start)
.\scripts\build.ps1 -Run      # Start service in separate window first

# Run specific test suite
cd test
go test -v ./api              # API integration tests
go test -v ./ui               # UI browser tests

# Run unit tests (colocated with source)
go test ./internal/...

# Run specific test
cd test
go test -v ./api -run TestListSources
```

**See:** `cmd/quaero-test-runner/README.md` for detailed documentation.

**Test Coverage:**
- **Unit Tests** (`internal/*/...`): Colocated with source code
  - Crawler service (9 tests)
  - Search service (8 tests)
  - Storage/SQLite (11 tests)
  - Config, identifiers, metadata (30 tests)
- **API Tests** (`test/api/`): HTTP endpoint testing
  - Sources API
  - Chat API
- **UI Tests** (`test/ui/`): Browser automation (chromedp)
  - Homepage workflows
  - Chat interface

### Code Quality

See [CLAUDE.md](CLAUDE.md) for:
- Agent-based development system
- Code quality standards
- Architecture patterns
- Testing requirements

## Configuration

### Priority Order

1. **CLI Flags** (highest)
2. **Environment Variables**
3. **Config File** (quaero.toml)
4. **Defaults** (lowest)

### Environment Variables

```bash
QUAERO_PORT=8080
QUAERO_HOST=localhost
QUAERO_LOG_LEVEL=info
```

### Configuration File

```toml
[server]
host = "localhost"
port = 8085
llama_dir = "./llama"

[sources.confluence]
enabled = true
spaces = ["TEAM", "DOCS"]

[sources.jira]
enabled = true
projects = ["DATA", "ENG"]

[sources.github]
enabled = false
token = "${GITHUB_TOKEN}"
repos = ["your-org/repo1"]

[llm]
mode = "offline"  # "offline", "cloud", or "mock"

[llm.offline]
model_dir = "./models"
embed_model = "nomic-embed-text-v1.5.Q8_0.gguf"
chat_model = "qwen2.5-7b-instruct-q4_k_m.gguf"
context_size = 2048
thread_count = 4
gpu_layers = 0
mock_mode = true  # Set to false to use actual models

[llm.audit]
enabled = true
log_queries = false  # PII protection

[jobs]
# Default jobs configuration

[jobs.crawl_and_collect]
enabled = true
schedule = "*/10 * * * *"  # Every 10 minutes

[jobs.scan_and_summarize]
enabled = true
schedule = "0 */2 * * *"  # Every 2 hours

[logging]
level = "debug"
output = ["console", "file"]

[storage]
type = "sqlite"

[storage.sqlite]
path = "./quaero.db"
enable_fts5 = true
enable_wal = true
cache_size_mb = 100
```

## Troubleshooting

### Server won't start

```bash
# Check port availability
netstat -an | grep 8080

# Try different port
./bin/quaero serve --port 8081
```

### Extension not connecting

1. Check server is running: http://localhost:8080/health
2. Check extension permissions in Chrome
3. Reload extension
4. Check browser console for errors

### Collection fails

1. Verify authentication in extension
2. Check server logs
3. Verify Atlassian instance URL
4. Check network connectivity

## Documentation

- [Architecture](docs/architecture.md) - System architecture and design
- [Dependency Injection](docs/dependency-injection.md) - Constructor-based DI pattern
- [Requirements](docs/requirements.md) - Current requirements
- [Remaining Requirements](docs/remaining-requirements.md) - Future work
- [CLAUDE.md](CLAUDE.md) - Development standards

## Current Status

**✅ Working:**
- Jira and Confluence collectors with event-driven architecture
- Document storage with force sync support
- Vector embeddings (mock mode) - 768-dimension
- Embedding API endpoint for testing
- Worker pool pattern for parallel processing
- LLM audit logging and monitoring
- Real-time WebSocket log streaming
- Cron-based scheduler for automated workflows
- Default scheduled jobs (crawl_and_collect, scan_and_summarize)
- Web UI for managing default jobs (enable/disable, schedule editing)

**⚠️ In Development:**
- Offline LLM integration (llama.cpp models)
- Vector search (sqlite-vec integration)
- Natural language query interface
- RAG pipeline

**❌ Not Yet Implemented:**
- GitHub collector
- Cloud LLM mode (OpenAI/Anthropic APIs)
- Multi-user support
- Semantic search UI

## Roadmap

See [docs/remaining-requirements.md](docs/remaining-requirements.md) for detailed roadmap.

**Current Sprint:**
- [x] Vector embeddings (mock mode working)
- [x] Embedding API endpoint
- [x] Unit and integration tests for embeddings
- [ ] Offline LLM integration (llama-server)
- [ ] sqlite-vec integration for vector search

**Next Sprint:**
- [ ] Natural language query interface
- [ ] RAG pipeline with context retrieval
- [ ] Semantic search UI

**Future:**
- [ ] GitHub collector
- [ ] Cloud LLM mode (OpenAI, Anthropic)
- [ ] Additional data sources (Slack, Linear)
- [ ] Multi-user support
- [ ] Cloud deployment option

## Contributing

See [CLAUDE.md](CLAUDE.md) for development guidelines and agent-based workflow.

## License

MIT

---

**Quaero: I seek knowledge. 🔍**
