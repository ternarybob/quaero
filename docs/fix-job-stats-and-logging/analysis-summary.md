# Job Statistics and Logging Issues - Analysis Summary

## Executive Summary

Three critical issues identified affecting job monitoring and statistics:

1. **Document Double Counting**: Jobs display incorrect document counts due to conflicting update paths
2. **Missing Job Logs**: Logs do not display for active parent jobs in the UI
3. **Count Persistence Issue**: Document counts reset after page refresh

## Problem Details

### Issue 1: Document Double Counting

**Symptom:** Job shows "0 Documents" during execution, then "34 Documents" after completion

**Root Cause:**
- Two competing mechanisms for setting document count:
  1. Event-based: `EventDocumentSaved` → increments `metadata["document_count"]` ✅ CORRECT
  2. Progress-based: `MarkCompleted()` → sets `result_count = progress.completed_urls` ❌ INCORRECT

**Evidence:**
```go
// File: internal/models/job_model.go, line 302-311
func (j *Job) MarkCompleted() {
    j.Status = JobStatusCompleted
    now := time.Now()
    j.CompletedAt = &now
    if j.Progress != nil {
        j.ResultCount = j.Progress.CompletedURLs  // ❌ OVERWRITES accurate metadata count
        j.FailedCount = j.Progress.FailedURLs
    }
}
```

**Impact:**
- Inaccurate document counts displayed in UI
- Confusion about actual documents created
- Data integrity issues for reporting

---

### Issue 2: Missing Job Logs

**Symptom:** Job Logs section is empty for active parent jobs

**Root Cause:**
- UI calls `/api/jobs/{id}/logs` which returns logs for SINGLE job only
- Parent jobs orchestrate children, their logs are minimal
- Need to use `/api/jobs/{id}/logs/aggregated?include_children=true` for parent jobs
- UI does not distinguish between parent and child jobs when fetching logs

**Evidence:**
```javascript
// File: pages/job.html, line 466-498
async loadJobLogs() {
    // Always uses single job endpoint - NO parent/child distinction
    const qs = this.selectedLogLevel === 'all' ? '' : ('?level=' + encodeURIComponent(this.selectedLogLevel));
    const response = await fetch(`/api/jobs/${this.jobId}/logs${qs}`);
    // ...
}
```

**API Endpoints Available:**
- `/api/jobs/{id}/logs` - Single job logs (for child crawler jobs)
- `/api/jobs/{id}/logs/aggregated` - Parent + all children logs (for parent orchestrator jobs)

**Impact:**
- Users cannot monitor job progress via logs
- Debugging active jobs is impossible
- Critical errors may go unnoticed

---

### Issue 3: Document Count Not Persisted

**Symptom:** Count shows 0 after page refresh, then updates to correct value

**Root Cause:**
- `IncrementDocumentCount()` updates metadata in-memory
- May not persist to database immediately
- Page refresh loads stale data from DB
- Real-time updates then correct the value via API polling

**Evidence:**
```go
// File: internal/jobs/processor/parent_job_executor.go, line 382-397
go func() {
    if err := e.jobMgr.IncrementDocumentCount(context.Background(), parentJobID); err != nil {
        // Increments metadata - but does this persist to DB?
        e.logger.Error().Err(err).Msg("Failed to increment document count")
        return
    }
}()
```

**Impact:**
- Inconsistent UI behavior on page refresh
- Loss of confidence in displayed statistics
- Potential data loss if job metadata not persisted

---

## Architecture Analysis

### Event-Driven Document Counting Flow

```
Child Crawler Job
    ↓
DocumentPersister.SaveCrawledDocument()  // Saves to documents table
    ↓
Publishes EventDocumentSaved {
    job_id: child_job_id,
    parent_job_id: parent_job_id,  ✅ Critical field
    document_id: doc_id
}
    ↓
ParentJobExecutor subscribes to EventDocumentSaved
    ↓
Calls jobMgr.IncrementDocumentCount(parent_job_id)
    ↓
Updates job.metadata["document_count"] += 1
    ↓
❓ Does this persist to database immediately?
```

