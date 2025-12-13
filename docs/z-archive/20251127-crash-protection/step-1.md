# Step 1: Investigate Crash Location with Detailed Logging

## Task Reference
- **Task File:** task-1.md
- **Group:** 1 (sequential)
- **Model Used:** claude-sonnet-4-5-20250929
- **Dependencies:** None

## Params
- Sandbox: /tmp/3agents/task-1/
- Source: C:/development/quaero/
- Output: C:/development/quaero/docs/fixes/20251127-crash-protection/

## Investigation Summary

### Log Analysis
The crash log shows normal operation until 16:05:22:
```
time=16:05:22 level=INF message="Job completed" job_id=72538116-2022-47ec-aaf9-4eba445aef79 job_type=crawler_url duration=1.234567s
```
**Then the log ends abruptly - no more entries, no "Job started" for next job.**

### Crash Location Hypothesis
The crash occurs in the `processNextJob()` loop between:
1. After completing job 72538116... at 16:05:22
2. Before the next "Job started" log

This narrows the crash to:
```go
// job_processor.go:111-120
for {
    select {
    case <-jp.ctx.Done():
        ...
    default:
        jp.processNextJob()  // <-- Crash somewhere in here
    }
}
```

Within `processNextJob()`, the crash must be between:
- Line 296-302: `deleteFn()` completing successfully (after "Job completed" log)
- Line 163: `jp.queueMgr.Receive(ctx)` call in next iteration

### Key Finding: Async Goroutines Without Panic Recovery

**Critical async goroutines that could crash the service:**

| File | Line | Function | Risk |
|------|------|----------|------|
| crawler_worker.go | 1385 | `publishCrawlerJobLog` | HIGH - frequently called |
| crawler_worker.go | 1430 | `publishCrawlerProgressUpdate` | HIGH - frequently called |
| crawler_worker.go | 1489 | `publishJobSpawnEvent` | MEDIUM - called on link discovery |
| agent_worker.go | 294 | `publishAgentJobLog` | MEDIUM |
| state/monitor.go | 301 | `publishParentJobProgress` | MEDIUM |
| state/monitor.go | 332 | `publishChildJobStats` | MEDIUM |
| state/monitor.go | 637 | `publishParentJobProgress` | MEDIUM |
| state/runtime.go | 73 | event publishing | MEDIUM |
| queue/manager.go | 507 | event publishing | MEDIUM |
| logs/consumer.go | 204 | log event publishing | LOW |
| document_persister.go | 99 | document event publishing | LOW |

### Most Likely Crash Scenarios

1. **Event Publisher Panic in Goroutine**
   - If `w.eventService.Publish()` panics (nil receiver, closed channel)
   - The goroutine crashes silently, taking down the whole process

2. **Badger Database Corruption**
   - `Receive()` in `badger_manager.go:114` iterates over Badger keys
   - If Badger has corruption, the iterator could panic

3. **Memory Pressure / OOM**
   - After 62 jobs, accumulated browser processes or leaked goroutines
   - Process killed by OS without chance to log

### Evidence Supporting Hypothesis
1. The crash happens after ~62 jobs (not immediately)
2. Test doesn't crash because it processes fewer jobs with different timing
3. No panic log despite having panic recovery at line 97-107
4. The log shows no error, just stops

### Recommended Fixes
1. **Add SafeGo wrapper** for all async event publishing goroutines
2. **Add crash file logging** in panic recovery (direct file write, not logger)
3. **Add pre-receive logging** to narrow down exact crash point
4. **Add process-level crash protection** with SIGABRT handling

## Async Goroutine Spawn Points Needing Recovery

### High Priority (in job processing path)
1. `internal/queue/workers/crawler_worker.go:1385` - publishCrawlerJobLog
2. `internal/queue/workers/crawler_worker.go:1430` - publishCrawlerProgressUpdate
3. `internal/queue/workers/crawler_worker.go:1489` - publishJobSpawnEvent
4. `internal/queue/workers/agent_worker.go:294` - publishAgentJobLog

### Medium Priority (in monitoring/state path)
5. `internal/queue/state/monitor.go:65` - already has recovery!
6. `internal/queue/state/monitor.go:301` - publishParentJobProgress
7. `internal/queue/state/monitor.go:332` - publishChildJobStats
8. `internal/queue/state/monitor.go:637` - publishParentJobProgress
9. `internal/queue/state/runtime.go:73` - event publishing

### Lower Priority (infrastructure)
10. `internal/queue/manager.go:507` - publishJobStatusUpdate
11. `internal/logs/consumer.go:204` - log event publishing
12. `internal/services/crawler/document_persister.go:99` - document event
13. `internal/handlers/job_definition_handler.go:489` - job execution
14. `internal/handlers/job_definition_handler.go:919` - job execution
15. `internal/app/app.go:732` - stale job detector
16. `internal/handlers/websocket.go:429` - client count ticker

## Acceptance Criteria
- [x] All async goroutine spawn locations documented (16 found)
- [x] Hypothesis documented in step output
- [x] Critical crash points identified
- [x] Compiles successfully (no changes made)

## Output for Dependents
- **List of async goroutine spawn points:** 16 locations identified
- **Hypothesis:** Event publisher goroutine panic in crawler_worker.go (lines 1385, 1430, 1489)
- **Secondary hypothesis:** Badger database iterator panic in Receive()

## Status: ✅ COMPLETE
