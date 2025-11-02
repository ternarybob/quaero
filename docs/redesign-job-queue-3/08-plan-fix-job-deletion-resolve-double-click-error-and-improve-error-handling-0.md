I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Root Cause Analysis

**Issue 1: Double-Click Error in Batch Deletion**
- `deleteSelectedJobs()` (line 2392-2468) disables the button AFTER starting deletion (line 2408)
- No check for already-disabled state at function entry
- Concurrent calls can occur if user double-clicks before button is disabled
- Single job deletion (`deleteJob()` at line 1347) HAS idempotency check (line 1352-1354)

**Issue 2: Generic Error Messages**
- Backend `DeleteJobHandler` (line 862) returns: `"Failed to delete job"` without error details
- Frontend (line 2435-2436) captures `response.text()` but backend sends plain text via `http.Error()`
- No distinction between error types: running job, not found, cascade failure, database error
- Manager.DeleteJob logs detailed errors but doesn't propagate them to handler

**Issue 3: Duplicate Notifications**
- Lines 2455-2462 show both success AND error notifications for partial failures
- This is actually CORRECT behavior (not a bug) - user needs to know both outcomes
- Real issue: error notification shows only first failure (line 2460: `results.failed[0].error`)
- Should show summary of all failures or most common error type

**Issue 4: Basic Confirmation Dialog**
- Line 2401: Uses browser `confirm()` with truncated job IDs
- Shows: `"Jobs: abc12345..., def67890..."`
- Missing: job names, URLs, status, child count, cascade warning
- No way to review what will be deleted before confirming

**Issue 5: Not Optimistic**
- Lines 2425-2433: Jobs removed from arrays AFTER successful API response
- User sees no immediate feedback during deletion
- If deletion takes time, UI appears frozen
- Should remove immediately and rollback on error

## Design Decisions

**Decision 1: Idempotency Strategy**
- Add disabled state check at function entry (before confirmation)
- Use a deletion-in-progress flag instead of just button.disabled
- Reason: Button might be re-enabled by other code, flag is more reliable
- Pattern: `if (this.isDeletingJobs) return;`

**Decision 2: Structured Error Response Format**
- Backend returns JSON instead of plain text:
  ```
  {
    "error": "Failed to delete job",
    "details": "Cannot delete running job",
    "job_id": "abc123",
    "status": "running",
    "child_count": 5
  }
  ```
- Frontend parses JSON and displays specific error messages
- Fallback to plain text for backward compatibility

**Decision 3: Error Message Specificity**
- Distinguish error types in backend:
  - Running job: "Cannot delete running job {id}. Cancel it first."
  - Not found: "Job {id} not found"
  - Cascade failure: "Deleted job but {n} children failed"
  - Database error: "Database error: {details}"
- Include actionable guidance in error messages

**Decision 4: Confirmation Dialog Enhancement**
- Replace `confirm()` with custom modal dialog
- Show table of jobs to be deleted with: ID (truncated), Name, Status, Child Count
- Add cascade warning: "This will also delete {n} child jobs"
- Add checkbox: "I understand this action cannot be undone"
- Buttons: "Cancel" (default) and "Delete" (danger style)

**Decision 5: Optimistic Update Strategy**
- Remove jobs from UI immediately after confirmation
- Store removed jobs in temporary array for rollback
- On error: restore jobs to original positions in arrays
- On success: clear temporary array
- Show loading indicator during deletion (subtle, non-blocking)

**Decision 6: Batch Deletion Error Handling**
- Continue deleting remaining jobs even if some fail (current behavior is correct)
- Show single notification with summary: "Deleted 5 of 7 jobs. 2 failed."
- Add "View Details" button to notification that shows error list
- Log all errors to console for debugging

## Alternative Approaches Considered

**Alternative 1: Abort on First Error**
- Stop batch deletion when first error occurs
- **Rejected**: User expects all selected jobs to be attempted
- Partial deletion is better than no deletion

