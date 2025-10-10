# Quaero Architecture

**Version:** 3.0
**Last Updated:** 2025-10-06
**Status:** Active Development

---

## Overview

Quaero is a knowledge collection and search system that gathers documentation from multiple sources (Confluence, Jira, GitHub) and provides semantic search capabilities using vector embeddings and LLM integration.

**Critical Design Principle:** Quaero operates in two mutually exclusive modes to address fundamentally different security requirements:
- **Cloud Mode:** For personal/non-sensitive use (data sent to external APIs)
- **Offline Mode:** For corporate/government/sensitive data (guaranteed local processing)

**Inspiration:** Memory categorization and tool-based RAG from [Agent Zero](https://github.com/agent0ai/agent-zero), adapted for enterprise knowledge management with strict data privacy controls.

**Key Differences from Agent Zero:**
- **Deployment:** Native Go binary with embedded inference (no Docker)
- **Security:** Explicit cloud vs offline modes with audit trail
- **Storage:** SQLite with FTS5 + vector embeddings
- **LLM Strategy:** Single provider per mode (simplicity over flexibility)
- **Scope:** Focused knowledge base for enterprise documentation

---

## Security Architecture

### Mode Enforcement

**CRITICAL REQUIREMENT:** The system MUST prevent accidental data exfiltration.

```
User Configures Mode
    ↓
    ├─ Cloud Mode?
    │   ├─ Display WARNING
    │   ├─ Require explicit confirmation flag
    │   ├─ Log all API calls
    │   └─ Proceed with cloud provider
    │
    └─ Offline Mode?
        ├─ Verify model files exist
        ├─ Block all external network calls
        ├─ Log all operations locally
        └─ Proceed with embedded inference
```

### Data Classification Rules

**When Offline Mode is REQUIRED:**
- Government data (any level: local, state, federal)
- Healthcare records (HIPAA, privacy legislation)
- Financial information (customer data, internal financials)
- Personal information (PII, employee records)
- Confidential business data (trade secrets, strategic plans)
- Any data where breach would cause legal/reputational harm

**When Cloud Mode is Acceptable:**
- Personal notes and documentation
- Public documentation
- Non-confidential research
- Educational materials
- Data you own and accept risk for

**Reference Incident:** [ABC News: Northern Rivers data breach via ChatGPT](https://www.abc.net.au/news/2025-10-06/data-breach-northern-rivers-resilient-homes-program-chatgpt/105855284)

---

## Deployment Modes

### Cloud Mode (Personal/Non-Sensitive Data)

**Use Case:** Personal knowledge management where cloud provider access is acceptable.

**Architecture:**
```
Quaero Binary
    ↓
    └─ Google Gemini API
       ├─ Embeddings: text-embedding-004 (768d)
       └─ Chat: gemini-1.5-flash
```

**Requirements:**
- Internet connectivity
- Gemini API key
- Explicit risk acknowledgment in config
- **NO Docker required**

**Data Flow:**
```
Document → Quaero → Gemini API (Google servers) → Embedding/Response → Quaero
```

**Security Properties:**
- ❌ Data leaves local machine
- ❌ Subject to Google's terms of service
- ❌ Potential for data retention/analysis
- ✅ Fast, high-quality results
- ✅ Simple setup

### Offline Mode (Corporate/Government/Sensitive Data)

**Use Case:** Enterprise/government use where data MUST remain local.

**Architecture:**
```
Quaero Binary
    ↓
    └─ Embedded llama.cpp
       ├─ Embeddings: nomic-embed-text-v1.5.gguf (768d)
       └─ Chat: qwen2.5-7b-instruct-q4.gguf
```

**Requirements:**
- Model files downloaded once (~5GB total)
- 8-16GB RAM
- Multi-core CPU (8+ cores recommended)
- **NO Docker required**
- **NO internet required** (after initial model download)

**Data Flow:**
```
Document → Quaero → llama.cpp (local inference) → Embedding/Response → Quaero
```

**Security Properties:**
- ✅ All data stays on local machine
- ✅ No network calls (verifiable)
- ✅ Audit trail for compliance
- ✅ Works air-gapped
- ⚠️ Slower inference (2-5 seconds per query)
- ⚠️ Lower quality than GPT-4/Claude

---

## Current Implementation Status

### ✅ Phase 1.0 - Core Infrastructure (COMPLETE)
- Web-based UI with real-time updates
- SQLite storage with FTS5 full-text search
- Chrome extension authentication
- Jira & Confluence collectors
- WebSocket for live log streaming
- RESTful API endpoints
- HTTP server with graceful shutdown
- Dependency injection architecture
- Test suite (integration & unit tests)

### ✅ Phase 1.1 - Vector Embeddings (COMPLETE)
- Document model with normalized structure
- Embedding service with provider abstraction
- Document service with automatic embedding
- Processing service for background vectorization
- CRON scheduler for periodic processing
- SQLite persistence with binary embedding storage
- Documents UI for browsing vectorized content
- API endpoints for document management

### 🚧 Phase 1.2 - Dual Mode LLM (IN PROGRESS)

**Cloud Mode Implementation:**
- [ ] Gemini API client (embeddings + chat)
- [ ] Configuration validation for API key
- [ ] Warning system for cloud mode usage
- [ ] Risk acknowledgment requirement
- [ ] API call logging for audit

**Offline Mode Implementation:**
- [ ] llama.cpp Go bindings integration
- [ ] Model file management (download, verify, load)
- [ ] Embedded inference for embeddings
- [ ] Embedded inference for chat
- [ ] Network isolation verification
- [ ] Local-only audit trail

**Common Requirements:**
- [ ] Mode selection and validation
- [ ] Health checks on startup
- [ ] Graceful degradation
- [ ] Error handling with helpful messages
- [ ] Performance monitoring

### 🚧 Phase 1.3 - RAG Pipeline (PLANNED)
- Memory area categorization (Main, Fragments, Solutions, Facts)
- RAG service with tool-based architecture
- Similarity threshold filtering (default 0.7)
- Embedding cache with LRU eviction
- Hybrid search (FTS5 + vector)
- Context builder for relevant passages
- Answer generation with citations
- Query interface (CLI & Web)

### 📋 Phase 2.0 - GitHub Integration (PLANNED)
- GitHub service implementation
- Repository and wiki collection
- GitHub storage schema
- GitHub UI page

### 📋 Phase 3.0 - Advanced Search (PLANNED)
- Vector similarity search (sqlite-vec)
- Hybrid search implementation
- Image processing and OCR
- Additional data sources (Slack, Linear)

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│  Browser (Chrome)                                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Quaero Chrome Extension                           │   │
│  │  • Captures Atlassian auth (cookies, tokens)       │   │
│  │  • Connects via WebSocket                          │   │
│  │  • Sends auth data to server                       │   │
│  └──────────────────┬───────────────────────────────────┘   │
└────────────────────┼───────────────────────────────────────┘
                     │ WebSocket: ws://localhost:8080/ws
                     │
                     ↓
┌─────────────────────────────────────────────────────────────┐
│  Quaero Server (Single Go Binary)                           │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  HTTP Server (internal/server/)                     │   │
│  │  • Routes, middleware, graceful shutdown            │   │
│  └──────────────────┬───────────────────────────────────┘   │
│                     │                                        │
│  ┌──────────────────▼───────────────────────────────────┐   │
│  │  Handlers (internal/handlers/)                      │   │
│  │  • WebSocket, UI, Collector, Document, Data        │   │
│  └──────────────────┬───────────────────────────────────┘   │
│                     │                                        │
│  ┌──────────────────▼───────────────────────────────────┐   │
│  │  Services (internal/services/)                      │   │
│  │  • Atlassian (auth, Jira, Confluence)              │   │
│  │  • Documents (management, search)                   │   │
│  │  • LLM (mode-specific implementations)              │   │
│  │  • Processing (extraction, vectorization)           │   │
│  └──────────────────┬───────────────────────────────────┘   │
│                     │                                        │
│  ┌──────────────────▼───────────────────────────────────┐   │
│  │  Storage (internal/storage/sqlite/)                 │   │
│  │  • SQLite DB, Persistence               │   │
│  └──────────────────┬───────────────────────────────────┘   │
└────────────────────┼────────────────────────────────────────┘
                     │
                     ↓
┌─────────────────────────────────────────────────────────────┐
│  SQLite Database (./quaero.db)                              │
│  • jira_projects, jira_issues                               │
│  • confluence_spaces, confluence_pages                      │
│  • documents (with embeddings)                              │
│  • document_chunks                                          │
│  • documents_fts (FTS5)                                     │
│  • audit_log (data access trail)                            │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ↓
      ┌──────────────┴──────────────┐
      │                             │
      ↓                             ↓
┌───────────────────┐    ┌───────────────────┐
│  CLOUD MODE       │    │  OFFLINE MODE     │
│                   │    │                   │
│  Gemini API:      │    │  Embedded Models: │
│  • text-embed-004 │    │  • nomic-embed    │
│  • gemini-1.5     │    │  • qwen2.5-7b     │
│                   │    │                   │
│  Requires:        │    │  Requires:        │
│  • Internet       │    │  • Model files    │
│  • API key        │    │  • 8-16GB RAM     │
│  • Risk accept    │    │  • Multi-core CPU │
│                   │    │                   │
│  Data leaves      │    │  Data stays       │
│  machine ⚠️       │    │  local ✓          │
└───────────────────┘    └───────────────────┘
```

---

## Core Components

### 1. LLM Service Interface

**Location:** `internal/services/llm/`

**Unified interface for both modes:**

```go
package llm

type Service interface {
    // Generate embedding for text
    Embed(ctx context.Context, text string) ([]float32, error)
    
    // Generate chat completion
    Chat(ctx context.Context, messages []Message) (string, error)
    
    // Health check
    HealthCheck(ctx context.Context) error
    
    // Get mode information
    GetMode() Mode
    
    // Get audit trail (for offline mode)
    GetAuditLog() []AuditEntry
}

type Mode string

const (
    ModeCloud   Mode = "cloud"
    ModeOffline Mode = "offline"
)
```

### 2. Cloud Mode Implementation

**Location:** `internal/services/llm/cloud/`

**Gemini API integration:**

```go
package cloud

type GeminiClient struct {
    apiKey      string
    embedModel  string
    chatModel   string
    httpClient  *http.Client
    logger      arbor.ILogger
    auditLog    *AuditLog
}

func NewGeminiClient(config *Config, logger arbor.ILogger) (*GeminiClient, error) {
    if config.APIKey == "" {
        return nil, fmt.Errorf("GEMINI_API_KEY required for cloud mode")
    }
    
    // Warn about cloud usage
    logger.Warn().Msg("⚠️  CLOUD MODE: Data will be sent to Google Gemini API")
    logger.Warn().Msg("⚠️  Do NOT use with government, healthcare, or confidential data")
    
    if !config.ConfirmRisk {
        return nil, fmt.Errorf("cloud mode requires explicit risk acceptance: set confirm_risk = true")
    }
    
    return &GeminiClient{
        apiKey:     config.APIKey,
        embedModel: "text-embedding-004",
        chatModel:  "gemini-1.5-flash",
        httpClient: &http.Client{Timeout: 30 * time.Second},
        logger:     logger,
        auditLog:   NewAuditLog(logger),
    }, nil
}

func (c *GeminiClient) Embed(ctx context.Context, text string) ([]float32, error) {
    // Log API call
    c.auditLog.Record(AuditEntry{
        Timestamp: time.Now(),
        Mode:      "cloud",
        Operation: "embed",
        Provider:  "gemini",
    })
    
    // Call Gemini API
    // ... implementation
}

func (c *GeminiClient) Chat(ctx context.Context, messages []Message) (string, error) {
    // Log API call
    c.auditLog.Record(AuditEntry{
        Timestamp: time.Now(),
        Mode:      "cloud",
        Operation: "chat",
        Provider:  "gemini",
    })
    
    // Call Gemini API
    // ... implementation
}
```

### 3. Offline Mode Implementation

**Location:** `internal/services/llm/offline/`

**Embedded llama.cpp integration:**

```go
package offline

import (
    llama "github.com/go-skynet/go-llama.cpp"
)

type EmbeddedLLM struct {
    embedModel  *llama.LLama
    chatModel   *llama.LLama
    logger      arbor.ILogger
    auditLog    *AuditLog
    config      *Config
}

func NewEmbeddedLLM(config *Config, logger arbor.ILogger) (*EmbeddedLLM, error) {
    // Verify model files exist
    if !fileExists(config.EmbedModelPath) {
        return nil, fmt.Errorf("embedding model not found: %s", config.EmbedModelPath)
    }
    if !fileExists(config.ChatModelPath) {
        return nil, fmt.Errorf("chat model not found: %s", config.ChatModelPath)
    }
    
    logger.Info().Msg("✓ OFFLINE MODE: All processing will be local")
    logger.Info().Str("embed_model", config.EmbedModelPath).Msg("Loading embedding model")
    logger.Info().Str("chat_model", config.ChatModelPath).Msg("Loading chat model")
    
    // Load embedding model
    embedModel, err := llama.New(
        config.EmbedModelPath,
        llama.SetContext(512),
        llama.SetEmbeddings(true),
        llama.SetThreads(config.Threads),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load embedding model: %w", err)
    }
    
    // Load chat model
    chatModel, err := llama.New(
        config.ChatModelPath,
        llama.SetContext(config.ContextSize),
        llama.SetThreads(config.Threads),
        llama.SetGPULayers(config.GPULayers),
    )
    if err != nil {
        embedModel.Close()
        return nil, fmt.Errorf("failed to load chat model: %w", err)
    }
    
    // Verify network isolation (sanity check)
    if err := verifyOfflineCapability(); err != nil {
        logger.Warn().Err(err).Msg("Network detected but offline mode active")
    }
    
    return &EmbeddedLLM{
        embedModel: embedModel,
        chatModel:  chatModel,
        logger:     logger,
        auditLog:   NewAuditLog(logger),
        config:     config,
    }, nil
}

func (e *EmbeddedLLM) Embed(ctx context.Context, text string) ([]float32, error) {
    // Log operation locally
    e.auditLog.Record(AuditEntry{
        Timestamp: time.Now(),
        Mode:      "offline",
        Operation: "embed",
        Provider:  "llama.cpp",
    })
    
    // Generate embedding using llama.cpp
    embeddings, err := e.embedModel.Embeddings(text)
    if err != nil {
        return nil, fmt.Errorf("embedding generation failed: %w", err)
    }
    
    return embeddings, nil
}

func (e *EmbeddedLLM) Chat(ctx context.Context, messages []Message) (string, error) {
    // Log operation locally
    e.auditLog.Record(AuditEntry{
        Timestamp: time.Now(),
        Mode:      "offline",
        Operation: "chat",
        Provider:  "llama.cpp",
    })
    
    // Format messages for model
    prompt := formatMessagesForLlama(messages)
    
    // Generate response
    response, err := e.chatModel.Predict(
        prompt,
        llama.SetTokens(512),
        llama.SetTemperature(0.7),
    )
    if err != nil {
        return "", fmt.Errorf("chat generation failed: %w", err)
    }
    
    return response, nil
}

func (e *EmbeddedLLM) Close() error {
    if err := e.embedModel.Close(); err != nil {
        return err
    }
    if err := e.chatModel.Close(); err != nil {
        return err
    }
    return nil
}
```

### 4. Audit Log System

**Location:** `internal/services/llm/audit.go`

**Required for compliance and data breach investigation:**

```go
package llm

type AuditEntry struct {
    Timestamp   time.Time
    Mode        string  // "cloud" or "offline"
    Operation   string  // "embed", "chat", "search"
    Provider    string  // "gemini" or "llama.cpp"
    DocumentID  string  // Optional: which document (NOT content)
    Success     bool
    ErrorMsg    string
}

type AuditLog struct {
    entries []AuditEntry
    logger  arbor.ILogger
    mu      sync.RWMutex
}

func NewAuditLog(logger arbor.ILogger) *AuditLog {
    return &AuditLog{
        entries: make([]AuditEntry, 0),
        logger:  logger,
    }
}

func (a *AuditLog) Record(entry AuditEntry) {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    a.entries = append(a.entries, entry)
    
    // Log to structured logger
    a.logger.Info().
        Str("mode", entry.Mode).
        Str("operation", entry.Operation).
        Str("provider", entry.Provider).
        Bool("success", entry.Success).
        Msg("LLM operation")
    
    // TODO: Persist to SQLite for permanent audit trail
}

func (a *AuditLog) GetEntries(since time.Time) []AuditEntry {
    a.mu.RLock()
    defer a.mu.RUnlock()
    
    var filtered []AuditEntry
    for _, entry := range a.entries {
        if entry.Timestamp.After(since) {
            filtered = append(filtered, entry)
        }
    }
    return filtered
}
```

### 5. Configuration Validation

**Location:** `internal/common/config.go`

**Strict validation to prevent misconfiguration:**

```go
func ValidateLLMConfig(config *LLMConfig) error {
    // Mode must be explicitly set
    if config.Mode != "cloud" && config.Mode != "offline" {
        return fmt.Errorf("llm.mode must be 'cloud' or 'offline', got: %s", config.Mode)
    }
    
    // Cloud mode validation
    if config.Mode == "cloud" {
        if config.Cloud.APIKey == "" {
            return fmt.Errorf("cloud mode requires api_key")
        }
        if !config.Cloud.ConfirmRisk {
            return fmt.Errorf(
                "cloud mode requires explicit risk acceptance\n" +
                "Set confirm_risk = true in config to acknowledge data will be sent to external APIs",
            )
        }
    }
    
    // Offline mode validation
    if config.Mode == "offline" {
        if config.Offline.EmbedModelPath == "" {
            return fmt.Errorf("offline mode requires embed_model_path")
        }
        if config.Offline.ChatModelPath == "" {
            return fmt.Errorf("offline mode requires chat_model_path")
        }
        if !fileExists(config.Offline.EmbedModelPath) {
            return fmt.Errorf("embedding model file not found: %s", config.Offline.EmbedModelPath)
        }
        if !fileExists(config.Offline.ChatModelPath) {
            return fmt.Errorf("chat model file not found: %s", config.Offline.ChatModelPath)
        }
    }
    
    return nil
}
```

---

## Data Flow Diagrams

### Cloud Mode Document Processing

```
1. User triggers collection
   ↓
2. Scraper fetches Confluence/Jira data
   ↓
3. Store in source tables
   ↓
4. ProcessingService extracts documents
   ↓
5. DocumentService.SaveDocument()
   ↓
6. LLMService.Embed() → Gemini API Call
   ⚠️  DATA SENT TO GOOGLE SERVERS
   ↓
7. Receive 768-dim embedding vector
   ↓
8. Store in SQLite with binary encoding
   ↓
9. Update FTS5 index
   ↓
10. Log audit entry (cloud API call)
```

### Offline Mode Document Processing

```
1. User triggers collection
   ↓
2. Scraper fetches Confluence/Jira data
   ↓
3. Store in source tables
   ↓
4. ProcessingService extracts documents
   ↓
5. DocumentService.SaveDocument()
   ↓
6. LLMService.Embed() → llama.cpp local inference
   ✓ ALL DATA STAYS ON LOCAL MACHINE
   ↓
7. Generate 768-dim embedding (2-3 seconds)
   ↓
8. Store in SQLite with binary encoding
   ↓
9. Update FTS5 index
   ↓
10. Log audit entry (local operation)
```

### RAG Query Flow (Cloud Mode)

```
1. User asks natural language question
   ↓
2. LLMService.Embed(query) → Gemini API
   ⚠️  QUERY SENT TO GOOGLE
   ↓
3. Perform vector search + FTS5 hybrid
   ↓
4. Build context from top-k results
   ↓
5. LLMService.Chat(context + question) → Gemini API
   ⚠️  CONTEXT + QUESTION SENT TO GOOGLE
   ↓
6. Receive answer with citations
   ↓
7. Display to user
```

### RAG Query Flow (Offline Mode)

```
1. User asks natural language question
   ↓
2. LLMService.Embed(query) → llama.cpp
   ✓ LOCAL PROCESSING (2-3 sec)
   ↓
3. Perform vector search + FTS5 hybrid
   ↓
4. Build context from top-k results
   ↓
5. LLMService.Chat(context + question) → llama.cpp
   ✓ LOCAL PROCESSING (3-5 sec)
   ↓
6. Receive answer (lower quality than GPT-4)
   ↓
7. Display to user
```

---

## Directory Structure

```
quaero/
├── cmd/
│   ├── quaero/
│   │   ├── main.go                  # Entry point
│   │   ├── serve.go                 # HTTP server command
│   │   └── version.go               # Version command
│   └── quaero-chrome-extension/     # Chrome extension
│
├── internal/
│   ├── common/                      # Stateless utilities
│   │   ├── config.go                # TOML config with LLM mode validation
│   │   ├── logger.go                # Arbor logger
│   │   ├── banner.go                # Startup banner
│   │   └── version.go               # Version management
│   │
│   ├── services/
│   │   ├── llm/                     # LLM service (NEW)
│   │   │   ├── service.go           # Interface definition
│   │   │   ├── factory.go           # Mode-based factory
│   │   │   ├── audit.go             # Audit log system
│   │   │   ├── cloud/               # Cloud mode implementation
│   │   │   │   └── gemini.go        # Gemini API client
│   │   │   └── offline/             # Offline mode implementation
│   │   │       ├── llama.go         # llama.cpp integration
│   │   │       └── models.go        # Model management
│   │   │
│   │   ├── embeddings/              # Embedding service (uses LLM service)
│   │   │   └── embedding_service.go
│   │   │
│   │   ├── documents/
│   │   │   └── document_service.go
│   │   │
│   │   ├── processing/
│   │   │   ├── processing_service.go
│   │   │   └── scheduler.go
│   │   │
│   │   └── atlassian/               # Jira & Confluence
│   │       ├── auth_service.go
│   │       ├── jira_scraper_service.go
│   │       └── confluence_scraper_service.go
│   │
│   ├── handlers/                    # HTTP handlers
│   ├── storage/sqlite/              # SQLite storage
│   ├── interfaces/                  # Service interfaces
│   └── models/                      # Data models
│
├── models/                          # Model files (offline mode)
│   ├── nomic-embed-text-v1.5-q8.gguf       # ~150MB
│   └── qwen2.5-7b-instruct-q4.gguf         # ~4.5GB
│
├── pages/                           # Web UI
├── test/                            # Tests
├── scripts/                         # Build scripts
└── docs/                            # Documentation
```

---

## Model Files (Offline Mode)

### Required Models

**Embedding Model:**
- Name: `nomic-embed-text-v1.5-q8.gguf`
- Size: ~150MB
- Dimensions: 768
- Source: https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF

**Chat Model:**
- Name: `qwen2.5-7b-instruct-q4_k_m.gguf`
- Size: ~4.5GB
- Parameters: 7B (quantized to 4-bit)
- Source: https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF

### Model Download Process

```bash
# Create models directory
mkdir -p models

# Download embedding model
curl -L -o models/nomic-embed-text-v1.5-q8.gguf \
  https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.q8_0.gguf

# Download chat model
curl -L -o models/qwen2.5-7b-instruct-q4.gguf \
  https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_k_m.gguf

# Verify checksums (TODO: add actual checksums)
sha256sum models/*.gguf
```

---

## API Endpoints

### HTTP Endpoints

```
GET  /                              - Dashboard UI
GET  /confluence                    - Confluence UI
GET  /jira                          - Jira UI
GET  /documents                     - Documents UI

POST /api/collect/jira              - Trigger Jira collection
POST /api/collect/confluence        - Trigger Confluence collection

GET  /api/data/jira/projects        - Get Jira projects
GET  /api/data/jira/issues          - Get Jira issues
GET  /api/data/confluence/spaces    - Get Confluence spaces
GET  /api/data/confluence/pages     - Get Confluence pages

GET  /api/documents/stats           - Document statistics
GET  /api/documents                 - List documents with filtering
POST /api/documents/process         - Trigger document processing

# Processing Engine Operational Control (NEW)
GET  /api/processing/status         - Get processing engine status
POST /api/documents/{id}/reprocess  - Force reprocess single document
DELETE /api/documents/{id}/embedding - Wipe single document embedding
DELETE /api/embeddings              - Wipe all embeddings (destructive)

GET  /api/llm/mode                  - Get current LLM mode
GET  /api/llm/audit                 - Get audit log entries
GET  /api/llm/health                - LLM health check

GET  /health                        - Health check
```

### Processing Status Response

```json
{
  "total_documents": 1250,
  "processed_count": 1205,
  "pending_count": 42,
  "failed_count": 3,
  "last_run_timestamp": "2025-10-06T12:00:00Z",
  "next_run_timestamp": "2025-10-06T18:00:00Z",
  "engine_status": "IDLE"  // or "RUNNING"
}
```

### Operational Control Endpoints

**Wipe All Embeddings:**
```bash
DELETE /api/embeddings

Response:
{
  "message": "All embeddings cleared",
  "documents_affected": 1250,
  "status": "All documents marked PENDING"
}
```

**Use Cases:**
- Switching from Cloud to Offline mode (different embedding dimensions)
- Upgrading to a new embedding model
- Recovering from data corruption
- Fresh start / reset

**Wipe Single Document Embedding:**
```bash
DELETE /api/documents/{id}/embedding

Response:
{
  "message": "Embedding cleared for document",
  "document_id": "doc_123",
  "status": "PENDING"
}
```

**Use Cases:**
- Document content was updated
- Old embedding is stale
- Troubleshooting specific document issues

**Force Reprocess Document:**
```bash
POST /api/documents/{id}/reprocess

Response:
{
  "message": "Document reprocessing initiated",
  "document_id": "doc_123",
  "status": "PENDING",
  "note": "Processing will occur on next engine run or immediately if triggered"
}
```

**Use Cases:**
- Immediate re-vectorization after document edit
- Testing changes to processing logic
- Bypassing scheduled run for urgent updates

---

## Technology Stack

**Language:** Go 1.25+

**Core Libraries:**
- `github.com/ternarybob/arbor` - Logging (REQUIRED)
- `github.com/ternarybob/banner` - Banners (REQUIRED)
- `github.com/pelletier/go-toml/v2` - TOML config (REQUIRED)
- `github.com/spf13/cobra` - CLI framework
- `github.com/gorilla/websocket` - WebSocket
- `modernc.org/sqlite` - SQLite driver
- `github.com/robfig/cron/v3` - CRON scheduling
- `github.com/go-skynet/go-llama.cpp` - llama.cpp bindings (offline mode)

**Storage:** SQLite with FTS5

**Frontend:** Vanilla HTML/CSS/JavaScript

**Browser:** Chrome Extension (Manifest V3)

**LLM Providers:**
- **Cloud:** Google Gemini API
- **Offline:** llama.cpp with GGUF models

---

## Performance Characteristics

### Cloud Mode

**Document Processing:**
- Embedding generation: ~50-100ms per document
- API rate limits: 60 requests/minute (Gemini free tier)
- Batch processing: Sequential with rate limiting

**Query Performance:**
- Query embedding: ~50-100ms
- Chat completion: ~500-1000ms
- Total RAG query: ~1-2 seconds

### Offline Mode

**Document Processing:**
- Embedding generation: ~2-3 seconds per document
- No rate limits (CPU-bound)
- Batch processing: Parallel with CPU thread pool

**Query Performance:**
- Query embedding: ~2-3 seconds
- Chat completion: ~3-5 seconds (varies by prompt length)
- Total RAG query: ~5-8 seconds

**Resource Usage:**
- RAM: 8-16GB (models loaded in memory)
- CPU: High usage during inference
- Disk: ~5GB for model files

---

## Security Considerations

### Cloud Mode Security

**Risks:**
- Data transmitted to Google servers
- Subject to Google's data retention policies
- Potential for unauthorized access if API key leaked
- No guarantee of data deletion

**Mitigations:**
- Explicit warnings on startup
- Required risk acknowledgment in config
- Audit log of all API calls
- API key stored in environment variables (not committed to git)
- HTTPS for all API communications

### Offline Mode Security

**Guarantees:**
- All processing occurs on local machine
- No network calls (verifiable)
- No data transmission to external services
- Complete control over data lifecycle

**Implementation:**
- Network isolation verification on startup
- Comprehensive audit trail stored locally
- Model files verified via checksum
- Air-gap capable after initial model download

### Audit Trail Requirements

**All operations must be logged:**
- Timestamp
- Mode (cloud/offline)
- Operation (embed/chat/search)
- Success/failure
- Error messages (if any)
- Document ID (metadata only, not content)

**Storage:**
- SQLite table: `audit_log`
- Retention: Configurable (default: 90 days)
- Export: JSON format for compliance reporting

---

## Remaining Work

### Phase 1.2 - Dual Mode LLM (Current Focus)

**Cloud Mode:**
- [ ] Implement Gemini API client for embeddings
- [ ] Implement Gemini API client for chat
- [ ] Add API key validation
- [ ] Add risk acknowledgment requirement
- [ ] Add startup warnings
- [ ] Add API call audit logging

**Offline Mode:**
- [ ] Integrate go-llama.cpp bindings
- [ ] Implement model file management
- [ ] Implement embedding generation via llama.cpp
- [ ] Implement chat generation via llama.cpp
- [ ] Add model file verification (checksums)
- [ ] Add network isolation checks
- [ ] Add local audit logging

**Common:**
- [ ] Create LLM service interface
- [ ] Implement mode-based factory
- [ ] Add configuration validation
- [ ] Add health check endpoints
- [ ] Update UI to show current mode
- [ ] Add audit log viewer in UI
- [ ] Update documentation

**Processing Engine Enhancements:**
- [ ] Add document processing status field
- [ ] Implement FindUnprocessedDocuments()
- [ ] Add processing state management (PENDING/PROCESSED/FAILED)
- [ ] Create operational control endpoints:
  - [ ] GET /api/processing/status
  - [ ] DELETE /api/embeddings
  - [ ] DELETE /api/documents/{id}/embedding
  - [ ] POST /api/documents/{id}/reprocess
- [ ] Update UI to show processing status
- [ ] Add failed document viewer
- [ ] Implement retry logic for failed documents

### Phase 1.3 - RAG Pipeline
- [ ] Memory area categorization
- [ ] Tool-based RAG architecture
- [ ] Similarity threshold filtering
- [ ] Embedding cache (LRU)
- [ ] Hybrid search implementation
- [ ] Context builder
- [ ] Citation system
- [ ] Query interface (CLI & Web)

### Phase 2.0 - GitHub Integration
- [ ] GitHub service implementation
- [ ] Repository and wiki collection
- [ ] GitHub storage schema
- [ ] GitHub UI page

### Phase 3.0 - Advanced Search
- [ ] sqlite-vec integration
- [ ] Vector similarity search
- [ ] Hybrid search optimization
- [ ] Image processing and OCR

---

## Testing Strategy

### Unit Tests
- LLM service interface implementations
- Mode validation logic
- Audit log functionality
- Configuration parsing

### Integration Tests
- End-to-end cloud mode workflow
- End-to-end offline mode workflow
- Mode switching
- Error handling

### Performance Tests
- Embedding generation benchmarks (cloud vs offline)
- Chat generation benchmarks
- Large document processing
- Concurrent request handling

### Security Tests
- Network isolation verification (offline mode)
- API key validation (cloud mode)
- Audit log completeness
- Configuration validation

---

**Last Updated:** 2025-10-06
**Status:** Active Development
**Version:** 3.0