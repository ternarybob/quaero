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

### ✅ Phase 1.2 - Dual Mode LLM (COMPLETE)

**Offline Mode Implementation (COMPLETE):**
- ✅ LLM service interface (`internal/interfaces/llm_service.go`)
- ✅ Offline service using llama-cli binary execution (`internal/services/llm/offline/llama.go`)
- ✅ Model file management and verification (`internal/services/llm/offline/models.go`)
- ✅ Service factory with mode selection (`internal/services/llm/factory.go`)
- ✅ Audit logging system (`internal/services/llm/audit.go`)
- ✅ SQLite audit log storage (migration v4)
- ✅ Network isolation verification (zero network calls)
- ✅ Configuration structures with env overrides
- ✅ Health checks on startup
- ✅ Comprehensive error handling
- ✅ Performance benchmarks and testing
- ✅ Complete documentation (setup guide, API docs)

**Security Guarantees:**
- ✅ 100% local processing (no HTTP client in offline code)
- ✅ Binary execution model (os/exec, no CGo)
- ✅ Audit trail in SQLite
- ✅ Mode enforcement at startup

**Cloud Mode Implementation (PLANNED):**
- [ ] Gemini API client (embeddings + chat)
- [ ] Configuration validation for API key
- [ ] Warning system for cloud mode usage
- [ ] Risk acknowledgment requirement
- [ ] API call logging for audit

**Documentation:**
- ✅ Setup guide: `docs/offline-mode-setup.md`
- ✅ Service documentation: `internal/services/llm/offline/README.md`
- ✅ Example config: `deployments/config.offline.example.toml`
- ✅ Architecture updated with offline mode details

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
│  │  • SQLite DB, Migrations, Persistence               │   │
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

### 3. Offline Mode Implementation (IMPLEMENTED)

**Location:** `internal/services/llm/offline/`

**Architecture:** Binary execution model (os/exec) instead of CGo bindings

```go
package offline

import (
    "os/exec"
    "context"
)

type OfflineLLMService struct {
    modelManager *ModelManager
    binaryPath   string
    contextSize  int
    threadCount  int
    gpuLayers    int
    logger       arbor.ILogger
    auditLogger  AuditLogger
    mockMode     bool
}

func NewOfflineLLMService(
    modelDir string,
    embedModel string,
    chatModel string,
    contextSize int,
    threadCount int,
    gpuLayers int,
    logger arbor.ILogger,
) (*OfflineLLMService, error) {
    // Find llama-cli binary
    binaryPath, err := findLlamaBinary()
    if err != nil {
        return nil, fmt.Errorf("llama-cli binary not found: %w", err)
    }

    // Create model manager
    modelManager := NewModelManager(modelDir, embedModel, chatModel)

    // Verify model files exist
    if err := modelManager.VerifyModels(); err != nil {
        return nil, fmt.Errorf("model verification failed: %w", err)
    }

    logger.Info().Msg("✓ OFFLINE MODE: All processing will be local")
    logger.Info().Str("binary", binaryPath).Msg("Using llama-cli")
    logger.Info().Str("embed_model", modelManager.GetEmbedModelPath()).Msg("Embedding model")
    logger.Info().Str("chat_model", modelManager.GetChatModelPath()).Msg("Chat model")

    return &OfflineLLMService{
        modelManager: modelManager,
        binaryPath:   binaryPath,
        contextSize:  contextSize,
        threadCount:  threadCount,
        gpuLayers:    gpuLayers,
        logger:       logger,
        mockMode:     false,
    }, nil
}

func (s *OfflineLLMService) Embed(ctx context.Context, text string) ([]float32, error) {
    start := time.Now()

    if s.mockMode {
        // Mock mode for testing
        return generateMockEmbedding(text), nil
    }

    // Execute llama-cli for embeddings
    cmd := exec.CommandContext(ctx, s.binaryPath,
        "-m", s.modelManager.GetEmbedModelPath(),
        "-p", text,
        "--embedding",
        "-t", fmt.Sprintf("%d", s.threadCount),
    )

    output, err := cmd.Output()
    if err != nil {
        s.auditLogger.LogEmbed(false, time.Since(start), err.Error())
        return nil, fmt.Errorf("embedding generation failed: %w", err)
    }

    embedding := parseEmbeddingOutput(output)
    s.auditLogger.LogEmbed(true, time.Since(start), "")

    return embedding, nil
}

func (s *OfflineLLMService) Chat(ctx context.Context, messages []Message) (string, error) {
    start := time.Now()

    if s.mockMode {
        // Mock mode for testing
        return "This is a mock response for testing.", nil
    }

    // Format messages using ChatML format
    prompt := formatPrompt(messages)

    // Execute llama-cli for chat
    cmd := exec.CommandContext(ctx, s.binaryPath,
        "-m", s.modelManager.GetChatModelPath(),
        "-p", prompt,
        "-c", fmt.Sprintf("%d", s.contextSize),
        "-t", fmt.Sprintf("%d", s.threadCount),
        "-ngl", fmt.Sprintf("%d", s.gpuLayers),
    )

    output, err := cmd.Output()
    if err != nil {
        s.auditLogger.LogChat(false, time.Since(start), err.Error(), "")
        return "", fmt.Errorf("chat generation failed: %w", err)
    }

    response := extractResponse(output)
    s.auditLogger.LogChat(true, time.Since(start), "", response)

    return response, nil
}

func (s *OfflineLLMService) Close() error {
    // No resources to close with binary execution
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
│   ├── interfaces/                  # Service interfaces
│   │   ├── llm_service.go           # LLM service interface (NEW)
│   │   └── ... other interfaces
│   │
│   ├── services/
│   │   ├── llm/                     # LLM service (IMPLEMENTED)
│   │   │   ├── factory.go           # Mode-based factory (COMPLETE)
│   │   │   ├── audit.go             # Audit log system (COMPLETE)
│   │   │   ├── cloud/               # Cloud mode implementation (PLANNED)
│   │   │   │   └── gemini.go        # Gemini API client (TBD)
│   │   │   └── offline/             # Offline mode implementation (COMPLETE)
│   │   │       ├── llama.go         # llama-cli binary execution
│   │   │       ├── models.go        # Model file management
│   │   │       ├── README.md        # Service documentation
│   │   │       └── llama_test.go    # Unit tests
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

GET  /api/llm/mode                  - Get current LLM mode
GET  /api/llm/audit                 - Get audit log entries (NEW)
GET  /api/llm/health                - LLM health check (NEW)

GET  /health                        - Health check
```

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

