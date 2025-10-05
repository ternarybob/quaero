# Quaero Architecture

**Version:** 2.0
**Last Updated:** 2025-10-05
**Status:** Active Development

---

## Overview

Quaero is a knowledge collection and search system that gathers documentation from Atlassian (Confluence, Jira) using browser extension authentication and provides a web-based interface for accessing the data.

### Current Implementation Status

**✅ Implemented:**
- Web-based UI with real-time updates
- SQLite storage with full-text search
- Chrome extension authentication
- Jira & Confluence collectors
- WebSocket for live log streaming
- RESTful API endpoints

**🚧 In Progress:**
- Vector embeddings for semantic search
- RAG pipeline integration
- Natural language query interface

**📋 Planned:**
- GitHub collector
- Additional data sources (Slack, Linear)
- Multi-user support

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│  Browser (Chrome)                                           │
│  ┌────────────────────────────────────────────────┐         │
│  │  Quaero Chrome Extension                       │         │
│  │  • Captures Atlassian auth (cookies, tokens)   │         │
│  │  • Connects via WebSocket                      │         │
│  │  • Sends auth data to server                   │         │
│  └──────────────────┬─────────────────────────────┘         │
└────────────────────┼──────────────────────────────────────────┘
                     │ WebSocket: ws://localhost:8080/ws
                     │
                     ↓
┌─────────────────────────────────────────────────────────────┐
│  Quaero Server (Go HTTP/WebSocket)                          │
│                                                              │
│  ┌─────────────────────────────────────────────────┐        │
│  │  HTTP Server (internal/server/)                 │        │
│  │  • Routes (routes.go)                           │        │
│  │  • Middleware (middleware.go)                   │        │
│  │  • Graceful shutdown                            │        │
│  └──────────────────┬──────────────────────────────┘        │
│                     │                                        │
│  ┌──────────────────▼──────────────────────────────┐        │
│  │  Handlers (internal/handlers/)                  │        │
│  │  • WebSocketHandler - Real-time comms           │        │
│  │  • UIHandler - Serves web pages                 │        │
│  │  • CollectorHandler - Collection triggers       │        │
│  │  • DataHandler - API endpoints                  │        │
│  │  • ScraperHandler - Scraping operations         │        │
│  └──────────────────┬──────────────────────────────┘        │
│                     │                                        │
│  ┌──────────────────▼──────────────────────────────┐        │
│  │  Services (internal/services/atlassian/)        │        │
│  │  • AtlassianAuthService - Auth management       │        │
│  │  • JiraScraperService - Jira collection         │        │
│  │  • ConfluenceScraperService - Confluence        │        │
│  └──────────────────┬──────────────────────────────┘        │
│                     │                                        │
│  ┌──────────────────▼──────────────────────────────┐        │
│  │  Storage Manager (internal/storage/sqlite/)     │        │
│  │  • SQLite database                              │        │
│  │  • Full-text search (FTS5)                      │        │
│  │  • Migrations                                   │        │
│  │  • JiraStorage, ConfluenceStorage, AuthStorage  │        │
│  └─────────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────────┘
                     │
                     ↓
