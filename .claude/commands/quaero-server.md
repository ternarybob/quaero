# quaero-server

Implements the HTTP server for Quaero, handling authentication reception from browser extension and serving API endpoints.

## Usage

```
/quaero-server <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: Server Infrastructure

1. **Server Core** (`internal/server/server.go`)
   - HTTP server initialization
   - Route configuration
   - Middleware setup
   - Graceful shutdown

2. **Server Structure**
   ```go
   type Server struct {
       app      *app.App
       router   *http.ServeMux
       server   *http.Server
       logger   *arbor.Logger
       config   *ServerConfig
   }

   type ServerConfig struct {
       Host string
       Port int
   }
   ```

### Phase 2: Route Definitions

1. **Routes** (`internal/server/routes.go`)
   - `/api/auth` - Receive auth from extension
   - `/api/status` - Server and collection status
   - `/health` - Health check endpoint
   - `/api/query` (optional) - Query endpoint
   - `/api/collectors` (optional) - Collector management

2. **Route Setup**
   ```go
   func (s *Server) setupRoutes() *http.ServeMux {
       mux := http.NewServeMux()

       // Auth endpoint (primary)
       mux.HandleFunc("/api/auth", s.handleAuth)

       // Status endpoints
       mux.HandleFunc("/api/status", s.handleStatus)
       mux.HandleFunc("/health", s.handleHealth)

       // Optional API endpoints
       if s.config.EnableAPI {
           mux.HandleFunc("/api/query", s.handleQuery)
           mux.HandleFunc("/api/collectors", s.handleCollectors)
       }

       return mux
   }
   ```

### Phase 3: HTTP Handlers

1. **Auth Handler** (`internal/server/handlers.go`)
   - Receives ExtensionAuthData from browser extension
   - Validates request
   - Stores auth via AuthManager
   - Triggers collection orchestrator
   - Returns success response

2. **Status Handler**
   - Returns server status
   - Collection status for each source
   - Last collection time
   - Document count

3. **Health Check Handler**
   - Simple OK response
   - For monitoring/orchestration

### Phase 4: Middleware

1. **Logging Middleware** (`internal/server/middleware.go`)
   - Request logging with arbor
   - Response time tracking
   - Error logging

2. **CORS Middleware**
   - Allow extension origin
   - Handle preflight requests

3. **Recovery Middleware**
   - Panic recovery
   - Error responses

### Phase 5: Application Orchestration

1. **App Container** (`internal/app/app.go`)
   - Holds all application components
   - Initializes dependencies
   - Provides components to server

2. **App Structure**
   ```go
   type App struct {
       Config          *common.Config
       Logger          *arbor.Logger
       AuthManager     *auth.Manager
       Collector       *collector.Orchestrator
       Storage         storage.Storage
       RAG             rag.Engine
   }

   func New() *App {
       // Load config
       config := common.LoadFromFile("config.toml")

       // Init logger
       logger := common.InitLogger(config)

       // Init storage
       storage := ravendb.NewStore(config.Storage, logger)

       // Init auth
       authManager := auth.NewManager(logger)

       // Init collector
       collector := collector.NewOrchestrator(authManager, storage, logger)

       // Init RAG
       ragEngine := rag.NewEngine(storage, llm, config, logger)

       return &App{
           Config:      config,
           Logger:      logger,
           AuthManager: authManager,
           Collector:   collector,
           Storage:     storage,
           RAG:         ragEngine,
       }
   }
   ```

### Phase 6: Server Command

1. **Serve Command** (`cmd/quaero/serve.go`)
   - Cobra command for `quaero serve`
   - Initialize App
   - Create and start Server
   - Handle signals for graceful shutdown

2. **Command Implementation**
   ```go
   var serveCmd = &cobra.Command{
       Use:   "serve",
       Short: "Start HTTP server",
       Long:  `Starts Quaero server to receive auth from extension`,
       Run:   runServe,
   }

   func runServe(cmd *cobra.Command, args []string) {
       // Initialize app
       app := app.New()

       // Create server
       srv := server.New(app, config.Server.Host, config.Server.Port, logger)

       logger.Info("Starting Quaero server", "host", config.Server.Host, "port", config.Server.Port)
       logger.Info("Waiting for authentication from browser extension...")

       // Start server
       if err := srv.Start(); err != nil {
           logger.Fatal("Server failed", "error", err)
       }
   }
   ```

### Phase 7: Testing

1. **Handler Tests** (`internal/server/handlers_test.go`)
   - Test auth endpoint
   - Test status endpoint
   - Test health endpoint
   - Mock AuthManager and Collector

2. **Integration Tests** (`test/integration/server_test.go`)
   - Full server startup
   - Extension auth flow
   - Concurrent requests

## Server Architecture

```
Browser Extension
   ↓ POST /api/auth
