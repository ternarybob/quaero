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

## CRITICAL: OS DETECTION AND COMMAND EXECUTION

**Failure to follow these instructions will result in your removal from the project.**

### Operating System Detection (MANDATORY)

**BEFORE executing ANY shell command, you MUST determine the operating system:**

```
┌─────────────────────────────────────────────────────────────────┐
│ OS DETECTION CHECKLIST                                          │
│                                                                  │
│ 1. Check context/environment information for OS indicators      │
│ 2. Look for shell type: powershell, bash, zsh, cmd              │
│ 3. Check path separators: \ = Windows, / = Unix/Linux/macOS     │
│ 4. Check workspace path format:                                 │
│    - C:\... or D:\... = Windows                                 │
│    - /home/... or /Users/... = Unix/Linux/macOS                 │
│                                                                  │
│ WHEN IN DOUBT: Ask the user which OS they are running           │
└─────────────────────────────────────────────────────────────────┘
```

### OS-Specific Command Reference

| Task | Windows (PowerShell) | Linux/macOS (Bash) |
|------|---------------------|-------------------|
| Build | `.\scripts\build.ps1` | `./scripts/build.sh` |
| Build + Deploy | `.\scripts\build.ps1 -Deploy` | `./scripts/build.sh --deploy` |
| Build + Run | `.\scripts\build.ps1 -Run` | `./scripts/build.sh --run` |
| Run tests | `go test -v ./test/...` | `go test -v ./test/...` |
| List directory | `Get-ChildItem` or `dir` | `ls -la` |
| Create directory | `New-Item -ItemType Directory -Path "dir"` or `mkdir dir` | `mkdir -p dir` |
| Remove file | `Remove-Item file` | `rm file` |
| Remove directory | `Remove-Item -Recurse dir` | `rm -rf dir` |
| Environment variable | `$env:VAR_NAME` | `$VAR_NAME` |
| Set env variable | `$env:VAR_NAME = "value"` | `export VAR_NAME="value"` |
| Path separator | `\` | `/` |
| Check port | `netstat -an \| findstr :8085` | `lsof -i :8085` or `netstat -an \| grep 8085` |
| Kill process | `Stop-Process -Name quaero` | `pkill quaero` or `kill $(pgrep quaero)` |
| Find files | `Get-ChildItem -Recurse -Filter "*.go"` | `find . -name "*.go"` |
| File content | `Get-Content file.txt` | `cat file.txt` |

### Command Execution Rules

**NEVER do this:**
```bash
# ❌ WRONG: Using bash commands on Windows
./scripts/build.sh          # Won't work on Windows
rm -rf ./bin                 # Won't work on Windows
export VAR=value             # Won't work on Windows PowerShell

# ❌ WRONG: Using PowerShell commands on Linux/macOS
.\scripts\build.ps1          # Won't work on Linux/macOS
Remove-Item -Recurse bin     # Won't work on Linux/macOS
$env:VAR = "value"           # Won't work on Linux/macOS bash
```

**ALWAYS do this:**
```
# ✅ CORRECT: Detect OS first, then use appropriate commands

# If Windows detected:
.\scripts\build.ps1 -Run

# If Linux/macOS detected:
./scripts/build.sh --run
```

### WSL (Windows Subsystem for Linux) Environment

**When operating in WSL:**

```
┌─────────────────────────────────────────────────────────────────┐
│ WSL DETECTION                                                    │
│                                                                  │
│ Indicators you are in WSL:                                      │
│ • Path contains /mnt/c/ or /mnt/d/                              │
│ • uname -r shows "Microsoft" or "WSL"                           │
│ • /proc/version contains "Microsoft"                            │
│                                                                  │
│ WSL RULES:                                                       │
│ 1. Normalize paths: replace \ with /                            │
│ 2. Use Linux commands (bash, not powershell)                    │
│ 3. Go/build tools: use powershell.exe for Windows Go            │
│ 4. Test execution: run via powershell.exe from Windows path     │
└─────────────────────────────────────────────────────────────────┘
```

**WSL Command Patterns:**

| Task | WSL Command |
|------|-------------|
| Run Go tests | `powershell.exe -Command "cd C:\\path\\to\\project; go test -v ./..."` |
| Build | `powershell.exe -Command "cd C:\\path\\to\\project; .\\scripts\\build.ps1"` |
| Path conversion | `/mnt/c/dev/project` → `C:\\dev\\project` |

**Path Normalization:**
```bash
# Convert WSL path to Windows path
WSL_PATH="/mnt/c/development/quaero"
WIN_PATH=$(echo "$WSL_PATH" | sed 's|/mnt/\([a-z]\)/|\U\1:\\|' | sed 's|/|\\|g')
# Result: C:\development\quaero
```

---

## BUILD AND TEST INSTRUCTIONS

### Build Instructions

**Building, compiling, and running the application MUST be done using the appropriate script for your OS:**

#### Windows (PowerShell)
```powershell
# Development build (silent, no deployment, no version increment)
.\scripts\build.ps1