┌─────────────────────────────────────────────────────────────┐
│  SQLite Database (./quaero.db)                              │
│  • jira_projects, jira_issues                               │
│  • confluence_spaces, confluence_pages                      │
│  • auth_credentials                                         │
│  • Full-text search indexes                                 │
└─────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

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
│       ├── sidepanel.js             # Side panel interface
│       └── content.js               # Page content interaction
│
├── internal/
│   ├── common/                      # Stateless utilities
│   │   ├── config.go                # Configuration loading (TOML)
│   │   ├── logger.go                # Logger initialization (arbor)
│   │   ├── banner.go                # Startup banner (ternarybob/banner)
│   │   └── version.go               # Version management
│   │
│   ├── app/                         # Application orchestration
│   │   └── app.go                   # App initialization & manual dependency wiring
│   │
│   ├── services/                    # Stateful services (receiver methods)
│   │   └── atlassian/               # Jira & Confluence
│   │       ├── auth_service.go      # Authentication management
│   │       ├── jira_scraper_service.go
│   │       ├── jira_projects.go
│   │       ├── jira_issues.go
│   │       ├── jira_data.go
│   │       ├── confluence_scraper_service.go
│   │       ├── confluence_spaces.go
│   │       ├── confluence_pages.go
│   │       └── confluence_data.go
│   │
│   ├── handlers/                    # HTTP handlers (constructor injection)
│   │   ├── websocket.go             # WebSocket handler
│   │   ├── ui.go                    # Web UI handler
│   │   ├── collector.go             # Collection endpoints
│   │   ├── scraper.go               # Scraping endpoints
│   │   ├── data.go                  # Data API endpoints
│   │   └── api.go                   # General API
│   │
│   ├── storage/                     # Storage layer
│   │   ├── factory.go               # Storage factory
│   │   └── sqlite/                  # SQLite implementation
│   │       ├── manager.go           # Storage manager
│   │       ├── connection.go        # DB connection
│   │       ├── migrations.go        # Schema migrations
│   │       ├── jira_storage.go      # Jira persistence
│   │       ├── confluence_storage.go # Confluence persistence
│   │       └── auth_storage.go      # Auth persistence
│   │
│   ├── server/                      # HTTP server
│   │   ├── server.go                # Server implementation
│   │   ├── routes.go                # Route definitions
│   │   └── middleware.go            # Middleware
│   │
│   ├── interfaces/                  # Service interfaces
│   │   ├── storage.go               # Storage interfaces
│   │   └── atlassian.go             # Atlassian interfaces
│   │
│   └── models/                      # Data models
│       └── atlassian.go             # Atlassian data structures
│
├── pages/                           # Web UI (NOT CLI)
│   ├── index.html                   # Main dashboard
│   ├── confluence.html              # Confluence UI
│   ├── jira.html                    # Jira UI
│   ├── partials/                    # Reusable components
│   │   ├── navbar.html
│   │   └── service-logs.html
│   └── static/                      # Static assets
│       ├── common.css
│       └── partial-loader.js
│
├── test/                            # Testing
│   ├── integration/                 # Integration tests
│   ├── ui/                          # UI tests
│   ├── run-tests.ps1                # Test runner script
│   └── README.md
│
├── scripts/                         # Build scripts
│   └── build.ps1                    # Build script
│
└── docs/                            # Documentation
    ├── architecture.md              # This file
    ├── requirements.md              # Requirements doc
    └── remaining-requirements.md    # Remaining work