### Log Storage and Retrieval Flow

```
Service Layer
    ↓
jobMgr.AddJobLog(job_id, level, message)
    ↓
LogService.AppendLog() → job_logs table
    ↓
SELECT * FROM job_logs WHERE job_id = ? ORDER BY created_at DESC
    ↓
Returns logs for SINGLE job only
```

**Problem:** Parent jobs have minimal logs. Need aggregation:

```
Aggregated Logs Query (working correctly in API):
    ↓
SELECT * FROM job_logs
WHERE job_id = parent_id OR job_id IN (
    SELECT id FROM jobs WHERE parent_id = parent_id
)
ORDER BY created_at ASC
```

---

## Solution Strategy

### Phase 1: Fix Document Counting (Steps 1-2)
**Goal:** Single source of truth for document counts
**Actions:**
1. Remove `ResultCount` assignment from `MarkCompleted()`
2. Ensure `IncrementDocumentCount()` persists to database
3. Extract `document_count` from metadata in API responses

### Phase 2: Fix Log Display (Step 3)
**Goal:** Show comprehensive logs for parent jobs
**Actions:**
1. Detect parent job type in UI
2. Use aggregated logs endpoint for parent jobs
3. Maintain single job logs for child jobs

### Phase 3: Ensure Persistence (Step 4)
**Goal:** Document count survives page refresh
**Actions:**
1. Verify database persistence in `IncrementDocumentCount()`
2. Add transaction handling if needed
3. Add logging for metadata updates

### Phase 4: Real-Time Enhancements (Step 5 - Optional)
**Goal:** Live log updates without polling
**Actions:**
1. Subscribe to WebSocket events for log updates
2. Reduce API polling frequency
3. Improve user experience with instant feedback

---

## Key Files Requiring Changes

### Backend Changes:
```
internal/models/job_model.go
  └─ MarkCompleted() - Remove ResultCount assignment

internal/jobs/manager.go
  └─ IncrementDocumentCount() - Verify DB persistence

internal/handlers/job_handler.go
  └─ convertJobToMap() - Extract document_count from metadata
```

### Frontend Changes:
```
pages/job.html
  └─ loadJobLogs() - Detect parent job and use aggregated endpoint
  └─ Optional: Add WebSocket subscription for real-time updates
```

---

## Risk Assessment

**Low Risk:**
- Step 1: Removing progress-based count assignment (isolated change)
- Step 3: UI log loading logic (client-side only)
- Step 4: API response enrichment (additive change)

**Medium Risk:**
- Step 2: Database persistence verification (requires transaction handling)
- Step 5: WebSocket integration (new real-time dependency)

**Mitigation:**
- Thorough testing with running jobs
- Database transaction isolation for metadata updates
- Graceful degradation if WebSocket unavailable
- Backward compatibility for existing jobs

---

## Testing Requirements

### Unit Tests:
- [ ] Test `MarkCompleted()` does not modify `ResultCount`
- [ ] Test `IncrementDocumentCount()` persists to database
- [ ] Test metadata JSON serialization/deserialization

### Integration Tests:
- [ ] Test document count increments during job execution
- [ ] Test aggregated logs endpoint returns parent + child logs
- [ ] Test log level filtering works correctly

### End-to-End Tests:
- [ ] Run parent job and verify real-time document count
- [ ] Refresh page and verify count persists
- [ ] Verify logs display for active parent jobs
- [ ] Verify logs update in real-time (if WebSocket implemented)

---

## Success Metrics

1. **Document Count Accuracy**:
   - Count matches actual documents in database
   - No discrepancy between running and completed states

2. **Log Availability**:
   - Logs visible for 100% of parent jobs
   - Aggregated logs include all child job messages

