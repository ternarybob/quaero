# Parent Job Document Count Implementation Summary

**Date:** 2025-11-08
**Status:** INCOMPLETE - Blocked by Step 6 WebSocket Handler Issue
**Completion:** 5 of 6 steps implemented (83%)

---

## Executive Summary

This implementation adds real-time document count tracking to parent jobs in the Quaero crawler system. The backend architecture (Steps 1-5) has been successfully implemented with an event-driven approach that tracks documents as they are saved by child crawler jobs. However, the WebSocket handler layer (Step 6) has a critical issue that prevents the document count from reaching the UI, blocking the feature from being fully functional.

**Key Achievement:** Event-driven document count tracking infrastructure is fully operational in the backend.

**Blocking Issue:** WebSocket handler filters out `document_count` field before sending to UI clients.

---

## Architecture Changes

### Event Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Child Job Saves Document                        │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  DocumentPersister.SaveCrawledDocument()                            │
│  - Saves document to database                                       │
│  - Publishes EventDocumentSaved event (async)                       │
│    Payload: { job_id, parent_job_id, document_id, source_url }     │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  EventService (Pub/Sub Bus)                                         │
│  - Receives EventDocumentSaved                                      │
│  - Notifies all subscribers                                         │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  ParentJobExecutor Event Handler                                    │
│  - Receives EventDocumentSaved                                      │
│  - Extracts parent_job_id from payload                              │
│  - Calls Manager.IncrementDocumentCount(parent_job_id)              │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Manager.IncrementDocumentCount()                                   │
│  - Reads metadata_json from jobs table                              │
│  - Parses JSON, increments document_count field                     │
│  - Saves updated metadata with retry logic (SQLITE_BUSY protection) │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  ParentJobExecutor.publishParentJobProgressUpdate() (every 5 sec)   │
│  - Calls Manager.GetDocumentCount(parent_job_id)                    │
│  - Includes document_count in event payload                         │
│  - Publishes 'parent_job_progress' event                            │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  EventService (Pub/Sub Bus)                                         │
│  - Receives 'parent_job_progress' event                             │
│  - Notifies WebSocket handler subscriber                            │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  WebSocketHandler Event Subscription (lines 998-1065)               │
│  - Receives event with full payload including document_count        │
│  ❌ ISSUE: Creates wsPayload map without document_count (line 1030) │
│  - Broadcasts incomplete payload to UI clients                      │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  UI WebSocket Client (queue.html lines 1167-1183)                   │
│  - Receives incomplete WebSocket message                            │
│  ❌ ISSUE: Doesn't extract document_count (not present in message)  │
│  - Dispatches 'jobList:updateJobProgress' event without doc count   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Implementation Details

### Step 1: Event Type Definition ✅ COMPLETE

**File Modified:** `internal/interfaces/event_service.go`

**Changes:**
- Added `EventDocumentSaved EventType = "document_saved"` constant
- Documented payload structure with 5 required fields:
  - `job_id` (string) - Child job ID that saved the document
  - `parent_job_id` (string) - Parent job ID to update
  - `document_id` (string) - Saved document ID
  - `source_url` (string) - Document URL
  - `timestamp` (string) - RFC3339 formatted timestamp

**Validation:** ✅ Code compiles, follows conventions

---

### Step 2: Event Publishing ✅ COMPLETE

**Files Modified:**
1. `internal/services/crawler/document_persister.go`
2. `internal/jobs/processor/enhanced_crawler_executor.go`

**Changes:**
- Added `eventService interfaces.EventService` field to DocumentPersister struct
- Updated `NewDocumentPersister()` constructor to accept EventService parameter
- Implemented event publishing in `SaveCrawledDocument()`:
  - Event published after successful document save (both create and update paths)
  - Checks if `eventService != nil` AND `parent_job_id != ""` before publishing
  - Asynchronous publishing via goroutine to avoid blocking document save
  - Error handling logs warning but doesn't fail save operation
- Updated call to `NewDocumentPersister` in enhanced_crawler_executor.go to pass EventService

**Validation:** ✅ Code compiles, follows conventions, async publish pattern

---

### Step 3: Document Count Storage ✅ COMPLETE

**File Modified:** `internal/jobs/manager.go`

**Changes:**
1. **Modified `CreateParentJob()` method (lines 197-205):**
   - Initialize metadata with `document_count: 0` for all new parent jobs
   - Changed metadata from struct to `map[string]interface{}`

