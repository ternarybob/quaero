# Step 5: Standardize App.go and Startup Logging

## Task Reference
- **Task File:** task-5.md
- **Group:** 3 (sequential)
- **Dependencies:** Tasks 2, 3, 4

## Actions Taken
1. Reviewed app.go - found ~50+ Info logs for individual service initialization
2. Converted all "X service initialized" messages to Debug
3. Converted all "X worker registered" messages to Debug
4. Converted all "X manager registered" messages to Debug
5. Converted configuration loading logs to Debug
6. Kept only the final "Application initialization complete" as Info

## Files Modified
- `internal/app/app.go` - 50+ Info to Debug conversions

### Specific Changes
| Original Info Message | New Level | Rationale |
|----------------------|-----------|-----------|
| Log consumer initialized | Debug | Interim setup |
| Job processor started | Debug | Interim setup |
| WebSocket handlers started | Debug | Interim setup |
| Storage layer initialized | Debug | Interim setup |
| Applied key/value replacements | Debug | Interim setup |
| LLM service initialized | Debug | Interim setup |
| Status service initialized | Debug | Interim setup |
| System logs service initialized | Debug | Interim setup |
| Queue manager initialized | Debug | Interim setup |
| Job manager initialized | Debug | Interim setup |
| Job processor initialized | Debug | Interim setup |
| Job service initialized | Debug | Interim setup |
| Variables service initialized | Debug | Interim setup |
| Config service initialized | Debug | Interim setup |
| Connector service initialized | Debug | Interim setup |
| Crawler service initialized | Debug | Interim setup |
| Crawler URL worker registered | Debug | Interim setup |
| GitHub Log worker registered | Debug | Interim setup |
| Job monitor created | Debug | Interim setup |
| Database maintenance worker registered | Debug | Interim setup |
| Transform service initialized | Debug | Interim setup |
| Places service initialized | Debug | Interim setup |
| Chat service initialized | Debug | Interim setup |
| Agent service initialized | Debug | Interim setup |
| Agent worker registered | Debug | Interim setup |
| Crawler manager registered | Debug | Interim setup |
| Transform manager registered | Debug | Interim setup |
| Reindex manager registered | Debug | Interim setup |
| Database maintenance manager registered | Debug | Interim setup |
| Places search manager registered | Debug | Interim setup |
| Agent manager registered | Debug | Interim setup |
| Orchestrator initialized | Debug | Interim setup |
| Scheduler service started | Debug | Interim setup |
| EventSubscriber initialized | Debug | Interim setup |
| KV handler initialized | Debug | Interim setup |
| Stale job detector started | Debug | Interim setup |

### Remaining Info Logs (Correct)
- "Application initialization complete" - final summary, appropriate for Info
- Warning messages for failed service initialization - appropriate for Warn
- Error messages for fatal initialization errors - appropriate for Error

## Decisions Made
- **Condensed startup**: Startup now shows ~6 Info lines instead of ~50+
- **Debug for all interim**: Each service init is an interim update, not significant
- **Info for summary only**: Only final "complete" message is Info

## Acceptance Criteria
- [x] app.go startup produces ~6 Info lines instead of ~50+
- [x] All individual service init messages are Debug
- [x] Compiles successfully
- [x] Startup is visibly cleaner

## Verification
```
go build -o /tmp/quaero-test ./cmd/quaero/...
Result: Pass
```

## Status: COMPLETE