```

---

## Core Components

### 1. Startup Sequence (cmd/quaero/main.go)

**Required Order:**
1. Configuration loading (`common.LoadFromFile`)
2. CLI flag overrides (`common.ApplyCLIOverrides`)
3. Logger initialization (`common.InitLogger`)
4. Banner display (`common.PrintBanner`) - MANDATORY
5. Application initialization (`app.New`)
6. Server start

### 2. Configuration System

**Priority Order:**
1. CLI flags (highest)
2. Environment variables
3. Config file (`quaero.toml`)
4. Defaults (lowest)

**Required Libraries:**
- `github.com/pelletier/go-toml/v2` - TOML configuration

### 3. Logging System

**Required Library:**
- `github.com/ternarybob/arbor` - Structured logging

**Forbidden:**
- `fmt.Println`
- `log.Println`
- Any other logging library

### 4. Banner System

**Required Library:**
- `github.com/ternarybob/banner` - Startup banner

**Must Display:**
- Application name
- Version
- Server host and port
- Configuration source

### 5. Storage Layer

**Implementation:** SQLite with FTS5

**Components:**
- `StorageManager` interface
- `JiraStorage` interface
- `ConfluenceStorage` interface
- `AuthStorage` interface

**Features:**
- Full-text search (FTS5)
- Schema migrations
- Transaction support

### 6. Services (internal/services/)

**Pattern:** Stateful services with receiver methods

**AtlassianAuthService:**
- Receives auth from extension
- Stores credentials in database
- Provides auth to collectors

**JiraScraperService:**
- Fetches projects
- Fetches issues
- Stores in SQLite

**ConfluenceScraperService:**
- Fetches spaces
- Fetches pages
- Stores in SQLite

### 7. Handlers (internal/handlers/)

**Pattern:** Constructor-based dependency injection with interfaces

**WebSocketHandler:**
- Real-time communication
- Log streaming
- Status updates
- Auth reception from extension

**UIHandler:**
- Serves HTML pages
- Template rendering
- Static file serving

**CollectorHandler:**
- Trigger collections
- Collection status
- Progress monitoring

**DataHandler:**
- API endpoints for data
- CRUD operations

### 8. Web UI (pages/)

**Architecture:** Server-side rendered HTML with JavaScript

**Pages:**
- `index.html` - Dashboard
- `confluence.html` - Confluence UI
- `jira.html` - Jira UI

**Features:**
- Real-time log streaming (WebSocket)
- Collection triggering
- Status monitoring
- Data browsing

### 9. Chrome Extension

**Location:** `cmd/quaero-chrome-extension/`

**Purpose:** Capture Atlassian authentication

**Flow:**
1. User navigates to Atlassian
2. Extension captures cookies/tokens
3. Extension connects to `ws://localhost:8080/ws`
4. Extension sends auth data
5. Server stores and uses for collection

---

## Authentication Flow

```
1. User logs into Atlassian (handles 2FA, SSO automatically)
   ↓
2. Extension extracts auth state:
   • Cookies (.atlassian.net domain)
   • Local storage tokens
   • Session tokens
   • Cloud ID, ATL tokens
   ↓
3. Extension connects to WebSocket:
   ws://localhost:8080/ws
   ↓
4. Extension sends AuthData message:
   {
     "type": "auth",
     "payload": {
       "cookies": [...],
       "tokens": {...},
       "baseUrl": "https://company.atlassian.net"
     }
   }
   ↓
5. Server stores in auth_credentials table
   ↓
6. Collectors use stored auth for API calls
   ↓
7. Extension refreshes auth periodically
```

---

## Data Collection Flow

```
1. User triggers collection via Web UI
   ↓
2. CollectorHandler receives request
   ↓
3. Service (Jira/Confluence) loads auth from database
   ↓
4. Service makes API calls with stored credentials
   ↓
5. Service processes responses
   ↓
6. Service stores in SQLite
   ↓
7. Service sends progress via WebSocket
   ↓
8. UI updates in real-time
```

---

## WebSocket Protocol

**Endpoint:** `ws://localhost:8080/ws`

### Client → Server Messages

**Auth Data:**
```json
{
  "type": "auth",
  "payload": {
    "cookies": ["session=abc123"],
    "localStorage": {"key": "value"},
    "cloudId": "...",
    "baseUrl": "https://company.atlassian.net"
  }
}
```

### Server → Client Messages

**Log Stream:**
```json
{
  "type": "log",
  "payload": {
    "timestamp": "15:04:05",
    "level": "info",
    "message": "Collection started",
    "service": "confluence"
  }
}
```

**Status Update:**
```json
{
  "type": "status",
  "payload": {
    "service": "confluence",
    "status": "running",
    "progress": 42,
    "total": 100
  }
}
```

---

## Clean Architecture Patterns

### internal/common/ - Stateless Utilities

**Rules:**
- ✅ Pure functions only
- ✅ No state
- ❌ No receiver methods

**Example:**
```go
// ✅ CORRECT
func LoadFromFile(path string) (*Config, error) {
    // Pure function
}

// ❌ WRONG
func (c *Config) Load() error {
    // Belongs in services/
}
```

### internal/services/ - Stateful Services

**Rules:**
- ✅ Receiver methods required
- ✅ State management
- ✅ Implement interfaces