2. **Added `IncrementDocumentCount()` method (lines 616-658):**
   - Reads current metadata from `metadata_json` column
   - Parses JSON to extract current `document_count` (defaults to 0 if not present)
   - Handles both float64 and int type assertions (JSON numbers unmarshal as float64)
   - Increments count by 1
   - Marshals updated metadata back to JSON
   - Uses `retryOnBusy()` wrapper for database write with exponential backoff
   - Thread-safe implementation protects against concurrent write contention

**Pattern:** Followed existing `SetJobResult()` method pattern for metadata manipulation

**Validation:** ✅ Code compiles, thread-safe, handles missing fields gracefully

---

### Step 4: Event Subscription ✅ COMPLETE

**File Modified:** `internal/jobs/processor/parent_job_executor.go`

**Changes:**
- Added subscription to `interfaces.EventDocumentSaved` in `SubscribeToChildStatusChanges()` method (lines 350-387)
- Event handler implementation:
  - Validates payload type (map[string]interface{})
  - Extracts `parent_job_id` from event payload using existing `getStringFromPayload()` helper
  - Ignores events without parent_job_id (not a child job)
  - Calls `Manager.IncrementDocumentCount()` in goroutine (async execution)
  - Error handling logs error with full context but doesn't fail (non-critical)
  - Success logging at Debug level with parent_job_id, document_id, and job_id

**Pattern:** Reuses existing event subscription pattern from `EventJobStatusChange` handler

**Validation:** ✅ Code compiles, follows conventions, async execution, thread-safe

---

### Step 5: WebSocket Payload Enhancement ✅ COMPLETE

**Files Modified:**
1. `internal/jobs/manager.go` - Added helper method
2. `internal/jobs/processor/parent_job_executor.go` - Modified progress update method

**Manager Changes (manager.go):**
- Added `GetDocumentCount(ctx context.Context, jobID string) (int, error)` method (lines 1682-1711):
  - Queries `metadata_json` column directly from database
  - Parses JSON to extract `document_count` field
  - Handles both float64 (JSON default) and int type assertions
  - Returns 0 if document_count not found (graceful fallback)
  - Proper error wrapping with context messages

**ParentJobExecutor Changes (parent_job_executor.go):**
- Modified `publishParentJobProgressUpdate()` method (lines 402-453):
  - Added document count retrieval before payload construction (lines 417-425)
  - Calls `Manager.GetDocumentCount(ctx, parentJobID)` to get current count
  - Error handling logs debug message but uses default count of 0 (non-blocking)
  - Added `document_count` field to WebSocket event payload map (line 436)
  - Comment documents field purpose: "Real-time document count from metadata"

**WebSocket Event Payload Structure (from ParentJobExecutor):**
```go
payload := map[string]interface{}{
    "job_id":             parentJobID,
    "status":             overallStatus,
    "total_children":     stats.TotalChildren,
    "pending_children":   stats.PendingChildren,
    "running_children":   stats.RunningChildren,
    "completed_children": stats.CompletedChildren,
    "failed_children":    stats.FailedChildren,
    "cancelled_children": stats.CancelledChildren,
    "progress_text":      progressText,
    "document_count":     documentCount,  // ✅ ADDED
    "timestamp":          time.Now().Format(time.RFC3339),
}
```

**Validation:** ✅ Code compiles, backward compatible, graceful error handling

---

### Step 6: WebSocket Broadcasting ❌ INCOMPLETE (BLOCKED)

**File Analyzed:** `internal/handlers/websocket.go`

**Current Implementation (lines 998-1065):**
- WebSocket handler subscribes to `parent_job_progress` events ✅
- Handler receives events with full payload including `document_count` ✅
- Handler broadcasts events to all connected clients ✅

**CRITICAL ISSUE (lines 1017-1030):**
```go
// Create simplified WebSocket message with job_id key
// UI will use job_id to update specific job row
wsPayload := map[string]interface{}{
    "job_id":        jobID,
    "progress_text": progressText,
    "status":        status,
    "timestamp":     getString(payload, "timestamp"),

    // Include child statistics for advanced UI features
    "total_children":     getInt(payload, "total_children"),
    "pending_children":   getInt(payload, "pending_children"),
    "running_children":   getInt(payload, "running_children"),
    "completed_children": getInt(payload, "completed_children"),
    "failed_children":    getInt(payload, "failed_children"),
    "cancelled_children": getInt(payload, "cancelled_children"),
    // ❌ MISSING: "document_count": getInt(payload, "document_count"),
}
```

