# Complete: Fix Document Count Display + Add Keyword Extraction Test

## Execution Stats
| Metric | Value |
|--------|-------|
| Total Steps | 3 |
| Duration | ~5 min |

## Changes Made

### 1. Fixed agent_worker.go ([internal/jobs/worker/agent_worker.go:226-244](internal/jobs/worker/agent_worker.go#L226-L244))
**Problem:** Document count showed "0 Documents" for agent jobs because events were published asynchronously
**Fix:** Changed from async `Publish()` in goroutine to synchronous `PublishSync()`

```go
// Before (async - document count not updated before job completes)
go func() {
    if err := w.eventService.Publish(context.Background(), event); err != nil {
        jobLogger.Warn().Err(err).Msg("Failed to publish DocumentUpdated event")
    }
}()

// After (sync - document count updated before job completes)
if err := w.eventService.PublishSync(ctx, event); err != nil {
    jobLogger.Warn().Err(err).Msg("Failed to publish DocumentUpdated event")
}
```

### 2. Refactored queue_test.go ([test/ui/queue_test.go](test/ui/queue_test.go))
**Structure:**
- Created `queueTestContext` struct to hold shared state
- Extracted helper methods: `triggerJob()`, `monitorJob()`, `runPlacesJob()`, `runKeywordExtractionJob()`
- No code duplication between tests

**Tests:**
- `TestQueue` - Runs only the Places job (5 min timeout)
- `TestQueueWithKeywordExtraction` - Runs Places + Keyword Extraction in sequence (10 min timeout)

```go
// TestQueue - Places job only
func TestQueue(t *testing.T) {
    qtc, cleanup := newQueueTestContext(t, 5*time.Minute)
    defer cleanup()
    qtc.runPlacesJob()
}

// TestQueueWithKeywordExtraction - Both jobs in sequence
func TestQueueWithKeywordExtraction(t *testing.T) {
    qtc, cleanup := newQueueTestContext(t, 10*time.Minute)
    defer cleanup()
    qtc.runPlacesJob()           // Creates documents
    qtc.runKeywordExtractionJob() // Processes documents, validates count
}
```

## Root Cause
The job monitor subscribes to `EventDocumentUpdated` events and increments the document count. But:
- **Places jobs** use `PublishSync()` → count incremented before job completes → UI shows correct count
- **Agent jobs** used `Publish()` in goroutine → job completes first → UI shows 0 documents

## Verification
```bash
# Build passes
go build ./...  # ✅ Pass
```

## Files Modified
- `internal/jobs/worker/agent_worker.go` - Sync event publishing
- `test/ui/queue_test.go` - Refactored with new test

## Expected Result After Fix
The "Keyword Extraction" job will now display the correct document count (e.g., "20 Documents") instead of "0 Documents", matching the number of documents processed from the Places job.
