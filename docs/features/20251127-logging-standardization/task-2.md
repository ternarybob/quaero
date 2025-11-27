# Task 2: Standardize Queue System

## Metadata
- **ID:** 2
- **Group:** 2
- **Mode:** concurrent
- **Skill:** @go-coder
- **Critical:** no
- **Depends:** 1
- **Blocks:** 5

## Paths
```yaml
sandbox: /tmp/3agents/task-2/
source: C:/development/quaero/
output: C:/development/quaero/docs/features/20251127-logging-standardization/
```

## Files to Modify
- `internal/queue/orchestrator.go` - Standardize log levels
- `internal/queue/state/monitor.go` - Standardize log levels
- `internal/queue/managers/agent_manager.go` - Standardize log levels
- `internal/queue/managers/crawler_manager.go` - Standardize log levels
- `internal/queue/managers/database_maintenance_manager.go` - Standardize log levels
- `internal/queue/managers/places_search_manager.go` - Standardize log levels
- `internal/queue/managers/reindex_manager.go` - Standardize log levels
- `internal/queue/managers/transform_manager.go` - Standardize log levels

## Requirements
Apply the following log level rules:
1. **Info**: Only for orchestrator/manager start, major phase transitions, completion summaries
2. **Debug**: For interim updates (job enqueued, status changed, etc.)
3. **Trace**: For detailed internal state tracking
4. **Warn**: For recoverable issues
5. **Error**: For actual failures

### Specific Transformations:
- `Info().Msg("Job definition loaded")` -> Debug (interim update)
- `Info().Msg(" Parent job status updated")` -> Debug (interim update)
- `Info().Msg("Child jobs enqueued")` -> Debug (interim update)
- `Info().Msg("Manager registered")` -> Keep Info (one-time startup)
- `Debug().Msg(...)` detailed internal state -> Trace

## Acceptance Criteria
- [ ] orchestrator.go uses Info only for orchestration start/end and major milestones
- [ ] monitor.go uses Info sparingly, Debug for status updates
- [ ] All manager files use Info only for registration and major events
- [ ] Detailed state tracking moved from Debug to Trace
- [ ] Compiles successfully

## Context
The queue system orchestrates job execution. Info should reflect high-level orchestration events visible to operators, while Debug shows the job processing flow for troubleshooting.

## Dependencies Input
Pattern established in Task 1 for workers

## Output for Dependents
Queue system follows the same pattern, ready for app.go integration
