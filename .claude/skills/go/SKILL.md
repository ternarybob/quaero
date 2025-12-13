# Go Skill for Quaero

## Purpose
Specialized Go development patterns for the Quaero knowledge base system.

## Project Context
- **Language:** Go 1.25+
- **Storage:** BadgerDB (embedded key-value store)
- **Web UI:** HTML templates, Alpine.js, Bulma CSS
- **Crawler:** chromedp for JavaScript rendering
- **Job Queue:** Badger-backed persistent queue
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2
- **LLM:** Google ADK with Gemini models
- **MCP:** Model Context Protocol for internal agent tools

## Package Structure
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
│   ├── services/                    # Stateful business services
│   │   ├── crawler/                 # Website crawler service
│   │   ├── events/                  # Pub/sub event service
│   │   ├── scheduler/               # Cron scheduler
│   │   ├── llm/                     # LLM abstraction layer (Google ADK)
│   │   ├── documents/               # Document service
│   │   ├── chat/                    # Chat service (RAG)
│   │   ├── search/                  # Search service
│   │   └── jobs/                    # Job executor & registry
│   │       ├── executor.go          # Job definition executor
│   │       ├── registry.go          # Action type registry
│   │       └── actions/             # Action handlers
│   ├── queue/                       # Badger-backed job queue
│   │   ├── badger_manager.go        # Queue manager
│   │   └── types.go                 # Queue message types
│   ├── jobs/                        # Job management
│   │   ├── manager.go               # Job CRUD operations
│   │   └── types/                   # Job type implementations
│   ├── storage/                     # Data persistence layer
│   │   ├── factory.go               # Storage factory
│   │   └── badger/                  # Badger implementation
│   ├── interfaces/                  # Service interfaces
│   └── models/                      # Data models
├── pages/                           # Web UI templates + static
├── test/                            # Go-native test infrastructure
│   ├── api/                         # API integration tests
│   └── ui/                          # UI tests (chromedp)
├── scripts/                         # Build scripts (build.ps1, build.sh)
└── docs/                            # Documentation
```

## Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process document %s: %w", docID, err)
}

// Use arbor structured logging
logger.Error("document processing failed",
    "doc_id", docID,
    "error", err,
)
```

## Handler Pattern
```go
// Handlers receive dependencies via struct (constructor DI)
type DocumentHandler struct {
    storage    interfaces.DocumentStorage
    logger     *arbor.Logger
    eventSvc   interfaces.EventService
}

func NewDocumentHandler(storage interfaces.DocumentStorage, logger *arbor.Logger, eventSvc interfaces.EventService) *DocumentHandler {
    return &DocumentHandler{
        storage:  storage,
        logger:   logger,
        eventSvc: eventSvc,
    }
}

func (h *DocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
    // Chi URL params
    id := chi.URLParam(r, "id")
    
    doc, err := h.storage.GetByID(r.Context(), id)
    if err != nil {
        h.logger.Error("failed to get document", "id", id, "error", err)
        http.Error(w, "document not found", http.StatusNotFound)
        return
    }
    
    // Return JSON or render template
    if r.Header.Get("Accept") == "application/json" {
        render.JSON(w, r, doc)
        return
    }
    h.renderTemplate(w, "document.html", doc)
}
```

## Service Layer
```go
// Services implement interfaces, receive dependencies via constructor
type DocumentService struct {
    storage  interfaces.DocumentStorage
    logger   *arbor.Logger
    eventSvc interfaces.EventService
}

func NewDocumentService(storage interfaces.DocumentStorage, logger *arbor.Logger, eventSvc interfaces.EventService) *DocumentService {
    return &DocumentService{
        storage:  storage,
        logger:   logger,
        eventSvc: eventSvc,
    }
}

// Methods return (result, error) - never panic
func (s *DocumentService) GetByID(ctx context.Context, id string) (*models.Document, error) {
    doc, err := s.storage.GetByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get document %s: %w", id, err)
    }
    return doc, nil
}
```

## Storage Layer (BadgerDB)
```go
// Storage implements interfaces for testability
type BadgerDocumentStorage struct {
    db     *badger.DB
    logger *arbor.Logger
}

func (s *BadgerDocumentStorage) GetByID(ctx context.Context, id string) (*models.Document, error) {
    var doc models.Document
    err := s.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte("doc:" + id))
        if err != nil {
            return err
        }
        return item.Value(func(val []byte) error {
            return json.Unmarshal(val, &doc)
        })
    })
    if err == badger.ErrKeyNotFound {
        return nil, fmt.Errorf("document not found: %s", id)
    }
    return &doc, err
}

func (s *BadgerDocumentStorage) Save(ctx context.Context, doc *models.Document) error {
    return s.db.Update(func(txn *badger.Txn) error {
        data, err := json.Marshal(doc)
        if err != nil {
            return fmt.Errorf("marshal document: %w", err)
        }
        return txn.Set([]byte("doc:"+doc.ID), data)
    })
}
```

