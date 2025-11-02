I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**What Already Works**:
1. ✅ `job.Error` field exists in `CrawlJob` model and is persisted to database
2. ✅ `GetLogsByLevel()` method exists in `JobLogStorage` (line 135 in job_log_storage.go)
3. ✅ `job.html` has client-side log filtering with dropdown (lines 77-83, 160-184)
4. ✅ CSS has `.terminal-error` class for red highlighting (line 434-436 in quaero.css)
5. ✅ API already returns the error field (it's in the model with JSON tag)

**What Needs Implementation**:
1. ❌ `GetJobLogsHandler()` doesn't support `?level=error` query parameter
2. ❌ `queue.html` doesn't display the `job.Error` field for failed jobs
3. ❌ No quick "Show Errors Only" toggle button in job.html (only dropdown exists)
4. ❌ Error logs could be more visually prominent (current red is subtle)

**Architecture Insight**:
- **queue.html** = Job list/queue management (uses Alpine.js `jobList` component)
- **job.html** = Individual job detail page (uses Alpine.js `jobDetailPage` component)
- The user wants error display in BOTH pages

### Approach

## High-Level Strategy

The implementation focuses on surfacing job failure reasons in the UI by:

1. **Backend Enhancement**: Extend `GetJobLogsHandler()` to support server-side log level filtering via query parameter
2. **Queue UI Enhancement**: Display the `job.Error` field prominently in job cards for failed jobs
3. **Job Detail UI Enhancement**: Add a quick "Show Errors Only" toggle button alongside the existing dropdown filter
4. **CSS Enhancement**: Ensure error logs are visually distinct with red highlighting

**Key Design Decisions**:
- Use **server-side filtering** for the API (more efficient than client-side for large log sets)
- Keep **client-side filtering** in job.html for responsive UX (already implemented)
- Display error messages as **alert boxes** in queue.html (more prominent than inline text)
- Maintain **backward compatibility** - existing log fetching without level parameter continues to work

### Reasoning

I explored the codebase by:
1. Reading the four files mentioned by the user
2. Discovering that `job.html` already has client-side log filtering with a dropdown
3. Finding that `GetLogsByLevel()` already exists in the storage layer
4. Identifying that `queue.html` displays job cards but doesn't show the error field
5. Confirming that CSS already has `.terminal-error` class for red error highlighting
6. Understanding the distinction between queue.html (job list) and job.html (individual job detail page)

## Mermaid Diagram

sequenceDiagram
    participant User as User (Browser)
    participant QueueUI as Queue UI<br/>(queue.html)
    participant JobUI as Job Detail UI<br/>(job.html)
    participant Handler as JobHandler
    participant LogStorage as JobLogStorage
    participant DB as SQLite Database

    Note over User,DB: Scenario 1: Viewing Failed Job in Queue

    User->>QueueUI: View job list
    QueueUI->>Handler: GET /api/jobs
    Handler->>DB: Fetch jobs with error field
    DB-->>Handler: Jobs with error populated
    Handler-->>QueueUI: JSON response (includes error field)
    QueueUI->>QueueUI: Render job cards
    QueueUI->>QueueUI: Check if job.status === 'failed' && job.error exists
    QueueUI->>User: Display red alert box with failure reason

    Note over User,DB: Scenario 2: Viewing Job Logs with Error Filter

    User->>JobUI: Navigate to job detail page
    JobUI->>Handler: GET /api/jobs/{id}/logs
    Handler->>LogStorage: GetLogs(jobID, 1000)
    LogStorage->>DB: SELECT * FROM job_logs WHERE job_id = ?
    DB-->>LogStorage: All logs (newest first)
    LogStorage-->>Handler: All logs
    Handler-->>JobUI: JSON response with all logs
    JobUI->>JobUI: Client-side filter by selectedLogLevel
    JobUI->>User: Display filtered logs with red highlighting

    Note over User,DB: Scenario 3: Server-Side Error Filtering (NEW)

    User->>JobUI: Click "Errors Only" toggle
    JobUI->>JobUI: Set selectedLogLevel = 'error'
    JobUI->>Handler: GET /api/jobs/{id}/logs?level=error
    Handler->>Handler: Parse level parameter
    Handler->>LogStorage: GetLogsByLevel(jobID, "error", 1000)
    LogStorage->>DB: SELECT * FROM job_logs WHERE job_id = ? AND level = 'error'
    DB-->>LogStorage: Error logs only
    LogStorage-->>Handler: Error logs
    Handler-->>JobUI: JSON response with error logs only
    JobUI->>User: Display error logs with enhanced red styling

## Proposed File Changes

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\storage\sqlite\job_log_storage.go
- internal\interfaces\storage.go

**Add server-side log level filtering support (lines 388-444)**

Update `GetJobLogsHandler()` to support optional `?level=error` query parameter:

**After line 408** (after parsing the `order` parameter), add level parameter parsing:
- Parse `level` query parameter: `level := r.URL.Query().Get("level")`
- Validate level is one of: "error", "warning", "info", "debug" (case-insensitive)
- If level is empty or "all", use existing `GetLogs()` method
- If level is specified, call `h.logService.GetLogsByLevel(ctx, jobID, level, 1000)` instead

**Update the response** (line 438-443):
- Add `"level"` field to response map to indicate which filter was applied
- Example: `"level": level` (or "all" if no filter)

**Add logging** for debugging:
- Log when level filtering is requested: `h.logger.Debug().Str("job_id", jobID).Str("level", level).Msg("Fetching logs with level filter")`

**Error handling**:
- If `GetLogsByLevel()` fails, fall back to `GetLogs()` and log a warning
- Ensure backward compatibility - requests without `level` parameter work as before

**Reference**: The storage method `GetLogsByLevel()` already exists at line 135 in `internal/storage/sqlite/job_log_storage.go`

### pages\queue.html(MODIFY)

References: 

- pages\static\quaero.css(MODIFY)
- internal\handlers\job_handler.go(MODIFY)

**Add failure reason display in job cards (after line 257)**

Insert a new section to display the error message for failed jobs:

**Location**: After the metadata section (line 257, after the "Show Configuration" link), before the parent progress display (line 259)

**Add conditional error display**:
- Use Alpine.js template: `<template x-if="item.job.status === 'failed' && item.job.error">`
- Create an alert box with red styling:
  - Use Spectre CSS `.toast` or custom alert styling
  - Icon: `<i class="fas fa-exclamation-circle"></i>`
  - Display: `item.job.error` (the concise error message)
  - Style: Red background (`background-color: #f8d7da`), red border-left (`border-left: 4px solid var(--color-danger)`)
  - Font size: `0.875rem` for readability
  - Margin: `margin-top: 0.8rem` to separate from metadata

**Example structure**:
```html
<template x-if="item.job.status === 'failed' && item.job.error">
  <div style="margin-top: 0.8rem; padding: 0.75rem; background-color: #f8d7da; border-left: 4px solid var(--color-danger); border-radius: 4px; font-size: 0.875rem;">
    <i class="fas fa-exclamation-circle" style="color: var(--color-danger); margin-right: 0.5rem;"></i>
    <strong>Failure Reason:</strong> <span x-text="item.job.error"></span>
  </div>
</template>
```

**Ensure visibility**:
- The error box should be prominent and immediately visible in the job card
- Use the same styling as toast-error (lines 526-530 in `pages/static/quaero.css`)

**Reference**: The `job.error` field is already returned by the API (verified in `internal/handlers/job_handler.go` lines 164-187 where jobs are enriched)

### pages\job.html(MODIFY)

**Add "Show Errors Only" quick toggle button (lines 77-90)**

Enhance the log controls section to include a quick toggle button alongside the existing dropdown:

**After line 83** (after the dropdown closing tag), add a quick toggle button:
- Button text: "Errors Only" with icon `<i class="fas fa-exclamation-triangle"></i>`
- Button class: `btn btn-sm btn-error` (when active) or `btn btn-sm` (when inactive)
- Alpine.js click handler: `@click="toggleErrorsOnly()"`
- Alpine.js dynamic class: `:class="selectedLogLevel === 'error' ? 'btn-error' : ''"`
- Title attribute: "Toggle error logs only"

**Add Alpine.js method** in the `jobDetailPage` component (after line 324):
- Method name: `toggleErrorsOnly()`
- Logic: Toggle `selectedLogLevel` between "error" and "all"
- Implementation:
  ```javascript
  toggleErrorsOnly() {
      this.selectedLogLevel = this.selectedLogLevel === 'error' ? 'all' : 'error';
  }
  ```

**Update the dropdown** (line 77):
- Keep the existing dropdown for granular control
- The dropdown and toggle button should sync (both control `selectedLogLevel`)

**Visual design**:
- Place the toggle button between the dropdown and the auto-scroll button
- Use consistent spacing with other buttons (already defined in navbar-section)

**Reference**: The `filteredLogs` computed property (lines 160-184) already handles filtering by `selectedLogLevel`, so no changes needed there

### pages\static\quaero.css(MODIFY)

References: 

- pages\job.html(MODIFY)

**Enhance error log highlighting (lines 434-436)**

Make error logs more visually prominent:

**Update `.terminal-error` class** (line 434-436):
- Increase color brightness: Change from `#f85149` to `#ff6b6b` (brighter red)
- Add font weight: `font-weight: 600;` for bold text
- Add optional background highlight: `background-color: rgba(248, 81, 73, 0.1);` for subtle background
- Add padding if background is used: `padding: 0.1rem 0.2rem;`
- Add border-radius: `border-radius: 2px;`

**Example enhanced style**:
```css
.terminal-error {
    color: #ff6b6b;
    font-weight: 600;
    background-color: rgba(248, 81, 73, 0.1);
    padding: 0.1rem 0.2rem;
    border-radius: 2px;
}
```

**Add a new class for error alert boxes** (after line 436):
- Class name: `.job-error-alert`
- Purpose: Style the error message boxes in queue.html job cards
- Properties:
  - `background-color: #f8d7da;`
  - `border-left: 4px solid var(--color-danger);`
  - `color: #721c24;`
  - `padding: 0.75rem;`
  - `border-radius: var(--border-radius);`
  - `margin-top: 0.8rem;`
  - `font-size: 0.875rem;`
  - `display: flex;`
  - `align-items: flex-start;`
  - `gap: 0.5rem;`

**Consistency check**:
- Ensure the new `.job-error-alert` class matches the existing `.toast-error` styling (lines 526-530)
- Both should use the same color scheme for visual consistency

**Reference**: The terminal error class is used in `job.html` at line 110 where log level classes are applied

### internal\models\crawler_job.go(MODIFY)

References: 

- internal\handlers\job_handler.go(MODIFY)

**Verify Error field serialization (line 51)**

No code changes needed - this is a verification step:

**Check the Error field definition** (around line 51):
- Ensure the field has the correct JSON tag: `json:"error,omitempty"`
- Ensure the field is exported (capitalized): `Error string`
- The `omitempty` tag ensures empty errors aren't included in JSON responses

**If the field doesn't exist or has incorrect tags**:
- Add or fix the field definition
- Ensure it's placed logically with other status-related fields (near `Status`, `CompletedAt`, etc.)

**Documentation**:
- The field should have a comment explaining its purpose (as implemented in the previous phase)
- Comment should mention the expected format: "Category: Brief description"

**Reference**: This field was added in the previous implementation phase ("Populate job Error field on failure"). The handler at line 169 in `job_handler.go` already includes this field in the response via `convertJobToMap()` which uses JSON marshaling.