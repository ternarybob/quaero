# Quaero: Knowledge Search System
## Monorepo Architecture & Migration Guide

**Quaero** (Latin: "I seek, I search") - A local knowledge base system with natural language query capabilities.

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
**Quaero** [KWAI-roh] - Latin verb meaning "I seek, I search, I inquire"
- First person singular present active indicative of *quaerō*
- Perfect for a knowledge retrieval system
- Implies active searching and questioning

### Purpose
Quaero is a self-contained knowledge base system that:
- Collects documentation from multiple sources (Confluence, Jira, GitHub, Slack, Linear, etc.)
- Processes and stores content with full-text and vector search
- Provides natural language query interface using local LLMs (Ollama)
- Runs completely offline on a single machine
- Uses browser extension for seamless authentication

### Technology Stack
- **Language:** Go 1.21+
- **Storage:** RavenDB (document store with vector search)
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
- Update extension name: "Quaero Authentication"
- Update notifications to say "Quaero"

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
│       ├── collect.go               # 'quaero collect' - Manual collection
│       ├── query.go                 # 'quaero query' - Ask questions
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
│  │  Quaero Auth Extension                         │         │
│  │  • Extracts cookies, tokens, localStorage      │         │
│  │  • Sends to Quaero service every 30 min        │         │
│  └──────────────────┬─────────────────────────────┘         │
└────────────────────┼──────────────────────────────────────────┘
                     │ POST /api/auth
                     │ (auth credentials)
                     ↓
┌─────────────────────────────────────────────────────────────┐
│  Quaero Server (Go - HTTP service)                          │
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
│  Quaero CLI or Query Service                                │
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
2. Quaero extension extracts complete auth state:
   • All cookies (.atlassian.net)
   • localStorage tokens
   • sessionStorage tokens
   • cloudId, atl_token
   • User agent
   ↓
3. Extension POSTs to Quaero server:
   POST http://localhost:8080/api/auth
   {
     "cookies": [...],
     "tokens": {...},
     "baseUrl": "https://yourcompany.atlassian.net"
   }
   ↓
4. Quaero server stores auth credentials
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

**Quaero Server (Go):**
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
    Short: "Quaero - Knowledge search system",
    Long:  `Quaero (Latin: "I seek") - A local knowledge base system with natural language queries.`,
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
    Long:  `Starts the Quaero server which receives authentication from the browser extension and runs background collection.`,
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
    
    log.Printf("Quaero server starting on %s:%s", serverHost, serverPort)
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
        fmt.Printf("Quaero version %s\n", Version)
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

```bash
# Collection
quaero collect --all

# Query
quaero query "How to onboard new user?"

# Server mode
quaero serve
```

---

## Configuration

```yaml
# config.yaml
app:
  name: "Quaero"
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
.PHONY: build test migrate

# Build
build:
	go build -o bin/quaero ./cmd/quaero
	go build -o bin/quaero-server ./cmd/quaero-server

# Test
test:
	go test ./...

test-auth:
	go test ./internal/auth/...

test-jira:
	go test ./internal/sources/jira/...

test-confluence:
	go test ./internal/sources/confluence/...

test-integration:
	go test ./test/integration/...

# Migrate from aktis-parser
migrate:
	./scripts/migrate_from_aktis.sh

# Run
serve:
	go run ./cmd/quaero-server

collect:
	go run ./cmd/quaero collect --all

query:
	go run ./cmd/quaero query "$(Q)"

# Clean
clean:
	rm -rf bin/ data/
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
4. **Create extension repo** - Separate out the extension
5. **Test auth flow** - Verify end-to-end
6. **Implement storage** - RavenDB integration
7. **Build RAG** - Query engine
8. **Add more sources** - GitHub, Slack, etc.

---

## Success Criteria

✅ Extension authenticates and sends to quaero server  
✅ Jira data collected and stored  
✅ Confluence data collected with images  
✅ Can answer: "How to onboard a new user?"  
✅ Can answer: "How is the team performing?"  
✅ Can answer: "Show me the data architecture" (with diagrams)  

---

**Quaero: I seek knowledge. 🔍**
