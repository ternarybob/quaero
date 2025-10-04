# quaero-setup

Initializes or migrates the Quaero knowledge search system monorepo from aktis-parser, setting up clean architecture with all necessary components.

## Usage

```
/quaero-setup <project-path> [aktis-parser-path]
```

## Arguments

- `project-path` (required): Path where Quaero monorepo will be created (e.g., C:\development\quaero)
- `aktis-parser-path` (optional): Path to existing aktis-parser for migration

## What it does

### Phase 1: Project Initialization

1. **Directory Structure Creation**
   - Creates complete monorepo structure per spec
   - Sets up `cmd/quaero/` with CLI architecture
   - Creates `pkg/models/` for shared types
   - Establishes `internal/` structure for all components
   - Sets up `test/`, `docs/`, `scripts/`, `.github/workflows/`

2. **Module Initialization**
   - Generates `go.mod` with `github.com/ternarybob/quaero`
   - Adds required dependencies (Cobra, RavenDB client, rod, testify)
   - Includes ternarybob libraries (arbor, banner)

3. **Core Interfaces Definition**
   - Creates `pkg/models/document.go` - Core document model
   - Creates `pkg/models/source.go` - Source interface
   - Creates `pkg/models/storage.go` - Storage interface
   - Creates `pkg/models/rag.go` - RAG interface

### Phase 2: Migration (if aktis-parser-path provided)

1. **Authentication Migration**
   ```
   aktis-parser/internal/auth/handler.go → quaero/internal/auth/manager.go
   aktis-parser/internal/auth/store.go   → quaero/internal/auth/store.go
   ```
   - Refactors to new Storage interface
   - Updates to use new document model

2. **Jira Client Migration**
   ```
   aktis-parser/internal/jira/client.go → quaero/internal/sources/jira/client.go
   aktis-parser/internal/jira/types.go  → quaero/internal/sources/jira/models.go
   ```
   - Adapts to Source interface
   - Converts to new Document model

3. **Confluence Client Migration**
   ```
   aktis-parser/internal/confluence/client.go → quaero/internal/sources/confluence/api.go
   aktis-parser/internal/confluence/types.go  → quaero/internal/sources/confluence/models.go
   ```
   - Adapts to Source interface
   - Implements new Document conversion

4. **HTTP Server Migration**
   ```
   aktis-parser/cmd/service.go → quaero/internal/server/server.go
                                  quaero/cmd/quaero/serve.go
   ```
   - Refactors into server component and CLI command

### Phase 3: Component Implementation

1. **Application Orchestration** (`internal/app/`)
   - `app.go` - Main app struct with all components
   - `config.go` - Configuration management

2. **HTTP Server** (`internal/server/`)
   - `server.go` - Server implementation
   - `handlers.go` - HTTP handlers for auth endpoint
   - `routes.go` - Route definitions

3. **Collection Orchestration** (`internal/collector/`)
   - `orchestrator.go` - Manages collection workflow
   - `scheduler.go` - Background scheduling

4. **CLI Commands** (`cmd/quaero/`)
   - `main.go` - Root Cobra command
   - `serve.go` - HTTP server command
   - `collect.go` - Manual collection command
   - `query.go` - Query command
   - `version.go` - Version command

### Phase 4: Source Implementations

Creates basic structure for all sources in `internal/sources/`:

1. **Confluence** (`internal/sources/confluence/`)
   - `api.go` - REST API client
   - `scraper.go` - Browser scraper
   - `processor.go` - Document converter
   - `confluence.go` - Source interface implementation

2. **Jira** (`internal/sources/jira/`)
   - `client.go` - API client
   - `processor.go` - Document converter
   - `jira.go` - Source interface implementation

3. **GitHub** (`internal/sources/github/`)
   - `client.go` - GitHub API client
   - `processor.go` - Document converter
   - `github.go` - Source interface implementation

4. **Future Sources** (placeholders)
   - `internal/sources/slack/`
   - `internal/sources/linear/`
   - `internal/sources/notion/`

### Phase 5: Infrastructure Components

1. **Storage Layer** (`internal/storage/`)
   - `ravendb/store.go` - RavenDB implementation
   - `ravendb/queries.go` - Search queries
   - `mock/store.go` - Mock for testing

2. **RAG Engine** (`internal/rag/`)
   - `engine.go` - RAG orchestration
   - `search.go` - Search logic
   - `context.go` - Context building
   - `vision.go` - Image processing

3. **LLM Integration** (`internal/llm/`)
   - `ollama/client.go` - Ollama API client
   - `ollama/vision.go` - Vision model support
   - `mock/client.go` - Mock LLM for testing

4. **Processing Utilities** (`internal/processing/`)
   - `chunker.go` - Text chunking
   - `ocr.go` - OCR processing
   - `markdown.go` - HTML to Markdown conversion
   - `images.go` - Image handling

### Phase 6: Testing & CI/CD

1. **Integration Tests** (`test/integration/`)
   - `auth_flow_test.go` - Extension → service flow
   - `confluence_flow_test.go` - Confluence collection
   - `jira_flow_test.go` - Jira collection
   - `e2e_query_test.go` - End-to-end query test

2. **Test Fixtures** (`test/fixtures/`)
   - `auth_payload.json` - Sample extension auth data
   - `confluence_page.html` - Sample Confluence page
   - `jira_issues.json` - Sample Jira issues