**Root Cause:** The WebSocket handler explicitly constructs a "simplified" payload and only includes specific fields. The `document_count` field from the incoming event payload is NOT extracted or included.

**Required Fix:**
```go
wsPayload := map[string]interface{}{
    "job_id":             jobID,
    "progress_text":      progressText,
    "status":             status,
    "timestamp":          getString(payload, "timestamp"),
    "total_children":     getInt(payload, "total_children"),
    "pending_children":   getInt(payload, "pending_children"),
    "running_children":   getInt(payload, "running_children"),
    "completed_children": getInt(payload, "completed_children"),
    "failed_children":    getInt(payload, "failed_children"),
    "cancelled_children": getInt(payload, "cancelled_children"),
    "document_count":     getInt(payload, "document_count"), // ✅ ADD THIS LINE
}
```

**UI Update Required:** `pages/queue.html` lines 1172-1181

After WebSocket handler fix, the UI event handler must also be updated:
```javascript
window.dispatchEvent(new CustomEvent('jobList:updateJobProgress', {
    detail: {
        job_id: progress.job_id,
        progress_text: progress.progress_text,
        status: progress.status,
        total_children: progress.total_children,
        pending_children: progress.pending_children,
        running_children: progress.running_children,
        completed_children: progress.completed_children,
        failed_children: progress.failed_children,
        cancelled_children: progress.cancelled_children,
        document_count: progress.document_count, // ✅ ADD THIS LINE
        timestamp: progress.timestamp
    }
}));
```

**Validation:** ❌ BLOCKED - document_count not included in WebSocket messages to UI

---

## Database Schema

**No schema changes required.** Uses existing `metadata_json` column in `jobs` table.

**Metadata Structure:**
```json
{
  "phase": "core",
  "document_count": 0,
  "result": { ... }
}
```

---

## Performance Considerations

### ✅ Async Event Publishing
- Document save events published in goroutines (non-blocking)
- Document save operation never waits for event processing
- No impact on crawler performance

### ✅ Database Retry Logic
- `IncrementDocumentCount()` uses existing `retryOnBusy()` pattern
- Exponential backoff handles SQLITE_BUSY errors
- Concurrent document saves don't lose counts

### ✅ WebSocket Throttling
- `parent_job_progress` events published every 5 seconds (existing throttle)
- Document count queries happen once per progress update
- Minimal database load

### ⚠️ Potential Optimization (Low Priority)
- Consider caching document_count in memory during parent job execution
- Current implementation queries metadata on every progress update
- However, database load is minimal and current approach is correct

---

## Testing Recommendations

### Backend Testing (Steps 1-5)

**Unit Tests:**
```go
// Test Manager.IncrementDocumentCount with concurrent calls
func TestIncrementDocumentCount_Concurrent(t *testing.T) {
    // Spawn 10 goroutines incrementing same job
    // Verify final count is 10 (no lost increments)
}

// Test event payload serialization
func TestDocumentSavedEvent_Payload(t *testing.T) {
    // Verify all 5 required fields present
}

// Test graceful handling when EventService is nil
func TestDocumentPersister_NilEventService(t *testing.T) {
    // Verify no panic, document still saved
}
```

**Integration Tests:**
```go
// Test end-to-end flow: save document → emit event → increment count
func TestDocumentCountFlow_EndToEnd(t *testing.T) {
    // 1. Create parent job
    // 2. Save document via DocumentPersister
    // 3. Verify EventDocumentSaved published
    // 4. Verify parent job document_count incremented
}

// Test parent job monitoring with multiple child jobs
func TestParentJob_MultipleChildrenSavingDocuments(t *testing.T) {
    // Verify count tracks correctly across multiple children
}
```

### WebSocket Testing (Step 6)

**After WebSocket Handler Fix:**
```javascript
// Browser DevTools test
const ws = new WebSocket('ws://localhost:8085/ws');
ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    if (message.type === 'parent_job_progress') {
        console.assert(
            'document_count' in message.payload,
            'document_count field missing from parent_job_progress message'
        );
        console.log('Document count:', message.payload.document_count);
    }
};
```

