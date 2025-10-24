# Quaero

**Quaero** (Latin: "I seek, I search") - A local knowledge collection and search system.

## Overview

Quaero is a single-executable Windows service designed to run locally on your machine. It crawls websites using personal cookie authentication, converts content to markdown for LLM consumption, and provides natural language query capabilities through Model Context Protocol (MCP) integration.

### Key Features

- 🔐 **Cookie-Based Authentication** - Chrome extension captures session cookies
- 🕸️ **Website Crawler** - Depth-based crawling starting from seed URLs
- 📝 **Markdown Conversion** - Converts web pages to LLM-friendly markdown
- 💾 **SQLite Storage** - Local database for documents and metadata
- 🎯 **Job Manager** - Persistent queue-based job execution system
- 📚 **Document Summarization** - LLM-powered content summaries
- 🤖 **MCP Integration** - Model Context Protocol for LLM chat interfaces
- 🌐 **Web Interface** - Browser-based UI for job management and monitoring
- ⏰ **Scheduled Jobs** - Automated crawling and summarization tasks

## Technology Stack

- **Language:** Go 1.25+
- **Storage:** SQLite with persistent job queue (goqite)
- **Web UI:** HTML templates, Alpine.js, Bulma CSS
- **Crawler:** chromedp for JavaScript rendering, HTML to Markdown conversion
- **Job Queue:** goqite (SQLite-backed persistent queue)
- **Authentication:** Chrome extension → HTTP POST
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2
- **MCP:** Model Context Protocol for LLM integration (planned)

## Quick Start

### Prerequisites

- Go 1.25+
- Chrome browser
- SQLite support

### Installation

```powershell
# Clone the repository
git clone https://github.com/ternarybob/quaero.git
cd quaero

# Build (Windows only)
.\scripts\build.ps1
```

**Important:** Building MUST use `.\scripts\build.ps1`. Direct `go build` is not supported for production builds.

### Configuration

Create `quaero.toml` in your project directory:

```toml
[server]
host = "localhost"
port = 8085

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

```powershell
# Start the server (after building)
.\bin\quaero.exe

# Or build and run in one step
.\scripts\build.ps1 -Run
```

### Installing Chrome Extension

1. Open Chrome and navigate to `chrome://extensions/`
2. Enable "Developer mode" (top right)
3. Click "Load unpacked"
4. Select the `cmd/quaero-chrome-extension/` directory

### Using Quaero

1. **Start the server:**
   ```powershell
   .\scripts\build.ps1 -Run
   ```

2. **Navigate to a website:**
   - Go to any website you want to crawl (e.g., Confluence, Jira, documentation sites)
   - Log in normally (handles 2FA, SSO, etc.)

3. **Capture Authentication:**
   - Click the Quaero extension icon
   - Click "Capture Authentication"
   - Extension sends cookies to Quaero server

4. **Create a Crawl Job:**
   - Open http://localhost:8085
   - Click "New Job"
   - Enter seed URL(s) to start crawling from
   - Configure crawl depth, filters, and options
   - Click "Start Job"

5. **Monitor Progress:**
   - View job progress in real-time
   - Browse collected documents
   - Check job logs for details

6. **Query Knowledge Base** (upcoming):
   - Ask natural language questions
   - LLM uses MCP to access crawled content
   - Get answers with source citations

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

### UI Framework

**Framework:** Vanilla JavaScript with Alpine.js and Bulma CSS

**Important:** The project uses Alpine.js for client-side interactivity and Bulma CSS for styling.

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
quaero serve --port 8085

# With custom config
quaero serve --config /path/to/quaero.toml
```

### Version

```bash
# Show version
quaero version
```

## Architecture

### Core Components

#### 1. Crawler Service
The crawler service (`internal/services/crawler/`) manages web crawling operations:

**Responsibilities:**
- Creates and manages crawl jobs
- Orchestrates depth-first crawling from seed URLs
- Handles JavaScript rendering with chromedp
- Converts HTML pages to markdown
- Filters and discovers child links
- Applies include/exclude URL patterns
- Tracks job progress and completion

**Key Features:**
- Cookie-based authentication (from Chrome extension)
- Configurable crawl depth
- Domain filtering (stay within domain or expand)
- URL pattern matching (regex include/exclude)
- Max pages limit
- Rate limiting and concurrency control
- JavaScript rendering support

#### 2. Job Manager
The job manager (`internal/jobs/`) handles job lifecycle and execution:

**Job Queue System:**
- Persistent queue backed by goqite (SQLite)
- Jobs survive application restarts
- Worker pool processes messages (5 workers default)
- Type-based routing (crawler_url, summarizer, cleanup)
- Automatic retry with visibility timeout
- Dead-letter handling after 3 attempts

**Job Types:**
1. **crawler_url** - Process individual URLs
   - Fetch and parse HTML
   - Convert to markdown
   - Save to document storage
   - Discover and enqueue child URLs
   - Track progress (completed/pending/failed)

2. **summarizer** - Generate document summaries
   - Batch process documents
   - LLM-powered summarization
   - Extract keywords
   - Update document metadata

3. **cleanup** - Maintenance tasks
   - Remove old completed jobs
   - Clean up job logs
   - Configurable age threshold

#### 3. Document Storage
The document storage (`internal/storage/sqlite/`) manages crawled content:

**Document Model:**
- Unique document ID
- Source URL and type
- Title and markdown content
- Detail level (full, summary, brief)
- Metadata (tags, timestamps, keywords)
- Creation and update timestamps

**Storage Features:**
- SQLite database with FTS5 full-text search
- Document deduplication by URL
- Batch operations for performance
- Metadata queries and filtering
- Document versioning support

#### 4. Scheduler Service
The scheduler (`internal/services/scheduler/`) manages automated tasks:

**Default Jobs:**
1. **crawl_and_collect** (every 10 minutes)
   - Refreshes configured sources
   - Crawls new pages
   - Updates existing documents

2. **scan_and_summarize** (every 2 hours)
   - Scans documents without summaries
   - Generates LLM-powered summaries
   - Extracts keywords

**Features:**
- Cron-based scheduling
- Job enable/disable controls
- Dynamic schedule updates
- Manual trigger support
- Prevents concurrent execution

#### 5. MCP Integration (Planned)
Model Context Protocol integration for LLM chat:

**Planned Features:**
- MCP server exposing document corpus
- Natural language query interface
- Context-aware responses
- Source citation with links
- Progressive thinking chain-of-thought

**Query Examples:**
- "How many backlog items are there?"
- "List all the projects"
- "How do I get access to this server?"
- Technical and developer-focused questions

**Implementation:**
- MCP resource provider for documents
- Vector similarity search (upcoming)
- Context retrieval and ranking
- Response generation with citations

### Authentication Flow

```
1. User logs into website (Jira, Confluence, etc.)
   ↓
