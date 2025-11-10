---
task: "Queue UI Improvements"
folder: queue-ui-improvements
complexity: medium
estimated_steps: 7
---

# Implementation Plan: Queue UI Improvements

## Problem Statement

The queue.html page needs several UI/UX improvements based on user feedback and recent parent job tracking work:

1. **Completed jobs document count:** Shows "0 Documents" when loading page even though documents were created
2. **Expand/collapse UI:** The '>' arrow and spawned children list is buggy and doesn't provide useful information
3. **Add ended datetime:** Missing 'ended' timestamp for completed/failed/cancelled jobs
4. **Link to job details:** "Show Configuration" button shows JSON instead of navigating to detail page
5. **Rename button:** "Show Configuration" should be "Job Details" for clarity
6. **Job detail page live status:** /job page doesn't show real-time status updates via WebSocket
7. **Job detail page live logs:** /job page doesn't show live logs for running jobs

## Architecture Analysis

**Current State:**
- `pages/queue.html` - Queue management page with Alpine.js components
- `pages/job.html` - Job detail page (GET /job?id={id})
- `internal/handlers/job_handler.go` - Job API endpoints
- `internal/handlers/websocket.go` - WebSocket real-time updates
- Recent work: Added `document_count` to parent jobs via WebSocket `parent_job_progress` events

**Key Files:**
- `C:\development\quaero\pages\queue.html` - Job list UI (Alpine.js)
- `C:\development\quaero\pages\job.html` - Job detail page (Alpine.js)
- `C:\development\quaero\internal\handlers\websocket.go` - WebSocket handler
- `C:\development\quaero\internal\jobs\manager.go` - Job metadata management
- `C:\development\quaero\internal\storage\sqlite\job_storage.go` - Job data persistence

**WebSocket Events:**
- `parent_job_progress` - Real-time parent job updates (includes `document_count`)
- `job_status_changed` - Job status updates
- Need to add: job-specific real-time updates for detail page

## Implementation Steps

### Step 1: Add ended_at timestamp to job storage

**Why:** Database needs to track when jobs end (completed/failed/cancelled) for display

**Depends on:** none

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\internal\models\job_model.go` (add EndedAt field)
- `C:\development\quaero\internal\storage\sqlite\job_storage.go` (update queries)
- `C:\development\quaero\internal\jobs\processor\processor.go` (set EndedAt on completion)

**Risk:** low (database column already exists as `finished_at`, just need to populate consistently)

**Implementation Details:**
- Verify `finished_at` column exists in jobs table schema
- Ensure `models.Job.FinishedAt` field is populated when job completes/fails/cancels
- Update job processor to set `FinishedAt = time.Now()` when job transitions to terminal state
- Terminal states: completed, failed, cancelled
- Field should be nullable (NULL for pending/running jobs)

**Testing:**
- Verify completed jobs have `finished_at` set
- Verify failed jobs have `finished_at` set
- Verify cancelled jobs have `finished_at` set
- Verify running jobs have `finished_at = NULL`

---

### Step 2: Persist document_count in job metadata on completion

**Why:** Completed jobs currently lose document count on page reload (only exists in WebSocket memory)

**Depends on:** none (builds on recent parent-job-document-count work)

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\internal\jobs\processor\parent_job_executor.go` (save count on completion)

**Risk:** low (already have IncrementDocumentCount, just need to ensure persistence)

**Implementation Details:**
- In `ParentJobExecutor.checkChildJobProgress()` method
- When parent job completes (all children done), ensure final document_count is saved
- Document count is already being tracked in metadata via `Manager.IncrementDocumentCount()`
- Verify count persists in `jobs.metadata_json` column
- No additional work needed if recent implementation already persists

**Testing:**
- Complete a parent job with child jobs creating documents
- Reload page (hard refresh)
- Verify document count displays correctly (not 0)
- Check database: `SELECT metadata_json FROM jobs WHERE id = ?` should contain `document_count`

---

### Step 3: Remove expand/collapse UI from queue.html

**Why:** The expand/collapse feature is buggy and the child job tree view doesn't provide useful information