**Alternative 2: Server-Side Batch Deletion Endpoint**
- Create `POST /api/jobs/batch-delete` endpoint
- Backend handles all deletions in transaction
- **Rejected**: Adds complexity, current approach works fine
- Client-side batch is more flexible (can show progress)

**Alternative 3: Soft Delete with Undo**
- Mark jobs as deleted, allow undo within 5 seconds
- **Rejected**: Over-engineering for this use case
- Confirmation dialog is sufficient safeguard

**Alternative 4: Disable All Delete Buttons During Batch**
- Prevent any deletion while batch is in progress
- **Rejected**: Too restrictive, user might want to delete other jobs
- Per-job idempotency is sufficient

## Testing Considerations

**Test Case 1: Double-Click Prevention**
- Rapidly click "Delete Selected" button twice
- Expected: Only one deletion request sent
- Verify: Check network tab for duplicate requests

**Test Case 2: Cascade Deletion**
- Delete parent job with 5 children
- Expected: All 6 jobs deleted (parent + children)
- Verify: Check database and UI for complete removal

**Test Case 3: Partial Failure**
- Select 3 jobs: 1 running, 2 completed
- Expected: 2 deleted, 1 fails with "Cannot delete running job"
- Verify: Notification shows "Deleted 2 of 3 jobs. 1 failed."

**Test Case 4: Optimistic Update Rollback**
- Mock API to return error for all deletions
- Expected: Jobs removed from UI, then restored on error
- Verify: Jobs reappear in original positions

**Test Case 5: Confirmation Dialog Details**
- Select jobs with various statuses and child counts
- Expected: Modal shows all job details and cascade warning
- Verify: User can review before confirming

**Test Case 6: Network Failure**
- Disconnect network during deletion
- Expected: Error notification with network error message
- Verify: Jobs restored to UI (optimistic rollback)

### Approach

Fix the job deletion double-click error and improve error handling by adding idempotency checks to `deleteSelectedJobs()`, enhancing the backend `DeleteJobHandler` to return structured error responses with specific details, implementing optimistic UI updates with rollback capability, and replacing the basic confirmation dialog with a detailed modal showing job names, URLs, and cascade information. All changes are localized to three files with no architectural modifications required.

### Reasoning

I explored the queue.html template (2987 lines), identified two deletion functions (`deleteJob()` at line 1347 with idempotency and `deleteSelectedJobs()` at line 2392 without), examined the DeleteJobHandler in job_handler.go (lines 827-862) which returns generic error messages, reviewed the Manager.DeleteJob implementation in manager.go (lines 135-236) which has robust cascade deletion logic, and analyzed the current error handling patterns showing that generic errors are returned without context.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant UI as queue.html
    participant Modal as Delete Confirmation Modal
    participant Handler as DeleteJobHandler
    participant Manager as JobManager
    participant Storage as JobStorage

    Note over User,Storage: BEFORE FIX: Double-Click Race Condition

    User->>UI: Double-click "Delete Selected"
    UI->>UI: Call deleteSelectedJobs() (1st click)
    UI->>UI: Call deleteSelectedJobs() (2nd click)
    Note over UI: No idempotency check!
    par First Request
        UI->>Handler: DELETE /api/jobs/abc123
    and Second Request
        UI->>Handler: DELETE /api/jobs/abc123
    end
    Handler-->>UI: 500 "Failed to delete job" (generic)
    UI-->>User: Show error notification (no details)

    Note over User,Storage: AFTER FIX: Idempotency + Structured Errors

    User->>UI: Click "Delete Selected" (3 jobs)
    UI->>UI: Check isDeletingJobs flag
    alt Already deleting
        UI->>UI: Return early (prevent duplicate)
    else Not deleting
        UI->>UI: Set isDeletingJobs = true
        UI->>UI: Fetch job details for modal
        UI->>Modal: Open with job details (names, status, children)
        Modal-->>User: Show confirmation dialog
        User->>Modal: Check "I understand" + Click "Delete"
        Modal->>UI: Confirm deletion
        
        Note over UI: Optimistic Update
        UI->>UI: Remove jobs from allJobs/filteredJobs
        UI->>UI: Store snapshot for rollback
        UI->>UI: renderJobs() - jobs disappear immediately
        
        loop For each selected job
            UI->>Handler: DELETE /api/jobs/{id}
            Handler->>Manager: DeleteJob(ctx, jobId)
            Manager->>Storage: GetChildJobs(ctx, jobId)
            Storage-->>Manager: [child1, child2, child3]
            
            alt Job is running
                Manager-->>Handler: error "cannot delete running job"
                Handler-->>UI: 400 JSON {error, details, status: "running"}
                UI->>UI: Restore job from snapshot (rollback)
                UI->>UI: Add to results.failed
            else Job deleted successfully
                Manager->>Storage: DeleteJob(ctx, child1)
                Manager->>Storage: DeleteJob(ctx, child2)
                Manager->>Storage: DeleteJob(ctx, child3)
                Manager->>Storage: DeleteJob(ctx, jobId)
                Manager-->>Handler: cascadeCount=3, nil
                Handler-->>UI: 200 JSON {message, cascade_deleted: 3}
                UI->>UI: Add to results.successful
            end
        end
        
        Note over UI: Show Results
        alt All successful
            UI-->>User: "Successfully deleted 3 jobs"
        else Partial failure
            UI-->>User: "Deleted 2 of 3 jobs. 1 failed: Cannot delete running job"
        else All failed
            UI->>UI: Restore all jobs from snapshot
            UI-->>User: "Failed to delete 3 jobs: {error details}"
        end
        
        UI->>UI: Set isDeletingJobs = false
        UI->>UI: Clear selectedJobIds
        UI->>UI: renderJobs() - update UI
    end

