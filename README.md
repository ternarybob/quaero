# Quaero

**Quaero** (Latin: "I seek, I search") - A local knowledge collection and search system.

## Overview

Enterprise knowledge is locked behind authenticated web applications (Confluence, Jira, documentation sites) where traditional RAG tools cannot access or safely store sensitive data. Quaero solves this by running entirely locally, capturing your authenticated browser sessions via a Chrome extension, and crawling pages to normalize them into searchable markdown with metadata.

All data is stored in a local Badger database. LLM-powered features (summarization, chat, keyword extraction) use Google Gemini via API.

## User Operations

### What You Can Do

| Operation | Location | Description |
|-----------|----------|-------------|
| **Capture Authentication** | Chrome Extension | Log into any website, click extension to capture session cookies |
| **Create Crawl Jobs** | `/jobs` → New Job | Define seed URLs, crawl depth, include/exclude patterns |
| **Monitor Jobs** | `/queue` | Real-time job progress, logs, cancel/retry controls |
| **Search Documents** | `/search` | Google-style queries with operators, semantic search |
| **Chat with AI** | `/chat` | Natural language questions with RAG-powered answers |
| **Browse Documents** | `/documents` | View collected content, metadata, summaries |
| **Configure System** | `/settings` | API keys, schedules, storage management |

### Typical Workflow

```
1. Install Chrome extension → Log into Confluence/Jira/docs site
2. Click extension → Cookies sent to Quaero server
3. Create job definition → Configure URLs, depth, patterns
4. Execute job → Crawler fetches and converts pages to markdown
5. Search/Chat → Query your private knowledge base
```

### Search Capabilities

**Advanced Query Syntax:**
- `"exact phrase"` - Match exact text
- `AND`, `OR`, `NOT` - Boolean operators
- `title:keyword` - Field-specific search
- `test*` - Wildcard matching
- Regex patterns supported

### API Quick Reference

```http
# Authentication
POST /api/auth              # Update cookies from extension
GET  /api/auth/status       # Check connection status

# Job Definitions
GET  /api/job-definitions   # List all job definitions
POST /api/job-definitions   # Create new job definition
POST /api/job-definitions/{id}/execute  # Run job

# Jobs & Queue
GET  /api/jobs              # List job executions
GET  /api/jobs/{id}/logs    # Get job logs
POST /api/jobs/{id}/cancel  # Cancel running job

# Documents & Search
GET  /api/documents         # List documents
POST /api/search            # Advanced search
POST /api/chat              # Chat with RAG

# System
GET  /api/health            # Health check
WS   /ws                    # Real-time updates
```

## Quick Start

### Prerequisites

- Go 1.25+
- Chrome browser
- Google Gemini API key (for AI features)

### Build & Run

```bash
# Clone
git clone https://github.com/ternarybob/quaero.git
cd quaero

# Build (Linux/macOS)
./scripts/build.sh

# Build (Windows)
.\scripts\build.ps1

# Run
./bin/quaero  # or .\bin\quaero.exe on Windows
```

**Important:** Always use the build scripts. Direct `go build` doesn't handle versioning and assets.

### Configuration

Create `quaero.toml`:

```toml
[server]
host = "localhost"
port = 8080

[storage.badger]
path = "./data"

[gemini]
google_api_key = "YOUR_API_KEY"  # Required for AI features
agent_model = "gemini-3-pro-preview"
chat_model = "gemini-3-pro-preview"
rate_limit = "4s"  # 15 RPM free tier

[search]
mode = "advanced"  # Google-style query parser
```

### Chrome Extension

1. Open `chrome://extensions/`
2. Enable "Developer mode"
3. Click "Load unpacked" → Select `cmd/quaero-chrome-extension/`
4. Navigate to any authenticated site, click extension to capture cookies

## Code Structure

