# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Commands

### Building
```bash
# Primary build command (increments version, outputs to bin/)
./scripts/build.ps1

# Build options
./scripts/build.ps1 -Clean -Test -Release    # Full release build
./scripts/build.ps1 -Run                     # Build and run in new terminal
```

### Testing
**📖 For complete testing instructions, see [test/README.md](test/README.md)**

```bash
# Run all tests (ALWAYS use this script)
./test/run-tests.ps1

# Test types
./test/run-tests.ps1 -type unit             # Fast unit tests
./test/run-tests.ps1 -type api              # API integration tests
./test/run-tests.ps1 -type ui               # Browser automation (auto-starts server)

# Pattern matching
./test/run-tests.ps1 -type ui -script PageLayout   # Run specific UI test
./test/run-tests.ps1 -script navbar               # Search across all test types
```

### Running
```bash
# Start server (uses auto-discovered quaero.toml)
./bin/quaero.exe serve

# Custom configuration
./bin/quaero.exe serve -c ./deployments/config.offline.example.toml -p 8080
```

## Architecture Overview

### Security-First Design
Quaero operates in two mutually exclusive modes to address different security requirements:
- **Cloud Mode**: Data sent to external APIs (Google Gemini) - for personal/non-sensitive use only
- **Offline Mode**: All processing local via llama.cpp - required for government/healthcare/corporate data

### Key Components

#### 1. Event-Driven Core
- `EventService` implements pub/sub pattern for `EventCollectionTriggered` and `EventEmbeddingTriggered`
- Services auto-subscribe to relevant events during initialization
- Scheduler triggers events on cron schedule (default: every minute)

#### 2. Dual-Mode LLM System (`internal/services/llm/`)
- `LLMService` interface abstracts cloud vs offline implementation
- **Cloud**: Gemini API for embeddings + chat (requires explicit risk acknowledgment)  
- **Offline**: llama.cpp with GGUF models (nomic-embed-text + qwen2.5-7b)
- All operations logged to audit trail for compliance

#### 3. Collection & Processing Pipeline
```
Scheduler → Events → Collectors (Jira/Confluence) → Documents → Embedding Coordinator → Vector Storage
```
- Collectors scrape APIs and create document records
- Processing service handles background vectorization with worker pools
- Documents support `force_sync_pending` and `force_embed_pending` flags

#### 4. Chrome Extension Authentication
- Extension captures Atlassian cookies/tokens from browser
- Connects via WebSocket to send auth data to server
- Server stores credentials for API calls

### Configuration System
Priority: CLI flags > Environment Variables > Config File > Defaults

Critical config sections:
- `[llm]` - Mode selection ("offline"/"cloud") 
- `[llm.offline]` - Model paths, thread count, GPU layers
- `[llm.audit]` - Compliance logging settings
- `[storage.sqlite]` - Database path, FTS5, vector dimensions

## Directory Structure Patterns

### Services vs Common Split
- `internal/services/`: Stateful services WITH receiver methods (databases, business logic)
- `internal/common/`: Stateless utilities WITHOUT receiver methods (config, logging, banners)

### Storage Layer
- `internal/storage/sqlite/`: Database implementations with migrations
- Supports SQLite with FTS5 full-text search + vector embeddings (768-dim)
- Tables: `documents`, `jira_*`, `confluence_*`, `audit_log`

### Web Interface
- `pages/`: HTML templates with WebSocket real-time updates
- NOT a CLI application - all collection via web UI
- Real-time log streaming to browser during operations

## Development Standards

### Required Libraries
- `github.com/ternarybob/arbor` - All logging (REQUIRED)
- `github.com/ternarybob/banner` - Startup banners (REQUIRED)
- `github.com/pelletier/go-toml/v2` - TOML config (REQUIRED)

### Code Organization Rules
- Services MUST use receiver methods: `func (s *Service) Method()`
- Common utilities MUST be stateless functions: `func LoadConfig()`
- All errors must be handled (no `_ = err`)
- Use dependency injection pattern with interfaces

### Startup Sequence (main.go)
1. Configuration loading (`common.LoadFromFile`)
2. Logger initialization (`common.InitLogger`) 
3. Banner display (`common.PrintBanner`)
4. Service initialization with proper dependency injection
5. Event subscriptions and scheduler start

## Key Implementation Details

### LLM Mode Validation
```go
// Config validation prevents accidental data exfiltration
if config.LLM.Mode == "cloud" && !config.LLM.Cloud.ConfirmRisk {
    return fmt.Errorf("cloud mode requires explicit risk acceptance")
}
```

### Document Processing States
- `PENDING`: Needs processing
- `PROCESSED`: Has valid embedding
- `FAILED`: Processing error
- Processing coordinator queries by state and uses worker pools

### Authentication Flow
```
User → Atlassian Login → Chrome Extension → WebSocket → Server Storage → API Credentials
```

### Supported Data Sources
- **Jira**: Projects, issues, metadata (`internal/services/atlassian/jira_*`)
- **Confluence**: Spaces, pages, content (`internal/services/atlassian/confluence_*`)
- **GitHub**: Planned for Phase 2.0

## Testing Architecture

**📖 Complete testing documentation: [test/README.md](test/README.md)**

### Test Organization
- Unit tests: `test/unit/` - Fast, isolated component tests
- API tests: `test/api/` - HTTP endpoint and database integration tests
- UI tests: `test/ui/` - ChromeDP browser automation (auto-starts server)

### Key Features
- **Automatic server management**: UI tests build app and start server automatically
- **Pattern matching**: Use `-script` parameter to filter tests (e.g., `-script PageLayout`)
- **Screenshot capture**: UI tests capture numbered screenshots for debugging
- **Timestamped results**: All results saved to `test/results/{type}-{filter}-{timestamp}/`

### Result Management
- Results include test output logs, coverage reports, and screenshots
- Always use `run-tests.ps1` - never `go test` directly
- Test runner handles build, server startup/shutdown automatically

## Agent-Based Development System

This project uses specialized autonomous agents in `.claude/agents/`:
- **overwatch.md**: Guardian (reviews all changes, delegates)
- **go-refactor.md**: Code quality (consolidates duplicates)
- **go-compliance.md**: Standards enforcement
- **test-engineer.md**: Testing and coverage
- **doc-writer.md**: Documentation maintenance

Code quality hooks prevent duplicate functions and enforce architectural patterns.

## Current Implementation Status

**✅ Complete**: Core infrastructure, vector embeddings, event-driven architecture, dual storage
**🚧 In Progress**: Dual-mode LLM implementation (Phase 1.2) 
**📋 Planned**: RAG pipeline (Phase 1.3), GitHub integration (Phase 2.0)

## Security Considerations

### Offline Mode Guarantees
- All processing occurs locally using llama.cpp
- No network calls (verifiable through code review)  
- Comprehensive audit trail stored in SQLite
- Works completely air-gapped after model download

### Cloud Mode Warnings
- Explicit risk acknowledgment required in config
- Startup warnings about data transmission
- All API calls logged for audit purposes
- ONLY for personal/non-sensitive data

The system enforces these modes strictly to prevent accidental data exfiltration in enterprise environments.