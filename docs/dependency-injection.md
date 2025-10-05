# Dependency Injection in Quaero

**Pattern:** Constructor-based Dependency Injection (Manual)

---

## Overview

Quaero uses **manual constructor-based dependency injection**, not framework-based DI (like C#'s ServiceCollection or Java's Spring). This is the idiomatic Go approach.

### What This Means

**We DO:**
- ✅ Accept dependencies as constructor parameters
- ✅ Manually wire dependencies in `app.New()`
- ✅ Use interfaces for abstraction where appropriate
- ✅ Keep initialization explicit and traceable

**We DON'T:**
- ❌ Use a DI container/framework
- ❌ Use reflection for auto-wiring
- ❌ Have automatic service registration
- ❌ Support lifetime management (Singleton/Scoped/Transient)

---

## Pattern Implementation

### 1. Define Dependencies

Services accept dependencies through constructor functions:

```go
// internal/handlers/collector.go
type CollectorHandler struct {
    jira       *atlassian.JiraScraperService
    confluence *atlassian.ConfluenceScraperService
    logger     arbor.ILogger
}

// Constructor function - dependencies injected here
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
```

### 2. Manual Wiring

Dependencies are wired manually in `internal/app/app.go`:

```go
// internal/app/app.go
func New(config *common.Config, logger arbor.ILogger) (*App, error) {
    app := &App{
        Config: config,
        Logger: logger,
    }

    // 1. Initialize storage (no dependencies)
    if err := app.initDatabase(); err != nil {
        return nil, err
    }

    // 2. Initialize services (depend on storage)
    if err := app.initServices(); err != nil {
        return nil, err
    }

    // 3. Initialize handlers (depend on services)
    if err := app.initHandlers(); err != nil {
        return nil, err
    }

    return app, nil
}

func (a *App) initServices() error {
    // Create auth service with storage dependency
    a.AuthService, _ = atlassian.NewAtlassianAuthService(
        a.StorageManager.AuthStorage(),
        a.Logger,
    )

    // Create Jira service with storage and auth dependencies
    a.JiraService = atlassian.NewJiraScraperService(
        a.StorageManager.JiraStorage(),
        a.AuthService,
        a.Logger,
    )

    // Create Confluence service with storage and auth dependencies
    a.ConfluenceService = atlassian.NewConfluenceScraperService(
        a.StorageManager.ConfluenceStorage(),
        a.AuthService,
        a.Logger,
    )

    return nil
}

func (a *App) initHandlers() error {
    // Manually wire handler dependencies
    a.CollectorHandler = handlers.NewCollectorHandler(
        a.JiraService,
        a.ConfluenceService,
        a.Logger,
    )

    a.UIHandler = handlers.NewUIHandler(
        a.JiraService,
        a.ConfluenceService,
    )

    a.WSHandler = handlers.NewWebSocketHandler()

    a.ScraperHandler = handlers.NewScraperHandler(
        a.AuthService,
        a.JiraService,
        a.ConfluenceService,
        a.WSHandler,
    )

    return nil
}
```

### 3. Dependency Graph

The wiring creates this dependency graph:

```
Config + Logger (from main.go)
    ↓
App
    ↓
StorageManager (SQLite)
    ↓
Services (AuthService, JiraService, ConfluenceService)
    ↓
Handlers (CollectorHandler, UIHandler, WSHandler, ScraperHandler)
    ↓
Server (HTTP server with routes)
```

---

## Benefits of This Approach

### 1. **Explicit and Traceable**

You can trace the entire dependency chain by reading `app.go`:

```go
// Easy to see: CollectorHandler depends on Jira + Confluence services
a.CollectorHandler = handlers.NewCollectorHandler(
    a.JiraService,
    a.ConfluenceService,
    a.Logger,
)
```

### 2. **No Magic**

No reflection, no annotations, no framework conventions. Just functions and structs.

### 3. **Compile-Time Safety**

Missing dependencies cause compile errors:

```go
// Compile error if signature changes
func NewCollectorHandler(
    jira *atlassian.JiraScraperService,
    confluence *atlassian.ConfluenceScraperService,
    logger arbor.ILogger,
    // If you add a new parameter here, all call sites must update
) *CollectorHandler
```

### 4. **Testable**

Easy to create test instances with mock dependencies:

```go
// In tests
mockJira := &MockJiraService{}
mockConfluence := &MockConfluenceService{}
mockLogger := arbor.NewLogger()

handler := handlers.NewCollectorHandler(
    mockJira,
    mockConfluence,
    mockLogger,
)
```

### 5. **No Hidden State**

All dependencies are visible in struct fields:

```go
type CollectorHandler struct {
    jira       *atlassian.JiraScraperService  // Clear what it depends on
    confluence *atlassian.ConfluenceScraperService
    logger     arbor.ILogger
}
```

---

## Comparison with Framework-Based DI

### C# (.NET Core) - What We DON'T Do

```csharp
// Service registration
services.AddScoped<IUserService, UserService>();
services.AddTransient<IEmailService, EmailService>();
services.AddSingleton<ICache, MemoryCache>();

// Automatic resolution via constructor
public class UserController : Controller
{
    private readonly IUserService _userService;
    private readonly IEmailService _emailService;

    // DI container auto-injects these
    public UserController(IUserService userService, IEmailService emailService)
    {
        _userService = userService;
        _emailService = emailService;
    }
}
```

### Go (Quaero) - What We DO

```go
// Manual construction and wiring
func New(config *common.Config, logger arbor.ILogger) (*App, error) {
    // Explicit creation
    storageManager, _ := storage.NewStorageManager(logger, config)
    authService, _ := atlassian.NewAtlassianAuthService(
        storageManager.AuthStorage(),
        logger,
    )
    jiraService := atlassian.NewJiraScraperService(
        storageManager.JiraStorage(),
        authService,
        logger,
    )

    // Manual wiring
    handler := handlers.NewCollectorHandler(
        jiraService,
        confluenceService,
        logger,
    )

    return app, nil
}
```

---

## Interface Usage

We use interfaces **only where needed for abstraction**, not everywhere:

### ✅ Good - Interface for Storage Abstraction

```go
// internal/interfaces/storage.go
type JiraStorage interface {
    SaveProject(project *models.JiraProject) error
    GetProjects() ([]*models.JiraProject, error)
}

// Service depends on interface
type JiraScraperService struct {
    storage interfaces.JiraStorage  // Interface - could be SQLite, Postgres, etc.
    auth    *AtlassianAuthService
    logger  arbor.ILogger
}
```

### ✅ Also Good - Concrete Type When No Abstraction Needed

```go
// Handler depends on concrete service type
type CollectorHandler struct {
    jira       *atlassian.JiraScraperService  // Concrete type - only one implementation
    confluence *atlassian.ConfluenceScraperService
    logger     arbor.ILogger
}
```

---

## Common Patterns

### Pattern 1: Service Layer

```go
// Service struct with dependencies
type JiraScraperService struct {
    storage interfaces.JiraStorage
    auth    *AtlassianAuthService
    logger  arbor.ILogger
}

// Constructor injection
func NewJiraScraperService(
    storage interfaces.JiraStorage,
    auth *AtlassianAuthService,
    logger arbor.ILogger,
) *JiraScraperService {
    return &JiraScraperService{
        storage: storage,
        auth:    auth,
        logger:  logger,
    }
}

// Methods use injected dependencies
func (j *JiraScraperService) FetchProjects() error {
    j.logger.Info().Msg("Fetching projects")
    // Use j.auth, j.storage
}
```

### Pattern 2: Handler Layer

```go
// Handler struct with service dependencies
type CollectorHandler struct {
    jira   *atlassian.JiraScraperService
    logger arbor.ILogger
}

// Constructor injection
func NewCollectorHandler(
    jira *atlassian.JiraScraperService,
    logger arbor.ILogger,
) *CollectorHandler {
    return &CollectorHandler{
        jira:   jira,
        logger: logger,
    }
}

// HTTP handler delegates to service
func (h *CollectorHandler) HandleCollect(w http.ResponseWriter, r *http.Request) {
    if err := h.jira.FetchProjects(); err != nil {
        h.logger.Error().Err(err).Msg("Collection failed")
        http.Error(w, "Collection failed", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

### Pattern 3: Optional Dependencies (Circular Reference Breaker)

Sometimes we need to inject dependencies after construction:

```go
// WebSocket handler created without auth service
wsHandler := handlers.NewWebSocketHandler()

// Auth service created
authService := atlassian.NewAtlassianAuthService(...)

// Later: inject auth service into WebSocket handler
wsHandler.SetAuthLoader(authService)
```

This is used when:
- Circular dependencies exist (A needs B, B needs A)
- Dependency is optional
- Dependency isn't available at construction time

---

## Testing with Constructor Injection

### Unit Test Example

```go
// Create mock dependencies
type MockJiraStorage struct {
    projects []*models.JiraProject
}

func (m *MockJiraStorage) SaveProject(p *models.JiraProject) error {
    m.projects = append(m.projects, p)
    return nil
}

func (m *MockJiraStorage) GetProjects() ([]*models.JiraProject, error) {
    return m.projects, nil
}

// Test
func TestJiraService_FetchProjects(t *testing.T) {
    // Create mocks
    mockStorage := &MockJiraStorage{}
    mockAuth := &MockAuthService{}
    logger := arbor.NewLogger()

    // Inject mocks via constructor
    service := atlassian.NewJiraScraperService(
        mockStorage,
        mockAuth,
        logger,
    )

    // Test
    err := service.FetchProjects()
    assert.NoError(t, err)
    assert.Len(t, mockStorage.projects, 5)
}
```

---

## Guidelines

### DO ✅

1. **Accept dependencies as constructor parameters**
   ```go
   func NewService(storage Storage, logger Logger) *Service
   ```

2. **Wire dependencies manually in app.go**
   ```go
   app.Service = NewService(app.Storage, app.Logger)
   ```

3. **Use interfaces for storage/external dependencies**
   ```go
   type Service struct {
       storage interfaces.Storage  // Interface for flexibility
   }
   ```

4. **Keep constructors simple**
   ```go
   func NewHandler(svc *Service) *Handler {
       return &Handler{svc: svc}
   }
   ```

### DON'T ❌

1. **Don't use global variables for dependencies**
   ```go
   // ❌ BAD
   var globalLogger arbor.ILogger

   func DoSomething() {
       globalLogger.Info().Msg("...")
   }
   ```

2. **Don't create dependencies inside methods**
   ```go
   // ❌ BAD
   func (h *Handler) Handle() {
       logger := arbor.NewLogger()  // Should be injected!
   }
   ```

3. **Don't hide dependencies**
   ```go
   // ❌ BAD - where does storage come from?
   type Service struct {
       // storage is hidden
   }

   func (s *Service) Save() {
       storage := getStorageSomehow()  // Magic!
   }
   ```

4. **Don't overuse interfaces**
   ```go
   // ❌ Unnecessary if only one implementation
   type IUserService interface {
       GetUser() *User
   }

   // ✅ Just use concrete type
   type UserService struct {}
   ```

---

## Summary

**Quaero uses constructor-based dependency injection:**

- Dependencies passed as constructor parameters
- Manual wiring in `app.New()` and `app.init*()` functions
- Interfaces used only for abstraction, not everywhere
- No DI framework, no reflection, no magic
- Explicit, traceable, and testable

This is **idiomatic Go**, not a limitation - it's the recommended approach for Go applications.

---

**See Also:**
- [internal/app/app.go](../../internal/app/app.go) - Main wiring logic
- [Effective Go - Constructors](https://go.dev/doc/effective_go#composite_literals)
- [Go Proverbs](https://go-proverbs.github.io/) - "Clear is better than clever"
