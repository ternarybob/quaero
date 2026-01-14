# Go Skill

**Prerequisite:** Read `.codebuff/skills/refactoring/SKILL.md` first.

## Project Context
- **Language:** Go 1.21+
- **Storage:** BadgerDB (embedded key-value store)
- **Logging:** Structured logging (arbor or similar)
- **Configuration:** TOML via go-toml/v2

## Package Structure
```
internal/
├── app/          # DI & orchestration (composition root)
├── common/       # Stateless utilities (NO receivers)
├── handlers/     # HTTP & WebSocket handlers
├── services/     # Stateful business services (WITH receivers)
├── storage/      # Data persistence (Badger)
├── interfaces/   # Service interfaces
├── models/       # Data models
├── queue/        # Job queue implementation
└── jobs/         # Job management
```

## Required Patterns

### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process %s: %w", id, err)
}

// Never bare return
if err != nil {
    return err  // ❌ WRONG
}
```

### Logging (Structured Only)
```go
// Correct: structured key-value logging
logger.Info("processing request", "id", requestID, "user", userID)
logger.Error("operation failed", "error", err, "context", ctx)

// Wrong: unstructured logging
fmt.Println("processing...")       // ❌
log.Printf("error: %v", err)       // ❌
```

### Constructor Injection
```go
// Correct: dependencies via constructor
func NewService(dep interfaces.Dependency, logger Logger) *Service {
    return &Service{dep: dep, logger: logger}
}

// Wrong: global state
var globalDB *badger.DB  // ❌
```

### Thin Handlers
```go
// Handlers should be thin - delegate to services
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    
    result, err := h.service.GetByID(r.Context(), id)
    if err != nil {
        h.respondError(w, err)
        return
    }
    
    h.respondJSON(w, result)
}

// If handler is 50+ lines, logic belongs in service
```

### Context Propagation
```go
// Always pass context to I/O operations
func (s *Service) FetchData(ctx context.Context, id string) (*Data, error) {
    return s.storage.Get(ctx, id)
}

// Never omit context
func (s *Service) FetchData(id string) (*Data, error) {  // ❌ Missing context
    return s.storage.Get(id)
}
```

### Interface Location
```go
// Interfaces belong in internal/interfaces/
// internal/interfaces/storage.go
type DocumentStorage interface {
    Get(ctx context.Context, id string) (*models.Document, error)
    Save(ctx context.Context, doc *models.Document) error
}
```

## Anti-Patterns (AUTO-FAIL)

```go
// ❌ Global state
var db *badger.DB
var config *Config

// ❌ Panic on errors
if err != nil {
    panic(err)
}

// ❌ Missing context
func DoWork() error { }

// ❌ Bare errors (no context)
return err

// ❌ fmt/log for logging
fmt.Println("debug")
log.Printf("error: %v", err)

// ❌ Business logic in handlers
// 50+ lines in handler = WRONG

// ❌ Direct go build
go build ./cmd/app  // Use build scripts!

// ❌ Dead code left behind
func oldHelper() { }  // Remove if replaced!

// ❌ Unused imports
import "unused/pkg"  // Clean up!

// ❌ Receivers in common/ package
func (c *Config) Method() { }  // common/ is stateless!
```

## Build & Test

```bash
# Build (ALWAYS use scripts - never `go build` directly)
./scripts/build.sh       # Linux/macOS
.\scripts\build.ps1      # Windows

# Tests
go test -v ./test/api/...     # API tests
go test -v ./test/ui/...      # UI tests
go test ./internal/...        # Unit tests
```

## Structure Rules

| Package | State | Receivers | Purpose |
|---------|-------|-----------|--------|
| `internal/common/` | Stateless | ❌ NO | Utility functions |
| `internal/services/` | Stateful | ✓ YES | Business logic |
| `internal/handlers/` | Thin | ✓ YES | HTTP routing only |
| `internal/storage/` | Stateful | ✓ YES | Data persistence |

## Rules Summary

1. **Use build scripts** - never `go build` directly
2. **Context everywhere** - pass `context.Context` to I/O operations
3. **Structured logging** - key-value pairs only
4. **Wrap errors** - always add context with `%w`
5. **Interface-based DI** - depend on interfaces, not concrete types
6. **Constructor injection** - all deps via `NewXxx()`
7. **Thin handlers** - business logic in services
8. **Remove dead code** - don't leave old functions
9. **Clean imports** - remove unused packages
10. **No receivers in common/** - stateless utilities only
