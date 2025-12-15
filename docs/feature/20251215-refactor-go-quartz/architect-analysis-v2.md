# Architect Analysis v2 - Actual Queue/Logging Issues
Date: 2025-12-15
Request: "Analyze actual issues with job queue execution and logging - NOT scheduler"

## CRITICAL: Previous Analysis Missed the Point

The previous `architect-analysis.md` focused on replacing `robfig/cron/v3` with `reugn/go-quartz` in the **scheduler service**. This was **NOT** the actual problem.

The user's question was: **"Is go-quartz the correct library to manage the queue?"**

**Answer: NO - go-quartz is for SCHEDULING, not QUEUEING.**

## Actual Issues Identified from Screenshot

Looking at the screenshot (`ksnip_20251215-194556.png`), the visible issues are:

### 1. Out-of-Order Log Line Numbers
```
     3  [INF] Processing item 1/50
    24  [INF] Child job f01b44b5 → running...
     1  [INF] Status changed: running
     2  [INF] Test job generator starting...
    25  [INF] Child job f2d16dcf → running...
     4  [INF] Processing item 2/50
     5  [INF] Processing item 3/50
     5  [INF] Processing item 3/50    <-- DUPLICATE LINE NUMBER
     6  [INF] Processing item 4/50
     6  [INF] Processing item 4/50    <-- DUPLICATE LINE NUMBER
```

**Root Cause:** The `LineNumber` field in logs is assigned **per-job**, but the UI aggregates logs from **multiple jobs** (parent + children). When child job logs (from `f01b44b5`, `f2d16dcf`) interleave with parent job logs, their independent line counters create apparent disorder.

### 2. Duplicate Line Numbers (5, 5, 6, 6, 7, 7)
The screenshot shows duplicate line numbers. This indicates:
- Either the same log is being displayed twice
- Or two different jobs have logs with the same `LineNumber` that are being merged

### 3. Queue Statistics Incorrect
The user reports: "queue statistics (no of queued jobs completed/running/pending/etc) is NOT correct"

This relates to `internal/queue/state/stats.go` and the WebSocket event `EventJobStats` published via `publishJobStats()` in `job_manager.go:629-669`.

### 4. Total Job Numbers Incorrect
Related to how `GetJobChildStats()` counts child jobs and how the UI displays them.

## What go-quartz Does vs What the Queue Needs

| Feature | go-quartz | Current BadgerQueueManager |
|---------|-----------|---------------------------|
| Purpose | Job scheduling (cron) | Job queuing (FIFO) |
| Execution | Trigger-based | Poll/dequeue-based |
| Persistence | In-memory | BadgerDB (durable) |
| Job state | Pending/Running | Pending/Running/Completed/Failed/Cancelled |
| Retry logic | No | Message visibility timeout, DLQ |
| Ordering | Time-based | FIFO by enqueue time |

**go-quartz is for scheduling "when" jobs run, not for queuing/executing job items in order.**

## Real Problems to Fix

### Problem 1: Log Line Number Ordering in Aggregated View

**Location:** `internal/storage/badger/log_storage.go`

The `LineNumber` is per-job (line 73-111), which is correct for single-job log retrieval. However, when logs are aggregated via `GetAggregatedLogs()` (`internal/logs/service.go:141-268`), the k-way merge uses `Sequence` or `FullTimestamp` for ordering, but the UI displays `line_number` from individual jobs.

**Solution:** The UI needs to either:
1. Display a computed index (1, 2, 3...) based on aggregated order, OR
2. Display `Sequence`/timestamp-based ordering for aggregated views

Current code at `pages/queue.html:727-728`:
```html
<span class="tree-log-num" x-text="hasStepEarlierLogs(...) ? (log.line_number || (logIdx + 1)) : (logIdx + 1)">
```

This mixes server-side `line_number` with client-side `logIdx + 1`, causing inconsistency.

### Problem 2: Concurrent Log Writes from Multiple Workers

When multiple workers write logs simultaneously (as in test_job_generator_worker), the `LineNumber` assignment via `sync.Map` + atomic increment (log_storage.go:75-111) should be thread-safe per-job, but:
- If multiple workers share the same parent job ID for logging, their logs interleave
- The k-way merge in `GetAggregatedLogs` uses `Sequence` for cross-job ordering

### Problem 3: Queue Statistics Not Updating Correctly

**Location:** `internal/queue/job_manager.go:629-669`

The `publishJobStats()` is throttled to 500ms and queries storage for counts. Issues:
1. Throttling may miss rapid state changes
2. Storage queries may not reflect in-flight status updates
3. WebSocket broadcast may be lost if client disconnects/reconnects

## Recommendation: DO NOT USE go-quartz for Queue Management

The current architecture with `BadgerQueueManager` is appropriate for job queuing. The issues are:

1. **Log display bug** - UI/frontend issue, not queue issue
2. **Statistics accuracy** - Event timing/throttling issue
3. **Job ordering** - Working correctly per the k-way merge algorithm

## Proposed Fixes (MODIFY existing code)

### Fix 1: Consistent Log Line Display in Aggregated View

Modify `pages/queue.html` to always use computed index for aggregated logs:
```javascript
// Always use sequential index (1, 2, 3...) for displayed logs
// The log.line_number is per-job, not per-aggregated-view
<span class="tree-log-num" x-text="logIdx + 1 + offset">
```

### Fix 2: Add Global Sequence Display (Optional)

For debugging, show the `log.sequence` or `log.full_timestamp` alongside the display line number.

### Fix 3: Verify Stats Publishing

Add logging to `publishJobStats()` to trace when stats are being sent and what values are included.

## Files to Investigate Further

| File | Issue | Priority |
|------|-------|----------|
| `pages/queue.html` | Log line number display logic | HIGH |
| `internal/logs/service.go` | k-way merge ordering | MEDIUM |
| `internal/queue/job_manager.go` | Stats publishing | HIGH |
| `internal/storage/badger/log_storage.go` | LineNumber assignment | LOW |

## Anti-Patterns Check

| Anti-Pattern | Current Status |
|--------------|----------------|
| Creating parallel structure | NOT NEEDED - fix existing |
| Duplicating existing logic | NOT NEEDED |
| Replacing working queue with scheduler | WRONG - don't do this |

## Verdict

**DO NOT replace BadgerQueueManager with go-quartz.** The go-quartz refactor in the scheduler service (already done) was correct for the scheduler but irrelevant to the queue/logging issues.

The actual issues are:
1. **Frontend bug**: Log line number display in aggregated view
2. **Backend enhancement**: Stats publishing accuracy
3. **Testing**: Verify with test_job_generator worker

## Success Criteria

- [ ] Log lines display in correct chronological order (1, 2, 3...)
- [ ] No duplicate line numbers in UI
- [ ] Queue statistics reflect actual job states
- [ ] Total job counts are accurate
