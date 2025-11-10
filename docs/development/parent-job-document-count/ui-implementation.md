# UI Implementation - Parent Job Document Count Display

**Date:** 2025-11-08
**Status:** ✅ COMPLETE

---

## Summary

Updated the Queue Management UI to display real-time document counts for parent crawler jobs. The document count now updates live via WebSocket as child jobs save documents.

## Files Modified

**File:** `pages/queue.html`

**Lines Changed:** 2 edits

---

## Implementation Details

### Change 1: Update `updateJobProgress()` Method

**Location:** Line 3136-3139

**Purpose:** Store the `document_count` field from WebSocket updates

**Code Added:**
```javascript
// Update document count for parent jobs (real-time count from metadata)
if (progress.document_count !== undefined) {
    job.document_count = progress.document_count;
}
```

**What It Does:**
- Receives `document_count` from WebSocket `parent_job_progress` events
- Stores the count in the job object for UI display
- Updates in real-time as child jobs save documents

---

### Change 2: Update `getDocumentsCount()` Method

**Location:** Line 2410-2413

**Purpose:** Display document count from metadata for parent jobs

**Code Added:**
```javascript
// For parent jobs, use document_count from metadata (real-time count via WebSocket)
if (job.child_count > 0 && job.document_count !== undefined && job.document_count !== null) {
    return job.document_count;
}
```

**What It Does:**
- Checks if job is a parent job (`child_count > 0`)
- Returns the `document_count` from job metadata (updated via WebSocket)
- Falls through to existing logic for child jobs (uses `result_count` or `progress.completed_urls`)

**Fallback Logic:**
1. **Parent jobs:** Use `document_count` from metadata (real-time WebSocket updates)
2. **Completed jobs:** Use `result_count` (authoritative snapshot)
3. **Active jobs:** Use `progress.completed_urls` (real-time counter)
4. **Fallback:** Use `result_count` if available
5. **Final fallback:** Display "N/A"

---

## User Interface Display

### Where Document Count Appears

**Location:** Job card metadata section (line 234-238)

**Display Format:**
```
📄 {count} Documents
```

**Example:**
```
📄 5 Documents
```

**Real-Time Behavior:**
- Count starts at 0 when parent job created
- Increments immediately when child jobs save documents
- Updates without page refresh (WebSocket-driven)
- Final count persists after job completion

---

## Data Flow

```
Child Job saves document
  ↓
DocumentPersister publishes EventDocumentSaved
  ↓
ParentJobExecutor increments count in database
  ↓
publishParentJobProgressUpdate() includes document_count in payload
  ↓
WebSocket handler broadcasts parent_job_progress event
  ↓
UI receives event via WebSocket
  ↓
updateJobProgress() stores document_count in job object
  ↓
getDocumentsCount() returns document_count for display
  ↓
UI shows "📄 {count} Documents" in job card
```

---

## Testing

### Manual Testing Steps

1. **Start Quaero server:**
   ```powershell
   .\scripts\build.ps1 -Run
   ```

2. **Open Queue Management page:**
   ```
   http://localhost:8085/queue
   ```

3. **Start News Crawler job:**
   - Click "New Job" button
   - Select "News Crawler" from job definitions
   - Click "Start Job"

4. **Observe document count:**
   - Initial count: "📄 0 Documents"
   - As child jobs save documents, count increments in real-time
   - Final count shows total documents saved

5. **Verify WebSocket updates (Optional):**
   - Open browser DevTools (F12)
   - Go to Network tab → WS (WebSocket filter)
   - Click WebSocket connection
   - Look for `parent_job_progress` messages
   - Verify `document_count` field is present and updating

---

## Edge Cases Handled

1. **Parent job with no children:** Falls through to existing logic (uses `result_count`)
2. **`document_count` is null/undefined:** Falls through to existing logic
3. **WebSocket connection lost:** Displays last known count (no real-time updates until reconnection)
4. **Child jobs without documents:** Count remains 0 (valid state)
5. **Historical parent jobs:** Count shows 0 until documents are saved (awaiting backfill migration)

---

## Backward Compatibility

✅ **Fully backward compatible**

- Child jobs continue using existing logic (`result_count` or `progress.completed_urls`)
- Jobs without `document_count` field fall through to existing behavior
- No breaking changes to existing UI components
- Graceful degradation if WebSocket updates fail

---

## Future Enhancements

### High Priority

1. **Add "Documents" column to job list table**
   - Currently only shows in job card metadata
   - Table column would provide better visibility

2. **Add document count to job details page**
   - Show in job details header/summary
   - Include in job statistics

### Medium Priority

3. **Add document count filter**
   - Filter jobs by document count range
   - Show only jobs with documents

4. **Add document count sorting**
   - Sort job list by document count
   - Ascending/descending order

### Low Priority

5. **Add document count trend chart**
   - Visualize document count over time
   - Show accumulation rate

6. **Add document count alerts**
   - Notify when count exceeds threshold
   - Highlight jobs with low/high document counts

---

## Performance Considerations

### UI Rendering

- **Minimal overhead:** Document count displayed in existing metadata section
- **No additional API calls:** Data arrives via existing WebSocket connection
- **Reactive updates:** Alpine.js handles DOM updates efficiently

### WebSocket Traffic

- **Incremental updates:** Only changed fields sent
- **Efficient payload:** `document_count` is single integer field
- **No throttling needed:** Updates only when documents saved (not high-frequency)

---

## Known Limitations

1. **No historical backfill:** Existing parent jobs show `document_count: 0` until new documents are saved
2. **No manual refresh:** Document count only updates via WebSocket (no manual reload button)
3. **No breakdown by child:** Shows total count only (not per-child breakdown)

---

## Rollback Plan

If issues arise, revert the two edits in `pages/queue.html`:

1. Remove lines 3136-3139 from `updateJobProgress()` method
2. Remove lines 2410-2413 from `getDocumentsCount()` method
3. Deploy updated file: `.\scripts\build.ps1 -Deploy`

**Risk:** VERY LOW - Changes are minimal and follow existing patterns

---

## Conclusion

The UI implementation is complete and functional. Parent job document counts now display in real-time in the Queue Management interface, providing users with immediate visibility into document collection progress.

**Next Steps:**
- Monitor user feedback
- Consider adding "Documents" column to job table
- Implement historical backfill for existing parent jobs

---

**Implementation:** Claude Sonnet
**Testing:** Manual (automated UI test recommended)
**Build Status:** ✅ SUCCESS
**Deploy Status:** ✅ READY
