# Task 5: Standardize App, Main, Storage, and Common

## Metadata
- **ID:** 5
- **Group:** 3
- **Mode:** sequential
- **Skill:** @go-coder
- **Critical:** no
- **Depends:** 2, 3, 4
- **Blocks:** Final Review

## Paths
```yaml
sandbox: /tmp/3agents/task-5/
source: C:/development/quaero/
output: C:/development/quaero/docs/features/20251127-logging-standardization/
```

## Files to Modify

### Application Core:
- `internal/app/app.go` - Condense startup logging
- `cmd/quaero/main.go` - Standardize startup/shutdown logging
- `cmd/quaero-mcp/main.go` - Standardize startup logging
- `cmd/quaero-mcp/handlers.go` - Standardize log levels

### Storage Layer:
- `internal/storage/badger/connection.go` - Standardize log levels
- `internal/storage/badger/document_storage.go` - Standardize log levels
- `internal/storage/badger/queue_storage.go` - Standardize log levels
- `internal/storage/badger/manager.go` - Standardize log levels
- `internal/storage/badger/load_variables.go` - Standardize log levels
- `internal/storage/badger/load_env.go` - Standardize log levels
- `internal/storage/badger/load_job_definitions.go` - Standardize log levels

### Common Utilities:
- `internal/common/banner.go` - Standardize log levels
- `internal/common/config.go` - Standardize log levels
- `internal/common/logger.go` - Standardize log levels
- `internal/common/replacement.go` - Standardize log levels
- `internal/common/url_utils.go` - Standardize log levels

### Logs Module:
- `internal/logs/consumer.go` - Standardize log levels
- `internal/logs/service.go` - Standardize log levels

### Jobs Module:
- `internal/jobs/service.go` - Standardize log levels

### Server:
- `internal/server/server.go` - Standardize log levels
- `internal/server/routes.go` - Standardize log levels
- `internal/server/middleware.go` - Standardize log levels
- `internal/server/route_helpers.go` - Standardize log levels

### GitHub Integration:
- `internal/githublogs/connector.go` - Standardize log levels

## Requirements
Apply the following log level rules:

### app.go Startup Logging (CRITICAL):
Currently has 56 Info logs for startup. Should be condensed to:
1. **Info**: "Application starting" (once)
2. **Info**: "Storage initialized" (once)
3. **Info**: "Services initialized" (once)
4. **Debug**: Individual service init messages
5. **Info**: "Application ready" (once)
6. **Info**: "Application shutting down" (once)
7. **Info**: "Application stopped" (once)

All the individual "X service initialized" messages should become Debug.

### Storage Layer:
- **Info**: Only for storage open/close
- **Debug**: For individual operations
- **Trace**: For detailed operations

### Common Utilities:
- **Info**: Only for significant config events
- **Debug**: For processing details
- **Trace**: For replacements and URL parsing

## Acceptance Criteria
- [ ] app.go startup produces ~6 Info lines instead of ~56
- [ ] main.go has clean startup/shutdown logging
- [ ] Storage layer uses Info only for open/close
- [ ] Common utilities use Debug/Trace for details
- [ ] Compiles successfully
- [ ] Startup is visibly cleaner

## Context
This is the integration task that condenses the verbose startup logging and ensures all remaining files follow the pattern.

## Dependencies Input
All other tasks complete, patterns established

## Output for Dependents
Final integration, ready for review
