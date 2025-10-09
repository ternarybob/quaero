# Quaero Project Standards

## Agent Autonomy

**IMPORTANT: All agents, commands, and hooks in this project operate with FULL AUTONOMY within the project directory.**

When working in this project:
- ✅ Agents make decisions without asking questions
- ✅ Commands execute automatically without confirmation
- ✅ Hooks enforce standards and block violations silently
- ✅ Best practices are applied automatically
- ✅ Architectural decisions are made based on established patterns

This ensures:
- **Faster execution** - No interruptions for confirmations
- **Consistent quality** - Standards applied uniformly
- **Reduced friction** - Agents work independently
- **Better outcomes** - Decisions based on proven patterns

Agents will still communicate what they're doing, but they won't ask permission.

---

## Agent-Based Development System

This project uses an **autonomous agent architecture** with specialized agents in `.claude/agents/`:

- **overwatch.md** - Guardian (always active, reviews all changes, delegates)
- **go-refactor.md** - Code quality (consolidates duplicates, optimizes structure)
- **go-compliance.md** - Standards enforcement (logging, startup, configuration)
- **test-engineer.md** - Testing (writes tests, ensures coverage)
- **collector-impl.md** - Collectors (Jira, Confluence, GitHub only)
- **doc-writer.md** - Documentation (maintains docs, requirements)

**Usage:** Overwatch reviews all Write/Edit automatically. Explicitly invoke: `> Use go-refactor to consolidate duplicates`

---

## Code Quality Enforcement System

This project includes an automated code quality enforcement system integrated with the agent architecture.

**Language-Specific Enforcement:**
- **Go**: Clean architecture patterns, receiver methods, directory structure compliance

### Automated Checks

#### Pre-Write Validation
Before any `Write` operation:
- File length validation (max 500 lines)
- Function length validation (max 80 lines)
- Forbidden pattern detection (TODO, FIXME)
- Error handling validation
- Directory structure compliance

#### Pre-Edit Duplicate Detection
Before any `Edit` operation:
- Scans entire codebase for existing functions
- Detects duplicate function names and signatures
- **BLOCKS** operation if duplicate found
- Provides exact file:line location of existing function

#### Post-Operation Indexing
After `Write` or `Edit`:
- Updates function index (.claude/go-function-index.json)
- Maintains registry of all functions with signatures
- Enables fast duplicate detection

### Code Standards

#### Function Structure
- **Max Lines**: 80 (ideal: 20-40)
- **Single Responsibility**: One purpose per function
- **Error Handling**: Comprehensive validation
- **Naming**: Descriptive, intention-revealing

#### File Structure
- **Max Lines**: 500
- **Modular Design**: Extract utilities to shared files
- **Clear Organization**: Logical grouping of related functions

### Compliance Enforcement

The hooks are **mandatory** and will:
- ❌ **BLOCK** operations that create duplicates
- ⚠️  **WARN** about quality issues
- ✅ **APPROVE** compliant code changes

This ensures:
- No duplicate function implementations
- Consistent code structure
- Maintainable codebase
- Professional code quality

---

## Go Structure Standards

### Required Libraries
- `github.com/ternarybob/arbor` - All logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML config

### Startup Sequence (main.go)
1. Configuration loading (`common.LoadFromFile`)
2. Logger initialization (`common.InitLogger`)
3. Banner display (`common.PrintBanner`)
4. Version management (`common.GetVersion`)
5. Service initialization
6. Handler initialization
7. Information logging

### Directory Structure
```
cmd/quaero/                      Main entry point
cmd/quaero-chrome-extension/     Chrome extension for authentication
internal/
  ├── common/                    Stateless utilities - NO receiver methods
  ├── services/                  Stateful services WITH receiver methods
  │   ├── atlassian/            Jira & Confluence collectors
  │   └── github/               GitHub collector
  ├── handlers/                  HTTP handlers (dependency injection)
  │   ├── websocket.go          WebSocket for real-time updates
  │   ├── collector.go          Collector endpoints
  │   └── ui.go                 Web UI handler (Go templates)
  ├── models/                    Data models
  ├── interfaces/                Service interfaces
  └── server/                    HTTP server
pages/                           Go template files
  ├── index.html                Main dashboard (Go template)
  ├── confluence.html           Confluence UI (Go template)
  ├── jira.html                 Jira UI (Go template)
  ├── partials/                 Reusable template components
  └── static/                   CSS, JS (Alpine.js)
test/                            Integration tests
docs/                            Documentation
scripts/                         Build scripts
.github/workflows/               CI/CD
```

### Frontend Architecture

**Server-Side Rendering:**
- Go's `html/template` package for all page rendering
- Templates in `pages/*.html`
- Server renders complete HTML pages
- Template composition with `{{template "name" .}}`

**Client-Side Interactivity:**
- **Alpine.js** for reactive data binding and UI interactions
- Declarative attribute-based syntax (`x-data`, `x-on`, `x-show`)
- Lightweight, no build step required
- Handles form interactions, dynamic content updates
- Works with WebSocket for real-time updates

