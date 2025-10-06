# Quaero Requirements

**quaero** (Latin: "I seek, I search") - A knowledge base system with strict data privacy controls.

Version: 3.0
Date: 2025-10-06
Status: Active Development

---

## Critical Security Requirement

**PRIMARY DESIGN CONSTRAINT:** Quaero must operate in two mutually exclusive modes to prevent accidental data exfiltration in regulated environments.

**Reference Incident:** [ABC News - Northern Rivers data breach via ChatGPT](https://www.abc.net.au/news/2025-10-06/data-breach-northern-rivers-resilient-homes-program-chatgpt/105855284)
- Government agency accidentally sent sensitive citizen data to OpenAI ChatGPT
- Data breach of personal information including addresses and damage reports
- Exactly the scenario Quaero's offline mode must prevent

---

## Project Overview

### Purpose

Quaero is a knowledge base system that:
- Collects documentation from approved sources (Confluence, Jira, GitHub)
- Processes and stores content with full-text and vector search
- Provides natural language query interface using LLM integration
- **Operates in two security modes:** Cloud (convenience) or Offline (compliance)
- Uses Chrome extension for seamless authentication
- Maintains comprehensive audit trail for compliance

### Technology Stack

- **Language:** Go 1.25+
- **Web UI:** HTML templates, vanilla JavaScript, WebSockets
- **Storage:** SQLite with FTS5 (full-text search) and vector embeddings
- **LLM Integration:** Mode-specific (Gemini API or embedded llama.cpp)
- **Browser Automation:** rod (for web scraping)
- **Authentication:** Chrome extension → WebSocket → HTTP service
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Banner:** github.com/ternarybob/banner (startup display)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2

---

## Security Modes (CRITICAL REQUIREMENTS)

### Rule 1: Mode Enforcement

**REQUIREMENT:** The system MUST prevent cloud mode usage with sensitive data.

**Implementation Requirements:**
1. Mode must be explicitly configured (no defaults)
2. Cloud mode requires `confirm_risk = true` flag
3. Cloud mode displays WARNING on every startup
4. Offline mode verifies model files exist before starting
5. Mode cannot be changed at runtime (requires restart)

### Rule 2: Data Classification

**REQUIREMENT:** Documentation MUST clearly state when each mode is required.

**Offline Mode is MANDATORY for:**
- Government data (any level: local, state, federal)
- Healthcare records (HIPAA, privacy legislation)
- Financial information (customer data, internal financials)
- Personal information (PII, employee records)
- Confidential business data (trade secrets, strategic plans)
- Any data where breach would cause legal/reputational harm

**Cloud Mode is ACCEPTABLE for:**
- Personal notes and documentation
- Public documentation
- Non-confidential research
- Educational materials
- Data you own and accept risk for

**Violation Consequence:** Data breach, legal liability, reputational damage

### Rule 3: Audit Trail

**REQUIREMENT:** All LLM operations MUST be logged for compliance verification.

**Audit Log Requirements:**
1. Every embed/chat operation logged
2. Includes: timestamp, mode, operation, provider, success/failure
3. Does NOT include document content (metadata only)
4. Stored in SQLite `audit_log` table
5. Exportable to JSON for compliance reporting
6. Configurable retention period (default: 90 days)

**Purpose:** Prove no data was sent to external APIs in offline mode

### Rule 4: Network Isolation (Offline Mode)

**REQUIREMENT:** Offline mode MUST be verifiable as network-isolated.

**Implementation Requirements:**
1. No HTTP client creation in offline mode code paths
2. Sanity check for internet connectivity on startup (log warning if detected)
3. All inference occurs via local llama.cpp bindings
4. Model files loaded from local disk
5. No DNS lookups, no socket connections

**Verification:** Code review + integration tests must confirm no network calls possible

### Rule 5: Risk Acknowledgment (Cloud Mode)

**REQUIREMENT:** Cloud mode MUST require explicit user acknowledgment.

**Implementation Requirements:**
1. Configuration flag: `confirm_risk = true` required
2. Startup banner shows WARNING in red/bold
3. Warning text includes:
   - "Data will be sent to Google Gemini API"
   - "Do NOT use with government, healthcare, or confidential data"
   - "You acknowledge and accept this risk"
4. Without acknowledgment, system refuses to start

**Example Warning:**
```
⚠️  ═══════════════════════════════════════════════════════════
⚠️  CLOUD MODE ACTIVE
⚠️  
⚠️  Data will be sent to Google Gemini API servers
⚠️  Do NOT use with:
⚠️    • Government data
⚠️    • Healthcare records
⚠️    • Financial information
⚠️    • Confidential business data
⚠️  
⚠️  You have acknowledged and accepted this risk
⚠️  ═══════════════════════════════════════════════════════════
```

---

## Deployment Modes

### Cloud Mode (Personal/Non-Sensitive Data)

**Use Case:** Personal knowledge management where cloud provider access is acceptable.

**Architecture:**
```
Single Go Binary
    ↓
    └─ Google Gemini API
       ├─ Embeddings: text-embedding-004 (768d)
       └─ Chat: gemini-1.5-flash
```

**Requirements:**
- Internet connectivity
- Google Gemini API key
- Explicit risk acknowledgment
- **NO Docker required**

**Setup Steps:**
```bash
# 1. Get API key from Google AI Studio
# https://aistudio.google.com/app/apikey

# 2. Configure Quaero
export GEMINI_API_KEY=your_key_here

# 3. Create config.toml
cat > config.toml << EOF
[llm]
mode = "cloud"

[llm.cloud]
api_key = "\${GEMINI_API_KEY}"
confirm_risk = true  # Required acknowledgment

[llm.cloud.embedding]
model = "text-embedding-004"
dimension = 768

[llm.cloud.chat]
model = "gemini-1.5-flash"
EOF

# 4. Run
./quaero serve --config config.toml
```

**Trade-offs:**
- ✅ Fast inference (~1-2 seconds per query)
- ✅ High-quality responses
- ✅ Simple setup
- ✅ No resource constraints
- ❌ Data sent to Google servers
- ❌ Requires internet
- ❌ Usage costs (minimal with free tier)
- ❌ NOT for sensitive data

### Offline Mode (Corporate/Government/Sensitive Data)

**Use Case:** Enterprise/government use where data MUST remain local.

**Architecture:**
```
Single Go Binary
    ↓
    └─ Embedded llama.cpp
       ├─ Embeddings: nomic-embed-text-v1.5.gguf (768d)
       └─ Chat: qwen2.5-7b-instruct-q4.gguf
```

**Requirements:**
- Model files (~5GB total)
- 8-16GB RAM
- Multi-core CPU (8+ cores recommended)
- **NO Docker required**
- **NO internet required** (after initial model download)

**Setup Steps:**
```bash
# 1. Download models (one-time, requires internet)
mkdir -p models

# Embedding model (~150MB)
curl -L -o models/nomic-embed-text-v1.5-q8.gguf \
  https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.q8_0.gguf

# Chat model (~4.5GB)
curl -L -o models/qwen2.5-7b-instruct-q4.gguf \
  https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_k_m.gguf

# 2. Verify checksums
sha256sum models/*.gguf
# nomic-embed-text-v1.5-q8.gguf: <checksum>
# qwen2.5-7b-instruct-q4.gguf: <checksum>

# 3. Create config.toml
cat > config.toml << EOF
[llm]
mode = "offline"

[llm.offline]
embed_model_path = "./models/nomic-embed-text-v1.5-q8.gguf"
chat_model_path = "./models/qwen2.5-7b-instruct-q4.gguf"
context_size = 4096
threads = 8
gpu_layers = 0  # Set > 0 if CUDA GPU available
EOF

# 4. Run (works completely offline)
./quaero serve --config config.toml
```

**Trade-offs:**
- ✅ All data stays local
- ✅ No network calls (verifiable)
- ✅ Works air-gapped
- ✅ Audit trail for compliance
- ✅ No ongoing costs
- ❌ Slower inference (~5-8 seconds per query)
- ❌ Lower quality than GPT-4/Claude
- ❌ Requires significant RAM
- ❌ Large model files

---

## Configuration System

### Priority Order

1. **CLI Flags** (highest priority)
2. **Environment Variables**
3. **Config File** (`config.toml`)
4. **Defaults** (lowest priority)

### Configuration File Format

**Complete Example:**

```toml
[server]
host = "localhost"
port = 8080

[logging]
level = "info"
format = "json"

[confluence]
base_url = "https://yourcompany.atlassian.net"

[jira]
base_url = "https://yourcompany.atlassian.net"

[github]
base_url = "https://api.github.com"
token = ""  # Set via environment variable

[storage]
type = "sqlite"
path = "./data/quaero.db"

[storage.sqlite]
enable_fts5 = true
enable_vector = true
cache_size_mb = 100
wal_mode = true

# ═══════════════════════════════════════════════════════════
# LLM CONFIGURATION - Choose ONE mode
# ═══════════════════════════════════════════════════════════

[llm]
mode = "offline"  # REQUIRED: "cloud" or "offline"

# ───────────────────────────────────────────────────────────
# Cloud Mode Configuration (for personal/non-sensitive use)
# ───────────────────────────────────────────────────────────
# [llm.cloud]
# api_key = "${GEMINI_API_KEY}"
# confirm_risk = true  # REQUIRED: Acknowledge data sent to Google
# 
# [llm.cloud.embedding]
# model = "text-embedding-004"
# dimension = 768
# 
# [llm.cloud.chat]
# model = "gemini-1.5-flash"
# temperature = 0.7
# max_tokens = 512

# ───────────────────────────────────────────────────────────
# Offline Mode Configuration (for sensitive data)
# ───────────────────────────────────────────────────────────
[llm.offline]
embed_model_path = "./models/nomic-embed-text-v1.5-q8.gguf"
chat_model_path = "./models/qwen2.5-7b-instruct-q4.gguf"
context_size = 4096
threads = 8
gpu_layers = 0  # Set to 35 for RTX 3090, 43 for RTX 4090, etc.

[processing]
schedule = "0 0 */6 * * *"  # Every 6 hours
enabled = true

[audit]
enabled = true
retention_days = 90
export_path = "./audit_logs"

# ═══════════════════════════════════════════════════════════
# PROCESSING ENGINE CONFIGURATION
# ═══════════════════════════════════════════════════════════

[processing]
# Enable/disable the background processing engine
enabled = true

# CRON expression for automated batch processing
# Format: second minute hour day month weekday
# Examples:
#   "0 0 */6 * * *"   - Every 6 hours
#   "0 0 0 * * *"     - Daily at midnight
#   "0 0 2 * * SUN"   - Sunday 2am
schedule = "0 0 */6 * * *"

# Maximum concurrent processing jobs
max_concurrent = 4

# Retry configuration for failed documents
retry_failed = true
max_retries = 3
retry_delay_minutes = 30
```

### Environment Variables

**Cloud Mode:**
```bash
# LLM Configuration
QUAERO_LLM_MODE=cloud
QUAERO_LLM_CLOUD_API_KEY=your_gemini_key_here
QUAERO_LLM_CLOUD_CONFIRM_RISK=true

# Server
QUAERO_PORT=8080
QUAERO_HOST=localhost
QUAERO_LOG_LEVEL=info

# Data Sources
QUAERO_GITHUB_TOKEN=ghp_xxx
```

**Offline Mode:**
```bash
# LLM Configuration
QUAERO_LLM_MODE=offline
QUAERO_LLM_OFFLINE_EMBED_MODEL_PATH=./models/nomic-embed-text-v1.5-q8.gguf
QUAERO_LLM_OFFLINE_CHAT_MODEL_PATH=./models/qwen2.5-7b-instruct-q4.gguf
QUAERO_LLM_OFFLINE_THREADS=8

# Server
QUAERO_PORT=8080
QUAERO_HOST=localhost
QUAERO_LOG_LEVEL=info

# Data Sources
QUAERO_GITHUB_TOKEN=ghp_xxx
```

---

## Approved Data Sources

**ONLY these collectors are approved:**

### 1. Confluence
- **Location:** `internal/services/atlassian/confluence_*`
- **Features:** Spaces, pages, attachments, images
- **API:** Confluence REST API v2
- **Authentication:** Cookies + token from Chrome extension

### 2. Jira
- **Location:** `internal/services/atlassian/jira_*`
- **Features:** Projects, issues, comments, attachments
- **API:** Jira REST API v3
- **Authentication:** Cookies + token from Chrome extension

### 3. GitHub
- **Location:** `internal/services/github/*`
- **Features:** Repositories, README files, wiki pages
- **API:** GitHub REST API v3
- **Authentication:** Personal access token

---

## LLM Service Requirements

### Interface Definition

**REQUIREMENT:** All LLM implementations must conform to this interface.

**Location:** `internal/services/llm/service.go`

```go
package llm

type Service interface {
    // Generate embedding for text (768 dimensions)
    Embed(ctx context.Context, text string) ([]float32, error)
    
    // Generate chat completion
    Chat(ctx context.Context, messages []Message) (string, error)
    
    // Health check
    HealthCheck(ctx context.Context) error
    
    // Get current mode
    GetMode() Mode
    
    // Get audit trail
    GetAuditLog() []AuditEntry
}

type Mode string

const (
    ModeCloud   Mode = "cloud"
    ModeOffline Mode = "offline"
)

type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
}

type AuditEntry struct {
    Timestamp   time.Time
    Mode        string  // "cloud" or "offline"
    Operation   string  // "embed", "chat"
    Provider    string  // "gemini" or "llama.cpp"
    DocumentID  string  // Optional
    Success     bool
    ErrorMsg    string
}
```

### Cloud Mode Implementation

**REQUIREMENT:** Gemini API client for embeddings and chat.

**Location:** `internal/services/llm/cloud/gemini.go`

**Key Features:**
- API key validation
- Risk acknowledgment verification
- Rate limiting (60 req/min for free tier)
- Comprehensive error handling
- Audit logging for all API calls
- Timeout handling (30 seconds default)

**Startup Validation:**
```go
func NewGeminiClient(config *Config, logger arbor.ILogger) (*GeminiClient, error) {
    // Validate API key
    if config.APIKey == "" {
        return nil, fmt.Errorf("GEMINI_API_KEY required for cloud mode")
    }
    
    // Verify risk acknowledgment
    if !config.ConfirmRisk {
        return nil, fmt.Errorf(
            "cloud mode requires explicit risk acceptance\n" +
            "Set confirm_risk = true in config to acknowledge:\n" +
            "  • Data will be sent to Google Gemini API\n" +
            "  • Do NOT use with sensitive data",
        )
    }
    
    // Display warning
    logger.Warn().Msg("⚠️  CLOUD MODE ACTIVE")
    logger.Warn().Msg("⚠️  Data will be sent to Google Gemini API")
    logger.Warn().Msg("⚠️  Do NOT use with government, healthcare, or confidential data")
    
    return &GeminiClient{
        apiKey:     config.APIKey,
        logger:     logger,
        auditLog:   NewAuditLog(logger),
    }, nil
}
```

### Offline Mode Implementation

**REQUIREMENT:** llama.cpp integration for local inference.

**Location:** `internal/services/llm/offline/llama.go`

**Key Features:**
- Model file validation (checksum verification)
- Memory management (models stay loaded)
- Thread pool for parallelism
- GPU support (optional, via gpu_layers config)
- Local audit logging
- Network isolation verification

**Startup Validation:**
```go
func NewEmbeddedLLM(config *Config, logger arbor.ILogger) (*EmbeddedLLM, error) {
    // Verify model files exist
    if !fileExists(config.EmbedModelPath) {
        return nil, fmt.Errorf("embedding model not found: %s", config.EmbedModelPath)
    }
    if !fileExists(config.ChatModelPath) {
        return nil, fmt.Errorf("chat model not found: %s", config.ChatModelPath)
    }
    
    // Verify checksums
    if err := verifyChecksum(config.EmbedModelPath, config.EmbedModelChecksum); err != nil {
        logger.Warn().Err(err).Msg("Embedding model checksum mismatch")
    }
    if err := verifyChecksum(config.ChatModelPath, config.ChatModelChecksum); err != nil {
        logger.Warn().Err(err).Msg("Chat model checksum mismatch")
    }
    
    logger.Info().Msg("✓ OFFLINE MODE ACTIVE")
    logger.Info().Msg("✓ All processing will be local")
    logger.Info().Str("embed_model", config.EmbedModelPath).Msg("Loading embedding model")
    logger.Info().Str("chat_model", config.ChatModelPath).Msg("Loading chat model")
    
    // Load models (this takes 10-30 seconds)
    embedModel, err := llama.New(
        config.EmbedModelPath,
        llama.SetContext(512),
        llama.SetEmbeddings(true),
        llama.SetThreads(config.Threads),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load embedding model: %w", err)
    }
    
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
    
    logger.Info().Msg("✓ Models loaded successfully")
    
    return &EmbeddedLLM{
        embedModel: embedModel,
        chatModel:  chatModel,
        logger:     logger,
        auditLog:   NewAuditLog(logger),
        config:     config,
    }, nil
}
```

---

## Required Libraries

### Mandatory Dependencies

**MUST USE:**
- `github.com/ternarybob/arbor` - All logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML configuration
- `github.com/go-skynet/go-llama.cpp` - Offline mode inference

**FORBIDDEN:**
- `fmt.Println` / `log.Println` for logging
- Any other logging library
- Any other config format (JSON, YAML)
- `github.com/ollama/ollama` - Use llama.cpp directly instead

---

## Startup Sequence

**REQUIRED ORDER in `main.go`:**

```go
func main() {
    // 1. Configuration Loading
    config, err := common.LoadFromFile(configPath)
    if err != nil {
        log.Fatal(err)
    }
    
    // 2. CLI Overrides
    common.ApplyCLIOverrides(config, serverPort, serverHost)
    
    // 3. Logger Initialization
    logger := common.InitLogger(config)
    
    // 4. Banner Display (MANDATORY - shows mode)
    common.PrintBanner(config, logger)
    
    // 5. Version Logging
    version := common.GetVersion()
    logger.Info().Str("version", version).Msg("Quaero starting")
    
    // 6. LLM Mode Validation (CRITICAL)
    if err := common.ValidateLLMConfig(config); err != nil {
        logger.Fatal().Err(err).Msg("Invalid LLM configuration")
    }
    
    // 7. LLM Service Initialization
    llmService, err := llm.NewService(config.LLM, logger)
    if err != nil {
        logger.Fatal().Err(err).Msg("Failed to initialize LLM service")
    }
    
    // 8. Storage Initialization
    db := sqlite.NewSQLiteDB(config, logger)
    
    // 9. Other Services
    embeddingService := embeddings.NewService(llmService, logger)
    documentService := documents.NewService(documentStorage, embeddingService, logger)
    processingService := processing.NewService(documentService, jiraStorage, confluenceStorage, logger)
    
    // 10. Scheduler
    scheduler := processing.NewScheduler(processingService, logger)
    scheduler.Start(config.Processing.Schedule)
    
    // 11. Handlers
    handlers := initHandlers(logger, documentService, processingService, ...)
    
    // 12. Server Start
    server := server.New(logger, config, handlers)
    server.Start()
}
```

---

## Banner Requirement

### MANDATORY Display

**MUST use:** `github.com/ternarybob/banner`

**MUST show:** Mode (cloud/offline) prominently

**Implementation:**
```go
import "github.com/ternarybob/banner"

func PrintBanner(cfg *Config, logger arbor.ILogger) {
    b := banner.New()
    b.SetTitle("Quaero")
    b.SetSubtitle("Knowledge Search System")
    b.AddLine("Version", common.GetVersion())
    
    // MODE IS CRITICAL INFORMATION
    if cfg.LLM.Mode == "cloud" {
        b.AddLine("Mode", "⚠️  CLOUD (data sent to API)")
    } else {
        b.AddLine("Mode", "✓ OFFLINE (local processing)")
    }
    
    b.AddLine("Server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
    b.AddLine("Config", cfg.LoadedFrom)
    b.Print()
    
    // Additional cloud mode warning
    if cfg.LLM.Mode == "cloud" {
        logger.Warn().Msg("═══════════════════════════════════════════════")
        logger.Warn().Msg("  CLOUD MODE: Data sent to external APIs")
        logger.Warn().Msg("  Do NOT use with sensitive data")
        logger.Warn().Msg("═══════════════════════════════════════════════")
    }
}
```

---

## Logging Standards

### Required Patterns

**Structured Logging:**
```go
logger.Info().
    Str("mode", "offline").
    Str("operation", "embed").
    Dur("duration", elapsed).
    Msg("Embedding generated")
```

**Cloud Mode API Calls:**
```go
logger.Info().
    Str("mode", "cloud").
    Str("provider", "gemini").
    Str("operation", "embed").
    Int("status_code", resp.StatusCode).
    Dur("latency", elapsed).
    Msg("API call completed")
```

**Offline Mode Operations:**
```go
logger.Info().
    Str("mode", "offline").
    Str("provider", "llama.cpp").
    Str("operation", "chat").
    Int("threads", config.Threads).
    Dur("inference_time", elapsed).
    Msg("Local inference completed")
```

---

## Audit Log Requirements

### Schema

**SQLite Table:**
```sql
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    mode TEXT NOT NULL,  -- 'cloud' or 'offline'
    operation TEXT NOT NULL,  -- 'embed', 'chat', 'search'
    provider TEXT NOT NULL,  -- 'gemini' or 'llama.cpp'
    document_id TEXT,  -- Optional: metadata only, NOT content
    success BOOLEAN NOT NULL,
    error_message TEXT,
    latency_ms INTEGER,
    created_at INTEGER DEFAULT (strftime('%s', 'now'))
);

CREATE INDEX idx_audit_timestamp ON audit_log(timestamp);
CREATE INDEX idx_audit_mode ON audit_log(mode);
CREATE INDEX idx_audit_operation ON audit_log(operation);
```

### Implementation

**Location:** `internal/services/llm/audit.go`

```go
type AuditLog struct {
    storage interfaces.AuditStorage
    logger  arbor.ILogger
    mu      sync.RWMutex
}

func (a *AuditLog) Record(entry AuditEntry) {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    // Log to structured logger
    a.logger.Info().
        Str("mode", entry.Mode).
        Str("operation", entry.Operation).
        Bool("success", entry.Success).
        Msg("LLM operation")
    
    // Persist to SQLite
    if err := a.storage.SaveAuditEntry(entry); err != nil {
        a.logger.Error().Err(err).Msg("Failed to save audit entry")
    }
}

func (a *AuditLog) Export(since time.Time, format string) ([]byte, error) {
    entries, err := a.storage.GetAuditEntries(since, time.Now())
    if err != nil {
        return nil, err
    }
    
    switch format {
    case "json":
        return json.MarshalIndent(entries, "", "  ")
    case "csv":
        return exportToCSV(entries)
    default:
        return nil, fmt.Errorf("unsupported format: %s", format)
    }
}
```

---

## Directory Structure Standards

### internal/common/ - Stateless Utilities

**Rules:**
- ✅ Pure functions only
- ✅ No state
- ❌ NO receiver methods

### internal/services/ - Stateful Services

**Rules:**
- ✅ MUST use receiver methods
- ✅ State management
- ✅ Implement interfaces

### internal/services/llm/ - LLM Service (NEW)

**Structure:**
```
internal/services/llm/
├── service.go           # Interface definition
├── factory.go           # Mode-based factory
├── audit.go             # Audit log system
├── cloud/               # Cloud mode implementation
│   ├── gemini.go        # Gemini API client
│   └── gemini_test.go   # Unit tests
└── offline/             # Offline mode implementation
    ├── llama.go         # llama.cpp integration
    ├── models.go        # Model management
    └── llama_test.go    # Unit tests
```

---

## Error Handling

### Configuration Errors

**MUST provide helpful error messages:**

```go
// Bad
return fmt.Errorf("invalid config")

// Good
return fmt.Errorf(
    "invalid LLM configuration: mode must be 'cloud' or 'offline', got '%s'\n" +
    "See config.toml.example for correct configuration",
    config.Mode,
)
```

### Cloud Mode Errors

```go
// Missing API key
if apiKey == "" {
    return fmt.Errorf(
        "GEMINI_API_KEY required for cloud mode\n" +
        "Get your API key from: https://aistudio.google.com/app/apikey\n" +
        "Then set: export GEMINI_API_KEY=your_key_here",
    )
}

// Missing risk acknowledgment
if !config.ConfirmRisk {
    return fmt.Errorf(
        "cloud mode requires explicit risk acceptance\n" +
        "Add to config.toml:\n" +
        "  [llm.cloud]\n" +
        "  confirm_risk = true\n" +
        "This acknowledges data will be sent to Google Gemini API",
    )
}
```

### Offline Mode Errors

```go
// Missing model files
if !fileExists(modelPath) {
    return fmt.Errorf(
        "model file not found: %s\n" +
        "Download models with:\n" +
        "  mkdir -p models\n" +
        "  curl -L -o models/qwen2.5-7b-instruct-q4.gguf \\\n" +
        "    https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_k_m.gguf",
        modelPath,
    )
}
```

---

## Testing Standards

### Test Coverage Goals

- **Critical paths:** 100%
- **LLM services:** 90%+
- **Configuration validation:** 100%
- **Audit logging:** 100%
- **Services:** 80%+
- **Handlers:** 80%+

### Required Tests

**LLM Service Tests:**
```
internal/services/llm/cloud/gemini_test.go:
  - API key validation
  - Risk acknowledgment requirement
  - Embedding generation
  - Chat completion
  - Error handling
  - Audit logging

internal/services/llm/offline/llama_test.go:
  - Model file validation
  - Model loading
  - Embedding generation
  - Chat completion
  - Error handling
  - Audit logging

internal/services/llm/factory_test.go:
  - Mode selection
  - Configuration validation
  - Service instantiation
```

**Integration Tests:**
```
test/integration/llm_cloud_test.go:
  - End-to-end cloud mode workflow
  - API communication
  - Rate limiting
  - Error scenarios

test/integration/llm_offline_test.go:
  - End-to-end offline mode workflow
  - Model inference
  - Network isolation verification
  - Performance benchmarks
```

### Testing Commands

**ALWAYS use the test script:**
```bash
./test/run-tests.ps1 -Type all
./test/run-tests.ps1 -Type unit
./test/run-tests.ps1 -Type integration
```

---

## Build Standards

**ALWAYS use the build script:**
```bash
./scripts/build.ps1
./scripts/build.ps1 -Clean -Release
```

**Build Output:**
```
bin/
├── quaero          # Main binary
└── version.txt     # Build info
```

---

## Deployment Checklist

### Cloud Mode Deployment

- [ ] Obtain Gemini API key
- [ ] Set environment variable: `GEMINI_API_KEY`
- [ ] Configure `llm.mode = "cloud"` in config.toml
- [ ] Set `confirm_risk = true`
- [ ] Verify API key works: `./quaero test-llm`
- [ ] Review startup warnings
- [ ] Configure audit log retention
- [ ] Deploy binary

### Offline Mode Deployment

- [ ] Download model files (~5GB)
- [ ] Verify model checksums
- [ ] Configure `llm.mode = "offline"` in config.toml
- [ ] Set model paths in config
- [ ] Configure thread count for CPU
- [ ] Configure GPU layers (if applicable)
- [ ] Test model loading: `./quaero test-llm`
- [ ] Verify network isolation
- [ ] Configure audit log retention
- [ ] Deploy binary + model files

---

## Chrome Extension

### Purpose

Captures authentication credentials from Atlassian sites (Confluence, Jira) and sends to Quaero server.

### Integration Flow

1. User navigates to Confluence/Jira in Chrome
2. User clicks extension icon
3. Extension captures cookies and tokens
4. Extension connects to `ws://localhost:8080/ws`
5. Extension sends `AuthData` message
6. Server receives and stores credentials
7. Collectors use credentials for API calls

### Installation

1. Open Chrome Extensions (`chrome://extensions/`)
2. Enable "Developer mode"
3. Click "Load unpacked"
4. Select `cmd/quaero-chrome-extension/`

---

## Compliance Rules

### Required Libraries

✅ **MUST USE:**
- `github.com/ternarybob/arbor` - Logging
- `github.com/ternarybob/banner` - Banners
- `github.com/pelletier/go-toml/v2` - TOML config
- `github.com/go-skynet/go-llama.cpp` - Offline inference

❌ **FORBIDDEN:**
- `fmt.Println` / `log.Println` for logging
- Any other logging library
- Any other config format (JSON, YAML)

### Architecture Rules

✅ **REQUIRED:**
- Stateless functions in `internal/common/`
- Receiver methods in `internal/services/`
- Interface injection in `internal/handlers/`
- Banner on startup showing mode
- Structured logging for all operations
- Audit trail for all LLM operations
- Configuration validation on startup

❌ **FORBIDDEN:**
- Receiver methods in `internal/common/`
- Direct service instantiation in handlers
- Ignored errors
- TODO/FIXME in committed code
- Hardcoded credentials
- Network calls in offline mode code paths

---

## Summary: Mode Comparison

| Aspect | Cloud Mode | Offline Mode |
|--------|-----------|--------------|
| **Use Case** | Personal/non-sensitive | Government/corporate/sensitive |
| **Infrastructure** | Gemini API | llama.cpp + model files |
| **Setup Complexity** | Simple (API key) | Moderate (model download) |
| **Docker Required** | No | No |
| **Internet Required** | Always | Setup only |
| **Data Privacy** | ❌ Sent to Google | ✅ Stays local |
| **Performance** | Fast (~1-2 sec) | Slower (~5-8 sec) |
| **Quality** | High (Gemini 1.5) | Good (Qwen 2.5 7B) |
| **Cost** | API usage | One-time (compute) |
| **Risk Acknowledgment** | Required | Not required |
| **Audit Trail** | API calls logged | Local ops logged |
| **Compliance** | ❌ Not for regulated data | ✅ Audit-ready |

---

**Last Updated:** 2025-10-06
**Status:** Active Development
**Version:** 3.0