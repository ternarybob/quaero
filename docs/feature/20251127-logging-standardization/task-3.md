# Task 3: Standardize Services

## Metadata
- **ID:** 3
- **Group:** 2
- **Mode:** concurrent
- **Skill:** @go-coder
- **Critical:** no
- **Depends:** 1
- **Blocks:** 5

## Paths
```yaml
sandbox: /tmp/3agents/task-3/
source: C:/development/quaero/
output: C:/development/quaero/docs/features/20251127-logging-standardization/
```

## Files to Modify
- `internal/services/crawler/service.go` - Standardize log levels
- `internal/services/crawler/executor.go` - Standardize log levels
- `internal/services/crawler/chromedp_pool.go` - Standardize log levels
- `internal/services/crawler/document_persister.go` - Standardize log levels
- `internal/services/crawler/html_scraper.go` - Standardize log levels
- `internal/services/crawler/link_extractor.go` - Standardize log levels
- `internal/services/crawler/content_processor.go` - Standardize log levels
- `internal/services/crawler/filters.go` - Standardize log levels
- `internal/services/agents/service.go` - Standardize log levels
- `internal/services/scheduler/scheduler_service.go` - Standardize log levels
- `internal/services/events/event_service.go` - Standardize log levels
- `internal/services/events/logger_subscriber.go` - Standardize log levels
- `internal/services/auth/service.go` - Standardize log levels
- `internal/services/chat/chat_service.go` - Standardize log levels
- `internal/services/chat/agent_loop.go` - Standardize log levels
- `internal/services/config/config_service.go` - Standardize log levels
- `internal/services/connectors/service.go` - Standardize log levels
- `internal/services/documents/document_service.go` - Standardize log levels
- `internal/services/kv/service.go` - Standardize log levels
- `internal/services/llm/gemini_service.go` - Standardize log levels
- `internal/services/mcp/router.go` - Standardize log levels
- `internal/services/mcp/document_service.go` - Standardize log levels
- `internal/services/places/service.go` - Standardize log levels
- `internal/services/search/factory.go` - Standardize log levels
- `internal/services/search/advanced_search_service.go` - Standardize log levels
- `internal/services/search/fts5_search_service.go` - Standardize log levels
- `internal/services/search/disabled_search_service.go` - Standardize log levels
- `internal/services/status/service.go` - Standardize log levels
- `internal/services/summary/summary_service.go` - Standardize log levels
- `internal/services/transform/service.go` - Standardize log levels
- `internal/services/workers/pool.go` - Standardize log levels
- `internal/services/jobs/service.go` - Standardize log levels

## Requirements
Apply the following log level rules:
1. **Info**: Only for service initialization complete, major operation results
2. **Debug**: For interim processing steps, cache operations, individual item processing
3. **Trace**: For detailed function tracing, parameter logging
4. **Warn**: For recoverable issues (cache miss, fallback used)
5. **Error**: For actual failures

### Key Patterns to Fix:
- Scheduler service has many Info logs for schedule checks -> Debug
- Crawler service has detailed Info logs -> Debug/Trace
- Event service Debug logs are detailed -> Trace

## Acceptance Criteria
- [ ] All service files use Info only for init and major results
- [ ] Processing details moved to Debug
- [ ] Internal tracing moved to Trace
- [ ] Warn/Error appropriately classified
- [ ] Compiles successfully

## Context
Services are the business logic layer. Info should show service health and major operations, not internal processing details.

## Dependencies Input
Pattern established in Task 1 for workers

## Output for Dependents
Services follow consistent pattern, ready for handlers and app.go
