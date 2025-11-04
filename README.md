# Quaero

**Quaero** (Latin: "I seek, I search") - A local knowledge collection and search system.

## Overview

Enterprise knowledge is locked behind authenticated web applications (Confluence, Jira, documentation sites) where traditional RAG tools cannot access or safely store sensitive data. Quaero solves this by running entirely locally on your machine, capturing your authenticated browser sessions via a Chrome extension, and crawling pages to normalize them into markdown with metadata. All data is stored in a local SQLite database with scheduled recrawls and LLM-powered summarization keeping your private knowledge base current - without any data ever leaving your machine.

Quaero is a local service (Windows, Linux, macOS) that provides fast full-text and semantic search, along with chat capabilities through integrated language models.

### Key Features

- 🔐 **Cookie-Based Authentication** - Chrome extension captures session cookies
- 🕸️ **Website Crawler** - Depth-based crawling starting from seed URLs
- 📝 **Markdown Conversion** - Converts web pages to LLM-friendly markdown
- 💾 **SQLite Storage** - Local database for documents and metadata
- 🎯 **Job Manager** - Persistent queue-based job execution system
- 📚 **Document Summarization** - LLM-powered content summaries
- 🔍 **Advanced Search** - Google-style query parser with FTS5 and vector search
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
- **MCP:** Model Context Protocol for internal agent tools

## Quick Start

### Prerequisites

- Go 1.25+
- Chrome browser
- SQLite support

### Installation

#### Windows (PowerShell)

```powershell
# Clone the repository
git clone https://github.com/ternarybob/quaero.git
cd quaero

# Build
.\scripts\build.ps1
```

#### Linux/macOS (Bash)

```bash
# Clone the repository
git clone https://github.com/ternarybob/quaero.git
cd quaero

# Build
./scripts/build.sh
```

**Important:** Always use the build scripts (`build.ps1` on Windows, `build.sh` on Linux/macOS). Direct `go build` is not supported for production builds as it doesn't handle versioning and assets correctly.

### Configuration

Create `quaero.toml` in your project directory (or use the default from `deployments/local/quaero.toml`):

```toml
# Server configuration
[server]
host = "localhost"
port = 8080  # Default port (can be overridden with --port flag or QUAERO_SERVER_PORT env var)

# Storage configuration
[storage]
type = "sqlite"

[storage.sqlite]
path = "./data/quaero.db"
enable_fts5 = true           # Full-text search
enable_vector = true         # Vector embeddings for semantic search
embedding_dimension = 768    # Matches nomic-embed-text model output
cache_size_mb = 64          # SQLite cache size
wal_mode = true             # Write-ahead logging for better concurrency
busy_timeout_ms = 5000      # Busy timeout in milliseconds

# LLM configuration
[llm]
mode = "offline"  # "offline" (local, secure) or "cloud" (external API)

[llm.offline]
model_dir = "./models"
embed_model = "nomic-embed-text-v1.5-q8.gguf"
chat_model = "qwen2.5-7b-instruct-q4.gguf"

[llm.audit]
enabled = true      # Enable audit logging
log_queries = false # Don't log query text (PII protection)

# Search configuration
[search]
mode = "advanced"  # "advanced" (Google-style), "fts5", or "disabled"
case_sensitive_multiplier = 3
case_sensitive_max_cap = 1000

# Job configuration
[jobs.crawl_and_collect]
enabled = true
auto_start = false      # Don't run on startup
schedule = "*/5 * * * *"  # Every 5 minutes (minimum interval)
```

### Running the Server

#### Windows
```powershell
# Start the server (after building)
.\bin\quaero.exe

# Or build and run in one step
.\scripts\build.ps1 -Run

# With custom port
.\bin\quaero.exe --port 9090
```

#### Linux/macOS
```bash
# Start the server (after building)
./bin/quaero

# With custom config file
./bin/quaero --config deployments/local/quaero.toml

# With environment variables
QUAERO_SERVER_PORT=9090 ./bin/quaero
```

#### Docker
```bash
# Build and run with Docker
docker-compose -f deployments/docker/docker-compose.yml up
```

### Installing Chrome Extension

1. Open Chrome and navigate to `chrome://extensions/`
2. Enable "Developer mode" (top right)
3. Click "Load unpacked"
4. Select the `cmd/quaero-chrome-extension/` directory
5. **Configure server URL** in extension settings if not using default `http://localhost:8080`

## LLM Setup (Offline Mode)

**Security & Privacy**: Offline mode is the default and recommended configuration for Quaero. All LLM processing happens locally on your machine with no network calls or data transmission.

