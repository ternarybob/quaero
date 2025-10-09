# Quaero Agent System

This directory contains specialized AI agents for autonomous development and maintenance of the Quaero project.

## Agent Architecture

```
Overwatch Agent (Guardian)
    ├── Go Refactor Agent
    ├── Go Compliance Agent
    ├── Test Engineer Agent
    ├── Collector Implementation Agent
    └── Documentation Agent
```

## Agents

### [overwatch.md](overwatch.md) - Project Guardian

**Role:** Always-active guardian of architecture and code quality

**Responsibilities:**
- Reviews ALL Write/Edit operations automatically
- Enforces Go clean architecture patterns
- Blocks non-compliant code changes
- Detects duplicate functions across codebase
- Delegates to specialist agents
- Final approval authority

**When Invoked:** Automatically on all code changes

**Key Enforcements:**
- `internal/common/` must have NO receiver methods
- `internal/services/` must USE receiver methods
- ALL logging via `arbor` (NO `fmt.Println`)
- Banner MUST be displayed on startup
- Startup sequence must follow required order
- Configuration priority: CLI > Env > File > Defaults

---

### [go-refactor.md](go-refactor.md) - Code Quality Specialist

**Role:** Consolidates duplicates and applies clean architecture

**Responsibilities:**
- Eliminates duplicate code
- Extracts common utilities
- Splits large files/functions
- Applies SOLID principles
- Optimizes code structure

**When Invoked:** When duplicates found or code quality issues detected

**Key Tasks:**
- Consolidate duplicate functions
- Move stateless functions to `internal/common/`
- Move stateful code to `internal/services/`
- Extract repeated logic to utilities
- Interface-based design

---

### [go-compliance.md](go-compliance.md) - Standards Enforcer

**Role:** Enforces Go standards and Quaero-specific requirements

**Responsibilities:**
- Validates startup sequence order
- Ensures logging compliance (arbor only)
- Verifies banner display
- Checks configuration patterns
- Enforces error handling standards

**When Invoked:** When code violates Go idioms or Quaero standards

**Key Checks:**
- Startup sequence: Config → Logger → Banner → Services
- No `fmt.Println` or `log.Println`
- Banner displayed with version/host/port
- Configuration priority order followed
- No ignored errors (`_ =`)

---

### [test-engineer.md](test-engineer.md) - Testing Specialist

**Role:** Comprehensive testing and coverage

**Responsibilities:**
- Writes unit tests
- Creates integration tests
- Ensures 80%+ coverage
- Fixes test failures
- Maintains test quality

**When Invoked:** For new features or when tests needed

**Key Patterns:**
- Table-driven tests
- Testify library (assert, require, mock)
- Integration tests in `test/integration/`
- Test fixtures in `test/fixtures/`
- Coverage enforcement in CI/CD

---

### [collector-impl.md](collector-impl.md) - Data Source Specialist

**Role:** Implements and maintains collectors

**Responsibilities:**
- Implements Jira, Confluence, GitHub collectors
- API integration
- Browser scraping (rod)
- Data transformation
- Progress reporting via WebSocket

**When Invoked:** For collector implementation or improvements

**Approved Collectors:**
- Jira (`internal/services/atlassian/jira_*`)
- Confluence (`internal/services/atlassian/confluence_*`)
- GitHub (`internal/services/github/*`)

**Key Patterns:**
- Implement `Collector` interface
- Use `AuthData` from Chrome extension
- Rate limiting and retries
- Progress reporting to Web UI
- Image extraction support

---

### [doc-writer.md](doc-writer.md) - Documentation Specialist

**Role:** Maintains accurate documentation

**Responsibilities:**
- Updates requirements documentation
- Maintains API documentation
- Creates developer guides
- Ensures accuracy with code
- Removes obsolete information

**When Invoked:** For documentation updates or new features

**Key Documents:**
- `docs/requirements.md` - Project requirements
- `docs/api.md` - API specification
- `docs/development.md` - Developer guide
- `CLAUDE.md` - Project standards

---

## How to Use Agents

### Automatic Invocation

Overwatch automatically reviews all Write/Edit operations. No explicit command needed.

### Explicit Invocation

```bash
# In Claude Code
> Use go-refactor to consolidate duplicate HTTP clients
> Have test-engineer write integration tests for WebSocket handler
> Ask doc-writer to update the API documentation
> Use overwatch to review the current codebase structure
```

### Coordinated Workflows

```bash
> Implement new GitHub wiki collection

# Automatic workflow:
# 1. Overwatch analyzes requirements
# 2. Delegates to collector-impl for implementation
# 3. Test-engineer creates integration tests
# 4. Go-compliance validates standards
# 5. Doc-writer updates documentation
# 6. Overwatch gives final approval
```

## Agent Coordination

### Workflow

1. **Change Request** - Developer makes code change
2. **Automatic Review** - Overwatch reviews automatically
3. **Violation Detection** - Blocks if issues found
4. **Delegation** - Delegates to appropriate specialist
5. **Specialist Action** - Specialist fixes issue
6. **Re-Review** - Overwatch validates fix
7. **Approval** - Approves when compliant

### Delegation Rules

| Issue Detected | Delegated To |
|---------------|--------------|
| Duplicate code | go-refactor |
| Architecture violations | go-compliance |
| Missing tests | test-engineer |
| Collector implementation | collector-impl |
| Outdated documentation | doc-writer |

## Quaero-Specific Standards

### Required Libraries

✅ **MUST USE:**
- `github.com/ternarybob/arbor` - ALL logging
- `github.com/ternarybob/banner` - Startup banners (MANDATORY)
- `github.com/pelletier/go-toml/v2` - TOML configuration

❌ **FORBIDDEN:**
- `fmt.Println` / `log.Println` for logging
- Any other logging library

### Architecture Rules

**`internal/common/`:**
- Stateless utilities ONLY
- NO receiver methods
- Pure functions

**`internal/services/`:**
- Stateful services
- MUST use receiver methods
- Implement interfaces

**`internal/handlers/`:**
- HTTP request handling
- Dependency injection (interfaces)
- Thin layer, delegate to services

### Collectors

**ONLY These:**
1. Jira
2. Confluence
3. GitHub

**No Others** without explicit approval.

### Web UI (NOT CLI)

- Templates in `pages/*.html`
- WebSocket for real-time updates
- NO CLI commands for collection

### Startup Sequence

**REQUIRED ORDER:**
1. Configuration loading
2. Logger initialization
3. **Banner display** (MANDATORY)
4. Version logging
5. Service initialization
6. Handler initialization
7. Server start

## Benefits

### Code Quality
- No duplicate implementations
- Consistent architecture
- Professional code standards
- Comprehensive error handling

### Efficiency
- Automated reviews
- Specialist expertise
- Coordinated workflows
- Reduced manual oversight

### Maintainability
- Clear patterns
- Enforced standards
- Updated documentation
- Test coverage

## Troubleshooting

### Agent Not Responding

Check `.claude/settings.local.json` for agent configuration.

### Blocked Operation

Review overwatch feedback for specific violations and required fixes.

### Incorrect Delegation

Overwatch will re-delegate if the wrong specialist was chosen.

---

**For more information, see:**
- [Project Standards (CLAUDE.md)](../../CLAUDE.md)
- [Requirements](../../docs/requirements.md)
- [Individual agent files](./) in this directory
