# Task 3: Enhance Panic Recovery with Crash File Logging

## Metadata
- **ID:** 3
- **Group:** 2
- **Mode:** concurrent
- **Skill:** @go-coder
- **Complexity:** medium
- **Model:** claude-sonnet-4-5-20250929
- **Critical:** no
- **Depends:** 1
- **Blocks:** 5

## Paths
```yaml
sandbox: /tmp/3agents/task-3/
source: C:/development/quaero/
output: C:/development/quaero/docs/fixes/20251127-crash-protection/
```

## Files to Modify
- `internal/queue/workers/job_processor.go` - Enhance existing panic recovery

## Requirements
Enhance the existing panic recovery in job_processor.go:

1. **Improve processJobs panic recovery**:
   - Write to crash file in addition to logger
   - Include all goroutines dump
   - Flush logger before any Fatal call
   - Add small delay before exit to ensure file writes complete

2. **Improve processNextJob panic recovery**:
   - Add job context to panic logs (job ID, type, config)
   - Include time since job started
   - Ensure message is deleted from queue to prevent infinite loop

3. **Add defensive checks**:
   - Check for nil queue manager before Receive
   - Check for nil job manager before operations
   - Log warnings for unexpected nil values

4. **Add job processing counter**:
   - Track total jobs processed since startup
   - Log every 50 jobs for health monitoring
   - Include in crash reports

## Acceptance Criteria
- [ ] Panic recovery writes to crash file
- [ ] All goroutines included in crash dump
- [ ] Job context included in panic logs
- [ ] Defensive nil checks added
- [ ] Job counter added with periodic logging
- [ ] Compiles successfully

## Context
Current panic recovery at line 94-107 uses `jp.logger.Fatal()` which may not write the message if the logger is in a bad state. Direct file writes are more reliable for crash scenarios.

## Dependencies Input
From Task 1: Specific crash location hypothesis

## Output for Dependents
- Enhanced panic recovery pattern
- Job processing statistics
