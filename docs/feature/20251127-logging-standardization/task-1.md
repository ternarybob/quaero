# Task 1: Standardize Queue Workers

## Metadata
- **ID:** 1
- **Group:** 1
- **Mode:** sequential
- **Skill:** @go-coder
- **Critical:** no
- **Depends:** none
- **Blocks:** 2, 3, 4

## Paths
```yaml
sandbox: /tmp/3agents/task-1/
source: C:/development/quaero/
output: C:/development/quaero/docs/features/20251127-logging-standardization/
```

## Files to Modify
- `internal/queue/workers/job_processor.go` - Standardize log levels
- `internal/queue/workers/crawler_worker.go` - Convert Info->Debug, Debug->Trace for detailed tracing
- `internal/queue/workers/agent_worker.go` - Standardize log levels
- `internal/queue/workers/github_log_worker.go` - Standardize log levels
- `internal/queue/workers/database_maintenance_worker.go` - Standardize log levels

## Requirements
Apply the following log level rules:
1. **Info**: Only for job start (one line) and job completion (one line with summary)
2. **Debug**: For interim progress updates during job execution
3. **Trace**: For detailed internal tracing (function entry, parameter logging, etc.)
4. **Warn**: For non-fatal issues that don't stop execution
5. **Error**: For actual errors that cause job failure

### Specific Transformations:
- `Info().Msg("Starting...")` -> Keep as Info (process start)
- `Info().Msg("...completed")` -> Keep as Info (process end)
- `Info().Msg("Processing...")` / interim status -> Debug
- `Debug().Msg("...")` detailed tracing -> Trace
- Keep Warn and Error as-is unless they're misclassified

## Acceptance Criteria
- [ ] job_processor.go uses Info only for processor start/stop
- [ ] crawler_worker.go uses Info only for job start/end
- [ ] agent_worker.go uses Info only for job start/end
- [ ] github_log_worker.go uses Info only for job start/end
- [ ] database_maintenance_worker.go uses Info only for operation start/end
- [ ] All detailed tracing moved from Debug to Trace
- [ ] Compiles successfully

## Context
These are the core worker files that execute queue jobs. They should follow consistent patterns where Info is reserved for high-level lifecycle events and Debug/Trace are used for internal processing details.

## Dependencies Input
N/A - First task

## Output for Dependents
Establishes the logging pattern for all other files to follow.