## Job Queue (Badger-backed)
```go
// Job types as constants
const (
    JobTypeCrawlerURL  = "crawler_url"
    JobTypeSummarizer  = "summarizer"
    JobTypeCleanup     = "cleanup"
)

// Queue message structure
type Message struct {
    ID         string          `json:"id"`
    JobID      string          `json:"job_id"`
    Type       string          `json:"type"`
    Payload    json.RawMessage `json:"payload"`
    CreatedAt  time.Time       `json:"created_at"`
    ReceiveCount int           `json:"receive_count"`
}

// QueueManager interface
type QueueManager interface {
    Enqueue(ctx context.Context, msg Message) error
    Receive(ctx context.Context) (*Message, func() error, error)
    Stats(ctx context.Context) (*QueueStats, error)
}

// Enqueue a crawl job
func (q *BadgerQueueManager) EnqueueCrawl(ctx context.Context, jobID string, url string, depth int) error {
    payload, _ := json.Marshal(CrawlPayload{URL: url, Depth: depth})
    return q.Enqueue(ctx, Message{
        ID:        uuid.New().String(),
        JobID:     jobID,
        Type:      JobTypeCrawlerURL,
        Payload:   payload,
        CreatedAt: time.Now(),
    })
}
```

## Manager/Worker Job Architecture
```go
// JobManager handles job CRUD and status tracking
type JobManager interface {
    CreateJob(ctx context.Context, job *models.Job) error
    UpdateJobStatus(ctx context.Context, id string, status string) error
    GetJob(ctx context.Context, id string) (*models.Job, error)
    SetJobResult(ctx context.Context, id string, result interface{}) error
    SetJobError(ctx context.Context, id string, err error) error
}

// JobExecutor orchestrates multi-step workflows from JobDefinitions
type JobExecutor struct {
    jobManager   interfaces.JobManager
    queueManager interfaces.QueueManager
    stepRegistry map[string]StepExecutor
    logger       *arbor.Logger
}

func (e *JobExecutor) Execute(ctx context.Context, jobDef *models.JobDefinition) (string, error) {
    // Create parent job
    parentJob := &models.Job{
        ID:     uuid.New().String(),
        Type:   jobDef.Type,
        Status: "running",
    }
    if err := e.jobManager.CreateJob(ctx, parentJob); err != nil {
        return "", fmt.Errorf("create parent job: %w", err)
    }
    
    // Execute steps sequentially
    for _, step := range jobDef.Steps {
        executor, ok := e.stepRegistry[step.Action]
        if !ok {
            return "", fmt.Errorf("unknown action: %s", step.Action)
        }
        if err := executor.Execute(ctx, parentJob.ID, step); err != nil {
            e.jobManager.SetJobError(ctx, parentJob.ID, err)
            return parentJob.ID, err
        }
    }
    
    return parentJob.ID, nil
}

// WorkerPool processes queue messages
type WorkerPool struct {
    queueManager interfaces.QueueManager
    jobManager   interfaces.JobManager
    executors    map[string]Executor
    concurrency  int
    logger       *arbor.Logger
}

func (w *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < w.concurrency; i++ {
        go w.worker(ctx, i)
    }
}

func (w *WorkerPool) worker(ctx context.Context, id int) {
    for {
        msg, deleteFn, err := w.queueManager.Receive(ctx)
        if err != nil {
            continue
        }
        
        w.jobManager.UpdateJobStatus(ctx, msg.JobID, "running")
        
        executor := w.executors[msg.Type]
        if err := executor.Execute(ctx, msg.JobID, msg.Payload); err != nil {
            w.jobManager.SetJobError(ctx, msg.JobID, err)
        } else {
            w.jobManager.UpdateJobStatus(ctx, msg.JobID, "completed")
        }
        
        deleteFn() // Remove from queue
    }
}
```

## Logging (arbor)
```go
// Use structured logging throughout
logger := arbor.NewLogger("service-name")

logger.Info("processing document", "doc_id", docID, "source", source)
logger.Error("failed to crawl", "url", url, "error", err)
logger.Debug("queue stats", "pending", stats.Pending, "active", stats.Active)

// Log service for real-time events (WebSocket broadcast)
type LogService interface {
    Log(level string, message string, fields ...interface{})
    SendEvent(eventType string, payload interface{})
}
```