⚠️ **Important**: If the `llama-server` binary is not found during startup, the service will automatically fall back to **MOCK mode**, which provides fake responses for testing only. Real embeddings and chat will not function in mock mode.

### Prerequisites

To run Quaero in offline mode, you need:
- The `llama-server` binary from llama.cpp
- Two model files: one for embeddings, one for chat completions
- Sufficient RAM (4-16GB depending on model size)

### Quick Start

#### 1. Download llama-server Binary

**Option A: Prebuilt Binaries (Recommended)**

Download from the official llama.cpp releases page: https://github.com/ggml-org/llama.cpp/releases

Choose the appropriate binary for your platform:
- **Windows**: `llama-b6922-bin-win-cpu-x64.zip` (CPU) or CUDA/ROCm variants
- **macOS**: `llama-b6922-bin-macos-arm64.zip` (Apple Silicon) or x64 (Intel)
- **Linux**: `llama-b6922-bin-ubuntu-x64.zip` (CPU) or Vulkan variant

Extract and place in `./llama/` directory:
```powershell
# Windows PowerShell
New-Item -ItemType Directory -Force -Path "./llama"
Expand-Archive -Path "llama-b6922-bin-win-cpu-x64.zip" -DestinationPath "./llama"
```

```bash
# Linux/macOS
mkdir -p ./llama
unzip llama-b6922-bin-ubuntu-x64.zip -d ./llama
```

**Option B: Package Managers**

```bash
# macOS/Linux (Homebrew)
brew install llama.cpp

# Windows (winget)
winget install llama.cpp

# MacPorts (macOS)
sudo port install llama.cpp

# Nix (macOS/Linux)
nix profile install nixpkgs#llama-cpp
```

**Option C: Build from Source**

See detailed instructions in `internal/services/llm/offline/README.md`

#### 2. Download Models

Create the models directory and download the recommended models:

```bash
# Create models directory
mkdir -p models

# Download embedding model (~137 MB)
wget https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q8_0.gguf \
  -O models/nomic-embed-text-v1.5-q8.gguf

# Download chat model (~4.3 GB) - choose ONE
wget https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_0.gguf \
  -O models/qwen2.5-7b-instruct-q4.gguf

# OR smaller option (~2.1 GB)
wget https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_0.gguf \
  -O models/qwen2.5-3b-instruct-q4.gguf
```

#### 3. Binary Placement

The service searches for `llama-server` in the following locations (in order):
1. `./llama/llama-server` (or `.exe` on Windows)
2. `./bin/llama-server` (or `.exe`)
3. `./llama-server` (or `.exe`)
4. System PATH

**Recommended placement**: `./llama/llama-server.exe` (Windows) or `./llama/llama-server` (Unix)

```powershell
# Windows
# Place llama-server.exe in the ./llama directory

# Linux/macOS
chmod +x ./llama/llama-server
```

#### 4. Verification

Start Quaero and check the startup logs:

**✅ Success** - Look for this message:
```
LLM service initialized in offline mode
```

**❌ Failure** - If you see this message, the binary or models are missing:
```
Failed to create offline LLM service, falling back to MOCK mode
```

Verify binary exists:
```powershell
# Windows
where llama-server
# Or check manually
Test-Path .\llama\llama-server.exe

# Linux/macOS
which llama-server
# Or check manually
ls -la ./llama/llama-server
```

Verify models exist:
```bash
ls -lh models/
# Should show:
# nomic-embed-text-v1.5-q8.gguf
# qwen2.5-7b-instruct-q4.gguf (or your chosen model)
```

### Troubleshooting

**Binary Not Found**: Ensure `llama-server` is in one of the search paths listed above.

**Models Not Found**: Check that model files exist in the `model_dir` configured in your quaero.toml (default: `./models`).

**Out of Memory**: Use smaller models (3B instead of 7B) or reduce `context_size` in configuration.

**Slow Performance**: Adjust `thread_count` to match your CPU cores, or enable GPU layers if available.

See `internal/services/llm/offline/README.md` for detailed troubleshooting and AGENTS.md for developer-focused debug steps.

### Mode Comparison

| Mode | Security | Performance | Requirements | Use Case |
|------|----------|-------------|--------------|----------|
| **offline** | ✅ 100% local | 🐌 CPU, ⚡ GPU | llama-server + models | Production, sensitive data |
| **mock** | ✅ 100% local | ⚡ Instant | None | Testing, development |
| **cloud** | ⚠️ External APIs | ⚡ Fast | API keys | Development, non-sensitive data |