Server (HTTP)
   ↓ Validate & Parse
AuthManager.StoreAuth()
   ↓
Orchestrator.TriggerCollection()
   ↓ Background
CollectorWorkers → Sources → Storage
```

## Code Structure

```go
// internal/server/server.go
type Server struct {
    app    *app.App
    router *http.ServeMux
    server *http.Server
    logger *arbor.Logger
}

func New(app *app.App, host string, port int, logger *arbor.Logger) *Server {
    s := &Server{
        app:    app,
        logger: logger,
    }

    s.router = s.setupRoutes()

    s.server = &http.Server{
        Addr:         fmt.Sprintf("%s:%d", host, port),
        Handler:      s.withMiddleware(s.router),
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
    }

    return s
}

func (s *Server) Start() error {
    return s.server.ListenAndServe()
}

// internal/server/handlers.go
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse auth data
    var authData auth.ExtensionAuthData
    if err := json.NewDecoder(r.Body).Decode(&authData); err != nil {
        s.logger.Error("Failed to parse auth data", "error", err)
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Store auth
    if err := s.app.AuthManager.StoreAuth("confluence", &authData); err != nil {
        s.logger.Error("Failed to store auth", "error", err)
        http.Error(w, "Internal error", http.StatusInternalServerError)
        return
    }

    s.app.AuthManager.StoreAuth("jira", &authData)

    // Trigger collection
    go s.app.Collector.TriggerCollection()

    // Success response
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "success",
    })
}
```

## Test-Driven Development (TDD) Workflow

**CRITICAL**: Follow TDD methodology for ALL code implementation.

### TDD Cycle (Red-Green-Refactor)

For EACH component, function, or feature:

1. **RED - Write Failing Test First**
   ```bash
   # Create test file before implementation
   touch internal/component/component_test.go

   # Write test that describes desired behavior
   func TestComponentBehavior(t *testing.T) {
       // Arrange - setup test data
       // Act - call the function (doesn't exist yet)
       // Assert - verify expected behavior
   }

   # Run test - should FAIL
   go test ./internal/component/... -v
   # Output: undefined: ComponentFunction
   ```

2. **GREEN - Write Minimal Code to Pass**
   ```bash
   # Implement just enough code to make test pass
   # Run test again
   go test ./internal/component/... -v
   # Output: PASS
   ```

3. **REFACTOR - Improve Code Quality**
   ```bash
   # Refactor while keeping tests green
   # Run test after each change
   go test ./internal/component/... -v
   ```

4. **REPEAT** - For next feature/function

### Testing Requirements by Component

**Before writing ANY implementation code:**

1. **Interfaces** - Create interface test
   ```go
   func TestInterfaceImplementation(t *testing.T) {
       var _ models.Source = (*ConfluenceSource)(nil) // Compile-time check
   }
   ```

2. **Core Functions** - Table-driven tests
   ```go
   func TestProcessMarkdown(t *testing.T) {
       tests := []struct{
           name     string
           input    string
           expected string
       }{
           {"basic html", "<p>test</p>", "test"},
           // Add more cases
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               result := ProcessMarkdown(tt.input)
               assert.Equal(t, tt.expected, result)
           })
       }
   }
   ```

3. **API Clients** - Mock HTTP responses
   ```go
   func TestGetPages(t *testing.T) {
       // Setup mock server
       server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           w.WriteHeader(200)
           json.NewEncoder(w).Encode(mockPages)
       }))
       defer server.Close()

       // Test with mock
       client := NewClient(server.URL)
       pages, err := client.GetPages()

       assert.NoError(t, err)
       assert.Len(t, pages, 2)
   }
   ```

4. **Integration Tests** - Test component interactions
   ```go
   func TestFullWorkflow(t *testing.T) {
       // Setup
       storage := mock.NewStorage()
       source := NewSource(config, storage)

       // Execute
       docs, err := source.Collect(context.Background())

       // Verify
       assert.NoError(t, err)
       assert.Greater(t, len(docs), 0)
   }
   ```

### Continuous Testing Workflow

**After EVERY code change:**

```bash
# 1. Run specific component tests
go test ./internal/component/... -v

