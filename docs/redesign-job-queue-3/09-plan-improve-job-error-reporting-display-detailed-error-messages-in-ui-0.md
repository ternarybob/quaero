I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State

**Error Infrastructure Already Exists:**
1. `CrawlJob.Error` field (crawler_job.go lines 65-69) - documented for user-friendly messages in format "Category: Brief description"
2. `formatJobError(category, err)` helper (crawler.go lines 43-95) - formats errors with context (HTTP status, timeout, network errors)
3. `UpdateJobStatus(jobID, status, errorMsg)` (job_storage.go lines 426-447) - persists error to database
4. Stale job detection (app.go lines 686-735) - already uses UpdateJobStatus with error message "Job stalled - no heartbeat for 15+ minutes"
5. UI error display (queue.html lines 287-293) - shows job.error in red alert box for failed jobs
6. Job logs modal (queue.html lines 538-615) - has error filtering UI (lines 555-568)

**Current Gaps:**
1. CrawlerJob.Execute() doesn't populate job.Error when errors occur - it just returns the error to the worker
2. Worker (worker.go lines 193-216) logs handler errors but doesn't call UpdateJobStatus to persist them
3. formatJobError() is never called - it's defined but unused
4. No "View Error Details" modal for long error messages
5. Logs modal doesn't auto-switch to error-only view for failed jobs
6. Stale job error message is generic - doesn't include actionable context

## Architecture Decisions

**Decision 1: Where to Populate job.Error**
- **Chosen**: In CrawlerJob.Execute() before returning error, call UpdateJobStatus to persist error message
- **Rationale**: Job handler has full context (URL, depth, config) to create detailed error messages
- **Alternative Rejected**: Populate in worker.go after handler fails - worker lacks job context for detailed messages

**Decision 2: Error Message Format**
- **Chosen**: Use formatJobError() to create "Category: Brief description" format with context
- **Categories**: "Validation", "Network", "Scraping", "Storage", "Timeout"
- **Context**: Include URL, HTTP status, timeout duration, retry attempts
- **Example**: "Network: Connection refused for https://example.com/page1"
- **Rationale**: Consistent format, user-friendly, actionable

**Decision 3: View Error Details Modal**
- **Chosen**: Create new modal similar to job logs modal structure
- **Trigger**: "View Details" link in error alert (queue.html line 291)
- **Content**: Full error message, job context (name, URL, status), timestamp, suggested actions
- **Rationale**: Keeps job cards clean while providing full details on demand

**Decision 4: Error Log Filtering**
- **Chosen**: Auto-switch logs modal to error-only view when opening for failed jobs
- **Implementation**: Check job status in openModal(), set selectedLogLevel='error' if status='failed'
- **User Control**: User can change filter back to 'all' if desired
- **Rationale**: Users opening logs for failed jobs want to see errors first

**Decision 5: Stale Job Error Enhancement**
- **Chosen**: Enhance stale job error message with actionable context
- **Format**: "Timeout: No activity for 15m - check network connectivity or increase timeout"
- **Location**: app.go line 714 (stale job detection)
- **Rationale**: Helps users diagnose and fix stale job issues

## Integration Points

**Point 1: CrawlerJob → UpdateJobStatus**
- When CrawlerJob.Execute() encounters error, call UpdateJobStatus before returning
- Use formatJobError() to create user-friendly message
- Include URL and depth in error context
- Pattern: `UpdateJobStatus(ctx, msg.JobID, "failed", formatJobError("Category", err))`

**Point 2: Worker → Job Status**
- Worker already handles handler errors (worker.go lines 193-216)
- No changes needed - worker logs error and deletes message
- Job status update happens in handler (CrawlerJob.Execute)

**Point 3: UI → Error Display**
- Existing error display (queue.html lines 287-293) already shows job.error
- Add "View Details" link that opens error details modal
- Modal shows full error message + job context

**Point 4: Logs Modal → Auto-Filter**
- In openModal() (queue.html line 2811), check job status
- If status='failed', set selectedLogLevel='error'
- Existing loadLogs() will fetch error-only logs

**Point 5: Stale Job Detection → Enhanced Error**
- Update error message in app.go line 714
- Use formatJobError("Timeout", fmt.Errorf("no activity for 15m"))
- Add actionable guidance in message