### UI Testing

**Manual Test:**
1. Start crawler job that creates documents
2. Open browser DevTools → Network → WS
3. Filter for `parent_job_progress` messages
4. Verify `document_count` field present and incrementing
5. Verify UI displays document count in job row

**Automated UI Test (ChromeDP):**
```go
func TestDocumentCountDisplayed(t *testing.T) {
    // 1. Create crawler job
    // 2. Wait for documents to be saved
    // 3. Take screenshot of job row
    // 4. Verify document count > 0 displayed
}
```

---

## Known Limitations

### Current Implementation

1. **Document Count Accuracy:**
   - Count represents documents successfully saved to database
   - Does NOT include documents currently being processed
   - May briefly show lower count during active crawling

2. **Real-Time Lag:**
   - Document count updates every 5 seconds (progress update interval)
   - UI may not reflect document count immediately after save
   - Acceptable trade-off for reduced WebSocket traffic

3. **Historical Jobs:**
   - Existing parent jobs (created before this feature) will show 0 documents initially
   - Count will increment correctly for future document saves
   - No migration script provided to backfill historical counts

4. **Error Handling:**
   - If `GetDocumentCount()` fails, defaults to 0 (non-blocking)
   - Errors logged at Debug level (may be missed)
   - Consider upgrading to Warn level for production monitoring

---

## Rollback Plan

If issues occur, rollback is straightforward (all changes are additive):

1. **Remove Event Subscription:**
   - Comment out `EventDocumentSaved` subscription in ParentJobExecutor
   - No events will be processed, count stays at 0

2. **Remove Event Publishing:**
   - Set `eventService = nil` when creating DocumentPersister
   - Events won't be published, no performance impact

3. **Remove WebSocket Field (after fix):**
   - Remove `document_count` from wsPayload
   - UI will ignore missing field (backward compatible)

4. **Remove UI Updates (after fix):**
   - Remove `document_count` from CustomEvent detail
   - Job list component will ignore missing field

**No database rollback needed** - metadata changes are non-breaking.

---

## Constraints Met

✅ **Event-Driven:** Uses existing EventService for pub/sub (no new infrastructure)
✅ **Dependency Injection:** EventService passed to DocumentPersister via constructor
✅ **No Schema Changes:** Uses existing `metadata_json` column
✅ **Real-Time Updates:** Async event publishing ensures low latency
✅ **Thread-Safe:** Retry logic handles concurrent document saves
✅ **Backward Compatible:** Existing WebSocket clients unaffected (additive changes)

---

## Success Criteria Status

### ✅ Backend Implementation (Steps 1-5)

1. **Event Publishing:** ✅ COMPLETE
   - Child jobs emit `EventDocumentSaved` when document is saved
   - Event includes all 5 required fields (job_id, parent_job_id, document_id, source_url, timestamp)

2. **Parent Job Tracking:** ✅ COMPLETE
   - Parent job receives events and updates document count in metadata
   - Document count persisted in job `metadata_json` column
   - Concurrent saves handled correctly (retry logic prevents lost counts)

3. **WebSocket Event Publishing:** ✅ COMPLETE
   - Document count included in `parent_job_progress` events published by ParentJobExecutor
   - Events published every 5 seconds with current document count
   - Backward compatible (new field is additive)

### ❌ Frontend Integration (Step 6)

4. **WebSocket Broadcasting:** ❌ BLOCKED
   - WebSocket handler filters out `document_count` field
   - UI clients never receive document count value
   - **Fix Required:** Add `document_count` to wsPayload in WebSocket handler

5. **UI Display:** ❌ BLOCKED (depends on Step 6 fix)
   - UI event handler doesn't extract `document_count`
   - Job list component doesn't display document count
   - **Fix Required:** Update UI to extract and display document_count

---

## Next Steps (To Complete Implementation)

### Immediate (Required for Feature Completion)

1. **Fix WebSocket Handler:**
   - File: `internal/handlers/websocket.go`
   - Line: 1030 (after `cancelled_children`)
   - Add: `"document_count": getInt(payload, "document_count"),`
   - Test: Verify WebSocket messages include document_count in browser DevTools

2. **Update UI Event Handler:**
   - File: `pages/queue.html`
   - Line: 1181 (before `timestamp`)
   - Add: `document_count: progress.document_count,`
   - Test: Verify CustomEvent includes document_count