# Deploy files to bin directory (stops service, deploys files)
.\scripts\build.ps1 -Deploy

# Build, deploy, and run (starts service in new terminal)
.\scripts\build.ps1 -Run
```

#### Linux/macOS (Bash)
```bash
# Development build
./scripts/build.sh

# Deploy files to bin directory
./scripts/build.sh --deploy

# Build, deploy, and run
./scripts/build.sh --run
```

**ONLY exception for direct `go` commands:** `go build` for compile tests (no output binary)

**Important Notes:**
- **Default build (no parameters)** - Builds executable silently, does NOT increment version, does NOT deploy files
- **Version management** - Version number in `.version` file is NEVER auto-incremented, only build timestamp updates
- **Deployment** - Use `-Deploy`/`--deploy` or `-Run`/`--run` to copy files (config, pages, Chrome extension) to bin/
- **Removed parameters** - `-Clean`, `-Verbose`, `-Release`, `-ResetDatabase` removed for simplicity. See `docs/simplify-build-script/migration-guide.md` for alternatives

### Testing Instructions

**CRITICAL: The test runner handles EVERYTHING automatically - do NOT run build scripts or start the service manually!**

**IMPORTANT: Do NOT create temporary files for testing or building (e.g., run_test.ps1, test_compile.go, etc.). Always use the official build and test commands:**

| OS | Build Command | Test Command |
|----|---------------|--------------|
| Windows | `.\scripts\build.ps1` | `go test -v ./test/...` |
| Linux/macOS | `./scripts/build.sh` | `go test -v ./test/...` |

**Note:** Go test commands are cross-platform and work the same on all operating systems.

## Project Overview

**Quaero** (Latin: "I seek, I search") - A knowledge collection system with RAG capabilities.

### Key Features

- 🔐 **Automatic Authentication** - Chrome extension captures credentials
- 📊 **Real-time Updates** - WebSocket-based live log streaming
- 💾 **Badger Storage** - Local embedded key-value database for documents and metadata
- 🌐 **Web Interface** - Browser-based UI for collection and browsing
- 🤖 **Cloud LLM** - Google ADK with Gemini models for embeddings and chat
- 🔍 **Vector Search** - 768-dimension embeddings for semantic search
- ⚡ **Fast Collection** - Efficient scraping and storage
- ⏰ **Scheduled Jobs** - Automated crawling and document summarization

### Technology Stack

- **Language:** Go 1.25+
- **Storage:** BadgerDB (embedded key-value store)
- **Web UI:** HTML templates, Alpine.js, Bulma CSS, WebSockets
- **LLM:** Google ADK with Gemini API (cloud-based embeddings and chat)
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
│  internal/storage/badger/               │  Data persistence
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

### Job System Architecture

**For comprehensive documentation of the job system, see [Manager/Worker Architecture](docs/architecture/MANAGER_WORKER_ARCHITECTURE.md).**

Quaero uses a Manager/Worker pattern for job orchestration and execution:
- **Managers** create parent jobs and define workflows
- **Workers** execute individual jobs from the queue
- **Monitors** monitor parent job progress and aggregate child statistics

The queue-based architecture uses a Badger-backed message queue for distributed job processing:

#### Job Naming Conventions (V2.0 - IMPLEMENTED)

**CRITICAL: Three distinct job domains with clear naming:**

1. **Jobs Domain** - Job definitions (user-defined workflows)
   - Type: `Job` or `JobDefinition`
   - Prefix: `Job`
   - Purpose: User-defined workflows in `job-definitions/` directory

2. **Queue Domain** - Queued work (immutable)
   - Type: `QueueJob`
   - Prefix: `Queue`
   - Purpose: Immutable job sent to message queue
   - Constructors: `NewQueueJob()`, `NewQueueJobChild()`
   - Deserialization: `QueueJobFromJSON()`

3. **Queue State Domain** - Runtime information (in-memory)
   - Type: `QueueJobState`
   - Prefix: `QueueJobState`
   - Purpose: In-memory runtime execution state
   - Constructors: `NewQueueJobState()`
   - Conversion: `QueueJobState.ToQueueJob()`

**Storage Architecture:**
- **BadgerDB stores:** `QueueJob` (immutable) ONLY
- **Runtime state:** `QueueJobState` (in-memory, NOT stored)
- **Conversion pattern:**
  ```go
  // Storage → In-Memory
  queueJob := storage.GetJob(id)
  jobState := models.NewQueueJobState(queueJob)

  // In-Memory → Storage
  queueJob := jobState.ToQueueJob()
  storage.SaveJob(queueJob)
  ```

**Migration completed:** See `docs/features/refactor-job-queues/MIGRATION_COMPLETE_SUMMARY.md` for details.

#### Directory Structure (Migration Complete - ARCH-009)

Quaero uses a Manager/Worker/Monitor architecture for job orchestration and execution:

**Directories:**
- `internal/jobs/manager/` - Job managers (orchestration layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `crawler_manager.go` (ARCH-004)
  - ✅ `database_maintenance_manager.go` (ARCH-004)
  - ✅ `agent_manager.go` (ARCH-004)
  - ✅ `transform_manager.go` (ARCH-009)
  - ✅ `reindex_manager.go` (ARCH-009)
  - ✅ `places_search_manager.go` (ARCH-009)
- `internal/jobs/worker/` - Job workers (execution layer)
  - ✅ `interfaces.go` (ARCH-003)
  - ✅ `crawler_worker.go` (ARCH-005) - Merged from crawler_executor.go + crawler_executor_auth.go
  - ✅ `agent_worker.go` (ARCH-006)
  - ✅ `job_processor.go` (ARCH-006) - Routes jobs to workers
  - ✅ `database_maintenance_worker.go` (ARCH-008)
- `internal/jobs/monitor/` - Parent job monitor (monitoring layer) with `interfaces.go`
- `internal/jobs/` root - Job definition orchestrator
  - ✅ `job_definition_orchestrator.go` (ARCH-009) - Routes job definition steps to managers

**Migration Progress:**
- Phase ARCH-003: ✅ Directory structure created
- Phase ARCH-004: ✅ 3 managers migrated (crawler, database_maintenance, agent)
- Phase ARCH-005: ✅ Crawler worker migrated (merged crawler_executor.go + crawler_executor_auth.go)
- Phase ARCH-006: ✅ Remaining worker files migrated (agent_worker.go, job_processor.go)
- Phase ARCH-008: ✅ Database maintenance worker migrated
- Phase ARCH-009: ✅ Final cleanup complete - 3 remaining managers migrated, executor/ directory removed (COMPLETE)

See [Manager/Worker Architecture](docs/architecture/MANAGER_WORKER_ARCHITECTURE.md) for complete details.

#### Interfaces

**Architecture (ARCH-003+, Completed ARCH-009, Consolidated in refactor-job-interfaces):**
- `StepManager` interface - `internal/interfaces/job_interfaces.go` (centralized)
  - Implementations (6 total):
    - `CrawlerManager` (ARCH-004)
    - `DatabaseMaintenanceManager` (ARCH-004)
    - `AgentManager` (ARCH-004)
    - `TransformManager` (ARCH-009)
    - `ReindexManager` (ARCH-009)
    - `PlacesSearchManager` (ARCH-009)
- `JobWorker` interface - `internal/interfaces/job_interfaces.go` (centralized)
  - Implementations (3 total):
    - `CrawlerWorker` (ARCH-005)
    - `AgentWorker` (ARCH-006)
    - `DatabaseMaintenanceWorker` (ARCH-008)
- `JobMonitor` interface - `internal/interfaces/job_interfaces.go` (centralized)
  - Implementation: `JobMonitor` (monitors parent job progress)
- `JobSpawner` interface - `internal/interfaces/job_interfaces.go` (centralized)
  - Supports workers that spawn child jobs during execution
- `JobDefinitionOrchestrator` - `internal/jobs/job_definition_orchestrator.go` (ARCH-009)
  - Routes job definition steps to registered managers

**Core Components:**
- `JobProcessor` - `internal/jobs/worker/job_processor.go` (ARCH-006)
  - Routes jobs from queue to registered workers
  - Manages worker pool lifecycle (Start/Stop)

**Core Components:**

1. **QueueManager** (`internal/queue/badger_manager.go`)
   - Manages Badger-backed job queue
   - Lifecycle management (Start/Stop/Restart)
   - Message operations: Enqueue, Receive, Extend, Close
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
2. Parent job message created and enqueued to Badger queue
   ↓
3. JobProcessor receives message from queue
   ↓
4. Worker routes message to appropriate handler (CrawlerWorker, AgentWorker, etc.)
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

- **Persistent Queue:** Badger-backed durable message storage
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

- **Job Definition Orchestration** (`internal/jobs/job_definition_orchestrator.go`):
  - Orchestrates multi-step workflows defined by users (JobDefinitions)
  - Executes steps sequentially with retry logic and error handling
  - Uses managers (e.g., CrawlerManager) to create parent jobs
  - Polls jobs asynchronously when wait_for_completion is enabled
  - Publishes progress events for UI updates
  - Supports error strategies: fail, continue, retry

- **Queue-Based Execution** (`internal/jobs/worker/`):
  - Workers handle individual task execution (CrawlerWorker, AgentWorker, etc.)
  - Process URLs, generate summaries, extract keywords
  - Monitors monitor parent job progress
  - Persistent queue with worker pool
  - Enable job spawning and depth tracking

**Both systems coexist and complement each other:**
- JobDefinitions can trigger jobs via action steps (e.g., "crawl", "agent")
- Managers create parent jobs that spawn child jobs into the queue
- Workers execute individual jobs pulled from the queue
- Monitors monitor parent job progress until all children complete

### Service Initialization Flow

The app initialization sequence in `internal/app/app.go` is critical:

1. **Storage Layer** - BadgerDB
2. **LLM Service** - Google ADK-based embeddings and chat (cloud mode)
3. **Embedding Service** - Uses LLM service
4. **Document Service** - Uses embedding service
5. **Chat Service** - RAG-enabled chat with LLM
6. **Event Service** - Pub/sub for system events
7. **Auth Service** - Generic web authentication
8. **Crawler Service** - ChromeDP-based web crawler
9. **Processing Service** - Document processing
10. **Embedding Coordinator** - Auto-subscribes to embedding events
11. **Scheduler Service** - Triggers events on cron (every 5 minutes)
12. **Agent Service** - Google ADK with Gemini models (optional, requires API key)
13. **Handlers** - HTTP/WebSocket handlers

**Important:** Services that subscribe to events must be initialized after the EventService but before any events are published. Agent Service is initialized after Scheduler Service and registers workers with JobProcessor and managers with job definition orchestration.

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

The LLM service provides embeddings and chat using Google ADK (Agent Development Kit) with Gemini models.

**Implementation:** `internal/services/llm/gemini_service.go` - Google ADK integration

**Embedding Model:** `gemini-embedding-001` with 768-dimension output (matches database schema)

**Chat Model:** `gemini-3-pro-preview` (high-quality, same as agent service)

**No Offline Mode:** Requires Google Gemini API key - no local inference or mock mode available

**Graceful Degradation:** If API key is missing, LLM service initialization fails with warning but application continues without chat/embedding features

**Configuration example:**
```toml
[llm]
google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"  # Required
embed_model_name = "gemini-embedding-001"      # Default
chat_model_name = "gemini-3-pro-preview"           # Default
timeout = "5m"                                  # Operation timeout
embed_dimension = 768                           # Must match storage config
```

**Environment variable overrides:**
- `QUAERO_LLM_GOOGLE_API_KEY` - API key
- `QUAERO_LLM_EMBED_MODEL_NAME` - Embedding model
- `QUAERO_LLM_CHAT_MODEL_NAME` - Chat model
- `QUAERO_LLM_TIMEOUT` - Timeout duration

**API key setup:** Get API key from: https://aistudio.google.com/app/apikey

**Note:** Free tier available with rate limits (15 requests/minute, 1500/day as of 2024)

### Storage Schema

**Documents:**
- Central unified storage for all source types
- Fields: id, source_id, source_type, title, content, embedding, embedding_model, last_synced, created_at, updated_at
- Full-text search index (title + content)
- Force sync flags: force_sync_pending, force_embed_pending

**Auth:**
- `auth_credentials` - Generic web authentication tokens and cookies

**Jobs:**
- `crawl_jobs` - Persistent job state and progress tracking
- `job_logs` - Unlimited job log history
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

### Agent Framework Architecture

**Agent framework provides AI-powered document processing using Google ADK (Agent Development Kit) with Gemini models.**

The agent framework enables intelligent document analysis and enrichment through AI agents that process existing documents in the database. Current capabilities include keyword extraction, with future support planned for summarization, classification, and entity extraction.

**Key Integration Points:**
- Queue-based job execution via `JobProcessor` and `AgentWorker`
- Document metadata storage in `documents` table
- Event publishing for workflow coordination
- Google ADK with Gemini API (no offline fallback)

**Core Components:**

**AgentService** (`internal/services/agents/service.go`):
- Manages Google ADK model lifecycle with `gemini.NewModel()`
- Registers agent processors in internal registry
- Routes execution requests by agent type
- Health check validates API key and model initialization
- Constructor: `NewService(config *common.AgentConfig, logger arbor.ILogger)`
- Returns `nil` if `GoogleAPIKey` is empty (graceful degradation)

**AgentProcessor Interface** (internal to service):
- `Execute(ctx context.Context, model model.LLM, input map[string]interface{}) (map[string]interface{}, error)`
- `GetType() string` - Returns agent type identifier
- Implemented by: `KeywordExtractor`, future agent types
- Agents receive pre-initialized ADK model, input parameters, and return structured results

**AgentWorker** (`internal/jobs/worker/agent_worker.go`):
- Queue-based job worker for individual agent jobs
- Job type: `"agent"`
- Workflow: Load document → Execute agent → Update metadata → Publish event
- Real-time logging via `publishAgentJobLog()`
- Handles document querying, agent execution, and metadata persistence

**AgentManager** (`internal/jobs/manager/agent_manager.go`):
- Job definition manager for orchestrating agent workflows
- Step action: `"agent"`
- Creates agent jobs for documents matching filter
- Supports agent chaining via sequential steps
- Validates agent type and document filter configuration

**Agent Execution Flow:**

```
User triggers job → JobDefinition → Job Orchestration → AgentManager
  ↓