## Proposed File Changes

### pages\queue.html(MODIFY)

References: 

- internal\handlers\job_handler.go(MODIFY)

**Add Idempotency Check to deleteSelectedJobs() (line 2392):**

Add deletion-in-progress flag to Alpine component state (in jobList component initialization, around line 1489):
- Add state variable: `isDeletingJobs: false`
- This flag prevents concurrent deletion operations

Modify deleteSelectedJobs() method (line 2392):
- Add idempotency check at function entry (before line 2393):
  - Check: `if (this.isDeletingJobs) return;`
  - Set flag: `this.isDeletingJobs = true`
  - This prevents double-click from triggering duplicate deletions
- Move button disable logic earlier (before confirmation dialog)
- Add try-finally block to ensure flag is cleared even on error
- Clear flag in finally block: `this.isDeletingJobs = false`

**Implement Optimistic UI Updates with Rollback (lines 2416-2441):**

Before deletion loop (after line 2415):
- Create snapshot of jobs to be deleted: `const jobsToDelete = Array.from(this.selectedJobIds).map(id => { ... })`
- For each job ID, find the job object in allJobs and store: `{ id, job, allJobsIndex, filteredJobsIndex }`
- Remove jobs from UI immediately (optimistic update):
  - Remove from allJobs: `this.allJobs = this.allJobs.filter(job => !this.selectedJobIds.has(job.id))`
  - Remove from filteredJobs: `this.filteredJobs = this.filteredJobs.filter(job => !this.selectedJobIds.has(job.id))`
  - Call renderJobs() to update UI immediately
- User sees jobs disappear instantly (better UX)

Modify deletion loop (lines 2417-2441):
- Keep existing fetch logic for each job
- On success: Add job ID to results.successful (no array manipulation needed - already removed)
- On failure: Add to results.failed AND restore job to arrays:
  - Find job in jobsToDelete snapshot
  - Restore to allJobs at original index: `this.allJobs.splice(job.allJobsIndex, 0, job.job)`
  - Restore to filteredJobs at original index: `this.filteredJobs.splice(job.filteredJobsIndex, 0, job.job)`
  - Call renderJobs() to show restored job

**Enhance Error Handling with Structured Responses (lines 2434-2440):**