2. Extension captures session cookies
   ↓
3. Extension sends POST to localhost:8085/api/auth
   ↓
4. Server stores cookies in SQLite
   ↓
5. Crawler uses cookies for authenticated requests
```

### Crawl Job Flow

```
1. User creates crawl job via UI
   ├─ Seed URLs
   ├─ Crawl depth
   ├─ Include/exclude patterns
   └─ Max pages
   ↓
2. Job manager creates job in database
   ↓
3. Seed URLs enqueued as crawler_url messages
   ↓
4. Worker pool pulls messages from queue
   ↓
5. For each URL:
   ├─ Fetch HTML (with chromedp if JavaScript)
   ├─ Convert to markdown
   ├─ Save document to SQLite
   ├─ Discover child links
   ├─ Filter links (patterns, depth, domain)
   ├─ Deduplicate URLs (database)
   └─ Enqueue valid child URLs
   ↓
6. Job completes when PendingURLs == 0
   ↓
7. UI displays progress and results
```

### Summarization Flow

```
1. Scheduler triggers scan_and_summarize job (cron: every 2 hours)
   ↓
2. summarizer job message enqueued
   ↓
3. Worker pulls and executes summarizer
   ↓
4. Batch query documents without summaries
   ↓
5. For each document:
   ├─ Truncate content to limit
   ├─ Send to LLM service
   ├─ Generate summary
   ├─ Extract keywords
   └─ Update document metadata
   ↓
6. Job completes, documents ready for search
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

```powershell
# Development build
.\scripts\build.ps1

# Production build
.\scripts\build.ps1 -Release

# Clean build
.\scripts\build.ps1 -Clean

# Build and run
.\scripts\build.ps1 -Run
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
```powershell
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
QUAERO_PORT=8085
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

```powershell
# Check port availability (default is 8085)
netstat -an | findstr :8085

# Check if config is valid
type quaero.toml

# Check logs in console output
```

### Extension not connecting

1. Check server is running: http://localhost:8085/health
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
- [AGENTS.md](AGENTS.md) - AI agent development standards
- [CLAUDE.md](CLAUDE.md) - Legacy agent standards (see AGENTS.md)

## Current Status

**✅ Working:**
- Website crawler with depth-based traversal
- Cookie-based authentication via Chrome extension
- HTML to Markdown conversion
- JavaScript rendering (chromedp)
- Persistent job queue (goqite/SQLite)
- Worker pool with job type routing
- Document storage with deduplication
- Job progress tracking (completed/pending/failed URLs)
- URL filtering (include/exclude patterns)
- Job management UI (create, monitor, logs)
- Scheduled jobs (crawl_and_collect, scan_and_summarize)
- Document summarization (LLM-powered)
- Keyword extraction
- Job logs with real-time updates

**⚠️ In Development (~75% Complete):**
- Image extraction from crawled pages (TODO)
- MCP (Model Context Protocol) integration
- Natural language query interface
- Vector embeddings for semantic search
- LLM chat with document context

**❌ Not Yet Implemented:**
- Progressive thinking chain-of-thought responses
- Source citations in chat responses
- Multi-user support
- Cloud deployment
- GitHub/GitLab source integration
- Slack/Teams integration

## Roadmap

See [docs/remaining-requirements.md](docs/remaining-requirements.md) and [docs/QUEUE_MANAGER_IMPLEMENTATION_STATUS.md](docs/QUEUE_MANAGER_IMPLEMENTATION_STATUS.md) for detailed status.

**Current Sprint (~75% Complete):**
- [x] Persistent job queue (goqite)
- [x] Worker pool with job routing
- [x] Crawler job implementation
- [x] Document storage and deduplication
- [x] Job progress tracking
- [x] Summarizer job implementation
- [ ] Image extraction from crawled pages
- [ ] Complete queue manager refactor (remaining 25%)

**Next Sprint:**
- [ ] MCP (Model Context Protocol) server
- [ ] Natural language query interface
- [ ] Vector embeddings for semantic search
- [ ] RAG pipeline with context retrieval
- [ ] Progressive thinking chain-of-thought
- [ ] Source citation system

**Future:**
- [ ] GitHub/GitLab source integration
- [ ] Slack/Teams messaging integration
- [ ] Multi-user support with authentication
- [ ] Cloud deployment option (Docker/K8s)
- [ ] Distributed queue (Redis/RabbitMQ)
- [ ] Advanced analytics and reporting

## Contributing

See [AGENTS.md](AGENTS.md) for AI agent development guidelines and workflow standards.

## License

MIT

---

**Quaero: I seek knowledge. 🔍**