```
quaero/
├── cmd/
│   ├── quaero/                  # Main server
│   ├── quaero-chrome-extension/ # Browser extension
│   └── quaero-mcp/              # MCP server for Claude
│
├── internal/
│   ├── app/                     # DI & application bootstrap
│   ├── server/                  # HTTP routing
│   ├── handlers/                # Request handlers (23 files)
│   │   ├── job_handler.go       # Job CRUD & execution
│   │   ├── document_handler.go  # Document management
│   │   ├── search_handler.go    # Search endpoints
│   │   ├── chat_handler.go      # Chat/RAG interface
│   │   └── websocket.go         # Real-time updates
│   │
│   ├── services/                # Business logic (24 packages)
│   │   ├── crawler/             # Website crawler (chromedp)
│   │   ├── search/              # Query parsing & search
│   │   ├── chat/                # RAG chat service
│   │   ├── llm/                 # Gemini integration
│   │   ├── scheduler/           # Cron scheduling
│   │   └── documents/           # Document storage
│   │
│   ├── queue/                   # Job queue (Badger-backed)
│   ├── jobs/                    # Job executor & actions
│   ├── storage/badger/          # Database layer
│   └── models/                  # Data structures
│
├── pages/                       # Web UI templates
│   ├── static/quaero.css        # Spectre theme customization
│   └── static/common.js         # Alpine.js components
│
├── test/                        # API & UI tests
└── scripts/                     # Build scripts
```

### Key Architectural Patterns

**Manager/Worker Pattern:** Job definitions create parent jobs that spawn child jobs to a persistent queue. Workers process jobs with visibility timeouts and automatic retries.

**Markdown + Metadata:** Documents stored as clean markdown (LLM-friendly) with JSON metadata. Enables two-step queries: filter metadata → reason on content.

**Event-Driven UI:** WebSocket broadcasts job events (created, started, completed, failed) for real-time UI updates.

## Tooling

### Frontend Stack

| Tool | Purpose |
|------|---------|
| **Spectre CSS** | Lightweight CSS framework for clean, responsive UI |
| **Alpine.js** | Reactive JavaScript without build steps |
| **Font Awesome** | Icons |
| **Marked.js** | Client-side markdown rendering |
| **Highlight.js** | Code syntax highlighting |

The UI uses Spectre CSS for styling with custom theming in `pages/static/quaero.css`. Alpine.js handles interactivity through components like `websocketManager`, `jobMonitor`, and `settingsAccordion` defined in `common.js`.

### Backend Stack

| Tool | Purpose |
|------|---------|
| **BadgerDB** | Embedded key-value database |
| **chromedp** | Headless Chrome for JS rendering |
| **gorilla/websocket** | Real-time communication |
| **robfig/cron** | Job scheduling |
| **google.golang.org/genai** | Gemini API client |
| **html-to-markdown** | HTML conversion |
| **ternarybob/arbor** | Structured logging |

### MCP Server

Quaero includes an MCP (Model Context Protocol) server for Claude Desktop integration:

```json
{
  "mcpServers": {
    "quaero": {
      "command": "/path/to/quaero-mcp",
      "env": { "QUAERO_CONFIG": "/path/to/quaero.toml" }
    }
  }
}
```

**Available Tools:**
- `search_documents` - Full-text search with filters
- `get_document` - Retrieve document by ID
- `list_recent_documents` - Recently updated docs
- `get_related_documents` - Find related content

## Security & Privacy

- **Local Storage:** All crawled content stored in local Badger database
- **No Cloud Sync:** Document content never leaves your machine
- **API Key Required:** Gemini API key needed for AI features (data sent to Google for processing)
- **Cookie Security:** Session cookies transmitted only to localhost

**Note:** If you require 100% local processing, Quaero's current AI features are not suitable as they use Google's cloud APIs.

## Testing

```bash
# Run all tests
cd test && go test ./...

# API tests only
go test -v ./api

# UI tests (requires Chrome)
go test -v ./ui

# Unit tests
go test ./internal/...
```

## Documentation

- [Architecture](docs/architecture/) - System design and patterns
- [Manager/Worker Architecture](docs/architecture/MANAGER_WORKER_ARCHITECTURE.md) - Job system details
- [AGENTS.md](AGENTS.md) - AI agent development guidelines

## Status

**Working:** Website crawler, cookie auth, job queue, document storage, advanced search, chat/RAG, scheduled jobs, real-time updates

**In Progress:** Image extraction, vector embeddings optimization

**Planned:** Multi-user auth, GitHub/GitLab integrations, Slack/Teams connectors

## License

MIT

---

**Quaero: I seek knowledge.**
