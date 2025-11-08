---
task: "Fix job document count double-counting and job logs not displaying"
folder: fix-job-stats-and-logging
complexity: low
estimated_steps: 3
---

# Implementation Plan

## Analysis

### Issue 1: Document Count Double-Counting

**Root Cause:**
The job queue UI is displaying the `completed_children` count (17 child jobs) as "Documents" instead of the actual `document_count` from metadata, or displaying both values causing them to sum to 34.

**Evidence:**
- Job queue shows "34 Documents" but Document Management shows "17" (exactly double)
- Job details page shows "Documents Created: 0" (incorrect - should show document_count from metadata)
- Parent job has 17 completed child jobs (URL processing tasks)
- Parent job metadata has `document_count: 17` (correctly tracked via EventDocumentSaved)

**Code Flow Analysis:**

1. **Document Save Event** (`internal/services/crawler/document_persister.go:79-108`):
   - Each crawled document publishes `EventDocumentSaved` with `parent_job_id`
   - Event handled by `ParentJobExecutor.SubscribeToChildStatusChanges()` (line 363-401)
   - Calls `jobMgr.IncrementDocumentCount(parent_job_id)` which correctly increments metadata

2. **Job Handler Response** (`internal/handlers/job_handler.go:1180-1193`):
   - `convertJobToMap()` correctly extracts `document_count` from metadata into `jobMap["document_count"]`
   - Job also has `completed_children` from child stats (line 187)

3. **Root Cause**:
   - The UI is likely displaying BOTH `completed_children` AND `document_count` or confusing the two
   - `completed_children` = number of child jobs completed = 17
   - `document_count` from metadata = documents saved = 17
   - UI shows 34 because it's adding or displaying both as "Documents"

### Issue 2: Job Logs Not Displaying

**Root Cause:**
The job logs API returns successfully but the frontend JavaScript fails to parse or display the response.

**Evidence:**
- Frontend error: "Failed to load logs: Failed to fetch job logs"
- API endpoint exists: `/api/jobs/{id}/logs` and `/api/jobs/{id}/logs/aggregated`
- Handlers correctly extract job ID from path at `pathParts[2]`

**Code Analysis:**

1. **Frontend Request** (`pages/job.html:466-507`):
   ```javascript
   async loadJobLogs() {
       const isParentJob = !this.job.parent_id || this.job.parent_id === '';
       const endpoint = isParentJob
           ? `/api/jobs/${this.jobId}/logs/aggregated`
           : `/api/jobs/${this.jobId}/logs`;

       const qs = this.selectedLogLevel === 'all' ? '' : ('?level=' + encodeURIComponent(this.selectedLogLevel));
       const response = await fetch(`${endpoint}${qs}`);
       if (!response.ok) {
           throw new Error('Failed to fetch job logs');
       }

       const data = await response.json();
       const rawLogs = data.logs || [];
       this.logs = rawLogs.map(log => this._parseLogEntry(log));
   }
   ```

2. **Backend Handlers** (`internal/handlers/job_handler.go`):
   - Line 412-520: `GetJobLogsHandler` - handles `/api/jobs/{id}/logs`
   - Line 524-659: `GetAggregatedJobLogsHandler` - handles `/api/jobs/{id}/logs/aggregated`
   - Both return: `{ "job_id": "...", "logs": [], "count": N, "order": "desc", "level": "all" }`

3. **Possible Issues**:
   - Job exists but has NO logs yet (returns empty array at line 500)
   - LogService fails to retrieve logs (error at line 483-485)
   - Response format mismatch between backend and frontend parsing

**Most Likely Cause**:
- For completed jobs, logs might not be stored in the `job_logs` table
- Or logs were cleared/deleted after job completion
- Frontend tries to fetch logs for a job that has no log entries

## Step 1: Fix Document Count Display in Job Queue

**Why:** The job queue is incorrectly displaying child job count as document count, or adding both values together resulting in double-counting.

**Depends on:** none

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\pages\queue.html` (job queue UI template)

**Risk:** low

**Implementation:**
1. Search for where "Documents" is displayed in the queue page
2. Verify the data binding is using `job.document_count` from metadata
3. Ensure it's NOT using `job.completed_children` as the document count
4. Check if Alpine.js or JavaScript is summing multiple fields
5. Update template to display ONLY `document_count` from metadata:
   ```javascript
   job.document_count || job.metadata?.document_count || 0
   ```

**Expected Outcome:**
- Job queue shows "17 Documents" (matching actual document count)
- No double-counting of completed children as documents

## Step 2: Fix Job Details "Documents Created" Display

**Why:** Job details page shows "Documents Created: 0" instead of the actual document count from metadata.

**Depends on:** Step 1

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\pages\job.html` (line 96-98)

**Risk:** low

**Implementation:**
1. Locate line 97 in `pages/job.html`:
   ```html
   <p class="text-small" x-text="job.result_count || '0'"></p>
   ```
2. Change to prioritize `document_count` from metadata:
   ```html
   <p class="text-small" x-text="job.document_count || job.metadata?.document_count || job.result_count || '0'"></p>
   ```
3. This ensures parent jobs show their cumulative document count from metadata
4. The backend already extracts this correctly in `convertJobToMap()` (lines 1182-1193)

**Expected Outcome:**
- Job details page shows "Documents Created: 17" (matching actual count)
- Parent jobs display correct cumulative document count

## Step 3: Investigate and Fix Job Logs Display Issue

**Why:** Job logs fail to load with a fetch error, preventing users from viewing job execution details.

**Depends on:** none

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `C:\development\quaero\internal\server\routes.go` (verify route registration)
- `C:\development\quaero\pages\job.html` (error handling in loadJobLogs)

**Risk:** low

**Implementation:**
1. Verify route registration in `internal/server/routes.go`:
   - Ensure `/api/jobs/{id}/logs` maps to `jobHandler.GetJobLogsHandler`
   - Ensure `/api/jobs/{id}/logs/aggregated` maps to `jobHandler.GetAggregatedJobLogsHandler`

2. Test API endpoints manually:
   ```powershell
   # Get a real job ID from the queue
   curl http://localhost:8085/api/jobs/{actual-job-id}/logs
   ```

3. Check if the issue is:
   - **404 Not Found**: Route not registered correctly
   - **Empty logs**: Job has no logs (normal for some jobs)
   - **500 Server Error**: LogService initialization issue

4. Update frontend error handling to distinguish between:
   - Job exists but has no logs (show "No logs available")
   - API error (show actual error message)
   - Network error (show connectivity message)

5. If logs exist but aren't displaying, check `_parseLogEntry()` at line 509 for parsing issues

**Expected Outcome:**
- Job logs load successfully for completed jobs
- Empty log cases show friendly message instead of error
- Both single job and aggregated logs work correctly

---

## Constraints
- Must maintain backward compatibility with existing job data
- Logs must work for both running and completed jobs
- Document counts must be accurate across all UI views
- Event-driven architecture must remain intact (document_saved events)

## Success Criteria
- **Document Count Fix:**
  - Job queue shows 17 documents (not 34)
  - Job details shows "Documents Created: 17" (not 0)
  - All counts match actual database count

- **Job Logs Fix:**
  - Clicking "Output" tab loads logs successfully
  - Both single job logs and aggregated logs work
  - Empty logs show friendly message, not error
  - No "Failed to fetch job logs" errors

- **All existing tests pass:**
  - No regressions in job management
  - Event system continues to work correctly
  - Parent/child job hierarchy remains intact