## Error Handling Patterns

**Pattern 1: Validation Errors**
```
if err := c.Validate(msg); err != nil {
    c.logger.LogJobError(err, "Validation failed")
    UpdateJobStatus(ctx, msg.JobID, "failed", formatJobError("Validation", err))
    return err
}
```

**Pattern 2: Network Errors**
```
if err := fetchURL(url); err != nil {
    c.logger.LogJobError(err, fmt.Sprintf("Failed to fetch URL: %s", url))
    UpdateJobStatus(ctx, msg.JobID, "failed", formatJobError("Network", err))
    return err
}
```

**Pattern 3: Timeout Errors**
```
if errors.Is(err, context.DeadlineExceeded) {
    c.logger.LogJobError(err, "Request timeout")
    UpdateJobStatus(ctx, msg.JobID, "failed", formatJobError("Timeout", err))
    return err
}
```

## Testing Considerations

**Test Case 1: Validation Error**
- Trigger: Invalid URL in job message
- Expected: job.error = "Validation: URL is required"
- Verify: Error displayed in UI, logs show error-level entry

**Test Case 2: Network Error**
- Trigger: Unreachable URL
- Expected: job.error = "Network: Connection refused for https://..."
- Verify: Error includes URL context

**Test Case 3: Stale Job**
- Trigger: Job runs > 15 minutes without heartbeat
- Expected: job.error = "Timeout: No activity for 15m - check network connectivity"
- Verify: Error is actionable

**Test Case 4: Error Details Modal**
- Trigger: Click "View Details" on failed job
- Expected: Modal opens with full error message and job context
- Verify: Modal shows job name, URL, timestamp, error message

**Test Case 5: Auto-Filter Logs**
- Trigger: Open logs modal for failed job
- Expected: selectedLogLevel='error', only error logs shown
- Verify: User can change filter to 'all'

### Approach

Enhance error handling in CrawlerJob to populate the job.Error field with detailed, user-friendly error messages using the existing formatJobError() helper. Update the UI to display errors prominently with a "View Error Details" modal for long messages. Implement error log filtering in the logs modal to automatically show error-level logs for failed jobs. The solution leverages existing infrastructure (Error field, formatJobError helper, UpdateJobStatus method, job logs modal) with minimal new code.

### Reasoning

I explored the codebase systematically: read crawler.go to understand the formatJobError() helper and current error handling, examined crawler_job.go to confirm the Error field structure, analyzed queue.html to see existing error display (lines 287-293) and logs modal (lines 538-615), reviewed worker.go to understand job execution flow (handler returns error → worker logs it), checked manager.go and job_storage.go to confirm UpdateJobStatus() persists error messages, examined app.go to see stale job detection (lines 686-735) that already uses UpdateJobStatus with error messages, and confirmed the logs modal has error filtering UI but needs backend support for auto-filtering failed jobs.

## Mermaid Diagram

