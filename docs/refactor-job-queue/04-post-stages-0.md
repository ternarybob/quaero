I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State:**
- Alpine.js jobList component manages hierarchical job display with expandable parent cards
- Child jobs displayed in unified list (childJobsList Map) within parent cards, not as separate cards
- WebSocket handlers exist for job_spawn (lines 910-924) and job_status_change (lines 882-891)
- Delete button lacks idempotency - can be clicked multiple times causing errors
- No 404 error handling when fetching deleted jobs via WebSocket updates
- Job type badges not displayed (pre_validation, crawler_url, post_summary)
- Backend already returns job_type field in API responses

**Key Integration Points:**
1. Job card rendering template (lines 180-336) - Add job_type badges
2. handleChildSpawned method (lines 1355-1379) - Already handles job_spawn events
3. updateJobInList method (lines 1877-1997) - Add 404 error handling
4. deleteJob function (lines 1156-1215) - Add idempotency check
5. getParentProgressText method (lines 1551-1571) - Enhance with job type breakdown

**Design Decision:**
Display job_type badges inline with status badges to show workflow stages (pre-validation → crawler URLs → post-summary). This provides visual clarity without cluttering the UI.

### Approach

Enhance the Queue Management UI to display hierarchical job trees with job type badges (pre-validation, crawler_url, post-summary), fix delete button idempotency, add 404 error handling, and improve WebSocket event handling for real-time child job updates. All changes are frontend-only since the backend already supports job_type field, parent_id filtering, and child statistics.

### Reasoning

Explored the queue.html file structure and identified the Alpine.js jobList component (lines 1320-2056) that manages job display. Reviewed the WebSocket event handlers (lines 848-960) for job_spawn and job_status_change events. Examined the deleteJob function (lines 1156-1215) and identified the missing idempotency check. Confirmed the backend job_handler.go already returns job_type field and child statistics. Analyzed the childJobsList Map structure (lines 1355-1379) used for displaying spawned URLs within parent cards.

## Mermaid Diagram

sequenceDiagram
    participant User as User
    participant UI as Queue UI (Alpine.js)
    participant WS as WebSocket
    participant API as Job Handler API
    participant Storage as Job Storage

    Note over User,Storage: Phase 1: Job Type Badge Display

    User->>UI: View parent job card
    UI->>UI: Render job_type badges (pre_validation, crawler_url, post_summary)
    UI->>UI: Display aggregate progress with type breakdown
    Note over UI: "42 jobs (1 pre-val, 35 URLs, 1 post-sum) - 30 completed, 5 running"

    Note over User,Storage: Phase 2: Delete Button Idempotency

    User->>UI: Click delete button
    UI->>UI: Check if button.disabled === true
    alt Button already disabled
        UI-->>User: Ignore click (no action)
    else Button enabled
        UI->>UI: Disable button immediately
        UI->>User: Show confirmation dialog
        alt User confirms
            UI->>API: DELETE /api/jobs/{id}
            API->>Storage: DeleteJob(id) [idempotent]
            Storage-->>API: 200 OK (or 404 if already deleted)
            API-->>UI: 200 OK
            UI->>UI: Remove job from DOM
            UI-->>User: Show success notification
        else User cancels
            UI->>UI: Re-enable button
            UI-->>User: No action taken
        end
    end

    Note over User,Storage: Phase 3: WebSocket Real-Time Updates

    WS->>UI: job_spawn event {parent_id, child_id, job_type, url}
    UI->>UI: handleChildSpawned(spawnData)
    UI->>UI: Add child to childJobsList with job_type
    UI->>UI: Update parent.child_count++
    UI->>UI: Re-render parent card with new child

    WS->>UI: job_status_change {job_id, status, job_type}
    UI->>UI: updateJobInList(update)
    UI->>API: GET /api/jobs/{job_id} (if not in local cache)
    alt Job exists
        API-->>UI: 200 OK {job with job_type}
        UI->>UI: Update job in allJobs array
        UI->>UI: Update childJobsList if child job
        UI->>UI: Re-render affected cards
    else Job deleted (404)
        API-->>UI: 404 Not Found
        UI->>UI: Log debug "Job deleted or not found"
        UI->>UI: Remove from local cache silently
        Note over UI: No error notification shown
    end

    Note over User,Storage: Phase 4: Enhanced Progress Display

    UI->>UI: getParentProgressText(job)
    UI->>UI: Query childJobsList for job_type breakdown
    UI->>UI: Count pre_validation, crawler_url, post_summary jobs
    UI->>UI: Format: "42 jobs (1 pre-val, 35 URLs, 1 post-sum)"
    UI->>UI: Append status: "30 completed, 5 failed, 7 running"
    UI-->>User: Display comprehensive progress text

    Note over User,Storage: Phase 5: Child Job List with Type Badges

    User->>UI: Expand parent job
    UI->>UI: Render childJobsList items
    loop For each child
        UI->>UI: Display job_type badge (icon + color)
        UI->>UI: Display status badge
        UI->>UI: Display URL and depth
    end
    UI-->>User: Show hierarchical child list with type indicators