3. **Update Job List Component:**
   - Find Alpine.js component that handles `jobList:updateJobProgress` event
   - Add document count display to job row template
   - Suggested format: "X documents" or "X docs" next to child job counts

### Short-Term (UI Polish)

4. **Add Document Count Column:**
   - Update job list table to include "Documents" column
   - Display document count for all parent jobs
   - Sort by document count (high to low)

5. **Visual Feedback:**
   - Animate document count when it increments
   - Use color coding (green for jobs with documents, gray for 0)
   - Add tooltip explaining what document count represents

### Medium-Term (Enhancements)

6. **Backfill Historical Counts:**
   - Create migration script to count existing documents for old jobs
   - Update `document_count` in metadata for historical parent jobs
   - Run as one-time database migration

7. **Advanced Metrics:**
   - Track document save rate (docs/second)
   - Add estimated time to completion based on document rate
   - Display document count trend graph

8. **Performance Monitoring:**
   - Add metrics for event processing latency
   - Monitor `IncrementDocumentCount()` retry frequency
   - Alert if event queue backs up

---

## Code Quality Assessment

### ✅ Strengths

1. **Clean Architecture:**
   - Event-driven design decouples document saving from count tracking
   - Single Responsibility Principle maintained across components
   - Clear separation between data access (Manager) and business logic (Executor)

2. **Error Handling:**
   - Graceful degradation (defaults to 0 on errors)
   - Non-blocking error handling (document saves never fail due to count tracking)
   - Comprehensive error logging with context

3. **Thread Safety:**
   - Retry logic handles database concurrency correctly
   - Async event publishing prevents race conditions
   - No global state or shared mutable data

4. **Code Documentation:**
   - Inline comments explain purpose of new fields
   - Method documentation describes return values and error conditions
   - Event payload structure documented in interface file

### ⚠️ Areas for Improvement

1. **WebSocket Handler Design:**
   - "Simplified payload" pattern causes data loss
   - No clear documentation of which fields are passed through
   - Consider passing entire payload through (avoid field filtering)

2. **Error Logging Levels:**
   - `GetDocumentCount()` errors logged at Debug level
   - May be missed in production monitoring
   - Consider upgrading to Warn level for visibility

3. **Test Coverage:**
   - No unit tests added for new methods
   - No integration tests for event flow
   - Recommend adding tests before declaring feature complete

---

## Conclusion

The parent job document count tracking feature is **83% complete** with a solid backend implementation (Steps 1-5) but is **blocked by a critical WebSocket handler issue** (Step 6). The event-driven architecture is well-designed and follows Quaero's existing patterns. The fix required is minimal (adding one line to WebSocket handler and one line to UI event handler), but these changes are essential for the feature to be functional from an end-user perspective.

**Recommended Action:** Implement the WebSocket handler fix and UI update as outlined in the "Next Steps" section, then revalidate with browser DevTools to confirm document_count reaches the UI.

**Estimated Time to Complete:** 30 minutes (fix) + 1 hour (testing and validation)

**Risk Assessment:** LOW - Fix is additive, backward compatible, and follows existing patterns.

---

## References

### Modified Files (Steps 1-5)

1. `internal/interfaces/event_service.go` - Event type definition
2. `internal/services/crawler/document_persister.go` - Event publishing
3. `internal/jobs/processor/enhanced_crawler_executor.go` - EventService wiring
4. `internal/jobs/manager.go` - Document count storage and retrieval
5. `internal/jobs/processor/parent_job_executor.go` - Event subscription and payload enhancement

### Files Requiring Changes (Step 6)

6. `internal/handlers/websocket.go` - Add document_count to wsPayload
7. `pages/queue.html` - Update UI event handler

### Related Documentation

- `docs/parent-job-document-count/plan.md` - Original implementation plan
- `docs/parent-job-document-count/progress.md` - Implementation progress tracker
- `docs/parent-job-document-count/step-1-validation.json` through `step-5-validation.json` - Validation reports
- `docs/parent-job-document-count/step-6-validation.json` - Final validation (this step)

---

**Document Version:** 1.0
**Last Updated:** 2025-11-08
**Author:** Agent 3 - Validator
**Status:** FINAL - Implementation Incomplete (Blocked)