sequenceDiagram
    participant Worker as Worker Pool
    participant Crawler as CrawlerJob
    participant FormatErr as formatJobError()
    participant Storage as JobStorage
    participant UI as Queue UI
    participant Modal as Error Details Modal
    participant LogsModal as Job Logs Modal

    Note over Worker,Storage: Error Handling Flow

    Worker->>Crawler: Execute(ctx, msg)
    Crawler->>Crawler: Validate(msg)
    alt Validation Fails
        Crawler->>FormatErr: formatJobError("Validation", err, msg.URL)
        FormatErr-->>Crawler: "Validation: URL is required"
        Crawler->>Storage: UpdateJobStatus(jobID, "failed", errorMsg)
        Storage-->>Crawler: Status updated
        Crawler->>Crawler: logger.LogJobError(err, errorMsg)
        Crawler-->>Worker: return error
    else Validation Succeeds
        Crawler->>Crawler: Process URL (future implementation)
        alt Network Error
            Crawler->>FormatErr: formatJobError("Network", err, msg.URL)
            FormatErr-->>Crawler: "Network: Connection refused for https://..."
            Crawler->>Storage: UpdateJobStatus(jobID, "failed", errorMsg)
            Crawler-->>Worker: return error
        else Timeout Error
            Crawler->>FormatErr: formatJobError("Timeout", err, msg.URL)
            FormatErr-->>Crawler: "Timeout: Request timeout for https://..."
            Crawler->>Storage: UpdateJobStatus(jobID, "failed", errorMsg)
            Crawler-->>Worker: return error
        else Success
            Crawler->>Storage: SaveJob(job) - update progress
            Crawler-->>Worker: return nil
        end
    end

    Note over UI,LogsModal: UI Error Display Flow

    UI->>UI: Render job card
    alt Job Status = Failed
        UI->>UI: Display error alert with job.error
        alt Error Length > 100
            UI->>UI: Show truncated error + "View Details" link
            User->>UI: Click "View Details"
            UI->>Modal: openErrorDetailsModal(job)
            Modal->>Modal: Display full error + job context
            Modal->>Modal: getSuggestedActions(error)
            Modal-->>User: Show error details + suggested actions
        else Error Length <= 100
            UI->>UI: Show full error inline
        end
    end

    User->>UI: Click "View Logs" on failed job
    UI->>LogsModal: openModal(jobId)
    LogsModal->>LogsModal: Check job.status
    alt Job Status = Failed
        LogsModal->>LogsModal: Set selectedLogLevel = 'error'
        LogsModal->>LogsModal: loadLogs() with level=error
        LogsModal-->>User: Display error-only logs
    else Job Status != Failed
        LogsModal->>LogsModal: Set selectedLogLevel = 'all'
        LogsModal->>LogsModal: loadLogs() with level=all
        LogsModal-->>User: Display all logs
    end

    Note over Worker,LogsModal: Stale Job Detection

    loop Every 5 minutes
        Worker->>Storage: GetStaleJobs(15 minutes)
        Storage-->>Worker: List of stale jobs
        alt Stale Jobs Found
            Worker->>Storage: UpdateJobStatus(jobID, "failed", "Timeout: No activity for 15+ minutes...")
            Storage-->>Worker: Status updated
            Worker->>UI: WebSocket update (job status changed)
            UI->>UI: Update job card with error message
        end
    end

## Proposed File Changes

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\storage\sqlite\job_storage.go
- internal\jobs\types\logger.go

**Enhance formatJobError() to Include More Context (lines 43-95):**

Add URL parameter to function signature:
- Change: `func formatJobError(category string, err error)` to `func formatJobError(category string, err error, url string)`
- Add URL to error messages where relevant
- For network errors: Include URL in message: `fmt.Sprintf("Network: %s for %s", briefCause, url)`
- For timeout errors: Include URL: `fmt.Sprintf("Timeout: Request timeout for %s", url)`
- For generic errors: Include URL if provided: `fmt.Sprintf("%s: %s (URL: %s)", category, errMsg, url)`

**Add HTTP Status Code Handling:**
- Add new error type check for HTTP errors (after line 85)
- Check if error message contains "HTTP" or status codes (404, 500, etc.)
- Extract status code and format: `"HTTP 404: Not Found for https://..."`
- Pattern: `if strings.Contains(errMsgLower, "404") { return fmt.Sprintf("HTTP 404: Not Found for %s", url) }`

**Update Execute() to Populate job.Error on Failure (lines 98-206):**

**Add JobStorage Dependency:**
- Add `JobStorage interfaces.JobStorage` to `CrawlerJobDeps` struct (line 18)
- Store in CrawlerJob struct via deps
- Use for UpdateJobStatus calls

**Validation Error Handling (lines 100-103):**
- After logging validation error (line 101), add:
  - `errorMsg := formatJobError("Validation", err, msg.URL)`
  - `c.deps.JobStorage.UpdateJobStatus(ctx, msg.JobID, "failed", errorMsg)`
- This persists validation errors to database
- Keep existing return statement

**Add Error Handling for URL Processing:**
- Currently Execute() is a simulation (lines 139-150)
- Add comment: `// TODO: When real URL processing is implemented, wrap in error handling:`
- Add example pattern:
  ```
  // if err := processURL(msg.URL); err != nil {
  //     errorMsg := formatJobError("Scraping", err, msg.URL)
  //     c.deps.JobStorage.UpdateJobStatus(ctx, msg.JobID, "failed", errorMsg)
  //     c.logger.LogJobError(err, fmt.Sprintf("Failed to process URL: %s", msg.URL))
  //     return fmt.Errorf("failed to process URL: %w", err)
  // }
  ```
