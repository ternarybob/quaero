# Task 3: Add Info Summary Logs to agent_worker.go

## Metadata
- **ID:** 3
- **Group:** 1
- **Mode:** concurrent
- **Skill:** @go-coder
- **Critical:** no
- **Depends:** none
- **Blocks:** 4

## Paths
```yaml
sandbox: /tmp/3agents/task-3/
source: C:/development/quaero/
output: docs/features/20251127-queue-job-logging/
```

## Files to Modify
- `internal/queue/workers/agent_worker.go` - Ensure worker has appropriate log levels

## Requirements
The agent_worker.go should use appropriate log levels. Since the job_processor.go will handle centralized Info-level logging, the worker should:

1. **Verify existing logs are at appropriate levels** - Debug for interim updates, Trace for detailed tracing

Current logging in Execute():
- Line 89-93: Debug "Starting agent job execution" - CORRECT (interim)
- Line 253-258: Debug "Agent job execution completed successfully" - CORRECT (interim)

The worker's Debug logs provide worker-specific details while the processor provides the canonical Info-level start/end.

## Acceptance Criteria
- [ ] Worker uses Debug for interim operation logs
- [ ] Worker uses Trace for detailed tracing
- [ ] No duplicate Info-level start/end logs (processor handles this)
- [ ] Compiles successfully

## Context
The agent_worker handles AI agent execution on documents. Its logs should focus on the agent-specific details at Debug/Trace levels, while the processor provides the canonical Info-level job lifecycle logs.

## Dependencies Input
N/A

## Output for Dependents
Worker logging is properly layered with processor logging.