Query documents (document_filter) → Create agent jobs → Enqueue to queue
  ↓
Queue → JobProcessor → AgentWorker
  ↓
Load document → AgentService.Execute() → KeywordExtractor
  ↓
ADK llmagent.New() → Gemini API → Parse response
  ↓
Update document.Metadata[agent_type] → Publish event
```

**Step-by-Step:**
1. User triggers agent job via UI or API (`POST /api/job-definitions/{id}/execute`)
2. Job orchestration reads job definition and executes agent steps sequentially
3. AgentManager queries documents matching `document_filter` criteria
4. For each document, creates individual agent job and enqueues to message queue
5. JobProcessor receives agent job from queue
6. AgentWorker loads document and calls `AgentService.Execute()`
7. AgentService routes to appropriate agent (e.g., `KeywordExtractor`)
8. Agent uses ADK's `llmagent.New()` to create agent with instructions
9. Agent runner sends request to Gemini API and processes response stream
10. Structured results stored in `document.Metadata[agentType]`
11. Event published for workflow coordination

**Note:** Both queue-based execution (via `AgentWorker`) and job definition orchestration (via `AgentManager`) are supported.

**Google ADK Integration:**

**Model Initialization:**
- Uses `gemini.NewModel(ctx, modelName, clientConfig)` from `google.golang.org/adk/model/gemini`
- Client config: `APIKey`, `Backend: genai.BackendGeminiAPI`
- Default model: `gemini-3-pro-preview` (high-quality)
- Model shared across all agents for efficiency
- Initialization happens once at service startup

**Agent Loop Pattern:**
- Uses `llmagent.New(config)` from `google.golang.org/adk/agent/llmagent`
- Config includes: `Model`, `Name`, `Instruction`, `GenerateContentConfig`
- Execution via `runner.New(config)` and `runner.Run(ctx, ...)`
- Event stream processing with `IsFinalResponse()` check
- Response parsing handles both simple arrays and structured objects

**No Offline Fallback:**
- Service initialization fails if `GoogleAPIKey` is empty
- Error message: "Google API key is required for agent service"
- Agent features unavailable if service initialization fails
- Graceful degradation: Service logs warning, application continues without agents
- Both LLM service and agent service require Google API keys - no offline fallback for either

**Agent Types:**

**Keyword Extractor** (`internal/services/agents/keyword_extractor.go`):
- Type identifier: `"keyword_extractor"`
- Input: `document_id`, `content`, `max_keywords` (5-15 range, default: 10)
- Output: `keywords` array, `confidence` map (optional)
- Metadata storage: `document.Metadata["keyword_extractor"]`
- Prompt engineering: Instructs model to extract domain-specific terms, avoid stop words, prioritize relevance
- Response parsing: Supports both simple array `["keyword1", "keyword2"]` and object with confidence scores
- Example output:
  ```json
  {
    "keywords": ["machine learning", "neural networks", "deep learning"],
    "confidence": {"machine learning": 0.95, "neural networks": 0.87, "deep learning": 0.82}
  }
  ```

**Future Agent Types:**
- **Summarizer**: Generate concise document summaries with configurable length
- **Classifier**: Categorize documents by topic/domain with confidence scores
- **Entity Extractor**: Extract named entities (people, places, organizations)
- All future agents will follow same `AgentProcessor` interface pattern (internal to service)
- Registration happens in `AgentService.NewService()` function

**Agent Chaining:**

**How It Works:**
- Multiple agent steps in job definition execute sequentially
- Each agent stores results in `document.Metadata[agentType]`
- Next agent can access previous results via metadata
- Example workflow: Keyword extractor → Summarizer (uses keywords for context)
- Document filter ensures same documents processed by all chained agents

**Configuration Pattern:**
```toml
[[steps]]
name = "extract_keywords"
action = "agent"
[steps.config]
agent_type = "keyword_extractor"
[steps.config.document_filter]
source_type = "crawler"
max_keywords = 10