**Depends on:** none

**Validation:** code_compiles, ui_displays_correctly

**Creates/Modifies:**
- `C:\development\quaero\pages\queue.html` (remove expand/collapse button and child tree)

**Risk:** low (removing unused UI, no backend changes)

**Implementation Details:**
- Remove lines 168-174: Expand/collapse button for parent jobs
- Remove lines 360-454: Child jobs list container (tree view)
- Remove Alpine.js methods related to child job expansion:
  - `toggleParentExpansion()`
  - `isNodeCollapsed()`
  - `toggleNodeCollapse()`
  - `childJobsList` Map
  - `expandedParentJobs` Set
  - Related state management
- Keep the parent job progress display (lines 280-359) - this is valuable
- Remove collapse state from local storage if persisted

**Visual Changes:**
- Parent jobs will no longer show '>' arrow
- Clicking parent job will navigate to job details (existing behavior)
- Progress text and stats remain visible inline

**Testing:**
- Verify parent jobs display without expand/collapse UI
- Verify clicking parent job navigates to detail page
- Verify no JavaScript errors in console
- Verify page loads faster (less DOM rendering)

---

### Step 4: Add ended timestamp display to queue.html

**Why:** Users need to see when jobs finished/failed/cancelled for debugging and audit purposes

**Depends on:** Step 1

**Validation:** code_compiles, ui_displays_correctly

**Creates/Modifies:**
- `C:\development\quaero\pages\queue.html` (add ended timestamp display)

**Risk:** low (UI change only)

**Implementation Details:**
- Add new metadata field after "Finished Time" (line 252-257)
- Display format: `ended: {formatted_date}` (consistent with created/started)
- Show only for jobs with status: completed, failed, cancelled
- Use `finished_at` field from job model
- Add Alpine.js helper method: `getEndedDate(job)`
- Format: `new Date(job.finished_at).toLocaleString()` or similar
- Icon: `fa-flag-checkered` for completed, `fa-times-circle` for failed/cancelled

**Code Example:**
```html
<!-- Ended Time (for terminal states) -->
<template x-if="item.job.finished_at && ['completed', 'failed', 'cancelled'].includes(item.job.status)">
    <div>
        <i class="fas fa-flag-checkered"></i>
        <span x-text="'ended: ' + getEndedDate(item.job)"></span>
    </div>
</template>
```

**Testing:**
- Verify completed jobs show "ended: {date}"
- Verify failed jobs show "ended: {date}"
- Verify cancelled jobs show "ended: {date}"
- Verify running jobs do NOT show ended timestamp
- Verify date format is human-readable

---

### Step 5: Replace "Show Configuration" with navigation to job detail page

**Why:** Users expect clicking the button to see full job details, not inline JSON

**Depends on:** none

**Validation:** code_compiles, ui_navigation_works

**Creates/Modifies:**
- `C:\development\quaero\pages\queue.html` (change button behavior and text)

**Risk:** low (UI behavior change)

**Implementation Details:**
- Change line 260-262 from toggle JSON to navigation link
- Update button text: "Show Configuration" → "Job Details"
- Update icon: `fa-code` → `fa-info-circle` or `fa-external-link-alt`
- Navigation target: `/job?id={job.id}` (existing job detail page)
- Remove `toggleJobJson()` click handler
- Remove inline JSON display section (lines 488-492)
- Remove `toggleJobJson()` Alpine.js method if no longer used elsewhere

**Code Example:**
```html
<!-- Job Details Link -->
<div>
    <a :href="'/job?id=' + item.job.id" class="text-primary" @click.stop>
        <i class="fas fa-info-circle"></i> Job Details
    </a>
</div>
```

**Visual Changes:**
- Button now navigates to dedicated job detail page
- No more inline JSON collapsible section
- Cleaner, more consistent UX with other job actions

**Testing:**
- Click "Job Details" button
- Verify navigates to `/job?id={job_id}`
- Verify detail page loads correctly
- Verify no broken links or console errors

---

### Step 6: Add real-time status updates to job detail page via WebSocket

**Why:** Job detail page should show live status/progress updates, not just polling every 2 seconds