- This provides template for future implementation

**Progress Update Error Handling (lines 154-165):**
- Wrap progress update in error check
- If SaveJob fails, log warning but don't fail job (non-critical)
- Pattern: `if saveErr := c.deps.JobStorage.SaveJob(ctx, job); saveErr != nil { c.logger.Warn()... }`
- Keep existing warning log (line 161)

**Child Job Enqueue Error Handling (lines 193-198):**
- Keep existing warning log (lines 194-198)
- Don't fail parent job if child enqueue fails
- This is correct behavior - partial success is acceptable

**Update ExecuteCompletionProbe Error Handling (lines 208-247):**

**Parent Job Load Error (lines 216-220):**
- After logging error (line 218), add:
  - `errorMsg := formatJobError("System", err, "")`
  - `c.deps.JobStorage.UpdateJobStatus(ctx, msg.JobID, "failed", errorMsg)`
- This persists probe errors

**Type Assertion Error (lines 222-226):**
- After logging error (line 224), add:
  - `errorMsg := formatJobError("System", fmt.Errorf("parent job is not a CrawlJob"), "")`
  - `c.deps.JobStorage.UpdateJobStatus(ctx, msg.JobID, "failed", errorMsg)`

**Parent Status Update Error (lines 234-237):**
- After logging error (line 235), add:
  - `errorMsg := formatJobError("System", err, "")`
  - `c.deps.JobStorage.UpdateJobStatus(ctx, msg.ParentID, "failed", errorMsg)`
- Note: Update parent job status, not probe job

**Add Helper Method for Error Persistence:**
- Add new method after Execute(): `func (c *CrawlerJob) failJobWithError(ctx context.Context, jobID string, category string, err error, url string) error`
- Consolidates error handling logic
- Calls formatJobError() and UpdateJobStatus()
- Logs error with JobLogger
- Returns original error for worker
- Pattern:
  ```
  func (c *CrawlerJob) failJobWithError(ctx context.Context, jobID string, category string, err error, url string) error {
      errorMsg := formatJobError(category, err, url)
      if updateErr := c.deps.JobStorage.UpdateJobStatus(ctx, jobID, "failed", errorMsg); updateErr != nil {
          c.logger.Warn().Err(updateErr).Msg("Failed to update job status")
      }
      c.logger.LogJobError(err, errorMsg)
      return err
  }
  ```
- Use this helper in all error paths

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\types\crawler.go(MODIFY)
- internal\storage\sqlite\job_storage.go

**Update CrawlerJobDeps to Include JobStorage (around line 385):**

Add JobStorage to deps struct:
- In crawlerJobDeps initialization (around line 385), add:
  - `JobStorage: a.StorageManager.JobStorage(),`
- This provides CrawlerJob access to UpdateJobStatus method
- Place after JobDefinitionStorage field

**Enhance Stale Job Error Message (line 714):**

Replace generic error message with actionable guidance:
- Current: `"Job stalled - no heartbeat for 15+ minutes"`
- New: `"Timeout: No activity for 15+ minutes - check network connectivity, increase timeout, or verify job is not stuck"`
- This provides users with actionable steps to resolve stale jobs
- Format matches "Category: Description" pattern from formatJobError()

**Add Job Context to Stale Job Logging (lines 721-725):**

Enhance success log with more context:
- Add job URL if available: `Str("url", job.SeedURLs[0] if len(job.SeedURLs) > 0 else "")`
- Add job type: `Str("job_type", string(job.JobType))`
- Add last heartbeat time: `Time("last_heartbeat", job.LastHeartbeat)`
- This helps diagnose why jobs became stale

**No Other Changes Needed:**
- Worker pool registration (lines 385-495) already correct
- Handler returns error → worker logs it → message deleted
- Job status update happens in handler (CrawlerJob.Execute)
- Stale job detection loop (lines 689-733) already functional

### pages\queue.html(MODIFY)

References: 

- internal\models\crawler_job.go(MODIFY)

**Enhance Error Display in Job Cards (lines 287-293):**