## Proposed File Changes

### pages\queue.html(MODIFY)

**1. Add job_type badge display in job card metadata section (after line 240):**

- Locate the metadata section with status badge (lines 234-240)
- After the status badge div, add new div for job_type badge:
  - Check if `item.job.job_type` exists and is not 'parent'
  - Display badge with appropriate styling:
    - `pre_validation` → Orange badge with icon `fa-check-circle`, text "Pre-Validation"
    - `crawler_url` → Blue badge with icon `fa-link`, text "URL Crawl"
    - `post_summary` → Purple badge with icon `fa-file-alt`, text "Post-Summary"
  - Use Bulma CSS classes: `label label-warning` (orange), `label label-info` (blue), `label label-primary` (purple)
  - Add tooltip with full job type description

**2. Fix delete button idempotency (lines 1156-1215):**

- At the start of `deleteJob` function (after line 1160), add idempotency check:
  - Check if button is already disabled: `if (button && button.disabled) return;`
  - This prevents double-clicks from triggering multiple DELETE requests
- Move button disable logic BEFORE the confirmation dialog (currently at line 1168)
  - Disable button immediately on first click
  - If user cancels confirmation, re-enable button before returning
- In the `finally` block (lines 1207-1214), only re-enable button if there was an error
  - On success, button remains disabled (job is deleted, button will be removed from DOM)
  - On error, re-enable button to allow retry

**3. Add 404 error handling in updateJobInList method (lines 1877-1997):**

- In the fetch block (lines 1886-1896) where job is fetched if not found locally:
  - Check response status before calling `response.ok`
  - If `response.status === 404`, log debug message "Job deleted or not found" and return early
  - This prevents error notifications when WebSocket sends updates for deleted jobs
  - Keep existing error handling for other HTTP errors (500, 503, etc.)

**4. Enhance getParentProgressText to show job type breakdown (lines 1551-1571):**

- After calculating total/completed/failed counts (lines 1556-1559), query childJobsList for job type breakdown:
  - Count pre_validation jobs: `childJobsList.get(job.id).filter(c => c.job_type === 'pre_validation').length`
  - Count crawler_url jobs: `childJobsList.get(job.id).filter(c => c.job_type === 'crawler_url').length`
  - Count post_summary jobs: `childJobsList.get(job.id).filter(c => c.job_type === 'post_summary').length`
- Update progress text format (line 1570):
  - FROM: `${total} child jobs spawned (${parts.join(', ')})`
  - TO: `${total} jobs (${preCount} pre-val, ${crawlerCount} URLs, ${postCount} post-sum) - ${parts.join(', ')}`
  - Only show non-zero counts to avoid clutter

**5. Update handleChildSpawned to store job_type (lines 1355-1379):**

- In childMeta object creation (lines 1360-1366), add job_type field:
  - `job_type: spawnData.job_type || 'crawler_url'` (default to crawler_url for backward compatibility)
- This ensures job_type is available for filtering and display in child job lists

**6. Enhance child job list display to show job_type badges (lines 286-299):**