### Using Quaero

1. **Start the server:**
   ```powershell
   # Windows
   .\scripts\build.ps1 -Run

   # Linux/macOS
   ./scripts/build.sh && ./bin/quaero
   ```

2. **Navigate to a website:**
   - Go to any website you want to crawl (e.g., Confluence, Jira, documentation sites)
   - Log in normally (handles 2FA, SSO, etc.)

3. **Capture Authentication:**
   - Click the Quaero extension icon
   - Extension sends cookies to server via `POST /api/auth`
   - Verify connection status in extension popup

4. **Access Web Interface:**
   - Open http://localhost:8080 (default port)
   - Navigate to Jobs page to create crawl jobs
   - Visit Queue page to monitor running jobs

5. **Create a Crawl Job:**
   - Go to Jobs page
   - Click "New Job Definition"
   - Configure sources, schedule, and crawl parameters
   - Execute job manually or wait for schedule

6. **Search and Query:**
   - Use Search page for advanced queries
   - Chat page for natural language questions with RAG

## Build and Test Instructions

**IMPORTANT:** The following instructions are critical for maintaining a stable development environment.

### Platform-Specific Build Instructions

#### Windows (PowerShell)
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

#### Linux/macOS (Bash)
```bash
# Development build
./scripts/build.sh

# Clean build
./scripts/build.sh --clean

# Release build (optimized)
./scripts/build.sh --release

# Build with tests
./scripts/build.sh --test
```

#### Docker
```bash
# Build Docker image
docker build -f deployments/docker/Dockerfile -t quaero:latest .

# Run with Docker Compose
docker-compose -f deployments/docker/docker-compose.yml up

# Production build with version
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILD=production \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  -f deployments/docker/Dockerfile \
  -t quaero:1.0.0 .
```

**Platform-Specific Notes:**
- **Windows:** UI tests require Chrome installed. Use PowerShell for scripts.
- **Linux:** Ensure execute permissions on build.sh (`chmod +x scripts/build.sh`)
- **macOS:** Requires Chrome or Chromium for UI tests
- **All Platforms:** Always use build scripts to ensure proper versioning and asset handling

### Testing Instructions

**CRITICAL: The test runner handles EVERYTHING automatically!**

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
│   │   ├── collection.go            # Manual sync endpoints
│   │   ├── document.go              # Document management
│   │   ├── scheduler.go             # Event triggers
│   │   ├── job_handler.go           # Job management API
│   │   ├── job_definition_handler.go # Job definition API
│   │   └── chat_handler.go          # Chat API
│   ├── services/                    # Stateful business services
│   │   ├── atlassian/               # Jira & Confluence transformers
│   │   │   ├── jira_transformer.go  # Jira data transformation
│   │   │   └── confluence_transformer.go # Confluence data transformation
│   │   ├── crawler/                 # Website crawler service
│   │   │   ├── service.go           # Core crawler logic
│   │   │   └── filters.go           # URL pattern filtering
│   │   ├── events/                  # Pub/sub event service
│   │   │   └── event_service.go
│   │   ├── scheduler/               # Cron scheduler
│   │   │   └── scheduler_service.go
│   │   ├── llm/                     # LLM abstraction layer
│   │   │   ├── factory.go           # LLM service factory
│   │   │   ├── audit.go             # Audit logging
│   │   │   └── offline/             # Offline llama.cpp implementation
│   │   ├── documents/               # Document service
│   │   ├── chat/                    # Chat service (RAG)
│   │   ├── search/                  # Search service (FTS5)
│   │   ├── summary/                 # Summary generation
│   │   ├── sources/                 # Source configuration
│   │   ├── status/                  # Status tracking
│   │   └── jobs/                    # Job executor & registry
│   │       ├── executor.go          # Job definition executor
│   │       ├── registry.go          # Action type registry
│   │       └── actions/             # Action handlers (crawler, summarizer)
│   ├── queue/                       # Queue-based job system
│   │   ├── manager.go               # Queue manager (goqite)
│   │   ├── worker.go                # Worker pool
│   │   └── types.go                 # Queue message types
│   ├── jobs/                        # Job management
│   │   ├── manager.go               # Job CRUD operations
│   │   └── types/                   # Job type implementations
│   │       ├── base.go              # BaseJob shared functionality
│   │       ├── crawler.go           # CrawlerJob (URL processing)
│   │       ├── summarizer.go        # SummarizerJob
│   │       └── cleanup.go           # CleanupJob
│   ├── storage/                     # Data persistence layer
│   │   └── sqlite/                  # SQLite implementation
│   │       ├── document_storage.go  # Document CRUD
│   │       ├── job_storage.go       # Job CRUD
│   │       ├── source_storage.go    # Source configuration
│   │       └── schema.go            # Database schema & migrations
│   ├── interfaces/                  # Service interfaces
│   │   ├── llm_service.go           # LLM abstraction
│   │   ├── event_service.go         # Event pub/sub
│   │   ├── queue_manager.go         # Queue operations
│   │   ├── job_storage.go           # Job persistence
│   │   └── ...                      # Other interfaces
│   └── models/                      # Data models
│       ├── document.go              # Document model
│       ├── job.go                   # Job models
│       ├── source.go                # Source configuration
│       └── config.go                # Configuration models
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
# Start server (no subcommand needed)
quaero