Modify error handling in deletion loop:
- Change line 2435 from `response.text()` to parse JSON:
  - Try to parse as JSON: `const errorData = await response.json()`
  - Extract specific error: `errorData.details || errorData.error || 'Unknown error'`
  - Fallback to text if JSON parse fails: `catch { errorText = await response.text() }`
- Store structured error in results.failed: `{ jobId, error: errorText, status: errorData.status, childCount: errorData.child_count }`
- This provides context for better error messages

**Improve Notification Messages (lines 2454-2462):**

Replace notification logic:
- If all successful: Show "Successfully deleted {count} job(s)"
- If all failed: Show "Failed to delete {count} job(s): {most common error}"
- If partial success: Show "Deleted {success} of {total} jobs. {failed} failed."
  - Add "View Details" button to notification (if supported by showNotification)
  - On click: Show modal with list of failed jobs and their errors
- Only show ONE notification (not separate success and error)
- Log detailed errors to console: `console.error('Deletion failures:', results.failed)`

**Replace Confirmation Dialog with Detailed Modal (lines 2401-2403):**

Replace basic confirm() with custom modal:
- Create modal HTML structure (add to page, around line 670 after job logs modal):
  - Modal title: "Confirm Job Deletion"
  - Warning message: "You are about to delete {count} job(s). This action cannot be undone."
  - Table showing jobs to delete:
    - Columns: Job ID (8 chars), Name, Status, Children
    - Rows: One per selected job
    - Highlight running jobs in red
  - Cascade warning (if any parent jobs): "This will also delete {total_children} child jobs."
  - Checkbox: "I understand this action is permanent"
  - Buttons: "Cancel" (default focus) and "Delete" (danger style, disabled until checkbox checked)
- Add Alpine component for modal (deleteConfirmModal):
  - State: isOpen, jobs, totalChildren, checkboxChecked
  - Methods: open(jobIds), close(), confirm()
- Call modal instead of confirm(): `await this.deleteConfirmModal.open(Array.from(this.selectedJobIds))`
- Modal fetches job details for display (name, status, child_count)
- Returns promise that resolves to true/false (confirmed/cancelled)

**Add Loading Indicator During Deletion (lines 2409-2410):**

Enhance loading state:
- Show subtle loading overlay on job cards container (not blocking)
- Add progress text: "Deleting {current} of {total} jobs..."
- Update progress after each deletion completes
- Remove overlay when all deletions complete (success or failure)
- Use Alpine reactive state: `deletionProgress: { current: 0, total: 0 }`

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\services\jobs\manager.go(MODIFY)
- internal\models\crawler_job.go

**Enhance DeleteJobHandler to Return Structured Error Responses (lines 827-873):**

Replace generic error handling with structured JSON responses:

**Add Error Response Helper Function (before DeleteJobHandler, around line 826):**
- Define struct: `type DeleteJobErrorResponse struct { Error string, Details string, JobID string, Status string, ChildCount int }`
- Add helper function: `writeDeleteError(w http.ResponseWriter, statusCode int, errorMsg string, details string, jobID string, jobStatus string, childCount int)`
- Function sets Content-Type to application/json
- Encodes DeleteJobErrorResponse struct to JSON
- Logs error with structured fields

**Modify Job Not Found Error (line 846-857):**
- Replace `http.Error()` with structured response
- If GetJobStatus returns error (job not found):
  - Call: `writeDeleteError(w, 404, "Job not found", fmt.Sprintf("Job %s does not exist", jobID), jobID, "", 0)`
  - Return structured JSON: `{"error": "Job not found", "details": "Job abc123 does not exist", "job_id": "abc123"}`

**Modify Running Job Error (line 853-856):**
- Replace `http.Error()` with structured response
- If job.Status == JobStatusRunning:
  - Call: `writeDeleteError(w, 400, "Cannot delete running job", "Job is currently running. Cancel it first.", jobID, string(job.Status), 0)`
  - Return structured JSON with status field: `{"error": "Cannot delete running job", "details": "Job is currently running. Cancel it first.", "job_id": "abc123", "status": "running"}`

