# Step 3: Update JobMonitor to track document updates

**Skill:** @go-coder
**Files:** `internal/jobs/monitor/job_monitor.go`

---

## Iteration 1

### Agent 2 - Implementation

Adding subscription to `EventDocumentUpdated` in JobMonitor to track document updates (not just creation).

**Analysis:**
- JobMonitor already subscribes to `EventDocumentSaved` (lines 371-410)
- Both events call `IncrementDocumentCount()` on the parent job
- Need to add similar handler for `EventDocumentUpdated`
- Keep same logic: increment document_count in parent job metadata

**Changes to make:**
Add subscription to EventDocumentUpdated after EventDocumentSaved subscription

**Starting implementation...**

### Changes Made

#### Added EventDocumentUpdated subscription to JobMonitor

**File:** `internal/jobs/monitor/job_monitor.go`
**Lines:** 412-452 (added after EventDocumentSaved subscription)

**Implementation:**
```go
// Subscribe to document_updated events for real-time document update count tracking
if err := m.eventService.Subscribe(interfaces.EventDocumentUpdated, func(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        m.logger.Warn().Msg("Invalid document_updated payload type")
        return nil
    }

    // Extract parent job ID from payload
    parentJobID := getStringFromPayload(payload, "parent_job_id")
    if parentJobID == "" {
        return nil // No parent job, ignore
    }

    // Extract additional fields for logging
    documentID := getStringFromPayload(payload, "document_id")
    jobID := getStringFromPayload(payload, "job_id")

    // Increment document count in parent job metadata (async operation)
    go func() {
        if err := m.jobMgr.IncrementDocumentCount(context.Background(), parentJobID); err != nil {
            m.logger.Error().Err(err).
                Str("parent_job_id", parentJobID).
                Str("document_id", documentID).
                Str("job_id", jobID).
                Msg("Failed to increment document count for parent job")
            return
        }

        m.logger.Debug().
            Str("parent_job_id", parentJobID).
            Str("document_id", documentID).
            Str("job_id", jobID).
            Msg("Incremented document count for parent job (document updated)")
    }()

    return nil
}); err != nil {
    m.logger.Error().Err(err).Msg("Failed to subscribe to EventDocumentUpdated")
    return
}
```

**Also updated log message (line 454):**
```go
m.logger.Info().Msg("JobMonitor subscribed to child job status changes, document_saved events, and document_updated events")
```

**Rationale:**
- Mirrors the existing EventDocumentSaved handler logic
- Uses same `IncrementDocumentCount()` method - both creation and updates count as progress
- Async goroutine prevents blocking event processing
- Detailed logging for debugging (includes document_id and job_id)
- Separate log message for updates: "document updated" vs generic increment

### Compilation Test

**Commands:**
```bash
go build -o /tmp/quaero_monitor ./internal/jobs/monitor
go build -o /tmp/quaero_full2 ./cmd/quaero
```

**Result:** ✅ Compiles cleanly - no errors

### Agent 3 - Validation

**Skill:** @go-coder

Validating Step 3 implementation...

**Compilation:**
✅ Compiles cleanly - no errors in job_monitor.go

**Code Quality:**

✅ **Follows existing patterns:**
- Identical structure to EventDocumentSaved handler (lines 371-410)
- Uses same helper function `getStringFromPayload()`
- Same async goroutine pattern for IncrementDocumentCount()
- Consistent error handling and logging

✅ **Correct event subscription:**
- Subscribes to `interfaces.EventDocumentUpdated`
- Validates payload type before processing
- Checks for parent_job_id (agent jobs are child jobs)
- Gracefully handles missing fields

✅ **Proper document count tracking:**
- Calls `IncrementDocumentCount()` for both creation AND updates
- This is correct: UI should show total document operations (created + updated)
- Async execution prevents blocking
- Error logging helps debug count issues

✅ **Logging improvements:**
- Updated subscription log to mention all three event types
- Debug log distinguishes "document updated" from "document saved"
- Includes all relevant IDs for troubleshooting

✅ **No breaking changes:**
- Existing EventDocumentSaved handler unchanged
- Added new handler alongside existing ones
- No changes to public API

**Issues Found:** None

**Quality Score:** 10/10

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
- JobMonitor now tracks both document creation and updates
- Parent jobs will show accurate document count for agent jobs
- Ready for end-to-end testing in Step 4

**→ Continuing to Step 4**