**Depends on:** none

**Validation:** code_compiles, websocket_updates_work

**Creates/Modifies:**
- `C:\development\quaero\pages\job.html` (add WebSocket connection)
- `C:\development\quaero\internal\handlers\websocket.go` (add job-specific event handler)

**Risk:** medium (adds WebSocket complexity, but uses existing infrastructure)

**Implementation Details:**

**Frontend (pages/job.html):**
- Add WebSocket connection in `init()` method (similar to queue.html)
- Connect to `/ws` endpoint
- Subscribe to job-specific events:
  - `parent_job_progress` - For parent job status updates
  - `job_status_changed` - For job status transitions
  - `job_log_entry` - For live log streaming (Step 7)
- Update `job` object reactively when WebSocket messages received
- Handle reconnection logic (existing pattern from queue.html)
- Auto-scroll logs when new entries arrive (if logs tab active)

**Backend (internal/handlers/websocket.go):**
- Verify `parent_job_progress` event already broadcasts to all clients (it does)
- Add `job_status_changed` event if not exists:
  ```go
  Payload: map[string]interface{}{
      "job_id": string,
      "status": string,
      "progress": map[string]interface{},
      "timestamp": string,
  }
  ```
- Emit `job_status_changed` when job status changes in processor
- Consider adding job ID filtering on client-side to reduce noise

**Code Example (job.html):**
```javascript
init() {
    // ... existing code ...

    // Connect to WebSocket for real-time updates
    this.connectWebSocket();
},

connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    this.ws = new WebSocket(wsUrl);

    this.ws.onmessage = (event) => {
        const data = JSON.parse(event.data);

        // Handle parent job progress updates
        if (data.type === 'parent_job_progress' && data.job_id === this.jobId) {
            // Update job object with new progress
            this.job.status_report = data;
            this.job.progress = data;
        }

        // Handle job status changes
        if (data.type === 'job_status_changed' && data.job_id === this.jobId) {
            this.job.status = data.status;
            this.job.progress = data.progress;
        }

        // Handle live logs (Step 7)
        if (data.type === 'job_log_entry' && data.job_id === this.jobId) {
            this.appendLog(data);
        }
    };

    this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
    };

    this.ws.onclose = () => {
        // Reconnect after 2 seconds
        setTimeout(() => this.connectWebSocket(), 2000);
    };
}
```

**Testing:**
- Open job detail page for running parent job
- Verify status updates in real-time (no manual refresh)
- Verify progress stats update live
- Verify WebSocket connection persists
- Verify reconnection works if connection drops
- Test with multiple browser tabs (all should update)

---

### Step 7: Add live log streaming to job detail page

**Why:** Users viewing job details should see logs in real-time, not stale logs requiring refresh

**Depends on:** Step 6

**Validation:** code_compiles, logs_stream_live

**Creates/Modifies:**
- `C:\development\quaero\pages\job.html` (add live log appending)
- `C:\development\quaero\internal\handlers\websocket.go` (add log streaming)
- `C:\development\quaero\internal\services\logs\service.go` (emit log events)

**Risk:** medium (adds real-time streaming, need to handle high-frequency events)

**Implementation Details:**

**Event Publishing (internal/services/logs/service.go):**
- When logs are written via `LogService.SaveLog()`, emit WebSocket event
- Add `eventService interfaces.EventService` to LogService constructor
- Publish event after successful log save:
  ```go
  event := interfaces.Event{
      Type: interfaces.EventJobLogEntry,
      Payload: map[string]interface{}{
          "job_id": jobID,
          "timestamp": log.Timestamp,
          "level": log.Level,
          "message": log.Message,
      },
  }
  go eventService.Publish(ctx, event)
  ```
- Only publish for active jobs (status = running)
- Consider throttling if log rate is very high (>10 logs/sec)

**WebSocket Handler (internal/handlers/websocket.go):**
- Subscribe to `EventJobLogEntry` in WebSocket handler
- Broadcast to all connected clients
- Clients filter by job_id on frontend
- Payload structure:
  ```go
  wsPayload := map[string]interface{}{
      "type": "job_log_entry",
      "job_id": getString(payload, "job_id"),
      "timestamp": getString(payload, "timestamp"),
      "level": getString(payload, "level"),
      "message": getString(payload, "message"),
  }
  ```