3. **Data Persistence**:
   - Document count survives page refresh with 100% accuracy
   - No race conditions in metadata updates

4. **User Experience**:
   - Real-time updates appear within 2 seconds
   - No manual refresh required for active jobs

---

## Recommended Execution Order

1. ✅ **Step 1** (1-2 hours): Remove progress-based count assignment
   - Quick win, immediate impact
   - Low risk, isolated change

2. ✅ **Step 2** (2-3 hours): Verify database persistence
   - Critical for data integrity
   - May require transaction handling

3. ✅ **Step 3** (1-2 hours): Fix log display for parent jobs
   - High user value
   - Client-side only, low risk

4. ✅ **Step 4** (1 hour): Ensure API includes document_count
   - Simple additive change
   - Complements Steps 1-2

5. 🔄 **Step 5** (2-3 hours, optional): WebSocket log updates
   - Enhancement, not critical
   - Can be deferred to later sprint

**Total Estimated Effort:** 7-11 hours (excluding optional Step 5)

---

## Appendix: Code References

### Document Counting Events

**Event Definition:**
```go
// File: internal/interfaces/event_service.go, line 176-184
EventDocumentSaved EventType = "document_saved"
// Payload:
// - job_id: child job ID
// - parent_job_id: parent job ID to update
// - document_id: saved document ID
// - source_url: document URL
// - timestamp: RFC3339 timestamp
```

**Event Publisher:**
```go
// File: internal/services/crawler/document_persister.go, line 79-108
if dp.eventService != nil && crawledDoc.ParentJobID != "" {
    payload := map[string]interface{}{
        "job_id":        crawledDoc.JobID,
        "parent_job_id": crawledDoc.ParentJobID,
        "document_id":   doc.ID,
        "source_url":    crawledDoc.SourceURL,
        "timestamp":     time.Now().Format(time.RFC3339),
    }
    event := interfaces.Event{
        Type:    interfaces.EventDocumentSaved,
        Payload: payload,
    }
    go func() {
        dp.eventService.Publish(context.Background(), event)
    }()
}
```

**Event Subscriber:**
```go
// File: internal/jobs/processor/parent_job_executor.go, line 363-400
e.eventService.Subscribe(interfaces.EventDocumentSaved, func(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    parentJobID := getStringFromPayload(payload, "parent_job_id")

    go func() {
        if err := e.jobMgr.IncrementDocumentCount(context.Background(), parentJobID); err != nil {
            e.logger.Error().Err(err).Msg("Failed to increment document count")
            return
        }
    }()

    return nil
})
```

### Log Aggregation Endpoint

**API Handler:**
```go
// File: internal/handlers/job_handler.go, line 522-659
func (h *JobHandler) GetAggregatedJobLogsHandler(w http.ResponseWriter, r *http.Request) {
    // Supports:
    // - level: error, warn, info, debug, all
    // - limit: max logs to return
    // - include_children: true/false
    // - order: asc (oldest-first) or desc (newest-first)
    // - cursor: pagination support

    logEntries, metadata, nextCursor, err := h.logService.GetAggregatedLogs(
        ctx, jobID, includeChildren, level, limit, cursor, order
    )
}
```

---

## Questions for Clarification

1. Should document_count include failed documents, or only successful saves?
   - **Current behavior**: Only successful saves (EventDocumentSaved published after successful SaveDocument)

2. Should logs display in chronological order (oldest-first) or reverse (newest-first)?
   - **Current behavior**: Database returns DESC (newest-first), handler can reverse via order parameter
   - **Recommendation**: Oldest-first for running jobs, newest-first for completed jobs

3. Should WebSocket updates be mandatory or optional enhancement?
   - **Recommendation**: Optional (Step 5) - polling works adequately, WebSocket is UX enhancement

4. How should we handle jobs created before this fix?
   - **Recommendation**: Graceful degradation - use ResultCount if metadata["document_count"] missing