**Enhance Deletion Error with Context (line 859-862):**
- Before calling DeleteJob, fetch job to get child count:
  - Call: `jobInterface, _ := h.jobManager.GetJob(ctx, jobID)`
  - Type assert to CrawlJob and get child_count
- If DeleteJob returns error:
  - Parse error message to determine type:
    - If contains "running": Use running job error response
    - If contains "not found": Use not found error response
    - Otherwise: Generic deletion error
  - Call: `writeDeleteError(w, 500, "Failed to delete job", err.Error(), jobID, "", childCount)`
  - Include child count in response for cascade context
  - Return structured JSON: `{"error": "Failed to delete job", "details": "database error: ...", "job_id": "abc123", "child_count": 5}`

**Add Success Response with Cascade Info (after line 862):**
- On successful deletion, return structured JSON (not just 200 OK)
- Response structure:
  ```
  {
    "message": "Job deleted successfully",
    "job_id": "abc123",
    "cascade_deleted": 5,
    "logs_deleted": true
  }
  ```
- Include cascade_deleted count (number of child jobs deleted)
- This provides confirmation of cascade deletion to user

**Add Logging for Deletion Attempts (line 861):**
- Log deletion attempt BEFORE calling DeleteJob:
  - Log: `h.logger.Info().Str("job_id", jobID).Int("child_count", childCount).Msg("Attempting to delete job")`
- Log successful deletion AFTER DeleteJob:
  - Log: `h.logger.Info().Str("job_id", jobID).Msg("Job deleted successfully")`
- This provides audit trail for deletions

### internal\services\jobs\manager.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\models\crawler_job.go

**Enhance DeleteJob to Return Cascade Deletion Count (lines 135-236):**

Modify DeleteJob signature and implementation:

**Change Return Type (line 141):**
- Current: `func (m *Manager) DeleteJob(ctx context.Context, jobID string) error`
- New: `func (m *Manager) DeleteJob(ctx context.Context, jobID string) (int, error)`
- Return value: (cascadeDeletedCount int, error)
- This allows handler to report cascade deletion count to user

**Track Cascade Deletion Count (lines 171-215):**
- Initialize counter: `totalCascadeDeleted := 0`
- In child deletion loop (line 182-200):
  - On successful child deletion: `totalCascadeDeleted++`
  - For recursive deletions, add returned count: `childCascadeCount, err := m.deleteJobRecursive(...)`
  - Accumulate: `totalCascadeDeleted += childCascadeCount`
- This tracks total jobs deleted in cascade (children + grandchildren + ...)

**Return Cascade Count (line 235):**
- Change: `return nil` to `return totalCascadeDeleted, nil`
- On error: `return 0, fmt.Errorf("failed to delete job: %w", err)`
- Handler can use this count in success response

**Improve Error Context in Cascade Failures (lines 208-214):**
- Current behavior: Logs errors but continues with parent deletion (correct)
- Enhancement: Include child error details in parent deletion error (optional):
  - If any child deletions failed AND parent deletion fails:
    - Return aggregated error: `fmt.Errorf("failed to delete job %s: %w (also failed to delete %d children)", jobID, err, len(errs))`
  - This provides full context of cascade failure
- Keep existing behavior: Parent deletion succeeds even if some children fail

**Add Validation for Running Jobs (lines 217-223):**
- Current: Cancels running jobs before deletion
- Enhancement: Return error instead of auto-cancelling:
  - Check: `if job.Status == models.JobStatusRunning`
  - Return: `return 0, fmt.Errorf("cannot delete running job %s: job is currently executing", jobID)`
  - This prevents accidental deletion of running jobs
  - Handler will return 400 Bad Request with clear error message
- Alternative: Keep auto-cancel behavior but log warning
- **Decision**: Return error (safer, explicit user action required)

**No Changes Needed:**
- Recursive deletion logic (lines 146-151) is correct
- Child job fetching (lines 166-169) is correct
- FK CASCADE for logs and seen_urls (line 226) is correct
- Error logging (lines 190-195, 210-213) is comprehensive
- The implementation is already robust and well-designed