# AGENTS.md

Quaero codebase rules for AI agents. Workflow defined in `.claude/commands/3agents.md`.

## OS DETECTION (MANDATORY)

**BEFORE any shell command, detect OS:**

| Indicator | OS | Shell |
|-----------|-----|-------|
| `C:\...` or `D:\...` | Windows | PowerShell |
| `/home/...` or `/Users/...` | Unix/Linux/macOS | Bash |
| `/mnt/c/...` | WSL | Bash (but `powershell.exe` for Go) |

## BUILD & TEST

| OS | Build | Test |
|----|-------|------|
| Windows | `.\scripts\build.ps1` | `go test -v ./test/...` |
| Linux/macOS | `./scripts/build.sh` | `go test -v ./test/...` |
| WSL | `powershell.exe -Command "cd C:\path; .\scripts\build.ps1"` | `powershell.exe -Command "cd C:\path; go test -v ./test/..."` |

**Flags:** `-Deploy` (deploy to bin/), `-Run` (deploy + start service)

## ARCHITECTURE

```
cmd/quaero/           → Entry point, CLI
internal/app/         → DI & orchestration (composition root)
internal/server/      → HTTP server & routing
internal/handlers/    → HTTP/WebSocket handlers
internal/services/    → Business logic (stateful, WITH receivers)
internal/common/      → Utilities (stateless, NO receivers)
internal/jobs/
  ├── manager/        → StepManager implementations
  ├── worker/         → JobWorker implementations
  └── monitor/        → JobMonitor implementations
internal/storage/     → BadgerDB persistence
internal/interfaces/  → All interface definitions
```

## ARCHITECTURE DOCS

| Doc | Path |
|-----|------|
| Manager/Worker | `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` |
| Test | `docs/TEST_ARCHITECTURE.md` |

## GO RULES

### Logging (github.com/ternarybob/arbor)
```go
// ✅ REQUIRED
logger.Info().Str("field", value).Msg("Message")
logger.Error().Err(err).Msg("Error occurred")
logger.Debug().Int("count", n).Msg("Debug info")
```

### Error Handling
```go
// ✅ REQUIRED - wrap with context
if err != nil {
    return fmt.Errorf("failed to process: %w", err)
}
```

### Structure Rules
| Location | Rule |
|----------|------|
| `internal/common/` | Stateless functions ONLY (no receivers) |
| `internal/services/` | Stateful services (WITH receivers) |

## FORBIDDEN

```go
fmt.Println("message")           // ❌ Use logger
log.Printf("message")            // ❌ Use logger
_ = someFunction()               // ❌ Handle all errors
// TODO: fix later               // ❌ No deferred TODOs
func (c *Config) Method() {}     // ❌ No receivers in common/
```

## CONFIG PARITY

Changes to `./bin` MUST mirror to:
- `./deployments/common`
- `./test/config`

## KEY LIBRARIES

| Library | Purpose |
|---------|---------|
| `github.com/ternarybob/arbor` | Structured logging (REQUIRED) |
| `github.com/ternarybob/banner` | Startup banners |
| `github.com/pelletier/go-toml/v2` | TOML config |
| `github.com/gorilla/websocket` | WebSocket |
| `github.com/chromedp/chromedp` | Browser automation |

## FRONTEND

- **Framework:** Alpine.js + Bulma CSS
- **Templates:** `pages/*.html` (Go html/template)
- **No:** React, Vue, SPA, HTMX