# 2. If pass, run all tests
go test ./... -v

# 3. Check coverage (must be >80%)
go test ./... -cover

# 4. If all pass, proceed to next test/feature
```

### Test Organization

```
internal/component/
├── component.go           # Implementation
├── component_test.go      # Unit tests
└── testdata/              # Test fixtures
    ├── input.json
    └── expected.json

test/integration/
├── component_flow_test.go # Integration tests
└── fixtures/              # Shared fixtures
```

### Testing Checklist

Before marking ANY component complete:
- [ ] Unit tests written BEFORE implementation
- [ ] All tests passing (`go test ./... -v`)
- [ ] Test coverage >80% (`go test -cover`)
- [ ] Table-driven tests for multiple cases
- [ ] Mock external dependencies
- [ ] Integration tests for workflows
- [ ] Edge cases tested (nil, empty, errors)
- [ ] Error paths tested


## Examples

### Implement Server
```
/quaero-server C:\development\quaero
```

### Add Custom Endpoints
```
/quaero-server C:\development\quaero --enable-api
```

## Validation

After implementation, verifies:
- ✓ Server starts successfully
- ✓ Routes configured correctly
- ✓ Auth endpoint receives extension data
- ✓ Status endpoint returns info
- ✓ Health check responds
- ✓ Middleware applied
- ✓ CORS headers set
- ✓ Logging functional
- ✓ Graceful shutdown works
- ✓ Unit tests passing
- ✓ Integration tests passing

## Output

Provides detailed report:
- Files created/modified
- Server implementation status
- Route definitions
- Handler implementations
- Middleware setup
- App orchestration
- Tests created

---

**Agent**: quaero-server

**Prompt**: Implement the HTTP server for Quaero at {{args.[0]}}.

## Implementation Tasks

1. **Create Server Core** (`internal/server/server.go`)
   - Server struct with App, router, logger
   - Initialization with host/port
   - Start and Stop methods
   - Graceful shutdown

2. **Define Routes** (`internal/server/routes.go`)
   - /api/auth for extension
   - /api/status for collection status
   - /health for health checks
   - Optional /api/query and /api/collectors

3. **Implement Handlers** (`internal/server/handlers.go`)
   - handleAuth - receive and store auth, trigger collection
   - handleStatus - return server and collection status
   - handleHealth - simple OK response
   - Optional handleQuery and handleCollectors

4. **Add Middleware** (`internal/server/middleware.go`)
   - Logging middleware (request/response)
   - CORS middleware (allow extension)
   - Recovery middleware (panic handling)
   - Apply to all routes

5. **Create App Orchestration** (`internal/app/app.go`)
   - App struct with all components
   - New() function to initialize dependencies
   - Proper initialization order
   - Dependency injection

6. **Implement Serve Command** (`cmd/quaero/serve.go`)
   - Cobra command for quaero serve
   - Initialize App
   - Create and start Server
   - Signal handling for shutdown

7. **Create Tests**
   - Unit tests for handlers
   - Mock AuthManager and Collector
   - Integration test for full server
   - Test extension auth flow

## Code Quality Standards

- Clean HTTP server architecture
- Structured logging (arbor)
- Comprehensive error handling
- CORS for extension
- Graceful shutdown
- Middleware pattern
- Dependency injection
- 80%+ test coverage

## Success Criteria

✓ Server starts on configured port
✓ Auth endpoint receives extension data
✓ Auth stored and collection triggered
✓ Status endpoint returns info
✓ Health check responds
✓ Middleware applied correctly
✓ CORS headers set
✓ Graceful shutdown works
✓ All tests passing