Add "View Details" link for long error messages:
- Check if error length > 100 characters
- If long, truncate to 100 chars and add "... [View Details]" link
- Link triggers error details modal: `@click.prevent="openErrorDetailsModal(item.job)"`
- If short, display full error as-is
- Pattern:
  ```html
  <template x-if="item.job.status === 'failed' && item.job.error">
      <div class="job-error-alert" style="...">
          <i class="fas fa-exclamation-circle" style="..."></i>
          <strong>Failure Reason:</strong>
          <span x-show="item.job.error.length <= 100" x-text="item.job.error"></span>
          <span x-show="item.job.error.length > 100">
              <span x-text="item.job.error.substring(0, 100) + '...'"></span>
              <a href="#" class="text-primary" @click.prevent="openErrorDetailsModal(item.job)" style="margin-left: 0.5rem;">
                  [View Details]
              </a>
          </span>
      </div>
  </template>
  ```

**Add Error Details Modal (after job logs modal, around line 616):**

Create new modal structure:
- Modal ID: `error-details-modal`
- Alpine component: `errorDetailsModal`
- Header: "Job Error Details - [Job Name]"
- Body sections:
  - Job Context: Name, ID (truncated), Status, URL (if available), Timestamp
  - Error Message: Full error text in monospace font
  - Suggested Actions: Based on error category (Network → check connectivity, Timeout → increase timeout, etc.)
- Footer: Close button
- Pattern:
  ```html
  <div id="error-details-modal" class="modal" x-data="errorDetailsModal" x-init="init()">
      <a href="#close" class="modal-overlay" @click.prevent="closeModal()"></a>
      <div class="modal-container" style="max-width: 700px;">
          <div class="modal-header">
              <a href="#close" class="btn btn-clear float-right" @click.prevent="closeModal()"></a>
              <div class="modal-title h5">Job Error Details - <span x-text="job?.name || 'Unknown'"></span></div>
          </div>
          <div class="modal-body">
              <div class="content">
                  <!-- Job Context -->
                  <div style="margin-bottom: 1rem;">
                      <h6>Job Context</h6>
                      <table class="table">
                          <tr><td><strong>Job ID:</strong></td><td><code x-text="job?.id?.substring(0, 16) || 'N/A'"></code></td></tr>
                          <tr><td><strong>Status:</strong></td><td><span class="label label-error" x-text="job?.status || 'N/A'"></span></td></tr>
                          <tr x-show="job?.seed_urls && job.seed_urls.length > 0"><td><strong>URL:</strong></td><td><a :href="job?.seed_urls[0]" target="_blank" x-text="job?.seed_urls[0]"></a></td></tr>
                          <tr x-show="job?.completed_at"><td><strong>Failed At:</strong></td><td x-text="job?.completed_at ? new Date(job.completed_at).toLocaleString() : 'N/A'"></td></tr>
                      </table>
                  </div>
                  <!-- Error Message -->
                  <div style="margin-bottom: 1rem;">
                      <h6>Error Message</h6>
                      <div style="padding: 1rem; background-color: #f8d7da; border-left: 4px solid var(--color-danger); border-radius: 4px; font-family: monospace; white-space: pre-wrap; word-break: break-word;" x-text="job?.error || 'No error message available'"></div>
                  </div>
                  <!-- Suggested Actions -->
                  <div x-show="getSuggestedActions(job?.error).length > 0">
                      <h6>Suggested Actions</h6>
                      <ul style="margin-left: 1.5rem;">
                          <template x-for="action in getSuggestedActions(job?.error)" :key="action">
                              <li x-text="action"></li>
                          </template>
                      </ul>
                  </div>
              </div>
          </div>
          <div class="modal-footer">
              <button type="button" class="btn" @click="closeModal()">Close</button>
          </div>
      </div>
  </div>
  ```

**Add errorDetailsModal Alpine Component (after jobLogsModal, around line 2950):**

Create component with state and methods:
- State: `job: null` (stores job object)
- Methods:
  - `init()` - listen for modal open events
  - `openModal(job)` - store job, show modal
  - `closeModal()` - hide modal, clear job
  - `getSuggestedActions(errorMsg)` - parse error category and return action list
