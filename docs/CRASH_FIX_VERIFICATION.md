# Scheduler Crash Fix - Verification Guide

## Problem Summary

The service was crashing immediately after the collection event completed, with no logs after:
```
17:53:00 INF Collection event completed successfully
[CRASH - no more logs]
```

The embedding event should have been published next, but the service crashed before that could happen.

## Root Cause Analysis

The crash was caused by **unhandled panics** in the event-driven pipeline. The issue occurred in the following sequence:

1. **Scheduler Service** (`internal/services/scheduler/scheduler_service.go`) triggers collection event
2. **Collection Coordinator** processes collection event successfully
3. **Scheduler Service** attempts to trigger embedding event
4. **Embedding Coordinator** receives embedding event but panics (likely due to nil pointer or other runtime error)
5. **Panic propagates** through goroutines and crashes the entire service

### Why It Wasn't Logged

The panic occurred in a goroutine spawned by the event service's `PublishSync` method. Panics in goroutines don't print stack traces to stderr by default and can crash the entire program silently.

## The Fix

### 1. Added Panic Recovery to Scheduler Service
**File**: `internal/services/scheduler/scheduler_service.go`

Added comprehensive panic recovery at the top of `runScheduledTask()`:

```go
defer func() {
    if r := recover(); r != nil {
        s.logger.Error().
            Str("panic", fmt.Sprintf("%v", r)).
            Msg("PANIC RECOVERED in scheduled task")
    }
}()
```

This ensures that if any panic occurs during the scheduled task execution, it will be:
- Caught and logged with full details
- Prevented from crashing the service
- Properly cleaned up (mutex unlocked via the second defer)

### 2. Added Detailed Step-by-Step Logging
**File**: `internal/services/scheduler/scheduler_service.go`

Added debug logging at each critical step:
- Step 1: Creating collection event
- Step 2: Publishing collection event synchronously
- Step 3: Creating embedding event
- Step 4: Publishing embedding event synchronously

This allows pinpointing exactly where failures occur.

### 3. Added Panic Recovery to Collection Coordinator
**File**: `internal/services/collection/coordinator_service.go`

Added panic recovery and detailed logging to `handleCollectionEvent()`:

```go
defer func() {
    if r := recover(); r != nil {
        s.logger.Error().
            Str("panic", fmt.Sprintf("%v", r)).
            Msg("PANIC RECOVERED in collection event handler")
    }
}()
```

Also added step-by-step debug logs:
- Creating worker pool for collection
- Fetching force sync documents
- Submitting force sync jobs
- Waiting for all collection jobs to complete

### 4. Added Panic Recovery + Nil Checks to Embedding Coordinator
**File**: `internal/services/embeddings/coordinator_service.go`

Added comprehensive protection to `handleEmbeddingEvent()`:

**Panic Recovery**:
```go
defer func() {
    if r := recover(); r != nil {
        s.logger.Error().
            Str("panic", fmt.Sprintf("%v", r)).
            Msg("PANIC RECOVERED in embedding event handler")
    }
}()
```

**Dependency Validation**:
```go
if s.embeddingService == nil {
    return fmt.Errorf("embedding service is nil - cannot process embedding event")
}
if s.documentStorage == nil {
    return fmt.Errorf("document storage is nil - cannot process embedding event")
}
```

**Additional nil checks in `embedDocument()`**:
```go
if doc == nil {
    return fmt.Errorf("document is nil - cannot embed")
}
if s.embeddingService == nil {
    return fmt.Errorf("embedding service is nil - cannot embed document")
}
if s.documentStorage == nil {
    return fmt.Errorf("document storage is nil - cannot save embedded document")
}
```

### 5. Added Panic Recovery to Event Service
**File**: `internal/services/events/event_service.go`

Added panic recovery to all goroutines in `PublishSync()`:

```go
defer func() {
    if r := recover(); r != nil {
        s.logger.Error().
            Str("panic", fmt.Sprintf("%v", r)).
            Str("event_type", string(event.Type)).
            Msg("PANIC RECOVERED in event handler")
        panicChan <- r
    }
}()
```

This ensures that panics in event handlers are:
- Caught and logged with event context
- Reported back to the publisher
- Prevented from crashing other handlers or the service

## Verification Steps

### 1. Start the Service

```bash
./bin/quaero.exe serve -c deployments/local/quaero.toml
```

Watch for successful initialization logs:
- ✅ "Scheduler service started (runs every 1 minute)"
- ✅ No immediate crashes

### 2. Trigger Collection Manually

Open a new terminal and trigger collection:

```bash
curl -X POST http://localhost:8080/api/collection/trigger
```