**Frontend (pages/job.html):**
- Modify `loadJobLogs()` to load historical logs once on init
- Add `appendLog(data)` method to add new logs from WebSocket
- Auto-scroll to bottom when new logs arrive (if `autoScroll` enabled)
- Show live indicator when WebSocket connected and job running
- Parse log entry and add to `logs` array:
  ```javascript
  appendLog(data) {
      const logEntry = this._parseLogEntry({
          timestamp: data.timestamp,
          level: data.level,
          message: data.message
      });

      this.logs.push(logEntry);

      // Auto-scroll if enabled
      if (this.autoScroll) {
          this.$nextTick(() => {
              const container = this.$refs.logContainer;
              if (container) {
                  container.scrollTop = container.scrollHeight;
              }
          });
      }
  }
  ```
- Add visual indicator for live logs:
  ```html
  <span x-show="job.status === 'running' && wsConnected" class="label label-success">
      <i class="fas fa-circle" style="animation: pulse 2s infinite;"></i> Live Logs
  </span>
  ```

**Optimization:**
- Limit log buffer size (keep last 1000 logs in memory)
- Use virtual scrolling if log count > 500 (optional)
- Throttle log rendering if receiving logs faster than 10/sec
- Stop streaming when job completes (status !== 'running')

**Testing:**
- Start a crawler job
- Navigate to job detail page
- Verify logs appear in real-time (no manual refresh)
- Verify auto-scroll works
- Verify log filtering by level works with live logs
- Verify logs stop streaming when job completes
- Test with high-frequency logging job (100+ logs)
- Verify no memory leaks (monitor browser memory over 5+ minutes)

---

## Constraints

- Must maintain backward compatibility with existing WebSocket infrastructure
- Must not break existing queue.html functionality
- No database schema changes (use existing columns)
- WebSocket events must be efficient (low latency, no flooding)
- Must work with existing Alpine.js components
- Must follow Quaero UI/UX patterns (Bulma CSS, icons)

## Success Criteria

1. **Document Count Persistence:**
   - Completed parent jobs show correct document count after page reload
   - Document count stored in `jobs.metadata_json` column
   - No "0 Documents" displayed for completed jobs with documents

2. **Expand/Collapse Removal:**
   - No expand/collapse UI visible on parent jobs
   - Child job tree view removed
   - Page loads faster (less DOM complexity)
   - No JavaScript errors

3. **Ended Timestamp:**
   - Completed jobs show "ended: {date}" metadata
   - Failed jobs show "ended: {date}" metadata
   - Cancelled jobs show "ended: {date}" metadata
   - Running jobs do NOT show ended timestamp

4. **Job Details Navigation:**
   - "Job Details" button navigates to `/job?id={id}`
   - Inline JSON display removed
   - Button text is "Job Details" not "Show Configuration"
   - Icon is `fa-info-circle` or similar (not `fa-code`)

5. **Real-Time Status Updates:**
   - Job detail page shows live status via WebSocket
   - Progress stats update in real-time
   - No polling required (2-second interval removed)
   - Works for both parent and child jobs

6. **Live Log Streaming:**
   - Logs appear in real-time on job detail page
   - Auto-scroll to latest log (when enabled)
   - Historical logs loaded on page init
   - Live logs append without flickering
   - Logs stop streaming when job completes

## Testing Strategy

**Unit Tests:**
- Test `getEndedDate()` Alpine.js helper method
- Test log parsing and appending logic
- Test WebSocket message filtering by job_id

**Integration Tests:**
- Test document count persistence across service restart
- Test WebSocket event publishing and broadcasting
- Test log streaming with high-frequency logging

**UI Tests:**
- Create UI test for queue page improvements (Step 3, 4, 5)
- Create UI test for job detail page live updates (Step 6, 7)
- Verify document count displays correctly after reload
- Verify logs stream in real-time
- Verify ended timestamp appears for completed jobs
- Verify navigation to job details works