# With custom port
quaero --port 8080

# With custom host
quaero --host 0.0.0.0

# With custom config
quaero --config /path/to/quaero.toml
```

### Version

```bash
# Show version
quaero version
```

## Security & Privacy

### Local-Only Operation (Offline Mode)

**Default Configuration:** Quaero runs in `offline` mode by default, ensuring:
- ✅ **All data stays local** - No network egress for crawled content
- ✅ **Local LLM inference** - Uses llama.cpp with local model files
- ✅ **SQLite storage** - Database files remain on your machine
- ✅ **No telemetry** - No usage data collection or phone-home

### Cloud Mode Risks

⚠️ **WARNING:** Cloud mode sends data to external APIs. Only enable if you understand the implications:

```toml
[llm]
mode = "cloud"  # ⚠️ SENDS DATA TO EXTERNAL APIS

[llm.cloud]
provider = "gemini"  # Data sent to Google/OpenAI/Anthropic
api_key = "${QUAERO_LLM_CLOUD_API_KEY}"
```

**Cloud Mode Implications:**
- Document content sent to third-party APIs
- Query text transmitted externally
- Subject to provider's data policies
- Not suitable for sensitive/classified data

### Audit Logging

Quaero includes audit logging for compliance:

```toml
[llm.audit]
enabled = true      # Log all LLM interactions
log_queries = false # Disable to protect PII in queries
```

Audit logs are stored in SQLite and include:
- Timestamp and request ID
- Model used and token counts
- Response metadata (not content if `log_queries=false`)
- User context (if multi-user support enabled)

### Authentication Security

- Chrome extension captures cookies locally
- Cookies transmitted only to localhost
- No cloud storage of credentials
- Session data encrypted at rest in SQLite

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

**Job Queue System (goqite):**
- Persistent queue backed by SQLite
- Jobs survive application restarts
- Worker pool processes messages (5 workers default)
- Visibility timeout (5 minutes default) - messages become visible for retry if not completed
- Max receive count (3 attempts) - messages move to dead-letter after exhausting retries
- Delayed completion probe - 5-second grace period after job completion to ensure all child URLs are processed
- Atomic progress updates - Pending/Total counts maintained consistently when spawning child URLs
- Heartbeat mechanism for long-running jobs to prevent visibility timeout

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

#### 4. Search Service
The search service (`internal/services/search/`) provides multiple search modes:

**Search Modes:**
- **advanced** (default) - Google-style query parser with operators:
  - Quoted phrases: `"exact match"`
  - Boolean operators: `AND`, `OR`, `NOT`
  - Field searches: `title:keyword`
  - Wildcards: `test*`
- **fts5** - Direct SQLite FTS5 full-text search
- **disabled** - Search disabled

**Features:**
- Case-sensitive search with multiplier (fetches 3x results, caps at 1000)
- SQLite FTS5 indexing on title + content
- Vector search support when enabled (`storage.sqlite.enable_vector=true`)
- Configurable embedding dimensions (768 for nomic-embed-text)
- Hybrid search combining keyword and semantic results

#### 5. Scheduler Service
The scheduler (`internal/services/scheduler/`) manages automated tasks:

**Default Jobs:**
1. **crawl_and_collect** (every 5 minutes minimum)
   - Refreshes configured sources
   - Crawls new pages
   - Updates existing documents

**Features:**
- Cron-based scheduling
- Job enable/disable controls
- `auto_start` flag for immediate execution on startup
- Dynamic schedule updates with 5-minute minimum interval
- Manual trigger support via API
- Prevents concurrent execution

#### 6. MCP Integration
Model Context Protocol integration (internal for Claude Code only):

**Current Status:**
- ⚠️ **Internal use only** - MCP endpoint is specifically for Claude Code integration
- Not a general-purpose MCP server implementation
- Provides document corpus access to Claude agents

**Supported Queries (via Claude Code):**
- "How many backlog items are there?"
- "List all the projects"
- "How do I get access to this server?"
- Technical and developer-focused questions

**Implementation Notes:**
- `/mcp` endpoint handles Claude-specific requests
- Documents exposed as MCP resources
- Query interface for agent tools only
- Not intended for external MCP clients

### Authentication Flow

```
1. User logs into website (Jira, Confluence, etc.)
   ↓
