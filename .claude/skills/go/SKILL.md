# Go Skill for Quaero

**Prerequisite:** Read `.claude/skills/refactoring/SKILL.md` before any code changes.

## Project Context
- **Language:** Go 1.25+
- **Storage:** BadgerDB (embedded key-value store)
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2
- **LLM:** Google ADK with Gemini models

## Package Structure
```
internal/
├── app/          # DI & orchestration
├── common/       # Stateless utilities
├── handlers/     # HTTP & WebSocket handlers
├── services/     # Stateful business services
├── storage/      # Data persistence (Badger)
├── interfaces/   # Service interfaces
├── models/       # Data models
├── queue/        # Badger-backed job queue
└── jobs/         # Job management
```

## Required Patterns

### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process %s: %w", id, err)
}
```

### Logging (arbor)
```go
logger.Info("message", "key", value)
logger.Error("failed", "error", err)
// NEVER: fmt.Println() or log.Printf()
```

### Constructor Injection
```go
func NewService(dep interfaces.Dependency) *Service {
    return &Service{dep: dep}
}
// NEVER: global state or service locators
```

### Handler Pattern
```go
type Handler struct {
    service interfaces.Service
    logger  *arbor.Logger
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
    // Thin handler - delegate to service
}
```

## Anti-Patterns (AUTO-FAIL)
```go
// ❌ Global state
var db *badger.DB

// ❌ Panic on errors
panic(err)

// ❌ Missing context
func DoWork() error { }  // needs ctx

// ❌ Bare errors
return err  // needs context

// ❌ fmt/log for logging
fmt.Println("debug")

// ❌ Business logic in handlers
func (h *Handler) Create(w, r) {
    // 50 lines of logic - WRONG
}

// ❌ Direct go build
go build ./cmd/quaero  // Use scripts/
```

## Build & Test
```bash
# Build (ALWAYS use scripts)
.\scripts\build.ps1      # Windows
./scripts/build.sh       # Linux/macOS

# Tests
go test -v ./test/api    # API tests
go test -v ./test/ui     # UI tests
go test ./internal/...   # Unit tests
```

## Rules Summary

1. Use build scripts - never `go build` directly
2. Context everywhere - pass `context.Context` to I/O
3. Structured logging - arbor with key-value pairs
4. Wrap errors - always add context with `%w`
5. Interface-based DI - depend on interfaces
6. Constructor injection - all deps via `NewXxx()`
7. Thin handlers - logic in services