3. **CI/CD Pipeline** (`.github/workflows/ci.yml`)
   - Unit tests with coverage
   - Integration tests
   - Build and artifact storage

### Phase 7: Configuration & Documentation

1. **Configuration Files**
   - `config.yaml` - Main application config
   - `deployments/docker/docker-compose.yml`
   - `deployments/local/` configs

2. **Scripts** (`scripts/`)
   - `setup.sh` - Initial setup
   - `migrate_from_aktis.sh` - Migration helper

3. **Documentation** (`docs/`)
   - `architecture.md` - System architecture
   - `migration.md` - Migration guide (from spec)
   - `authentication.md` - Auth flow explanation
   - `adding_collectors.md` - Guide for new sources

4. **Project Files**
   - `Makefile` - Build, test, run commands
   - `README.md` - Project overview
   - `CLAUDE.md` - Claude Code instructions

## Project Standards

### Required Libraries
- `github.com/ternarybob/arbor` - Structured logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML config
- `github.com/spf13/cobra` - CLI framework
- `github.com/go-rod/rod` - Browser automation
- RavenDB Go client - Document storage

### Directory Structure
```
quaero/
├── cmd/quaero/              # Single binary with subcommands
├── pkg/models/              # Public shared types
├── internal/
│   ├── app/                 # Application orchestration
│   ├── server/              # HTTP server
│   ├── collector/           # Collection orchestration
│   ├── auth/                # Authentication management
│   ├── sources/             # Data source implementations
│   ├── storage/             # Storage layer
│   ├── rag/                 # RAG engine
│   ├── llm/                 # LLM integration
│   └── processing/          # Processing utilities
├── test/                    # Integration tests
├── docs/                    # Documentation
├── scripts/                 # Build scripts
└── .github/workflows/       # CI/CD
```

### Authentication Flow
- Extension extracts auth from browser (cookies, tokens, localStorage)
- Extension POSTs to `/api/auth` endpoint
- Server stores auth credentials
- Collectors use stored auth for API calls
- Extension refreshes auth every 30 minutes

### Code Quality Standards
- Single responsibility principle
- Interface-based design
- Dependency injection pattern
- Comprehensive error handling
- Table-driven tests
- Clean architecture separation

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

### Initialize New Project
```
/quaero-setup C:\development\quaero
```

### Migrate from aktis-parser
```
/quaero-setup C:\development\quaero C:\development\aktis\aktis-parser
```

## Validation

After setup, verifies:
- ✓ Directory structure complete
- ✓ All interfaces defined
- ✓ go.mod with correct dependencies
- ✓ CLI commands scaffolded
- ✓ Server implementation ready
- ✓ Source interfaces implemented
- ✓ Tests structure created
- ✓ CI/CD configured
- ✓ Documentation complete

## Output

Provides detailed report:
- Files created/migrated
- Components implemented
- Dependencies added
- Tests scaffolded
- CI/CD status
- Next steps (run extension, configure sources)

---

**Agent**: quaero-setup

**Prompt**: Initialize the Quaero knowledge search system monorepo.

{{#if args.[1]}}
Migrate from existing aktis-parser at: {{args.[1]}}
{{/if}}

Target location: {{args.[0]}}

## Setup Tasks

1. **Create Directory Structure**
   - Complete monorepo layout per Quaero spec
   - cmd/, pkg/, internal/, test/, docs/, scripts/

2. **Initialize Module**
   - go.mod with github.com/ternarybob/quaero
   - Add all required dependencies

3. **Define Core Interfaces**
   - Document model (ID, Source, Title, ContentMD, Chunks, Images, Metadata)
   - Source interface (Name, Collect, SupportsImages)
   - Storage interface (Store, Search, VectorSearch)
   - RAG interface (Query, BuildContext)

{{#if args.[1]}}
4. **Migrate from aktis-parser**
   - Copy and refactor auth code
   - Adapt Jira client to Source interface
   - Adapt Confluence client to Source interface
   - Migrate HTTP server to new structure
{{/if}}

5. **Implement Components**
   - Application orchestration (app.go, config.go)
   - HTTP server (server.go, handlers.go, routes.go)
   - Collection orchestrator
   - CLI commands (serve, collect, query, version)

6. **Create Source Implementations**
   - Confluence (api.go, scraper.go, processor.go)
   - Jira (client.go, processor.go)
   - GitHub (client.go, processor.go)
   - Placeholders for Slack, Linear, Notion

7. **Setup Infrastructure**
   - RavenDB storage implementation
   - RAG engine (search, context, vision)
   - Ollama LLM client
   - Processing utilities (chunker, ocr, markdown, images)

8. **Create Tests**
   - Integration test structure
   - Test fixtures
   - Mock implementations

9. **Add CI/CD**
   - GitHub Actions workflow
   - Test, build, deploy pipeline

10. **Generate Documentation**
    - README.md with project overview
    - CLAUDE.md with architecture standards
    - Migration guide
    - Authentication flow docs
    - Adding collectors guide

## Success Criteria

✓ Project builds successfully
✓ All interfaces implemented
✓ CLI commands functional
✓ Server can receive auth from extension
✓ Source interfaces ready for implementation
✓ Tests scaffold complete
✓ Documentation comprehensive