[[steps]]
name = "generate_summary"
action = "agent"
[steps.config]
agent_type = "summarizer"
[steps.config.document_filter]
source_type = "crawler"
use_keywords = true  # Access metadata["keyword_extractor"]["keywords"]
```

**Best Practices:**
- Use same `document_filter` for chained steps to ensure consistency
- Order steps by dependency (keywords before summarization)
- Monitor job logs for each step's completion
- Test each agent individually before chaining
- Consider timeout implications for long chains

**Configuration:**

**Agent Config Section** (`quaero.toml`):
```toml
[agent]
google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"  # Required
model_name = "gemini-3-pro-preview"                # Default
max_turns = 10                                  # Agent conversation turns
timeout = "5m"                                  # Execution timeout
```

**Environment Variables:**
- `QUAERO_AGENT_GOOGLE_API_KEY` - Overrides config file
- `QUAERO_AGENT_MODEL_NAME` - Overrides model name
- `QUAERO_AGENT_MAX_TURNS` - Overrides max turns
- `QUAERO_AGENT_TIMEOUT` - Overrides timeout

**API Key Setup:**
- Get API key from: https://aistudio.google.com/app/apikey
- Free tier available with rate limits (15 requests/minute, 1500/day as of 2024)
- Store in config file or environment variable (environment takes precedence)
- API key required for agent service to initialize

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

**Note:** Only cloud mode (Google ADK) is currently supported.

To change embedding/chat behavior:
1. Modify implementation in `internal/services/llm/gemini_service.go`
2. Ensure interface compliance with `internal/interfaces/llm_service.go`
3. Update tests in `test/unit/`
4. Behavior is controlled via `[llm]` config and `QUAERO_LLM_*` environment variables

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

**Current Architecture:**
- All data stored locally in Badger
- Storage, crawling, and search operations are local
- LLM features require Google ADK (cloud) and send data to Google's API
- Audit logging for compliance

**Important:** LLM processing (embeddings and chat) requires Google Gemini API and sends data to Google's servers. This is not suitable for highly sensitive or classified data. All other operations (crawling, storage, search) remain local to your machine.

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
1. Port availability:
   - Windows: `netstat -an | findstr :8085`
   - Linux/macOS: `lsof -i :8085` or `netstat -an | grep 8085`
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
1. LLM service initialized with Google ADK
2. Valid Google API key configured
3. Scheduler is running (logs every 5 minutes)
4. Documents have `force_embed_pending=true` flag
5. Embedding coordinator started successfully

### Agent Service Issues

**Agent Service Not Initialized:**
- **Symptom**: Log message "Failed to initialize agent service - agent features will be unavailable"
- **Cause**: Missing or invalid Google API key
- **Solution**:
  - Check `quaero.toml` has `[agent]` section with `google_api_key` set
  - Or set environment variable: `QUAERO_AGENT_GOOGLE_API_KEY=your_key_here`
  - Get API key from: https://aistudio.google.com/app/apikey
  - Verify API key is valid (not expired, not revoked)
- **Verification**: Look for log message "Agent service initialized with Google ADK"

**Agent Jobs Fail with "Unknown Agent Type":**
- **Symptom**: Job fails with error "unknown agent type: {type}"
- **Cause**: Agent type not registered or typo in job definition
- **Solution**:
  - Check job definition `agent_type` matches registered agent (e.g., `"keyword_extractor"`)
  - Verify agent is registered in `service.go` `NewService()` function
  - Check service logs for "Agent registered" messages at startup
- **Available Types**: `keyword_extractor` (more coming soon)

**Agent Execution Timeout:**
- **Symptom**: Job fails with "context deadline exceeded" error
- **Cause**: Agent execution exceeds configured timeout (default: 5m)
- **Solution**:
  - Increase timeout in `quaero.toml`: `[agent] timeout = "10m"`
  - Or set environment variable: `QUAERO_AGENT_TIMEOUT=10m`
  - Check document size (large documents take longer to process)
  - Monitor Gemini API rate limits (may cause delays)
- **Note**: Timeout applies per agent execution, not per job

**Keywords Not Appearing in Document Metadata:**
- **Symptom**: Agent job completes but `metadata["keyword_extractor"]` is empty
- **Cause**: Document not updated or agent returned no keywords
- **Solution**:
  - Check job logs for "Document metadata updated successfully" message
  - Verify document has sufficient content (minimum ~100 words recommended)
  - Check agent response in logs for malformed JSON
  - Query document via `GET /api/documents/{id}` to verify metadata
- **Metadata Structure**:
  ```json
  {
    "keyword_extractor": {
      "keywords": ["keyword1", "keyword2", ...],
      "confidence": {"keyword1": 0.95, ...}
    }
  }
  ```

**Gemini API Rate Limit Errors:**
- **Symptom**: Job fails with "429 Too Many Requests" or "quota exceeded" error
- **Cause**: Exceeded Gemini API free tier rate limits
- **Solution**:
  - Reduce job concurrency in `quaero.toml`: `[queue] concurrency = 2`
  - Add delays between agent jobs (not currently supported, future feature)
  - Upgrade to paid Gemini API tier for higher limits
  - Monitor API usage at: https://aistudio.google.com/app/apikey
- **Free Tier Limits**: 15 requests per minute, 1500 requests per day (as of 2024)

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