## Configuration (TOML)
```go
// Config structure matches quaero.toml
type Config struct {
    Server  ServerConfig  `toml:"server"`
    Storage StorageConfig `toml:"storage"`
    Gemini  GeminiConfig  `toml:"gemini"`
    Jobs    JobsConfig    `toml:"jobs"`
    Logging LoggingConfig `toml:"logging"`
    Queue   QueueConfig   `toml:"queue"`
}

type GeminiConfig struct {
    GoogleAPIKey string `toml:"google_api_key"`
    AgentModel   string `toml:"agent_model"`
    ChatModel    string `toml:"chat_model"`
    Timeout      string `toml:"timeout"`
    RateLimit    string `toml:"rate_limit"`
}

// Load with go-toml
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }
    var cfg Config
    if err := toml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    return &cfg, nil
}
```

## Testing
```go
// Table-driven tests with test fixtures
func TestDocumentStorage_GetByID(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        setup   func(*badger.DB)
        want    *models.Document
        wantErr bool
    }{
        {
            name: "valid id",
            id:   "doc-123",
            setup: func(db *badger.DB) {
                // Insert test document
            },
            want:    &models.Document{ID: "doc-123"},
            wantErr: false,
        },
        {
            name:    "not found",
            id:      "missing",
            setup:   func(db *badger.DB) {},
            want:    nil,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            db := setupTestBadger(t)
            tt.setup(db)
            
            storage := NewBadgerDocumentStorage(db, arbor.NewLogger("test"))
            got, err := storage.GetByID(context.Background(), tt.id)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            // Assert document fields...
        })
    }
}

// Test infrastructure in test/ directory
// - test/main_test.go - TestMain fixture
// - test/helpers.go - Common utilities
// - test/api/ - API integration tests
// - test/ui/ - UI tests (chromedp)
```

## Build Commands
```bash
# Windows (PowerShell) - ALWAYS use build scripts
.\scripts\build.ps1           # Development build
.\scripts\build.ps1 -Deploy   # Build and deploy to bin/
.\scripts\build.ps1 -Run      # Build, deploy, and run

# Linux/macOS
./scripts/build.sh            # Development build
./scripts/build.sh --release  # Release build
./scripts/build.sh --test     # Build with tests

# Testing
cd test
go test -v ./api              # API tests
go test -v ./ui               # UI tests (requires Chrome)

# Unit tests (colocated)
go test ./internal/...
```

## Rules

1. **Always use build scripts** - Never `go build` directly (versioning, assets)
2. **No binaries in repo root** - Build outputs go to `bin/`
3. **Context everywhere** - Pass `context.Context` to all I/O calls
4. **Structured logging** - Use arbor with key-value pairs
5. **Wrap errors** - Always add context with `%w`
6. **Interface-based DI** - Services depend on interfaces, not implementations
7. **Constructor injection** - All dependencies via `NewXxx()` functions
8. **Keep handlers thin** - Business logic in services
9. **Tests in proper locations** - Unit tests colocated, integration in `test/`

## Anti-Patterns to Avoid

```go
// ❌ Don't use global state
var db *badger.DB

// ❌ Don't panic on errors  
if err != nil {
    panic(err)
}

// ❌ Don't ignore context
func DoWork() error { // missing ctx

// ❌ Don't use bare errors
return err // no context

// ❌ Don't use fmt.Println for logging
fmt.Println("processing...")  // Use arbor logger

// ❌ Don't put business logic in handlers
func (h *Handler) CreateDoc(w http.ResponseWriter, r *http.Request) {
    // 50 lines of logic here - WRONG, use services
}

// ❌ Don't bypass build scripts
go build ./cmd/quaero  // WRONG - use scripts/build.ps1 or build.sh
```

## Integration with Alpine.js + Bulma

Templates expose data for Alpine:
```html
<div class="container" x-data="{ docs: {{ .Documents | json }}, loading: false }">
    <div class="columns is-multiline">
        <template x-for="doc in docs" :key="doc.id">
            <div class="column is-4">
                <div class="card">
                    <div class="card-content">
                        <p class="title is-5" x-text="doc.title"></p>
                    </div>
                </div>
            </div>
        </template>
    </div>
</div>
```

Handler provides JSON helper:
```go
template.FuncMap{
    "json": func(v any) template.JS {
        b, _ := json.Marshal(v)
        return template.JS(b)
    },
}
```