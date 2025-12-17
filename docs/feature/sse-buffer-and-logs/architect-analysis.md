# Architect Analysis: SSE Buffer Overrun and Log Identification

## Issues

### Issue 1: SSE Buffer Overrun
**Location:** `internal/handlers/sse_logs_handler.go:311-316`

**Problem:** When high-throughput job logging occurs (e.g., 301 parallel workers), the SSE handler's buffered channel (capacity 500) fills faster than it can be drained. The log shows 1930 occurrences of "Buffer full, skipping entry" - meaning 1930 log entries were dropped.

**Root Cause Analysis:**
1. Buffer size is 500 entries (line 579)
2. Batch ticker starts at 1s intervals, backs off to 10s max
3. With 301 workers generating logs simultaneously, entries flood in faster than 50/second threshold
4. Non-blocking channel send (`select { case sub.logs <- entry: default: }`) drops entries

**Current Implementation:**
```go
// Line 579: Buffer of 500
logs:   make(chan jobLogEntry, 500),

// Line 311-316: Non-blocking send drops entries
select {
case sub.logs <- entry:
    routedCount++
default:
    h.logger.Warn().Str("job_id", matchJobID).Msg("[SSE DEBUG] Buffer full, skipping entry")
}
```

**Options:**
1. **Increase buffer size** - Simple but doesn't solve fundamental issue
2. **Apply backpressure** - Block sender, but would slow down workers
3. **Batch at source** - Aggregate before sending (current approach, just needs tuning)
4. **Discard strategy** - Keep newest or oldest N entries (current drops random)

**Recommendation:** EXTEND existing adaptive backoff mechanism with larger buffer and smarter batching. The current buffer is too small for parallel jobs with 300+ workers.

### Issue 2: Log Step/Worker Identification
**Location:** `internal/queue/state/runtime.go:47-48`

**Problem:** Log messages like "Status changed: running" and "Status changed: completed" appear multiple times without identifying WHICH step or worker changed status. This makes logs confusing and hard to debug.

**Current Implementation:**
```go
// Line 47-48 in runtime.go
logMessage := fmt.Sprintf("Status changed: %s", status)
if err := m.AddJobLog(ctx, jobID, "info", logMessage); err != nil {
```

The log is added to the job's log storage with job_id context, but the MESSAGE itself doesn't identify what changed.

**Problem Scope:**
- `runtime.go:47` - Status changes don't identify job type/name
- Job logs show generic messages without step context
- UI shows confusing repeated status messages

**Recommendation:** MODIFY existing log message to include job identification (name, type) in the message text itself.

## Analysis Summary

| Issue | Type | Files to Modify | Complexity |
|-------|------|-----------------|------------|
| SSE Buffer | EXTEND | `sse_logs_handler.go` | Medium |
| Log Identification | MODIFY | `runtime.go` | Low |

## Proposed Changes

### Issue 1: SSE Buffer Fix
1. Increase base buffer from 500 to 2000 entries
2. Adjust backoff thresholds for high-throughput scenarios
3. Add buffer utilization logging at debug level

### Issue 2: Log Identification
1. Modify `UpdateJobStatus` in `runtime.go` to include job name in status log
2. Format: "Status changed: {status} (job: {name}, type: {type})"

## Anti-Creation Check
- No new files needed
- All changes extend/modify existing code
- Follows existing patterns in codebase