**Example:**
```go
// ✅ CORRECT
type JiraScraperService struct {
    storage interfaces.JiraStorage
    auth    *AtlassianAuthService
    logger  arbor.ILogger
}

func (j *JiraScraperService) FetchProjects() error {
    j.logger.Info().Msg("Fetching projects")
    // Use j.storage, j.auth
}
```

### internal/handlers/ - HTTP Handlers

**Rules:**
- ✅ Constructor-based dependency injection
- ✅ Interface-based (where applicable)
- ✅ Thin layer - delegates to services

**Example:**
```go
// ✅ CORRECT - Constructor injection
type CollectorHandler struct {
    jira       *atlassian.JiraScraperService
    confluence *atlassian.ConfluenceScraperService
    logger     arbor.ILogger
}

func NewCollectorHandler(
    jira *atlassian.JiraScraperService,
    confluence *atlassian.ConfluenceScraperService,
    logger arbor.ILogger,
) *CollectorHandler {
    return &CollectorHandler{
        jira:       jira,
        confluence: confluence,
        logger:     logger,
    }
}

func (h *CollectorHandler) HandleCollect(w http.ResponseWriter, r *http.Request) {
    // Delegate to services
    err := h.jira.Collect()
    // Handle response
}
```

---

## API Endpoints

### HTTP Endpoints

```
GET  /                     - Dashboard UI
GET  /confluence           - Confluence UI
GET  /jira                 - Jira UI

POST /api/collect/jira     - Trigger Jira collection
POST /api/collect/confluence - Trigger Confluence collection

GET  /api/data/jira/projects - Get Jira projects
GET  /api/data/jira/issues   - Get Jira issues
GET  /api/data/confluence/spaces - Get Confluence spaces
GET  /api/data/confluence/pages  - Get Confluence pages

GET  /health               - Health check
```

### WebSocket Endpoint

```
WS   /ws                   - WebSocket connection
```

---

## Technology Stack

**Language:** Go 1.25+

**Libraries:**
- `github.com/ternarybob/arbor` - Logging (REQUIRED)
- `github.com/ternarybob/banner` - Banners (REQUIRED)
- `github.com/pelletier/go-toml/v2` - TOML config (REQUIRED)
- `github.com/spf13/cobra` - CLI framework
- `github.com/gorilla/websocket` - WebSocket
- `modernc.org/sqlite` - SQLite driver

**Storage:** SQLite with FTS5

**Frontend:** Vanilla HTML/CSS/JavaScript

**Browser:** Chrome Extension (Manifest V3)

---

## Testing

**Structure:**
```
test/
├── integration/           # Integration tests
│   ├── auth_test.go
│   ├── jira_test.go
│   └── confluence_test.go
├── ui/                   # UI tests
└── run-tests.ps1         # Test runner
```

**Commands:**
```bash
# Run all tests
./test/run-tests.ps1 -Type all

# Unit tests only
./test/run-tests.ps1 -Type unit

# Integration tests only
./test/run-tests.ps1 -Type integration
```

---

## Build & Deployment

**Build Script:** `scripts/build.ps1`

```bash
# Development build
./scripts/build.ps1

# Production build
./scripts/build.ps1 -Release

# Clean build
./scripts/build.ps1 -Clean
```

**Output:** `bin/quaero.exe` (Windows) or `bin/quaero` (Unix)

---

## Configuration Example

**File:** `quaero.toml`

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

---

## Error Handling

**Required Patterns:**

```go
// ✅ CORRECT - No ignored errors
data, err := loadData()
if err != nil {
    return fmt.Errorf("failed to load: %w", err)
}

// ❌ FORBIDDEN - Ignored errors
data, _ := loadData()
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
- Max 80 lines (ideal: 20-40)
- Single responsibility
- Comprehensive error handling

### File Structure
- Max 500 lines
- Modular design

### Forbidden Patterns
- `TODO:` comments
- `FIXME:` comments
- Hardcoded credentials
- Unused imports
- Dead code
- Ignored errors (`_ = err`)

---

**Last Updated:** 2025-10-05
**Status:** Active Development
**Version:** 2.0