- Pattern:
  ```javascript
  Alpine.data('errorDetailsModal', () => ({
      job: null,
      openListener: null,
      
      init() {
          this.openListener = (e) => this.openModal(e.detail.job);
          window.addEventListener('errorDetailsModal:open', this.openListener);
      },
      
      destroy() {
          if (this.openListener) {
              window.removeEventListener('errorDetailsModal:open', this.openListener);
          }
      },
      
      openModal(job) {
          this.job = job;
          const modal = document.getElementById('error-details-modal');
          modal.classList.add('active');
          document.body.classList.add('modal-open');
      },
      
      closeModal() {
          const modal = document.getElementById('error-details-modal');
          modal.classList.remove('active');
          document.body.classList.remove('modal-open');
          this.job = null;
      },
      
      getSuggestedActions(errorMsg) {
          if (!errorMsg) return [];
          const actions = [];
          const errorLower = errorMsg.toLowerCase();
          
          if (errorLower.includes('network') || errorLower.includes('connection')) {
              actions.push('Check network connectivity');
              actions.push('Verify the URL is accessible');
              actions.push('Check firewall or proxy settings');
          }
          if (errorLower.includes('timeout')) {
              actions.push('Increase timeout duration in job configuration');
              actions.push('Check if the target server is responding slowly');
              actions.push('Verify network latency is acceptable');
          }
          if (errorLower.includes('http 404') || errorLower.includes('not found')) {
              actions.push('Verify the URL is correct');
              actions.push('Check if the resource has been moved or deleted');
          }
          if (errorLower.includes('http 401') || errorLower.includes('unauthorized')) {
              actions.push('Check authentication credentials');
              actions.push('Verify API token or session is still valid');
          }
          if (errorLower.includes('http 403') || errorLower.includes('forbidden')) {
              actions.push('Check access permissions');
              actions.push('Verify account has required privileges');
          }
          if (errorLower.includes('http 500') || errorLower.includes('server error')) {
              actions.push('Contact the target server administrator');
              actions.push('Try again later - server may be experiencing issues');
          }
          if (errorLower.includes('validation')) {
              actions.push('Review job configuration for invalid parameters');
              actions.push('Check URL format and required fields');
          }
          if (errorLower.includes('stale') || errorLower.includes('no activity')) {
              actions.push('Check if the job is stuck in an infinite loop');
              actions.push('Increase heartbeat timeout if job legitimately takes longer');
              actions.push('Review job logs for the last activity before stalling');
          }
          
          return actions;
      }
  }));
  ```

**Add openErrorDetailsModal Helper to jobList Component (around line 1490):**

Add method to dispatch modal open event:
- Method: `openErrorDetailsModal(job)`
- Dispatches event: `window.dispatchEvent(new CustomEvent('errorDetailsModal:open', { detail: { job } }))`
- Place after other modal-related methods

**Update jobLogsModal to Auto-Filter Errors for Failed Jobs (line 2811):**

Modify openModal() to check job status:
- After setting currentJobId (line 2812), add:
  ```javascript
  // Auto-switch to error-only view for failed jobs
  const job = window.jobList?.allJobs?.find(j => j.id === jobId);
  if (job && job.status === 'failed') {
      this.selectedLogLevel = 'error';
  } else {
      this.selectedLogLevel = 'all';
  }
  ```
- This automatically filters to error logs when opening modal for failed jobs
- User can still change filter manually if desired

**Add toggleErrorsOnly() Method to jobLogsModal (after line 2950):**

Implement the method referenced in UI (line 566):
- Method: `toggleErrorsOnly()`
- Logic: `this.selectedLogLevel = (this.selectedLogLevel === 'error') ? 'all' : 'error'`
- Call loadLogs() after toggle
- Pattern:
  ```javascript
  toggleErrorsOnly() {
      this.selectedLogLevel = (this.selectedLogLevel === 'error') ? 'all' : 'error';
      this.loadLogs();
  }
  ```

### internal\models\crawler_job.go(MODIFY)

**No Changes Required:**

The Error field (lines 65-69) is already properly documented:
- Format: "Category: Brief description"
- Examples provided: "HTTP 404: Not Found", "Timeout: No activity for 10m"
- Usage documented: "Only populated when job status is 'failed'"
- Display guidance: "This field is displayed in the UI and should be actionable for users"

The existing documentation matches our implementation approach perfectly. No modifications needed to the model.

**Verification:**
- GetStatusReport() method (lines 318-372) already extracts errors from Error field (lines 366-369)
- Error field is properly serialized to JSON (line 69)
- Field is nullable (omitempty tag) so empty errors don't clutter responses

The model is ready to support enhanced error handling without any changes.