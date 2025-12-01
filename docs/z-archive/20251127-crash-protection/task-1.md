# Task 1: Investigate Crash Location with Detailed Logging

## Metadata
- **ID:** 1
- **Group:** 1
- **Mode:** sequential
- **Skill:** @debugger
- **Complexity:** medium
- **Model:** claude-sonnet-4-5-20250929
- **Critical:** no
- **Depends:** none
- **Blocks:** 2, 3, 4

## Paths
```yaml
sandbox: /tmp/3agents/task-1/
source: C:/development/quaero/
output: C:/development/quaero/docs/fixes/20251127-crash-protection/
```

## Files to Modify
- `internal/queue/workers/job_processor.go` - Add detailed logging before/after queue receive
- `internal/storage/badger/queue.go` - Add logging around Receive operations

## Requirements
Investigate the exact location where the crash occurs by adding strategic logging:

1. **Add pre-receive logging** in `processNextJob()`:
   - Log before calling `queueMgr.Receive(ctx)`
   - Log immediately after successful receive
   - This will help identify if crash is in Receive operation

2. **Add timing/state logging**:
   - Log the number of jobs processed since startup
   - Log memory usage periodically (every 10 jobs)
   - Log any pending goroutine count

3. **Check for silent failures**:
   - Review `queue.Manager` and `QueueManager` interface methods for places that might panic
   - Look for nil pointer dereferences in the chain

4. **Document findings**:
   - Create hypothesis about crash location
   - Identify all async goroutines that could panic without recovery

## Acceptance Criteria
- [ ] Strategic logging added to narrow down crash location
- [ ] Memory/goroutine monitoring added
- [ ] All async goroutine spawn locations documented
- [ ] Hypothesis documented in step output
- [ ] Compiles successfully

## Context
The log ends at `"Job completed"` for job 72538116-2022-47ec-aaf9-4eba445aef79 at 16:05:22.
No "Job started" appears for the next job, suggesting crash occurs either:
1. In the queue Receive operation
2. In the for loop between job completion and next receive
3. In the Badger database operation during message fetch

## Dependencies Input
N/A - this is the first task

## Output for Dependents
- List of async goroutine spawn points needing panic recovery
- Hypothesis about crash root cause
- Locations where logging was added
