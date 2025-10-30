I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Backend (Working Correctly):**
1. **Parent/Child Job Structure**: The system creates ONE parent job (with empty `parent_id`) and multiple child jobs (with `parent_id` set to parent's ID)
2. **Child Statistics**: `JobStorage.GetJobChildStats` correctly aggregates child counts by status (completed, failed)
3. **API Enrichment**: `JobHandler.ListJobsHandler` enriches each parent job with `child_count`, `completed_children`, `failed_children`
4. **ResultCount Field**: The `result_count` field is properly synced from `progress.completed_urls` when jobs reach terminal status (completed/failed/cancelled)
5. **WebSocket Events**: Job lifecycle events (created, started, completed, failed, cancelled) include `result_count` and `failed_count` fields

**Frontend Issues Identified:**

1. **Document Count Bug**: `getDocumentsCount()` method (line 1327) reads `job.progress.completed_urls` instead of `job.result_count`
   - For completed jobs, `result_count` is the authoritative snapshot
   - For running jobs, `progress.completed_urls` is updated in real-time
   - The method should use `result_count` if available, fallback to `progress.completed_urls`

2. **Parent Accordion CSS**: The chevron rotation CSS exists (lines 829-832 in quaero.css) and the `toggleParentExpansion()` method works correctly
   - The issue is likely that the `:class` binding on line 202 uses `expandedParents.has(item.job.id)` which should work
   - Need to verify the Alpine.js reactive binding is triggering

3. **Child Count Display**: The `getParentProgressText()` method (line 1300) correctly uses `job.child_count` from API enrichment
   - Should display actual count when > 0
   - The text "No child jobs" appears when `child_count` is 0 or undefined

4. **Parent Status Logic**: Status badges (lines 1345-1399) only show the parent's own `status` field
   - They don't derive status from child statistics
   - Need to implement logic: if all children completed → show "Completed", if any running → show "Orchestrating", if failures → show "Running (X failed)"

5. **Job Hierarchy**: When executing a Jira job definition:
   - Job definition creates ONE parent job (type: `job_definition`, entity: `job_definition`)
   - The crawl action within that job creates ANOTHER parent job (type: `jira`, entity: varies)
   - The crawler parent spawns child jobs (type: `crawler_url`)
   - This creates a two-level hierarchy that may confuse users

**Key Findings:**
- The backend data is correct and complete
- The UI needs fixes to properly display the data
- WebSocket updates already include `result_count` for real-time updates
- The parent/child hierarchy is working as designed, but the UI needs better status derivation logic

### Approach

## Solution Strategy

**Fix 1: Document Count Display**
Update `getDocumentsCount()` to prioritize `result_count` over `progress.completed_urls`, with proper fallback logic for running vs completed jobs.

**Fix 2: Parent Status Derivation**
Enhance `getStatusBadgeClass()` and `getStatusBadgeText()` to derive parent job status from child statistics when available, showing aggregate status like "Orchestrating (2 failed)" or "Completed".

**Fix 3: Child Count Display**
Ensure `getParentProgressText()` properly displays child counts with failure information when applicable.

**Fix 4: Parent Accordion**
Verify and fix the Alpine.js reactive binding for the chevron rotation class.

**Fix 5: Job Hierarchy Clarification**
Add visual indicators to distinguish job definition parents from crawler parents, and ensure the UI clearly shows the hierarchy levels.

**Approach:**
1. Update Alpine.js component methods in `pages/queue.html` to fix document count and status logic
2. Enhance status badge methods to derive parent status from child statistics
3. Add helper methods for status aggregation logic
4. Ensure WebSocket updates properly refresh parent statistics
5. Add CSS classes or visual indicators for different job types

### Reasoning

I explored the codebase by reading the queue.html template, job handler, job storage, crawler service, and models. I examined how parent/child jobs are created, how child statistics are aggregated, how the API enriches job data, and how the UI displays jobs. I also checked the CSS for chevron rotation and WebSocket event handling for real-time updates. I verified that `result_count` is properly synced and included in WebSocket events.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant UI as Queue UI (Alpine.js)
    participant API as Job Handler
    participant Storage as Job Storage
    participant WS as WebSocket
    
    Note over User,WS: Initial Page Load
    User->>UI: Open Queue Management
    UI->>API: GET /api/jobs?parent_id=root
    API->>Storage: ListJobs(parent_id=root)
    Storage-->>API: Parent jobs only
    API->>Storage: GetJobChildStats(parentIDs)
    Storage-->>API: Child statistics
    API-->>UI: Jobs with child_count, completed_children, failed_children
    UI->>UI: renderJobs() - Display parent jobs
    
    Note over User,WS: Expand Parent Job
    User->>UI: Click chevron to expand
    UI->>UI: toggleParentExpansion(parentId)
    UI->>UI: Add to expandedParents Set
    UI->>API: GET /api/jobs?parent_id={parentId}
    API->>Storage: ListJobs(parent_id=parentId)
    Storage-->>API: Child jobs
    API-->>UI: Child jobs array
    UI->>UI: Cache children in childJobsCache
    UI->>UI: renderJobs() - Show children
    UI->>UI: Rotate chevron 90° (CSS transition)
    
    Note over User,WS: Real-time Updates via WebSocket
    WS->>UI: EventJobCompleted (child job)
    UI->>UI: updateJobInList(update)
    UI->>UI: Update child job in cache
    UI->>UI: debouncedRefreshParent(parentId)
    UI->>API: GET /api/jobs/{parentId}
    API->>Storage: GetJob(parentId)
    Storage-->>API: Parent job
    API->>Storage: GetJobChildStats([parentId])
    Storage-->>API: Updated child statistics
    API-->>UI: Parent with updated stats
    UI->>UI: Update parent in allJobs
    UI->>UI: deriveParentStatus(job)
    UI->>UI: renderJobs() - Update UI
    
    Note over User,WS: Status Badge Logic
    UI->>UI: getStatusBadgeClass(type, status, job)
    UI->>UI: deriveParentStatus(job)
    alt All children completed
        UI->>UI: Return {status: 'completed', suffix: ''}
    else Some children failed
        UI->>UI: Return {status: 'running', suffix: ' (X failed)'}
    else Children still running
        UI->>UI: Return {status: 'running', suffix: ''}
    end
    UI->>UI: Apply CSS class based on derived status
    
    Note over User,WS: Document Count Display
    UI->>UI: getDocumentsCount(job)
    alt Job is completed/failed/cancelled
        UI->>UI: Return job.result_count
    else Job is running/pending
        UI->>UI: Return job.progress.completed_urls
    else Neither available
        UI->>UI: Return 'N/A'
    end

## Proposed File Changes

### pages\queue.html(MODIFY)

References: 

- internal\handlers\job_handler.go
- internal\storage\sqlite\job_storage.go
- internal\jobs\types\crawler.go
- internal\services\crawler\service.go
- internal\models\crawler_job.go
- pages\static\quaero.css(MODIFY)

## Fix 1: Update getDocumentsCount() Method (Line 1327)

**Current Implementation:**
```javascript
getDocumentsCount(job) {
    if (!job.progress) return 'N/A';
    try {
        const progress = typeof job.progress === 'string' ? JSON.parse(job.progress) : job.progress;
        if (progress && progress.completed_urls !== undefined) {
            return progress.completed_urls;
        }
    } catch (error) {
        console.warn('[Queue] Failed to parse job progress:', error);
    }
    return 'N/A';
}
```

**New Implementation:**
Replace the method to prioritize `result_count` field:
- For completed/failed/cancelled jobs: Use `job.result_count` (authoritative snapshot)
- For running/pending jobs: Use `job.progress.completed_urls` (real-time counter)
- Fallback to 'N/A' if neither is available

The logic should check `job.status` to determine which field to use:
- Terminal statuses (completed, failed, cancelled) → `result_count`
- Active statuses (pending, running) → `progress.completed_urls`
- Always fallback to the other field if primary is unavailable

**Rationale:** The `result_count` field is synced from `progress.completed_urls` when jobs reach terminal status (see `internal/jobs/types/crawler.go` line 694 and `internal/services/crawler/service.go` lines 660, 733). For completed jobs, `result_count` is the authoritative value. For running jobs, `progress.completed_urls` provides real-time updates.

---

## Fix 2: Enhance getParentProgressText() Method (Line 1300)

**Current Implementation:**
```javascript
getParentProgressText(job) {
    const completed = job.completed_children || 0;
    const total = job.child_count || 0;
    const percentage = total > 0 ? Math.round((completed / total) * 100) : 0;
    
    if (total > 0) {
        return `${completed}/${total} URLs completed (${percentage}%)`;
    } else {
        return `No child jobs`;
    }
}
```

**Enhancement:**
Add failure count information to the progress text:
- When `job.failed_children > 0`, append failure count: `"5/10 URLs completed (50%) - 2 failed"`
- When `total === 0`, show `"No child jobs"`
- When loading children, show `"Loading children..."`

Check if `loadingParents.has(job.id)` to show loading state.

**Rationale:** Users need to see failure information at a glance without expanding the parent job. The `failed_children` count is already provided by the API enrichment (see `internal/handlers/job_handler.go` lines 173-181).

---

## Fix 3: Implement Parent Status Derivation Logic

**Add New Helper Method:**
Create a new method `deriveParentStatus(job)` that returns an object with derived status information:
```javascript
deriveParentStatus(job) {
    // If not a parent job or no child stats, return original status
    if (!job.child_count || job.child_count === 0) {
        return { status: job.status, suffix: '' };
    }
    
    const total = job.child_count;
    const completed = job.completed_children || 0;
    const failed = job.failed_children || 0;
    
    // All children completed
    if (completed === total) {
        return { status: 'completed', suffix: '' };
    }
    
    // Some children failed but job is still running
    if (failed > 0 && job.status === 'running') {
        return { status: 'running', suffix: ` (${failed} failed)` };
    }
    
    // Job is running with children in progress
    if (job.status === 'running') {
        return { status: 'running', suffix: '' };
    }
    
    // Default to job's own status
    return { status: job.status, suffix: '' };
}
```

**Rationale:** Parent jobs should reflect the aggregate status of their children. When all children complete, the parent should show "Completed". When children are running, show "Orchestrating". When failures occur, show "Running (X failed)" to alert users.

---

## Fix 4: Update getStatusBadgeClass() Method (Line 1345)

**Current Implementation:**
The method returns CSS classes based on `job.status` only.

**Enhancement:**
For parent jobs (when `type === 'parent'` or `!job.parent_id`), call `deriveParentStatus(job)` first, then use the derived status for badge class selection:

```javascript
getStatusBadgeClass(type, status, job) {
    // For parent jobs, derive status from children
    if ((type === 'parent' || (type === 'flat' && !job.parent_id)) && job.child_count > 0) {
        const derived = this.deriveParentStatus(job);
        status = derived.status;
    }
    
    // Rest of the existing logic...
}
```

**Rationale:** Parent job badges should reflect the aggregate status of children, not just the parent's own status field. This provides accurate visual feedback to users.

---

## Fix 5: Update getStatusBadgeText() Method (Line 1389)

**Current Implementation:**
The method returns status text based on `job.status` only.

**Enhancement:**
For parent jobs, call `deriveParentStatus(job)` and append the suffix:

```javascript
getStatusBadgeText(type, status, job) {
    // For parent jobs, derive status from children
    let suffix = '';
    if ((type === 'parent' || (type === 'flat' && !job.parent_id)) && job.child_count > 0) {
        const derived = this.deriveParentStatus(job);
        status = derived.status;
        suffix = derived.suffix;
    }
    
    const statusTexts = {
        'pending': type === 'child' ? 'Pending' : type === 'parent' ? 'Queued' : (!job || !job.parent_id ? 'Queued' : 'Pending'),
        'running': type === 'child' ? 'Processing' : type === 'parent' ? 'Orchestrating' : (!job || !job.parent_id ? 'Orchestrating' : 'Processing'),
        'completed': type === 'child' ? 'Done' : 'Completed',
        'failed': 'Failed',
        'cancelled': 'Cancelled'
    };
    return (statusTexts[status] || 'Unknown') + suffix;
}
```

**Rationale:** Status text should include failure counts when applicable, e.g., "Orchestrating (2 failed)" to provide immediate visibility into job health.

---

## Fix 6: Verify Parent Accordion Chevron Rotation

**Current Implementation:**
The HTML template (line 201-204) has:
```html
<button class="expand-collapse-btn" @click.stop="toggleParentExpansion(item.job.id)"
        :class="expandedParents.has(item.job.id) ? 'expanded' : ''">
    <i class="fas fa-chevron-right"></i>
</button>
```

The CSS (lines 829-832 in `quaero.css`) has:
```css
.expand-collapse-btn.expanded i {
    transform: rotate(90deg);
}
```

**Verification:**
The implementation looks correct. The issue might be:
1. Alpine.js reactivity not triggering on Set changes
2. CSS specificity issues

**Fix:**
Ensure the Alpine.js binding uses a computed property or force re-render after Set changes. In the `toggleParentExpansion()` method (line 1233), after modifying `expandedParents`, call `this.renderJobs()` to force a re-render (already done on line 1249).

If the issue persists, add a transition for smooth rotation:
```css
.expand-collapse-btn i {
    transition: transform 0.2s ease;
}
```

**Rationale:** The chevron should rotate 90 degrees when expanded to provide visual feedback. The current implementation should work, but adding a transition improves UX.

---

## Fix 7: Add Visual Indicators for Job Types

**Enhancement:**
In the job card title section (line 199-211), add a visual indicator for job definition parents:

```html
<template x-if="item.type === 'parent' && item.job.entity_type === 'job_definition'">
    <span class="label label-info" style="margin-left: 0.5rem; font-size: 0.7rem;">JOB DEFINITION</span>
</template>
```

This helps users distinguish between:
- Job definition parents (orchestration layer)
- Crawler parents (actual crawl jobs)
- Child jobs (individual URL processing)

**Rationale:** The two-level hierarchy (job definition → crawler parent → crawler children) can be confusing. Visual indicators help users understand the job structure.

---

## Fix 8: Update WebSocket Handler Integration

**Current Implementation:**
The `updateJobInList()` method (line 1545) updates job fields from WebSocket events.

**Verification:**
Ensure the method updates `result_count` field:
```javascript
if (update.result_count !== undefined && update.result_count !== null) {
    job.result_count = update.result_count;
}
```

This is already implemented (lines 1575-1577), so WebSocket updates should work correctly.

**Enhancement:**
When a child job updates, the parent refresh is debounced (line 1650). This is correct and should update parent statistics including `child_count`, `completed_children`, and `failed_children`.

**Rationale:** Real-time updates ensure the UI reflects current job status without manual refresh. The debouncing prevents excessive API calls during high child job churn.

---

## Testing Checklist

After implementing changes, verify:
1. ✅ Document counts display correctly for both running and completed jobs
2. ✅ Parent status badges reflect aggregate child status
3. ✅ Parent progress text shows child counts with failure information
4. ✅ Chevron icon rotates when parent is expanded
5. ✅ Child jobs load and display when parent is expanded
6. ✅ WebSocket updates refresh document counts and parent statistics in real-time
7. ✅ Job definition parents are visually distinguished from crawler parents
8. ✅ Status badges show "Orchestrating (X failed)" when applicable
9. ✅ Completed parent jobs show "Completed" when all children finish
10. ✅ No console errors for undefined Alpine.js variables

### pages\static\quaero.css(MODIFY)

## Add Smooth Transition for Chevron Rotation

**Current Implementation (Line 825-827):**
```css
.expand-collapse-btn i {
    font-size: 0.875rem;
}
```

**Enhancement:**
Add a smooth transition for the chevron rotation:
```css
.expand-collapse-btn i {
    font-size: 0.875rem;
    transition: transform 0.2s ease;
}
```

**Rationale:** The transition provides smooth visual feedback when expanding/collapsing parent jobs, improving user experience. The 0.2s duration matches the card transition duration (line 124) for consistency.

---

## Add Job Type Badge Styles (Optional Enhancement)

If adding visual indicators for job definition parents, add these styles:

```css
/* Job type badges */
.job-type-badge {
    font-size: 0.7rem;
    padding: 0.2rem 0.4rem;
    margin-left: 0.5rem;
    vertical-align: middle;
}
```

**Rationale:** Consistent styling for job type indicators helps users quickly identify different job types in the queue.