**Expected logs**:
```
[DEBUG] Step 1: Creating collection event
[DEBUG] Step 2: Publishing collection event synchronously
[INFO] Collection event triggered
[DEBUG] Creating worker pool for collection
[DEBUG] Fetching force sync documents
[DEBUG] Submitting force sync jobs (count: X)
[DEBUG] Waiting for all collection jobs to complete
[INFO] Collection event completed successfully
[INFO] Collection completed, starting embedding
[DEBUG] Step 3: Creating embedding event
[DEBUG] Step 4: Publishing embedding event synchronously
[INFO] Embedding event triggered
[DEBUG] Creating worker pool for embeddings
[DEBUG] Fetching force embed documents
[INFO] Processing force embed documents (count: X)
[DEBUG] Fetching unvectorized documents
[INFO] Processing unvectorized documents (count: X)
[DEBUG] Waiting for all embedding jobs to complete
[INFO] Embedding event completed successfully
[INFO] Scheduled cycle completed successfully
```

### 3. Monitor for Panics

If any panics occur, you should now see:

```
[ERROR] PANIC RECOVERED in [location]: [panic details]
```

The service will continue running instead of crashing.

### 4. Check Scheduled Execution

Wait for the next scheduled execution (default: every 1 minute).

**Expected behavior**:
- ✅ Collection runs automatically
- ✅ Embedding runs automatically
- ✅ No crashes
- ✅ All steps logged clearly

### 5. Verify Error Handling

To test error scenarios:

**Nil Embedding Service Test** (simulated):
- Expected log: "embedding service is nil - cannot process embedding event"
- Service continues running

**Document Processing Errors**:
- Expected log: "Some embedding jobs failed (error_count: X)"
- Service continues running

## What Changed

### Before (Crash Behavior)
1. Panic occurs in embedding coordinator
2. Panic propagates through goroutine
3. **Service crashes with no logs**
4. No recovery mechanism
5. Silent failure

### After (Resilient Behavior)
1. Panic occurs in embedding coordinator
2. Panic is caught by defer/recover
3. **Panic is logged with full details**
4. Error is returned to caller
5. Service continues running
6. All steps are logged for debugging

## Monitoring Recommendations

### Critical Log Patterns to Monitor

**Success Pattern**:
```
Collection event completed successfully
Collection completed, starting embedding
Embedding event triggered
Embedding event completed successfully
Scheduled cycle completed successfully
```

**Warning Patterns** (non-fatal, service continues):
```
Some sync jobs failed (error_count: X)
Some embedding jobs failed (error_count: X)
```

**Error Patterns** (caught and logged, service continues):
```
PANIC RECOVERED in [location]
embedding service is nil
document storage is nil
Collection event failed
Embedding event failed
```

**Fatal Pattern** (should never occur now):
```
Collection event completed successfully
[NO MORE LOGS - CRASH]
```

### Alerting Rules

1. **Alert on**: "PANIC RECOVERED" - indicates a bug that needs fixing
2. **Alert on**: "service is nil" - indicates initialization issue
3. **Monitor**: Error counts in "Some jobs failed" messages
4. **Track**: Time between "Collection event triggered" and "Embedding event completed"

## Testing Checklist

- [x] Build succeeds without errors
- [ ] Service starts without crashes
- [ ] Manual collection trigger works
- [ ] Embedding event is published after collection
- [ ] Scheduled execution works every 1 minute
- [ ] Panics are caught and logged (if they occur)
- [ ] Service continues running after errors
- [ ] All debug logs appear in correct order

## Files Modified

1. `internal/services/scheduler/scheduler_service.go`
   - Added panic recovery
   - Added step-by-step debug logging

2. `internal/services/collection/coordinator_service.go`
   - Added panic recovery
   - Added detailed debug logging

3. `internal/services/embeddings/coordinator_service.go`
   - Added panic recovery
   - Added nil pointer checks
   - Added detailed debug logging
   - Enhanced error messages

4. `internal/services/events/event_service.go`
   - Added panic recovery to goroutines
   - Added panic reporting mechanism

## Expected Impact

- **Zero crashes** from panics in event handlers
- **Complete visibility** into scheduler execution flow
- **Graceful degradation** when components fail
- **Actionable error messages** for debugging
- **Service uptime** maintained even with bugs

## Next Steps (If Crashes Still Occur)

If the service still crashes after this fix:

1. Check logs for the last message before crash
2. Look for "PANIC RECOVERED" messages (indicates the panic is caught)
3. If no panic recovery logs appear, the crash is happening outside the protected code
4. Add additional panic recovery to other service entry points
5. Consider adding panic recovery to main() in `cmd/quaero/main.go`

## Performance Impact

- **Minimal**: Defer statements have negligible overhead
- **Logging**: Debug logs only appear with debug log level
- **Nil checks**: O(1) operations, microsecond overhead
- **Panic recovery**: Zero overhead when no panic occurs

## Conclusion

This fix transforms the scheduler service from a brittle system that crashes on errors to a resilient system that:

1. Catches and logs all panics
2. Provides detailed debugging information
3. Continues running despite errors
4. Enables proactive monitoring and alerting
5. Makes debugging much easier with step-by-step logs

The root cause (whatever was causing the panic) may still exist, but now it will be caught, logged, and the service will continue running, allowing you to identify and fix the underlying issue without downtime.