**NO client-side routing or SPA framework**
**NO htmx** - removed from architecture

### Quaero-Specific Requirements

**Collectors (ONLY These):**
1. **Jira** (`internal/services/atlassian/jira_*`)
2. **Confluence** (`internal/services/atlassian/confluence_*`)
3. **GitHub** (`internal/services/github/*`)

**Web UI (NOT CLI):**
- Go templates render server-side in `pages/*.html`
- Alpine.js handles client-side interactivity
- NO CLI commands for collection
- WebSocket for real-time updates
- Log streaming to browser

**Chrome Extension:**
- Location: `cmd/quaero-chrome-extension/`
- Captures authentication from Atlassian
- WebSocket communication with server

**Configuration Priority:**
1. CLI flags (highest)
2. Environment variables
3. Config file (`config.toml`)
4. Defaults (lowest)

**Banner Requirement:**
- MUST display on startup using `ternarybob/banner`
- MUST show version, host, port
- MUST log configuration source

### Critical Distinctions

#### `internal/services/` - Stateful Services (Receiver Methods)
```go
// ✅ CORRECT: Service with receiver methods
type SearchService struct {
    db     *sql.DB
    logger *arbor.Logger
}

func (s *SearchService) Search(ctx context.Context, query string) (*Result, error) {
    s.logger.Info("Searching", "query", query)
    return s.db.Query(query)
}
```

#### `internal/common/` - Stateless Utilities (Pure Functions)
```go
// ✅ CORRECT: Stateless pure function
func LoadFromFile(path string) (*Config, error) {
    // No receiver, no state
    return loadConfig(path)
}

// ❌ WRONG: Receiver method in common/
func (c *Config) LoadFromFile(path string) error {
    // This belongs in internal/services/
}
```

### Go-Specific Enforcement

#### Pre-Write/Edit Checks
- **Directory Rules**: Validates correct usage of `internal/common/` (no receivers) vs `internal/services/` (receivers required)
- **Duplicate Functions**: Prevents duplicate function names across codebase
- **Error Handling**: No ignored errors (`_ =`)
- **Logging Standards**: Must use `arbor` logger, no `fmt.Println`/`log.Println`
- **Startup Sequence**: Validates correct order in `main.go`
- **Interface Definitions**: Should be in `internal/interfaces/`

#### Example Violations

**❌ BLOCKED: Receiver method in internal/common/**
```go
// internal/common/config.go
func (c *Config) Load() error {  // ❌ ERROR
    // Common must be stateless!
}
```

**❌ BLOCKED: Stateless function in internal/services/**
```go
// internal/services/search_service.go
func Search(query string) error {  // ⚠️ WARNING
    // Services should use receiver methods!
}
```

**❌ BLOCKED: Using fmt.Println instead of logger**
```go
fmt.Println("Search completed")  // ❌ ERROR
logger.Info("Search completed")  // ✅ CORRECT
```

**❌ BLOCKED: Wrong startup sequence**
```go
common.InitLogger()      // ❌ ERROR
common.LoadFromFile()    // Must be first!
```

### Design Patterns

**Dependency Injection:**
```go
type SearchHandler struct {
    searchService interfaces.SearchService  // Interface, not concrete type
}

func NewSearchHandler(searchService interfaces.SearchService) *SearchHandler {
    return &SearchHandler{searchService: searchService}
}
```

**Interface-Based Design:**
```go
// internal/interfaces/search_service.go
type SearchService interface {
    Search(ctx context.Context, query string) (*Result, error)
    Index(ctx context.Context, data *Data) error
}
```

**Template Rendering:**
```go
// internal/handlers/ui.go
func (h *UIHandler) RenderPage(w http.ResponseWriter, r *http.Request) {
    data := struct {
        Title string
        Items []Item
    }{
        Title: "Dashboard",
        Items: h.service.GetItems(),
    }
    
    err := h.templates.ExecuteTemplate(w, "index.html", data)
    if err != nil {
        h.logger.Error("Template render failed", "error", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}
```

### Code Quality Rules
- Single Responsibility Principle
- Proper error handling (return errors, don't ignore)
- Interface-based design
- Table-driven tests
- DRY principle - consolidate duplicate code
- Remove unused/redundant functions
- Use receiver methods on services
- Keep common utilities stateless

### Testing Standards

**ALWAYS use the test script:**
```bash
./test/run-tests.ps1 -Type all
./test/run-tests.ps1 -Type unit
```

**NEVER use:**
```bash
cd test && go test      # ❌ WRONG
go test ./...           # ❌ WRONG
```

### Building Standards

**ALWAYS use the build script:**
```bash
./scripts/build.ps1 
./scripts/build.ps1 -Run
```

**NEVER use:**
```bash
go build                # ❌ WRONG
```

### Function Index

The hooks maintain `.claude/go-function-index.json` to track all functions and prevent duplicates.

**Rebuild index manually:**
```bash
node .claude/hooks/index-go-functions.js
```