# quaero: Knowledge Search System
## Monorepo Architecture & Migration Guide

**quaero** (Latin: "I seek, I search") - A local knowledge base system with natural language query capabilities.

Version: 1.0  
Date: 2025-10-04  
Author: ternarybob

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Repository Structure](#repository-structure)
3. [Migration from aktis-parser](#migration-from-aktis-parser)
4. [Directory Structure](#directory-structure)
5. [Core Architecture](#core-architecture)
6. [Authentication Flow](#authentication-flow)
7. [Implementation Guide](#implementation-guide)

---

## Project Overview

### Name Etymology
**quaero** [KWAI-roh] - Latin verb meaning "I seek, I search, I inquire"
- First person singular present active indicative of *quaerō*
- Perfect for a knowledge retrieval system
- Implies active searching and questioning

### Purpose
quaero is a self-contained knowledge base system that:
- Collects documentation from multiple sources (Confluence, Jira, GitHub, Slack, Linear, etc.)
- Processes and stores content with full-text and vector search
- Provides natural language query interface using local LLMs (Ollama)
- Runs completely offline on a single machine
- Uses browser extension for seamless authentication

### Technology Stack
- **Language:** Go 1.25+
- **CLI Framework:** Cobra (subcommands and flags)
- **Storage:** SQLite with FTS5 (full-text search) and sqlite-vec (vector embeddings)
- **LLM:** Ollama (Qwen2.5-32B for text, Llama3.2-Vision-11B for images)
- **Browser Automation:** rod (for web scraping)
- **Authentication:** Chrome extension → HTTP service
- **Testing:** Go testing + testify

---

## Repository Structure

### Main Repositories

```
github.com/ternarybob/
├── quaero/                       # Main monorepo (THIS REPO)
├── quaero-auth-extension/        # Chrome extension for auth
└── quaero-docs/                  # Documentation
```

### Why This Structure?

**Monorepo (quaero):**
- All collectors in one place
- Shared authentication management
- Single build and deploy
- Unified testing
- Easy cross-source features

**Separate Extension (quaero-auth-extension):**
- Different technology (JavaScript vs Go)
- Different release cycle
- Can be reused by others
- Browser extension packaging requirements

**Separate Docs (quaero-docs):**
- Clean separation of concerns
- Can generate static site
- Easy to version

---

## Migration from aktis-parser

### What to Migrate

#### ✅ Code to Move Into Monorepo

**From aktis-parser to quaero/internal/:**

1. **Authentication Logic** → `internal/auth/`
   ```
   aktis-parser/internal/auth/      → quaero/internal/auth/
   ├── handler.go                   → manager.go (renamed, refactored)
   └── store.go                     → store.go
   ```

2. **Jira Client** → `internal/sources/jira/`
   ```
   aktis-parser/internal/jira/      → quaero/internal/sources/jira/
   ├── client.go                    → client.go
   └── types.go                     → models.go (adapt to new Document model)
   ```

3. **Confluence Client** → `internal/sources/confluence/`
   ```
   aktis-parser/internal/confluence/ → quaero/internal/sources/confluence/
   ├── client.go                     → api.go
   └── types.go                      → models.go
   ```

4. **HTTP Server Logic** → `internal/server/` & `cmd/quaero/`
   ```
   aktis-parser/cmd/service.go      → quaero/internal/server/server.go
                                       quaero/cmd/quaero/serve.go
   (Refactor into server component and CLI command)
   ```

#### ✅ Extension to Move to Separate Repo

**From aktis-parser to quaero-auth-extension:**

```
aktis-parser/extension/              → quaero-auth-extension/
├── manifest.json                    → manifest.json (update name)
├── background.js                    → background.js
├── icon.png                         → icon.png
└── README.md                        → README.md
```

**Changes needed in extension:**
- Update `SERVICE_URL` to point to quaero server
- Update extension name: "quaero authentication"
- Update notifications to say "quaero"

#### ❌ Don't Migrate (Replaced)

- BoltDB storage → Using RavenDB instead
- Old scraping logic → Rewriting with better architecture
- Background workers → New orchestration in monorepo

### Migration Checklist

- [ ] Create new `quaero` monorepo
- [ ] Copy auth handler logic to `internal/auth/manager.go`
- [ ] Adapt Jira client to new interfaces
- [ ] Adapt Confluence client to new interfaces
- [ ] Port HTTP endpoints for auth reception
- [ ] Create `quaero-auth-extension` repo
- [ ] Update extension to use new service name
- [ ] Test auth flow end-to-end
- [ ] Archive `aktis-parser` repo (don't delete, keep for reference)

---

## Directory Structure

```
quaero/
├── cmd/
│   └── quaero/                      # Single binary with subcommands
│       ├── main.go                  # Root command & CLI setup
│       ├── serve.go                 # 'quaero serve' - HTTP server
│       ├── web.go                   # 'quaero web' - Development web UI
│       ├── collect.go               # 'quaero collect' - Manual collection
│       ├── query.go                 # 'quaero query' - Ask questions
│       ├── inspect.go               # 'quaero inspect' - Inspect storage
│       └── version.go               # 'quaero version' - Show version
│
├── pkg/
│   └── models/                      # Public shared types
│       ├── document.go              # Core document model
│       ├── source.go                # Source interface
│       ├── storage.go               # Storage interface
│       └── rag.go                   # RAG interface
│
├── internal/
│   ├── app/                         # Application orchestration
│   │   ├── app.go                   # Main app struct
│   │   └── config.go                # Configuration
│   │
│   ├── server/                      # HTTP server (for 'quaero serve')
│   │   ├── server.go                # Server implementation
│   │   ├── handlers.go              # HTTP handlers
│   │   └── routes.go                # Route definitions
│   │
│   ├── collector/                   # Collection orchestration
│   │   ├── orchestrator.go          # Manages collection workflow
│   │   └── scheduler.go             # Background scheduling
│   │
│   ├── auth/                        # Authentication management (← MIGRATED)
│   │   ├── manager.go               # Manages auth state from extension
│   │   ├── handler.go               # HTTP endpoint for extension
│   │   ├── store.go                 # Store auth credentials
│   │   └── types.go                 # Auth data structures
│   │
│   ├── sources/                     # Data source implementations
│   │   ├── confluence/              # (← MIGRATED from aktis-parser)
│   │   │   ├── api.go               # REST API client
│   │   │   ├── scraper.go           # Browser scraper
│   │   │   ├── processor.go         # Convert to documents
│   │   │   ├── confluence.go        # Source interface implementation
│   │   │   └── confluence_test.go
│   │   │
│   │   ├── jira/                    # (← MIGRATED from aktis-parser)
│   │   │   ├── client.go            # API client
│   │   │   ├── processor.go         # Convert to documents
│   │   │   ├── jira.go              # Source interface implementation
│   │   │   └── jira_test.go
│   │   │
│   │   ├── github/                  # (NEW)
│   │   │   ├── client.go
│   │   │   ├── processor.go
│   │   │   └── github.go
│   │   │
│   │   ├── slack/                   # (NEW - future)
│   │   ├── linear/                  # (NEW - future)
│   │   └── notion/                  # (NEW - future)
│   │
│   ├── storage/
│   │   ├── ravendb/
│   │   │   ├── store.go             # RavenDB implementation
│   │   │   ├── queries.go           # Search queries
│   │   │   └── store_test.go
│   │   └── mock/
│   │       └── store.go             # Mock for testing
│   │
│   ├── rag/
│   │   ├── engine.go                # RAG orchestration
│   │   ├── search.go                # Search logic
│   │   ├── context.go               # Context building
│   │   ├── vision.go                # Image processing
│   │   └── engine_test.go
│   │
│   ├── llm/
│   │   ├── ollama/
│   │   │   ├── client.go            # Ollama API client
│   │   │   ├── vision.go            # Vision model support
│   │   │   └── client_test.go
│   │   └── mock/
│   │       └── client.go            # Mock LLM for testing
│   │
│   └── processing/
│       ├── chunker.go               # Text chunking
│       ├── ocr.go                   # OCR processing
│       ├── markdown.go              # HTML to Markdown
│       └── images.go                # Image handling
│
├── test/
│   ├── integration/
│   │   ├── auth_flow_test.go        # Test extension → service flow
│   │   ├── confluence_flow_test.go
│   │   ├── jira_flow_test.go
│   │   └── e2e_query_test.go
│   └── fixtures/
│       ├── auth_payload.json        # Sample extension auth data
│       ├── confluence_page.html
│       └── jira_issues.json
│
├── data/                            # Runtime data (gitignored)
│   ├── images/
│   └── attachments/
│
├── web/                             # Development web interface
│   ├── static/
│   │   ├── index.html               # Main UI
│   │   ├── style.css                # Styling
│   │   └── app.js                   # Frontend logic
│   └── templates/
│       ├── inspect.html             # Storage inspector
│       ├── query.html               # Query interface
│       └── collectors.html          # Collector status
│
├── docs/
│   ├── architecture.md
│   ├── migration.md                 # This document
│   ├── authentication.md            # How auth flow works
│   └── adding_collectors.md
│
├── scripts/
│   ├── setup.sh
│   └── migrate_from_aktis.sh        # Helper script
│
├── .github/
│   └── workflows/
│       └── ci.yml
│
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Core Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│  Browser (User authenticated in Jira/Confluence)            │
│  ┌────────────────────────────────────────────────┐         │
│  │  quaero auth extension                         │         │
│  │  • Extracts cookies, tokens, localStorage      │         │
│  │  • Sends to quaero service every 30 min        │         │
│  └──────────────────┬─────────────────────────────┘         │
└────────────────────┼──────────────────────────────────────────┘
                     │ POST /api/auth
                     │ (auth credentials)
                     ↓
┌─────────────────────────────────────────────────────────────┐
│  quaero server (Go - HTTP service)                          │
│  ┌─────────────────────────────────────────────────┐        │
│  │  Auth Manager                                   │        │
│  │  • Receives auth from extension                 │        │
│  │  • Stores credentials securely                  │        │
│  │  • Provides auth to collectors                  │        │
│  └──────────────────┬──────────────────────────────┘        │
│                     │                                        │
│  ┌──────────────────▼──────────────────────────────┐        │
│  │  Collection Orchestrator                        │        │
│  │  • Triggers collection on auth update           │        │
│  │  • Manages collector lifecycle                  │        │
│  └──────────────────┬──────────────────────────────┘        │
│                     │                                        │
│  ┌──────────────────▼──────────────────────────────┐        │
│  │  Sources (All implement Source interface)       │        │
│  │  ├─ Confluence (uses auth)                      │        │
│  │  ├─ Jira (uses auth)                            │        │
│  │  ├─ GitHub (uses token)                         │        │
│  │  └─ Future: Slack, Linear, Notion...            │        │
│  └──────────────────┬──────────────────────────────┘        │
│                     │ []*Document                            │
│  ┌──────────────────▼──────────────────────────────┐        │
│  │  Storage (RavenDB)                              │        │
│  │  • Store documents                              │        │
│  │  • Full-text search                             │        │
│  │  • Vector search                                │        │
│  └──────────────────┬──────────────────────────────┘        │
└────────────────────┼──────────────────────────────────────────┘
                     │
                     │ Query
                     ↓
┌─────────────────────────────────────────────────────────────┐
│  quaero CLI or Query Service                                │
│  ┌─────────────────────────────────────────────────┐        │
│  │  RAG Engine                                     │        │
│  │  • Search relevant docs                         │        │
│  │  • Process images with vision model             │        │
│  │  • Build context                                │        │
│  │  • Generate answer via Ollama                   │        │
│  └──────────────────┬──────────────────────────────┘        │
└────────────────────┼──────────────────────────────────────────┘
                     │
                     ↓
               ┌───────────┐
               │  Ollama   │
               │ (Local)   │
               └───────────┘
```

---

## Authentication Flow

### How It Works

This is the **key innovation** from aktis-parser that we're preserving:

```
1. User logs into Jira/Confluence normally (handles 2FA, SSO, etc.)
   ↓
2. quaero extension extracts complete auth state:
   • All cookies (.atlassian.net)
   • localStorage tokens
   • sessionStorage tokens
   • cloudId, atl_token
   • User agent
   ↓
3. Extension POSTs to quaero server:
   POST http://localhost:8080/api/auth
   {
     "cookies": [...],
     "tokens": {...},
     "baseUrl": "https://yourcompany.atlassian.net"
   }
   ↓
4. quaero server stores auth credentials
   ↓
5. Collectors use stored auth to make API calls
   (No manual token management needed!)
   ↓
6. Extension refreshes auth every 30 minutes
```

### Code Structure

#### Extension → Server

**Extension (JavaScript):**
```javascript
// quaero-auth-extension/background.js
const SERVICE_URL = 'http://localhost:8080';

async function extractAuthState() {
  const cookies = await chrome.cookies.getAll({
    domain: '.atlassian.net'
  });
  
  // ... extract tokens from page ...
  
  const authData = {
    cookies: cookies,
    tokens: extractedTokens,
    baseUrl: currentURL
  };
  
  await fetch(`${SERVICE_URL}/api/auth`, {
    method: 'POST',
    body: JSON.stringify(authData)
  });
}
```

#### Server Receives Auth

**quaero server (Go):**
```go
// internal/auth/handler.go
package auth

type Handler struct {
    manager *Manager
}

func (h *Handler) HandleAuth(w http.ResponseWriter, r *http.Request) {
    var authData ExtensionAuthData
    json.NewDecoder(r.Body).Decode(&authData)
    
    // Store auth
    h.manager.StoreAuth("confluence", &authData)
    h.manager.StoreAuth("jira", &authData)
    
    // Trigger collection
    go h.triggerCollection()
    
    w.WriteHeader(http.StatusOK)
}
```

#### Collectors Use Auth

**Confluence Collector:**
```go
// internal/sources/confluence/api.go
type APIClient struct {
    auth    *auth.AuthData
    baseURL string
}

func (c *APIClient) makeRequest(endpoint string) (*http.Response, error) {
    req, _ := http.NewRequest("GET", c.baseURL+endpoint, nil)
    
    // Add cookies from extension
    for _, cookie := range c.auth.Cookies {
        req.AddCookie(cookie)
    }
    
    // Add tokens
    if c.auth.Tokens.AtlToken != "" {
        req.Header.Set("X-Atlassian-Token", c.auth.Tokens.AtlToken)
    }
    
    return http.DefaultClient.Do(req)
}
```

---

## Implementation Guide

### Phase 1: Setup & Migration (Week 1)

#### Step 1: Create Quaero Monorepo

```bash
# Create new repo
mkdir quaero
cd quaero
go mod init github.com/ternarybob/quaero

# Install dependencies
go get github.com/spf13/cobra           # CLI framework
go get github.com/stretchr/testify      # Testing
go get github.com/go-rod/rod            # Browser automation
# Add RavenDB client when ready

# Create directory structure
mkdir -p cmd/quaero
mkdir -p pkg/models
mkdir -p internal/{app,server,collector,auth,sources/{confluence,jira,github},storage,rag,llm,processing}
mkdir -p test/{integration,fixtures}
mkdir -p docs
mkdir -p scripts
```

#### Step 2: Migrate Auth Code

```bash
# From aktis-parser
cd ../aktis-parser

# Copy auth logic
cp internal/auth/handler.go ../quaero/internal/auth/handler.go
cp internal/auth/store.go ../quaero/internal/auth/store.go

# Refactor to new interfaces (manual)
```

**Migration script:**
```bash
#!/bin/bash
# scripts/migrate_from_aktis.sh

echo "Migrating from aktis-parser to quaero..."

# 1. Copy auth code
cp -r ../aktis-parser/internal/auth ./internal/auth

# 2. Copy Jira client
cp -r ../aktis-parser/internal/jira ./internal/sources/jira

# 3. Copy Confluence client  
cp -r ../aktis-parser/internal/confluence ./internal/sources/confluence

# 4. Copy extension (to separate repo location)
cp -r ../aktis-parser/extension ../quaero-auth-extension

echo "Migration complete. Manual refactoring needed:"
echo "1. Update auth code to use new Storage interface"
echo "2. Update Jira/Confluence to implement Source interface"
echo "3. Update extension SERVICE_URL"
```

#### Step 3: Create Extension Repo

```bash
# Create separate repo
mkdir ../quaero-auth-extension
cd ../quaero-auth-extension

# Copy from aktis-parser
cp -r ../aktis-parser/extension/* .

# Update manifest.json
# Change name to "Quaero Authentication"
# Update SERVICE_URL in background.js
```

#### Step 4: Define Core Interfaces

```go
// pkg/models/source.go
package models

type Source interface {
    Name() string
    Collect(ctx context.Context) ([]*Document, error)
    SupportsImages() bool
}

// pkg/models/document.go
type Document struct {
    ID          string
    Source      string
    Title       string
    ContentMD   string
    Chunks      []Chunk
    Images      []Image
    Metadata    map[string]interface{}
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Phase 2: Implement Sources (Week 2)

#### Adapt Migrated Code

**Before (aktis-parser):**
```go
// Old Jira client
type JiraClient struct {
    baseURL string
    client  *http.Client
}

func (j *JiraClient) GetIssues() ([]Issue, error) {
    // Returns Jira-specific types
}
```

**After (quaero):**
```go
// New Jira source
type JiraSource struct {
    client  *Client
    auth    *auth.AuthData
}

func (j *JiraSource) Name() string {
    return "jira"
}

func (j *JiraSource) Collect(ctx context.Context) ([]*models.Document, error) {
    // Fetch issues
    issues := j.client.GetAllIssues(ctx)
    
    // Convert to Documents
    var docs []*models.Document
    for _, issue := range issues {
        doc := j.issueToDocument(issue)
        docs = append(docs, doc)
    }
    return docs, nil
}

func (j *JiraSource) issueToDocument(issue *Issue) *models.Document {
    return &models.Document{
        ID:        fmt.Sprintf("jira-%s", issue.Key),
        Source:    "jira",
        Title:     issue.Summary,
        ContentMD: j.formatIssue(issue),
        Metadata: map[string]interface{}{
            "status":   issue.Status,
            "assignee": issue.Assignee,
            "created":  issue.Created,
        },
    }
}
```

### Phase 3: Server & Auth (Week 3)

#### Implement CLI with Cobra

**Install Cobra:**
```bash
go get github.com/spf13/cobra
```

**Main Command:**
```go
// cmd/quaero/main.go
package main

import (
    "fmt"
    "os"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "quaero",
    Short: "quaero - Knowledge search system",
    Long:  `quaero (Latin: "I seek") - A local knowledge base system with natural language queries.`,
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    rootCmd.AddCommand(serveCmd)
    rootCmd.AddCommand(collectCmd)
    rootCmd.AddCommand(queryCmd)
    rootCmd.AddCommand(versionCmd)
}
```

**Serve Command:**
```go
// cmd/quaero/serve.go
package main

import (
    "log"
    "github.com/spf13/cobra"
    "github.com/ternarybob/quaero/internal/server"
    "github.com/ternarybob/quaero/internal/app"
)

var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start HTTP server to receive auth from extension",
    Long:  `Starts the quaero server which receives authentication from the browser extension and runs background collection.`,
    Run:   runServe,
}

var (
    serverPort string
    serverHost string
)

func init() {
    serveCmd.Flags().StringVar(&serverPort, "port", "8080", "Server port")
    serveCmd.Flags().StringVar(&serverHost, "host", "localhost", "Server host")
}

func runServe(cmd *cobra.Command, args []string) {
    // Initialize app
    app := app.New()
    
    // Create and start server
    srv := server.New(app, serverHost, serverPort)
    
    log.Printf("quaero server starting on %s:%s", serverHost, serverPort)
    log.Println("Waiting for authentication from browser extension...")
    
    if err := srv.Start(); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
```

**Server Implementation:**
```go
// internal/server/server.go
package server

import (
    "fmt"
    "net/http"
    "github.com/ternarybob/quaero/internal/app"
    "github.com/ternarybob/quaero/internal/auth"
)

type Server struct {
    app  *app.App
    host string
    port string
}

func New(app *app.App, host, port string) *Server {
    return &Server{
        app:  app,
        host: host,
        port: port,
    }
}

func (s *Server) Start() error {
    mux := s.setupRoutes()
    addr := fmt.Sprintf("%s:%s", s.host, s.port)
    return http.ListenAndServe(addr, mux)
}

func (s *Server) setupRoutes() *http.ServeMux {
    mux := http.NewServeMux()
    
    // Auth endpoint (extension posts here)
    authHandler := auth.NewHandler(s.app.AuthManager, s.app.Collector)
    mux.HandleFunc("/api/auth", authHandler.HandleAuth)
    
    // Status endpoint
    mux.HandleFunc("/api/status", s.handleStatus)
    
    // Health check
    mux.HandleFunc("/health", s.handleHealth)
    
    return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
    // Return collection status
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}
```

**Collect Command:**
```go
// cmd/quaero/collect.go
package main

import (
    "context"
    "log"
    "github.com/spf13/cobra"
    "github.com/ternarybob/quaero/internal/app"
)

var collectCmd = &cobra.Command{
    Use:   "collect",
    Short: "Manually trigger collection from sources",
    Long:  `Triggers data collection from configured sources (Confluence, Jira, etc.)`,
    Run:   runCollect,
}

var (
    collectSource string
    collectAll    bool
)

func init() {
    collectCmd.Flags().StringVar(&collectSource, "source", "", "Specific source to collect from (confluence, jira, github)")
    collectCmd.Flags().BoolVar(&collectAll, "all", false, "Collect from all sources")
}

func runCollect(cmd *cobra.Command, args []string) {
    app := app.New()
    
    ctx := context.Background()
    
    if collectAll {
        log.Println("Collecting from all sources...")
        if err := app.Collector.CollectAll(ctx); err != nil {
            log.Fatalf("Collection failed: %v", err)
        }
    } else if collectSource != "" {
        log.Printf("Collecting from %s...\n", collectSource)
        if err := app.Collector.CollectSource(ctx, collectSource); err != nil {
            log.Fatalf("Collection failed: %v", err)
        }
    } else {
        log.Println("Please specify --source or --all")
        cmd.Help()
        return
    }
    
    log.Println("Collection complete!")
}
```

**Query Command:**
```go
// cmd/quaero/query.go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/spf13/cobra"
    "github.com/ternarybob/quaero/internal/app"
)

var queryCmd = &cobra.Command{
    Use:   "query [question]",
    Short: "Ask a natural language question",
    Long:  `Query the knowledge base using natural language. Returns an answer based on collected documentation.`,
    Args:  cobra.ExactArgs(1),
    Run:   runQuery,
}

var (
    queryIncludeSources bool
    queryIncludeImages  bool
)

func init() {
    queryCmd.Flags().BoolVar(&queryIncludeSources, "sources", false, "Include source references in answer")
    queryCmd.Flags().BoolVar(&queryIncludeImages, "images", false, "Process relevant images")
}

func runQuery(cmd *cobra.Command, args []string) {
    question := args[0]
    
    app := app.New()
    ctx := context.Background()
    
    log.Printf("Searching for: %s\n", question)
    
    answer, err := app.RAG.Query(ctx, question)
    if err != nil {
        log.Fatalf("Query failed: %v", err)
    }
    
    fmt.Println("\n" + answer.Text + "\n")
    
    if queryIncludeSources && len(answer.Sources) > 0 {
        fmt.Println("Sources:")
        for _, doc := range answer.Sources {
            fmt.Printf("  - %s (%s)\n", doc.Title, doc.Source)
        }
    }
}
```

**Version Command:**
```go
// cmd/quaero/version.go
package main

import (
    "fmt"
    "github.com/spf13/cobra"
)

var Version = "1.0.0"

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print version information",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("quaero version %s\n", Version)
    },
}
```

#### Test Auth Flow

```go
// test/integration/auth_flow_test.go
func TestExtensionAuthFlow(t *testing.T) {
    // Start test server
    server := setupTestServer(t)
    defer server.Close()
    
    // Simulate extension sending auth
    authData := loadFixture(t, "auth_payload.json")
    
    resp, err := http.Post(
        server.URL+"/api/auth",
        "application/json",
        bytes.NewBuffer(authData),
    )
    
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    
    // Verify auth was stored
    storedAuth := server.App.AuthManager.GetAuth("confluence")
    assert.NotNil(t, storedAuth)
    assert.NotEmpty(t, storedAuth.Cookies)
}
```

### Phase 4: Storage & RAG (Week 4)

Implement RavenDB storage and RAG engine as per original spec.

### Phase 5: CLI & Testing (Week 5)

#### Build and Test

```bash
# Build
make build

# Output: bin/quaero (single binary)

# Test all commands
./bin/quaero --help
./bin/quaero serve --help
./bin/quaero collect --help
./bin/quaero query --help

# Or install globally
make install

# Then use anywhere
quaero serve
quaero query "test question"
```

#### Create Example Queries

```bash
# Collection
quaero collect --all

# Query
quaero query "How to onboard new user?"

# Server mode
quaero serve
```

---

## Binary Architecture

### Single Binary, Multiple Commands

Quaero uses the **single binary pattern** (like Docker, Consul, Terraform):

```
quaero
├── serve      # Start HTTP server
├── web        # Start development web UI
├── collect    # Trigger collection
├── query      # Ask questions
├── inspect    # Inspect storage/documents
├── debug      # Debug tools
└── version    # Show version
```

**Benefits:**
- ✅ Simple deployment - one file
- ✅ Consistent interface
- ✅ Easy to install (`go install`)
- ✅ No confusion about which binary to run

### Usage Patterns

#### Pattern 1: Server Mode (Recommended)

Keep server running continuously:

```bash
# Start once
quaero serve

# Extension handles auth automatically
# Server collects in background
# Query anytime from another terminal
quaero query "your question"
```

**When to use:**
- Daily development workflow
- Want automatic collection
- Multiple users querying

#### Pattern 2: Manual Mode

Run collection manually when needed:

```bash
# No server running

# Collect manually (uses last stored auth)
quaero collect --all

# Then query
quaero query "your question"
```

**When to use:**
- One-off data collection
- Scheduled cron jobs
- Testing/development

#### Pattern 3: CLI Only

Just query without server:

```bash
# Assume data already collected
quaero query "your question"
```

**When to use:**
- Data already in RavenDB
- Read-only queries
- Script automation

---

## Development Interface Requirements

### Purpose

Developers need visibility into the data pipeline for debugging, verification, and optimization:

**What needs visualization:**
1. **Data Inputs** - What's being collected from each source
2. **Storage State** - What's in RavenDB, how it's structured
3. **Outputs** - Query results, relevance, source attribution
4. **Collection Status** - Which collectors ran, when, success/failure
5. **Processing Pipeline** - Chunking, OCR, image processing results

### CLI Interface (Primary)

Command-line tools for quick inspection and debugging:

#### Inspect Commands

```bash
# View storage stats
quaero inspect stats
# Output:
# Documents: 1,247
# Images: 89
# Sources: confluence (892), jira (355)
# Last updated: 2025-10-04 14:23:11

# List documents from a source
quaero inspect list --source confluence --limit 10

# Show specific document
quaero inspect doc confluence-page-12345
# Output: Full document with metadata, chunks, images

# View collection history
quaero inspect collections
# Output: Last 10 collection runs with status

# Search without LLM (raw search results)
quaero inspect search "authentication"
# Output: Raw search results with scores
```

#### Debug Commands

```bash
# Test a collector without storing
quaero collect --source confluence --dry-run --limit 5

# Show what would be chunked
quaero debug chunk --file sample.md

# Test OCR on image
quaero debug ocr --image path/to/diagram.png

# Show prompt that would be sent to LLM
quaero debug prompt "How to onboard a new user?"
```

#### Analysis Commands

```bash
# Show source breakdown
quaero analyze sources

# Show most common topics
quaero analyze topics

# Find documents without images
quaero analyze missing-images

# Storage size breakdown
quaero analyze storage
```

### Web Interface (Development)

Browser-based UI for richer visualization:

#### Start Web UI

```bash
# Start development web interface
quaero web

# Or combined with server
quaero serve --with-ui

# Accessible at http://localhost:8080/ui
```

#### Web UI Features

**1. Dashboard (Home Page)**
```
┌─────────────────────────────────────────┐
│  quaero Development Dashboard           │
├─────────────────────────────────────────┤
│  Collection Status                      │
│  ✓ Confluence: 892 docs (2h ago)       │
│  ✓ Jira: 355 issues (2h ago)           │
│  ○ GitHub: Not configured               │
│                                          │
│  Storage                                │
│  📊 Total Documents: 1,247              │
│  📊 Total Chunks: 15,430                │
│  📊 Images: 89                          │
│  📊 Attachments: 23                     │
│                                          │
│  Recent Queries                         │
│  • "How to onboard?" (3 sources)       │
│  • "Data architecture" (1 source)      │
└─────────────────────────────────────────┘
```

**2. Document Browser**
```
┌─────────────────────────────────────────┐
│  Documents                              │
├─────────────────────────────────────────┤
│  Filter: [confluence ▼] [all spaces ▼] │
│  Search: [________________] 🔍          │
│                                          │
│  ┌─ Confluence Page: Authentication    │
│  │  ID: confluence-page-12345          │
│  │  Space: TEAM                        │
│  │  Updated: 2025-10-03                │
│  │  Chunks: 12 | Images: 3             │
│  │  [View] [Raw JSON] [Delete]         │
│  └─                                     │
│                                          │
│  ┌─ Jira Issue: DATA-123              │
│  │  ...                                │
└─────────────────────────────────────────┘
```

**3. Document Detail View**
```
┌─────────────────────────────────────────┐
│  Authentication Guide                    │
│  confluence-page-12345                  │
├─────────────────────────────────────────┤
│  Metadata                               │
│  Source: confluence                     │
│  Space: TEAM                            │
│  URL: https://...                       │
│  Updated: 2025-10-03 14:23             │
│                                          │
│  Content (Markdown)                     │
│  ┌───────────────────────────────────┐ │
│  │ # Authentication                  │ │
│  │                                    │ │
│  │ Our system uses OAuth 2.0...      │ │
│  └───────────────────────────────────┘ │
│                                          │
│  Chunks (12)                            │
│  ┌─ Chunk 0 (Position: 0)             │
│  │  "# Authentication Our system..."  │
│  │  Vector: [0.23, -0.45, ...]       │
│  └─                                     │
│                                          │
│  Images (3)                             │
│  ┌─ OAuth Flow Diagram                │
│  │  [📷 Image Preview]                │
│  │  OCR Text: "Client -> Auth..."    │
│  │  Description: "Diagram showing..." │
│  └─                                     │
└─────────────────────────────────────────┘
```

**4. Query Debugger**
```
┌─────────────────────────────────────────┐
│  Query Debugger                         │
├─────────────────────────────────────────┤
│  Question:                              │
│  [How to onboard a new user?]          │
│  [Execute Query]                        │
│                                          │
│  Search Results (5 documents found)    │
│  ┌─ Onboarding Guide (score: 0.89)    │
│  │  confluence-page-456                │
│  │  Matched chunks: 3                  │
│  └─                                     │
│                                          │
│  Context Sent to LLM                   │
│  ┌───────────────────────────────────┐ │
│  │ Based on the following docs:      │ │
│  │                                    │ │
│  │ # Onboarding Guide                │ │
│  │ To onboard a new user...          │ │
│  └───────────────────────────────────┘ │
│                                          │
│  LLM Response                           │
│  ┌───────────────────────────────────┐ │
│  │ To onboard a new user:            │ │
│  │ 1. Request access...              │ │
│  └───────────────────────────────────┘ │
│                                          │
│  Processing Time: 2.3s                 │
│  Sources Used: 3                       │
└─────────────────────────────────────────┘
```

**5. Collector Management**
```
┌─────────────────────────────────────────┐
│  Collectors                             │
├─────────────────────────────────────────┤
│  ┌─ Confluence                         │
│  │  Status: ✓ Running                 │
│  │  Last run: 2h ago                  │
│  │  Documents: 892                    │
│  │  [Run Now] [Configure] [Logs]     │
│  └─                                     │
│                                          │
│  ┌─ Jira                               │
│  │  Status: ✓ Idle                    │
│  │  Last run: 2h ago                  │
│  │  Issues: 355                       │
│  │  [Run Now] [Configure] [Logs]     │
│  └─                                     │
│                                          │
│  ┌─ GitHub                             │
│  │  Status: ○ Not configured          │
│  │  [Configure]                        │
│  └─                                     │
└─────────────────────────────────────────┘
```

**6. Collection Logs**
```
┌─────────────────────────────────────────┐
│  Collection Log - Confluence           │
├─────────────────────────────────────────┤
│  2025-10-04 12:23:11  Started          │
│  2025-10-04 12:23:15  Authenticated    │
│  2025-10-04 12:23:20  Fetching pages.. │
│  2025-10-04 12:25:33  Page 1/50: Auth  │
│  2025-10-04 12:25:34  Processing...    │
│  2025-10-04 12:25:35  ✓ Stored         │
│  ...                                    │
│  2025-10-04 12:48:22  ✓ Complete       │
│  Total: 892 documents, 89 images       │
└─────────────────────────────────────────┘
```

### Implementation

#### CLI Commands Code

```go
// cmd/quaero/inspect.go
var inspectCmd = &cobra.Command{
    Use:   "inspect",
    Short: "Inspect storage and documents",
}

var inspectStatsCmd = &cobra.Command{
    Use:   "stats",
    Short: "Show storage statistics",
    Run:   runInspectStats,
}

var inspectDocCmd = &cobra.Command{
    Use:   "doc [id]",
    Short: "Show document details",
    Args:  cobra.ExactArgs(1),
    Run:   runInspectDoc,
}

var inspectListCmd = &cobra.Command{
    Use:   "list",
    Short: "List documents",
    Run:   runInspectList,
}

func init() {
    inspectCmd.AddCommand(inspectStatsCmd)
    inspectCmd.AddCommand(inspectDocCmd)
    inspectCmd.AddCommand(inspectListCmd)
    
    inspectListCmd.Flags().StringVar(&source, "source", "", "Filter by source")
    inspectListCmd.Flags().IntVar(&limit, "limit", 20, "Limit results")
}
```

#### Web UI Code

```go
// cmd/quaero/web.go
var webCmd = &cobra.Command{
    Use:   "web",
    Short: "Start development web interface",
    Long:  `Starts a web interface for inspecting data, testing queries, and monitoring collectors.`,
    Run:   runWeb,
}

var webPort string

func init() {
    webCmd.Flags().StringVar(&webPort, "port", "8080", "Web UI port")
}

func runWeb(cmd *cobra.Command, args []string) {
    app := app.New()
    
    // Serve static files
    http.Handle("/", http.FileServer(http.Dir("web/static")))
    
    // API endpoints for UI
    http.HandleFunc("/api/documents", app.HandleListDocuments)
    http.HandleFunc("/api/document/", app.HandleGetDocument)
    http.HandleFunc("/api/stats", app.HandleGetStats)
    http.HandleFunc("/api/query", app.HandleQuery)
    http.HandleFunc("/api/collectors", app.HandleCollectors)
    http.HandleFunc("/api/logs", app.HandleLogs)
    
    log.Printf("Web UI available at http://localhost:%s", webPort)
    http.ListenAndServe(":"+webPort, nil)
}
```

#### Simple HTML/JS UI

```html
<!-- web/static/index.html -->
<!DOCTYPE html>
<html>
<head>
    <title>quaero - Development Dashboard</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <nav>
        <h1>quaero</h1>
        <ul>
            <li><a href="#dashboard">Dashboard</a></li>
            <li><a href="#documents">Documents</a></li>
            <li><a href="#query">Query</a></li>
            <li><a href="#collectors">Collectors</a></li>
        </ul>
    </nav>
    
    <main id="app">
        <!-- Dynamic content loaded here -->
    </main>
    
    <script src="app.js"></script>
</body>
</html>
```

```javascript
// web/static/app.js
async function loadDashboard() {
    const stats = await fetch('/api/stats').then(r => r.json());
    
    document.getElementById('app').innerHTML = `
        <h2>Dashboard</h2>
        <div class="stats">
            <div class="stat">
                <h3>${stats.documents}</h3>
                <p>Documents</p>
            </div>
            <div class="stat">
                <h3>${stats.chunks}</h3>
                <p>Chunks</p>
            </div>
            <div class="stat">
                <h3>${stats.images}</h3>
                <p>Images</p>
            </div>
        </div>
    `;
}
```

### Development Workflow

**Typical development session:**

```bash
# Terminal 1: Start web UI
quaero web

# Terminal 2: Run collection
quaero collect --source confluence --dry-run

# Browser: http://localhost:8080
# - View what would be collected
# - Inspect chunking strategy
# - Test search queries
# - Debug RAG pipeline

# Terminal 2: Actually collect
quaero collect --source confluence

# Browser: Refresh to see new documents

# Terminal 2: Test query
quaero query "How to authenticate?"

# Browser: Use Query Debugger
# - See search results
# - View context sent to LLM
# - Inspect answer quality
```

### Why Both CLI and Web?

**CLI:**
- ✅ Fast for scripting/automation
- ✅ Works in SSH/remote environments
- ✅ Great for CI/CD pipelines
- ✅ Unix philosophy (composable)

**Web UI:**
- ✅ Visual inspection of documents
- ✅ Image preview
- ✅ Rich formatting
- ✅ Easier for non-technical users
- ✅ Better for debugging complex queries

### Production vs Development

**Development Mode (default):**
- Web UI enabled
- Verbose logging
- Debug endpoints
- No authentication

**Production Mode (future):**
```bash
quaero serve --production
# - Web UI requires auth
# - Reduced logging
# - Debug endpoints disabled
```

---

## Configuration

```yaml
# config.yaml
app:
  name: "quaero"
  version: "1.0.0"

server:
  port: 8080
  host: "localhost"

sources:
  confluence:
    enabled: true
    spaces: ["TEAM", "DOCS"]
  
  jira:
    enabled: true
    projects: ["DATA", "ENG"]
  
  github:
    enabled: true
    token: "${GITHUB_TOKEN}"
    repos:
      - "your-org/repo1"

storage:
  ravendb:
    urls: ["http://localhost:8080"]
    database: "quaero"
  
  filesystem:
    images: "./data/images"
    attachments: "./data/attachments"

llm:
  ollama:
    url: "http://localhost:11434"
    text_model: "qwen2.5:32b"
    vision_model: "llama3.2-vision:11b"
```

---

## Testing Strategy

### Unit Tests
```bash
# Test specific source
make test-jira
make test-confluence

# Test auth
make test-auth
```

### Integration Tests
```bash
# Test auth flow
make test-integration-auth

# Test full pipeline
make test-integration
```

### E2E Tests
```bash
# With real Ollama
make test-e2e
```

---

## Makefile

```makefile
.PHONY: build test migrate clean run-serve run-collect run-query

# Build single binary
build:
	go build -o bin/quaero ./cmd/quaero

# Install globally
install:
	go install ./cmd/quaero

# Test
test:
	go test ./...

test-unit:
	go test -short ./...

test-integration:
	go test ./test/integration/...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Test specific components
test-auth:
	go test ./internal/auth/...

test-jira:
	go test ./internal/sources/jira/...

test-confluence:
	go test ./internal/sources/confluence/...

test-rag:
	go test ./internal/rag/...

# Run commands
serve:
	go run ./cmd/quaero serve

web:
	go run ./cmd/quaero web

inspect-stats:
	go run ./cmd/quaero inspect stats

inspect-list:
	go run ./cmd/quaero inspect list --source confluence

collect:
	go run ./cmd/quaero collect --all

collect-confluence:
	go run ./cmd/quaero collect --source confluence

collect-jira:
	go run ./cmd/quaero collect --source jira

query:
	@if [ -z "$(Q)" ]; then \
		echo "Usage: make query Q=\"your question\""; \
	else \
		go run ./cmd/quaero query "$(Q)"; \
	fi

# Migrate from aktis-parser
migrate:
	./scripts/migrate_from_aktis.sh

# Development
fmt:
	go fmt ./...
	goimports -w .

lint:
	golangci-lint run

# Watch tests
watch:
	find . -name "*.go" | entr -c make test-unit

# Clean
clean:
	rm -rf bin/
	rm -rf data/
	rm -f coverage.out

# Setup development environment
setup:
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	./scripts/setup.sh

# Help
help:
	@echo "quaero Makefile Commands:"
	@echo "  make build            - Build quaero binary"
	@echo "  make install          - Install quaero globally"
	@echo "  make test             - Run all tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make serve            - Start server"
	@echo "  make web              - Start development web UI"
	@echo "  make inspect-stats    - Show storage statistics"
	@echo "  make inspect-list     - List documents"
	@echo "  make collect          - Collect from all sources"
	@echo "  make query Q=\"...\"    - Run a query"
	@echo "  make migrate          - Migrate from aktis-parser"
	@echo "  make clean            - Clean build artifacts"
```

---

## Migration Verification Checklist

After migration, verify:

- [ ] Extension sends auth to quaero server
- [ ] Server receives and stores auth correctly
- [ ] Jira collector uses stored auth successfully
- [ ] Confluence collector uses stored auth successfully
- [ ] Documents are created in new format
- [ ] Storage works (RavenDB instead of BoltDB)
- [ ] All tests pass
- [ ] Can query "How to onboard a new user?"

---

## Repository Naming

```
github.com/ternarybob/quaero               # Main monorepo
github.com/ternarybob/quaero-auth-extension # Chrome extension
github.com/ternarybob/quaero-docs          # Documentation
```

---

## Next Steps

1. **Create quaero repo** - Setup monorepo structure
2. **Run migration script** - Copy code from aktis-parser
3. **Refactor to interfaces** - Adapt to new architecture
4. **Implement CLI with Cobra** - Single binary with subcommands
5. **Create extension repo** - Separate out the extension
6. **Test auth flow** - Verify end-to-end
7. **Implement storage** - RavenDB integration
8. **Build RAG** - Query engine
9. **Add more sources** - GitHub, Slack, etc.

### Quick Start After Migration

```bash
# Build
make build

# Terminal 1: Start server
./bin/quaero serve

# Terminal 2: (after extension sends auth)
./bin/quaero query "How to onboard a new user?"
```

---

## Success Criteria

✅ Extension authenticates and sends to quaero server  
✅ Jira data collected and stored  
✅ Confluence data collected with images  
✅ Can answer: "How to onboard a new user?"  
✅ Can answer: "How is the team performing?"  
✅ Can answer: "Show me the data architecture" (with diagrams)  

---

**quaero: I seek knowledge. 🔍**