- In the child job item template (lines 287-298), add job_type badge before status badge:
  - Check `child.job_type` and display appropriate icon:
    - `pre_validation` → `fa-check-circle` icon, orange color
    - `crawler_url` → `fa-link` icon, blue color
    - `post_summary` → `fa-file-alt` icon, purple color
  - Use small badge size: `label label-sm`
  - Place before status badge for visual hierarchy

**7. Update WebSocket job_status_change handler (lines 882-891):**

- Enhance updateJobInList call to include job_type from WebSocket payload:
  - WebSocket message already includes job_type field (backend sends it)
  - Ensure job_type is passed through to updateJobInList method
  - Update handleChildJobStatus method (lines 1381-1398) to accept job_type parameter
  - Store job_type in childJobsList when updating child status

**Pattern Reference:** Follow existing badge styling patterns (lines 236-240 for status badges, lines 217-222 for entity type badges). Use Alpine.js `x-if` directives for conditional rendering and `:class` bindings for dynamic styling.

### pages\static\common.js(MODIFY)

References: 

- pages\queue.html(MODIFY)

**1. Add job_type color mapping utility function (after line 237):**

- Create `getJobTypeBadgeClass` function:
  - Parameters: `jobType` (string)
  - Returns: CSS class string for badge styling
  - Mapping:
    - `pre_validation` → `label-warning` (orange)
    - `crawler_url` → `label-info` (blue)
    - `post_summary` → `label-primary` (purple)
    - `parent` → `label-success` (green)
    - Default → `label` (gray)
- Create `getJobTypeIcon` function:
  - Parameters: `jobType` (string)
  - Returns: Font Awesome icon class string
  - Mapping:
    - `pre_validation` → `fa-check-circle`
    - `crawler_url` → `fa-link`
    - `post_summary` → `fa-file-alt`
    - `parent` → `fa-folder`
    - Default → `fa-question-circle`

**2. Add job_type display name utility function:**

- Create `getJobTypeDisplayName` function:
  - Parameters: `jobType` (string)
  - Returns: Human-readable display name
  - Mapping:
    - `pre_validation` → "Pre-Validation"
    - `crawler_url` → "URL Crawl"
    - `post_summary` → "Post-Summary"
    - `parent` → "Parent Job"
    - Default → "Unknown Type"

**3. Export utility functions to window object (after line 1070):**

- Add to global scope for use in queue.html Alpine.js components:
  - `window.getJobTypeBadgeClass = getJobTypeBadgeClass;`
  - `window.getJobTypeIcon = getJobTypeIcon;`
  - `window.getJobTypeDisplayName = getJobTypeDisplayName;`

**Rationale:** Centralizing job_type styling logic in common.js promotes code reuse and consistency across pages. These utilities can be used in other pages (jobs.html, job.html) if needed in the future.

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\models\crawler_job.go
- internal\storage\sqlite\job_storage.go

**Verification Only - No Changes Required:**

**1. Confirm GetJobHandler returns job_type field (lines 283-345):**
- Verify that `masked` job returned at line 344 includes `job_type` field from `models.CrawlJob` struct
- The `MaskSensitiveData()` method should preserve `job_type` field
- If job_type is missing in response, add explicit field mapping in `convertJobToMap` function (lines 886-899)

**2. Confirm ListJobsHandler returns job_type for child jobs (lines 59-279):**
- Verify that enriched jobs (lines 164-188) include `job_type` field
- Check that `convertJobToMap` function preserves `job_type` from `models.CrawlJob`
- Grouped response (lines 263-266) should include job_type for children

**3. Confirm DeleteJob is idempotent (lines 657-703):**
- Backend DeleteJob already has idempotency via JobStorage.DeleteJob (line 689)
- Storage layer (job_storage.go) implements idempotent delete with existence check
- No backend changes needed - frontend idempotency check is sufficient

**Expected Behavior:**
- GET /api/jobs/{id} returns job with `job_type` field
- GET /api/jobs?parent_id={id} returns children with `job_type` field
- DELETE /api/jobs/{id} returns 200 OK even if job already deleted (idempotent)

**If job_type is missing in API responses:**
- Add explicit field in `convertJobToMap` function:
  - `jobMap["job_type"] = string(job.JobType)` after line 896
- This ensures job_type is always present in JSON responses