2. Extension captures session cookies
   ↓
3. Extension sends POST to localhost:8080/api/auth
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

### Pages

#### Dashboard (`/`)
- System overview and status
- Quick access to main features
- Authentication status

#### Jobs (`/jobs`)
- Job definition management
- Create, edit, delete job definitions
- Configure sources and schedules
- Execute jobs manually

#### Queue (`/queue`)
- Active job monitoring
- Real-time job status updates
- Job logs and progress tracking
- Cancel or rerun jobs

#### Search (`/search`)
- Advanced search with query operators
- Full-text and semantic search
- Filter by source, date, type

#### Chat (`/chat`)
- Natural language queries
- RAG-enabled responses
- Document context integration

#### Documents (`/documents`)
- Browse collected documents
- View document metadata
- Force reprocessing

#### Settings (`/settings`)
- Application configuration
- LLM settings
- Storage management

## API Endpoints

### HTTP Endpoints

#### Authentication
```
POST /api/auth                          - Update authentication from Chrome extension
GET  /api/auth/status                   - Check authentication status
GET  /api/auth/list                     - List authenticated sources
```

#### Sources
```
GET  /api/sources                       - List all sources
GET  /api/sources/{id}                  - Get source by ID
POST /api/sources                       - Create new source
PUT  /api/sources/{id}                  - Update source
DELETE /api/sources/{id}                - Delete source
```

#### Job Definitions
```
GET  /api/job-definitions                - List all job definitions
GET  /api/job-definitions/{id}           - Get job definition by ID
POST /api/job-definitions                - Create new job definition
PUT  /api/job-definitions/{id}           - Update job definition
DELETE /api/job-definitions/{id}         - Delete job definition
POST /api/job-definitions/{id}/execute   - Execute job definition manually
```

#### Jobs
```
GET  /api/jobs                          - List all jobs (with pagination)
GET  /api/jobs/{id}                     - Get job by ID
POST /api/jobs/{id}/cancel              - Cancel running job
POST /api/jobs/{id}/retry               - Retry failed job
DELETE /api/jobs/{id}                   - Delete job
GET  /api/jobs/{id}/logs                - Get job logs
```

#### Documents
```
GET  /api/documents                     - List documents (with pagination)
GET  /api/documents/{id}                - Get document by ID
PUT  /api/documents/{id}                - Update document
DELETE /api/documents/{id}              - Delete document
POST /api/documents/search              - Search documents
```

#### Search
```
POST /api/search                        - Advanced search with query operators
```

#### Chat
```
POST /api/chat                          - Send chat message (RAG-enabled)
GET  /api/chat/history                  - Get chat history
```

#### System
```
GET  /api/version                       - Server version info
GET  /api/health                        - Health check endpoint
GET  /api/config                        - Get server configuration
```

#### MCP (Model Context Protocol)
```
POST /mcp                                - Handle MCP requests

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
- [AGENTS.md](AGENTS.md) - AI agent development standards
- [CLAUDE.md](CLAUDE.md) - Legacy agent standards (see AGENTS.md)

## Current Status

**✅ Working:**
- Generic website crawler with depth-based traversal
- Cookie-based authentication via Chrome extension
- HTML to Markdown conversion with chromedp
- Persistent job queue (goqite/SQLite)
- Worker pool with configurable concurrency
- Document storage with SQLite FTS5
- Job progress tracking with real-time WebSocket updates
- URL filtering (include/exclude regex patterns)
- Job management UI (create, monitor, execute)
- Scheduled jobs with cron expressions
- LLM-powered document summarization (offline/cloud modes)
- Advanced search with Google-style query parser
- Chat interface with RAG support
- Real-time job logs and status updates

**⚠️ In Progress:**
- Image extraction from crawled pages
- MCP endpoint (internal Claude Code use only)
- Vector embeddings optimization
- Source citation formatting

**❌ Not Yet Implemented:**
- Multi-user support with authentication
- GitHub/GitLab native integrations
- Slack/Teams connectors
- Distributed queue support (Redis/RabbitMQ)
- Cloud-native deployment (Kubernetes)

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
