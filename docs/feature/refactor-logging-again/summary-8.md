# Summary: Adaptive Backoff Rate Limiting for SSE Log Streams

## Issue
Client browser was being overloaded by high-throughput log streaming (100+ logs/second), causing performance issues.

## Previous Implementation
- Fixed 50ms batch interval
- Hard limit of 50 logs per batch (dropped excess)
- No adaptive behavior based on throughput

## New Implementation
Implemented adaptive backoff rate limiting with progressive intervals:

**Backoff Levels:** 1s → 2s → 3s → 4s → 5s → 10s (max)

### Algorithm
1. **Base interval**: Start at 1 second
2. **High throughput detection**: If >50 logs received in current interval, increase backoff level
3. **Recovery**: If <25 logs received, decrease backoff level (recover faster)
4. **Status change**: Always flush immediately and reset to base interval on job status change (completion)
5. **No dropping**: Logs are batched and sent together, not dropped

### Key Changes

**`internal/handlers/sse_logs_handler.go`:**

For job logs (`streamJobLogs`):
```go
backoffLevels := []time.Duration{
    1 * time.Second,
    2 * time.Second,
    3 * time.Second,
    4 * time.Second,
    5 * time.Second,
    10 * time.Second,
}
currentBackoffLevel := 0
currentInterval := backoffLevels[0]
const logsPerIntervalThreshold = 50

// On tick: adjust interval based on throughput
if logsReceivedThisInterval > logsPerIntervalThreshold {
    // Increase backoff
    currentBackoffLevel++
    currentInterval = backoffLevels[currentBackoffLevel]
} else if logsReceivedThisInterval < logsPerIntervalThreshold/2 {
    // Decrease backoff (recover)
    currentBackoffLevel--
    currentInterval = backoffLevels[currentBackoffLevel]
}

// Status change: flush immediately and reset to base
case status := <-sub.status:
    h.sendJobLogBatch(...)
    h.sendStatus(...)
    currentBackoffLevel = 0
    currentInterval = backoffLevels[0]
```

For service logs (`streamServiceLogs`): Same backoff strategy applied.

## Benefits
1. **Adaptive**: Automatically adjusts to log throughput
2. **No data loss**: Logs are batched, not dropped
3. **Fast recovery**: When throughput drops, interval decreases
4. **Completion guarantee**: Always flushes on status change (job completion)
5. **Configurable**: Easy to adjust thresholds and intervals

## Behavior Examples
- Low throughput (<50 logs/sec): 1 second batches
- Medium throughput (~100 logs/sec): 2-3 second batches
- High throughput (200+ logs/sec): 5-10 second batches
- Job completion: Immediate flush regardless of backoff level