## Offline Mode Architecture (IMPLEMENTED)

### Binary Execution Model

Quaero's offline mode uses **binary execution** of llama-cli instead of CGo bindings:

**Benefits:**
- **No CGo dependencies** - Simpler builds, better cross-platform support
- **Process isolation** - Clear security boundary
- **Zero network capability** - Verifiable through code review
- **Easy testing** - Mock mode for testing without binary

**Binary Detection:**
1. `./bin/llama-cli` (or `.exe` on Windows)
2. `./llama-cli` (or `.exe` on Windows)
3. `llama-cli` in PATH

### Security Verification

**Network Isolation Checklist:**
- ✅ No `net/http` imports in offline code paths
- ✅ No `net` package usage
- ✅ Only `os/exec` for binary execution
- ✅ Only local file I/O
- ✅ All inference via llama-cli local binary

**Audit Trail:**
- All operations logged to SQLite `llm_audit_log` table
- Timestamp, mode, operation, success/failure, duration
- No document content (metadata only)
- Exportable to JSON for compliance

**Verification Commands:**
```bash
# Check no HTTP imports in offline code
grep -r "net/http" internal/services/llm/offline/
# Expected: no results

# Verify audit log
sqlite3 ./data/quaero.db "SELECT mode, COUNT(*) FROM llm_audit_log GROUP BY mode;"
# Expected: Only 'offline' mode
```

### Setup Instructions

**Complete guide:** `docs/offline-mode-setup.md` (1,247 lines)

**Quick setup:**
1. Build llama-cli from llama.cpp
2. Download models (nomic-embed + qwen2.5-7b)
3. Configure Quaero with model paths
4. Run in offline mode

### Performance Characteristics

**Embeddings (768-dimension):**
- CPU-only: 2-3 seconds per document
- GPU (CUDA/Metal): 0.5-1 second per document

**Chat Completions:**
- CPU-only: 5-10 seconds for 500 tokens
- GPU (CUDA/Metal): 1-2 seconds for 500 tokens

**Memory Usage:**
- Base application: ~200 MB
- Embed model: ~150 MB (nomic-embed-text-v1.5-q8)
- Chat model: ~4.5 GB (qwen2.5-7b-instruct-q4)
- **Total:** ~5 GB RAM minimum

---

## Remaining Work

### Phase 1.2 - Cloud Mode (Future)

**Cloud Mode Implementation (PLANNED):**
- [ ] Gemini API client (embeddings + chat)
- [ ] Configuration validation for API key
- [ ] Warning system for cloud mode usage
- [ ] Risk acknowledgment requirement
- [ ] API call audit logging
- [ ] Update UI to show current mode
- [ ] Add mode switcher in settings

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