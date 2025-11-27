# Task 4: Standardize Handlers

## Metadata
- **ID:** 4
- **Group:** 2
- **Mode:** concurrent
- **Skill:** @go-coder
- **Critical:** no
- **Depends:** 1
- **Blocks:** 5

## Paths
```yaml
sandbox: /tmp/3agents/task-4/
source: C:/development/quaero/
output: C:/development/quaero/docs/features/20251127-logging-standardization/
```

## Files to Modify
- `internal/handlers/job_handler.go` - Standardize log levels
- `internal/handlers/job_definition_handler.go` - Standardize log levels
- `internal/handlers/document_handler.go` - Standardize log levels
- `internal/handlers/websocket.go` - Standardize log levels
- `internal/handlers/websocket_events.go` - Standardize log levels
- `internal/handlers/auth_handler.go` - Standardize log levels
- `internal/handlers/config_handler.go` - Standardize log levels
- `internal/handlers/connector_handler.go` - Standardize log levels
- `internal/handlers/kv_handler.go` - Standardize log levels
- `internal/handlers/mcp.go` - Standardize log levels
- `internal/handlers/page_handler.go` - Standardize log levels
- `internal/handlers/scheduler_handler.go` - Standardize log levels
- `internal/handlers/search_handler.go` - Standardize log levels
- `internal/handlers/system_logs_handler.go` - Standardize log levels
- `internal/handlers/helpers.go` - Standardize log levels

## Requirements
Apply the following log level rules:
1. **Info**: Only for significant user actions (authentication success, bulk operations)
2. **Debug**: For request handling details, parameter logging
3. **Trace**: For detailed request/response tracing
4. **Warn**: For invalid requests that are handled gracefully (400 errors)
5. **Error**: For actual server errors (500 errors)

### Key Patterns to Fix:
- Most handler Info logs are request handling -> Debug
- Debug logs for request params -> Trace
- Many Error logs for user errors (404, 400) -> Warn

## Acceptance Criteria
- [ ] All handler files use Info only for significant user actions
- [ ] Request handling details moved to Debug
- [ ] Parameter logging moved to Trace
- [ ] User errors (4xx) use Warn, server errors (5xx) use Error
- [ ] Compiles successfully

## Context
Handlers are the HTTP layer. Info should show significant user actions, not every request handled. Debug shows request flow for troubleshooting.

## Dependencies Input
Pattern established in Task 1 for workers

## Output for Dependents
Handlers follow consistent pattern, ready for app.go integration