**Manual Testing:**
- Open queue page, verify all improvements visible
- Start a crawler job, verify real-time updates on detail page
- Complete a job, verify ended timestamp and document count persist
- Reload page, verify no data loss
- Test with multiple concurrent jobs

## Rollback Plan

If issues occur during implementation:

1. **Step 1 (ended_at):** Revert processor changes, jobs will have NULL finished_at (non-breaking)
2. **Step 2 (document_count):** Already implemented, no rollback needed
3. **Step 3 (remove expand/collapse):** Restore removed HTML sections from git history
4. **Step 4 (ended timestamp):** Remove HTML template block (non-breaking)
5. **Step 5 (job details link):** Restore inline JSON display (non-breaking)
6. **Step 6 (WebSocket status):** Remove WebSocket connection code (falls back to polling)
7. **Step 7 (live logs):** Remove log event publishing and WebSocket handling (falls back to manual refresh)

All changes are additive or removals of unused features - no breaking changes to core functionality.

## Performance Considerations

**WebSocket Connections:**
- Each detail page opens one WebSocket connection
- Multiple tabs = multiple connections (acceptable for admin tool)
- Consider connection pooling if >100 concurrent users

**Log Streaming:**
- High-frequency logs (>100/sec) may overwhelm UI
- Implement throttling: batch logs every 100ms
- Limit in-memory log buffer to 1000 entries
- Use virtual scrolling for large log counts (>500)

**Database Queries:**
- No additional queries added (use existing fields)
- Document count already cached in metadata (no query overhead)
- finished_at already exists, just needs to be set consistently

**DOM Rendering:**
- Removing child job tree reduces DOM complexity (Step 3)
- Live log appending uses `x-for` reactive rendering (efficient)
- Auto-scroll only when enabled (user control)

## Dependencies

**Modified Components:**
- Job model (`models.Job`)
- Job storage (SQLite queries)
- Job processor (ParentJobExecutor)
- WebSocket handler (event broadcasting)
- Log service (event publishing)
- Queue page UI (Alpine.js)
- Job detail page UI (Alpine.js + WebSocket)

**Initialization Order:**
1. EventService (already initialized early)
2. LogService (needs EventService for log streaming)
3. JobProcessor (sets finished_at and document_count)
4. WebSocketHandler (broadcasts events to clients)

**No changes needed to:**
- Database schema (use existing columns)
- Job queue infrastructure
- Document persistence
- Authentication

## Risk Assessment

**Low Risk:**
- Step 1: ended_at timestamp (uses existing column)
- Step 2: document_count persistence (already implemented)
- Step 3: Remove expand/collapse (removes unused UI)
- Step 4: Add ended timestamp display (UI only)
- Step 5: Job details navigation (UI only)

**Medium Risk:**
- Step 6: WebSocket status updates (adds complexity, but uses proven pattern)
- Step 7: Live log streaming (high-frequency events, needs throttling)

**Mitigation:**
- Incremental implementation (test each step independently)
- Feature flags for WebSocket features (can disable if issues)
- Throttling for high-frequency log events
- Comprehensive testing before production deployment
- Rollback plan documented above

## Timeline Estimate

- **Step 1:** 30 minutes (ended_at timestamp)
- **Step 2:** 15 minutes (verify document_count persistence)
- **Step 3:** 45 minutes (remove expand/collapse UI)
- **Step 4:** 30 minutes (add ended timestamp display)
- **Step 5:** 30 minutes (job details navigation)
- **Step 6:** 90 minutes (WebSocket status updates)
- **Step 7:** 120 minutes (live log streaming)

**Total:** ~6 hours implementation + 2 hours testing = 8 hours

## Notes

- Steps 1-5 are independent and can be done in parallel
- Steps 6-7 depend on each other (WebSocket infrastructure needed for logs)
- Document count persistence (Step 2) builds on recent parent-job-document-count work
- Live log streaming (Step 7) is the most complex feature, may need iteration
- Consider adding feature flags for Steps 6-7 to enable gradual rollout
