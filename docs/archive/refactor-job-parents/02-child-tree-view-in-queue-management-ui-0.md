I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Backend (Phase 1 & 2 Complete):**
- ✅ `CrawlJob` model has `ParentID` field (line 36 in `crawler_job.go`)
- ✅ API `/api/jobs` supports `parent_id` and `grouped` query parameters
- ✅ API returns `child_count`, `completed_children`, `failed_children` for parent jobs
- ✅ `JobListOptions` includes `ParentID` and `Grouped` fields

**Frontend (Current State):**
- Jobs displayed as **flat cards** without hierarchy (lines 744-863 in `queue.html`)
- `renderJobs()` function creates simple card-based UI
- State management uses plain JavaScript variables (`allJobs`, `filteredJobs`, `selectedJobIds`)
- Alpine.js used for `queueStatsHeader` component (lines 1228-1316)
- WebSocket integration for real-time updates (`updateJobInList` function, lines 430-530)
- Filtering system with localStorage persistence (lines 282-345)
- Pagination with custom rendering (lines 923-988)
- Batch operations (select all, delete selected) implemented (lines 1319-1456)

**Key Design Decisions:**
1. **Lazy-load children**: Fetch children only when parent is expanded (reduces initial load)
2. **Parent-only default view**: Show only parent jobs by default with toggle to show all
3. **Expand/collapse state**: Store in memory (not localStorage) to avoid confusion on page reload
4. **Visual hierarchy**: Use indentation, icons, and different badge styles
5. **Aggregate progress**: Display "X/Y URLs completed" on parent cards
6. **Maintain compatibility**: Preserve existing features (filtering, pagination, batch operations, WebSocket updates)

### Approach

Implement a hierarchical tree view for the Queue Management UI by:

1. **Add UI controls** for toggling between parent-only and all-jobs views
2. **Modify data loading** to support `parent_id=root` for parent-only mode and lazy-load children on expand
3. **Refactor `renderJobs()`** to support tree structure with expand/collapse, indentation, and visual indicators
4. **Add CSS styles** for tree view hierarchy (indentation, icons, badges)
5. **Update WebSocket handler** to properly update parent/child jobs in tree view
6. **Enhance status badges** with different semantics for parent vs child jobs

The solution uses lazy-loading for children, maintains backward compatibility, and integrates seamlessly with existing filtering, pagination, and batch operations.

### Reasoning

I explored the codebase by:
1. Reading `queue.html` to understand current job rendering and state management
2. Examining `job_handler.go` to confirm API response structure with child statistics
3. Reviewing `crawler_job.go` to verify `ParentID` field implementation
4. Analyzing `common.js` and `quaero.css` to understand existing patterns
5. Studying WebSocket update logic and filter management
6. Confirming `JobListOptions` structure supports parent filtering

## Mermaid Diagram

sequenceDiagram
    participant User
    participant UI as Queue UI
    participant State as JS State
    participant API as /api/jobs
    participant WS as WebSocket

    Note over User,WS: Initial Load - Parent-Only View
    User->>UI: Load Queue Page
    UI->>State: Initialize (showChildJobs=false)
    UI->>API: GET /api/jobs?parent_id=root
    API-->>UI: Parent jobs with child_count stats
    UI->>UI: renderJobs() - Show parent cards
    Note over UI: Display: Folder icon, "45/100 URLs"

    Note over User,WS: User Expands Parent
    User->>UI: Click expand button on parent
    UI->>State: expandedParents.add(parentId)
    UI->>State: Check childJobsCache
    alt Children not cached
        UI->>API: GET /api/jobs?parent_id={parentId}
        API-->>UI: Child jobs array
        UI->>State: childJobsCache.set(parentId, children)
    end
    UI->>UI: renderJobs() - Show parent + children
    Note over UI: Children indented, file icons

    Note over User,WS: User Toggles View
    User->>UI: Click "Show All Jobs"
    UI->>State: showChildJobs=true
    UI->>State: Clear expandedParents, cache
    UI->>API: GET /api/jobs (no parent_id filter)
    API-->>UI: All jobs (parents + children)
    UI->>UI: renderJobs() - Flat list with hierarchy indicators

    Note over User,WS: Real-time Updates
    WS->>UI: job_status_change event (child job)
    UI->>State: Update allJobs array
    alt Child in cache
        UI->>State: Update childJobsCache
    end
    UI->>UI: renderJobs() - Refresh display
    Note over UI: Parent stats auto-update

    Note over User,WS: User Collapses Parent
    User->>UI: Click collapse button
    UI->>State: expandedParents.delete(parentId)
    UI->>UI: renderJobs() - Hide children
    Note over State: Cache retained for fast re-expand

## Proposed File Changes

### pages\queue.html(MODIFY)

**Add hierarchy state management variables** (after line 273):

Add new state variables:
- `showChildJobs` (boolean, default: false) - Controls whether to show all jobs or parent-only
- `expandedParents` (Set) - Tracks which parent jobs are expanded
- `childJobsCache` (Map) - Caches loaded children by parent ID to avoid re-fetching

Example:
```javascript
let showChildJobs = false; // Toggle between parent-only and all-jobs view
let expandedParents = new Set(); // Track expanded parent jobs
let childJobsCache = new Map(); // Cache: parentID -> array of child jobs
```

**Add toggle button for show/hide children** (after line 153, in filter section):

Add a toggle button next to the Filter button:
```html
<button type="button" class="btn btn-sm" onclick="toggleShowChildJobs()" id="toggle-children-btn">
    <i class="fas fa-sitemap"></i> <span id="toggle-children-text">Show All Jobs</span>
</button>
```

This button will toggle between "Show All Jobs" and "Show Parent Jobs Only".

**Modify `loadJobs()` function** (lines 378-423):

Update the API call to include `parent_id=root` when `showChildJobs` is false:

1. After line 398, add conditional parent_id parameter:
```javascript
if (!showChildJobs) {
    params.append('parent_id', 'root'); // Only fetch parent jobs
}
```

2. After receiving jobs (line 406), clear expanded state if switching views:
```javascript
if (!showChildJobs) {
    expandedParents.clear(); // Reset expanded state when switching to parent-only view
}
```

**Add `toggleShowChildJobs()` function** (after line 428):

Create function to toggle between parent-only and all-jobs views:
```javascript
function toggleShowChildJobs() {
    showChildJobs = !showChildJobs;
    const btn = document.getElementById('toggle-children-btn');
    const text = document.getElementById('toggle-children-text');
    
    if (showChildJobs) {
        text.textContent = 'Show Parent Jobs Only';
        btn.classList.add('btn-primary');
    } else {
        text.textContent = 'Show All Jobs';
        btn.classList.remove('btn-primary');
        expandedParents.clear(); // Clear expanded state
        childJobsCache.clear(); // Clear cache
    }
    
    currentPage = 1; // Reset to first page
    loadJobs();
}
```

**Add `loadChildJobs()` function** (after `toggleShowChildJobs`):

Create function to lazy-load children when parent is expanded:
```javascript
async function loadChildJobs(parentId) {
    // Check cache first
    if (childJobsCache.has(parentId)) {
        return childJobsCache.get(parentId);
    }
    
    try {
        const params = new URLSearchParams();
        params.append('parent_id', parentId);
        params.append('limit', 1000); // Load all children (reasonable limit)
        params.append('order_by', 'created_at');
        params.append('order_dir', 'DESC');
        
        const response = await fetch(`/api/jobs?${params.toString()}`);
        if (!response.ok) {
            throw new Error('Failed to fetch child jobs');
        }
        
        const data = await response.json();
        const children = data.jobs || [];
        
        // Cache the results
        childJobsCache.set(parentId, children);
        
        return children;
    } catch (error) {
        console.error('[Queue] Error loading child jobs:', error);
        return [];
    }
}
```

**Add `toggleParentExpansion()` function** (after `loadChildJobs`):

Create function to expand/collapse parent jobs:
```javascript
async function toggleParentExpansion(parentId) {
    if (expandedParents.has(parentId)) {
        // Collapse
        expandedParents.delete(parentId);
    } else {
        // Expand - load children if not cached
        expandedParents.add(parentId);
        await loadChildJobs(parentId);
    }
    
    // Re-render to show/hide children
    renderJobs();
}
```

**Completely refactor `renderJobs()` function** (lines 744-863):

Replace the entire function with a new implementation that supports tree view:

1. **Check if parent-only mode**: If `!showChildJobs`, render parent jobs with expand/collapse buttons
2. **For each parent job**:
   - Add expand/collapse icon (chevron-right when collapsed, chevron-down when expanded)
   - Display aggregate progress: "X/Y URLs completed" using `child_count`, `completed_children`
   - Add visual indicator (folder icon) for parent jobs
   - If expanded, render children with indentation
3. **For child jobs** (when expanded or in all-jobs mode):
   - Add indentation (margin-left: 2rem)
   - Add file icon to distinguish from parents
   - Show individual progress
   - Lighter background color
4. **Preserve existing features**:
   - Checkboxes for batch operations
   - Action buttons (rerun, cancel, delete)
   - JSON toggle
   - Status badges

Key changes:
- Add `data-parent-id` attribute to job cards for hierarchy tracking
- Add `job-card-parent` and `job-card-child` CSS classes
- Add expand/collapse button with onclick handler
- Conditionally render children based on `expandedParents` set
- Use `getParentStatusBadge()` and `getChildStatusBadge()` for different badge styles

**Add helper functions for tree rendering** (after `renderJobs`):

1. **`getParentStatusBadge(job)`**: Returns status badge with orchestration semantics
   - "pending" → "Queued" (yellow)
   - "running" → "Orchestrating" (blue)
   - "completed" → "Completed" (green)
   - "failed" → "Failed" (red)

2. **`getChildStatusBadge(status)`**: Returns status badge with task semantics
   - "pending" → "Pending" (gray)
   - "running" → "Processing" (blue)
   - "completed" → "Done" (green)
   - "failed" → "Failed" (red)

3. **`renderParentProgress(job)`**: Returns HTML for aggregate progress display
   - Format: "45/100 URLs completed (45%)" using `completed_children` and `child_count`
   - Show progress bar if available

4. **`renderChildProgress(job)`**: Returns HTML for individual child progress
   - Show URL being processed
   - Show completion status

**Update `updateJobInList()` WebSocket handler** (lines 430-530):

Modify to handle parent/child updates in tree view:

1. After line 456, add logic to update cached children:
```javascript
// If this is a child job and parent is expanded, update cache
if (job.parent_id && childJobsCache.has(job.parent_id)) {
    const cachedChildren = childJobsCache.get(job.parent_id);
    const childIndex = cachedChildren.findIndex(c => c.id === job.id);
    if (childIndex >= 0) {
        cachedChildren[childIndex] = job;
    } else {
        cachedChildren.unshift(job); // Add new child
    }
}
```

2. After line 493, add logic to refresh parent stats when child updates:
```javascript
// If child job updated, invalidate parent's cached stats
if (job.parent_id) {
    // Trigger re-fetch of parent job to get updated child stats
    const parentIndex = allJobs.findIndex(j => j.id === job.parent_id);
    if (parentIndex >= 0) {
        // Parent stats will be refreshed on next render
        // Could optionally fetch parent job here for immediate update
    }
}
```

**Update `matchesActiveFilters()` function** (lines 532-550):

No changes needed - filters apply to both parent and child jobs.

**Update batch operations** (lines 1319-1456):

No changes needed - checkboxes work the same for parent and child jobs. However, consider adding a note in the delete confirmation that deleting a parent will also delete its children (if backend enforces this).

**Add keyboard shortcuts** (in DOMContentLoaded, after line 1207):

Add keyboard shortcut to toggle view:
```javascript
if (e.key === 'h' && !e.ctrlKey && !e.metaKey) {
    // Toggle hierarchy view with 'h' key
    const activeElement = document.activeElement;
    if (activeElement.tagName !== 'INPUT' && activeElement.tagName !== 'TEXTAREA') {
        toggleShowChildJobs();
    }
}
```

### pages\static\quaero.css(MODIFY)

References: 

- pages\queue.html(MODIFY)

**Add tree view hierarchy styles** (after line 790, at end of file):

Add comprehensive CSS for the hierarchical tree view:

```css
/* 13. JOB QUEUE TREE VIEW HIERARCHY
   ========================================================================== */

/* Parent job card styling */
.job-card-parent {
    border-left: 3px solid var(--color-primary);
}

/* Child job card styling */
.job-card-child {
    margin-left: 2rem;
    background-color: #fafbfc;
    border-left: 3px solid var(--border-color);
}

/* Expand/collapse button */
.expand-collapse-btn {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 0.25rem 0.5rem;
    color: var(--text-secondary);
    transition: color 0.2s, transform 0.2s;
    display: inline-flex;
    align-items: center;
    justify-content: center;
}

.expand-collapse-btn:hover {
    color: var(--color-primary);
}

.expand-collapse-btn i {
    font-size: 0.875rem;
}

/* Rotate chevron when expanded */
.expand-collapse-btn.expanded i {
    transform: rotate(90deg);
}

/* Job type icons */
.job-type-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.5rem;
    height: 1.5rem;
    margin-right: 0.5rem;
    color: var(--text-secondary);
}

.job-type-icon.parent-icon {
    color: var(--color-primary);
}

.job-type-icon.child-icon {
    color: var(--text-secondary);
}

/* Aggregate progress display for parent jobs */
.parent-progress {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.5rem;
    padding: 0.5rem;
    background-color: var(--page-bg);
    border-radius: var(--border-radius);
    font-size: 0.875rem;
}

.parent-progress-text {
    color: var(--text-secondary);
}

.parent-progress-bar {
    flex: 1;
    height: 8px;
    background-color: #e0e0e0;
    border-radius: 4px;
    overflow: hidden;
}

.parent-progress-bar-fill {
    height: 100%;
    background-color: var(--color-success);
    transition: width 0.3s ease;
}

/* Status badge variants for parent jobs */
.label-orchestrating {
    background-color: #0757ba;
    color: white;
}

.label-queued {
    background-color: #ffb700;
    color: #333;
}

/* Status badge variants for child jobs */
.label-processing {
    background-color: #5755d9;
    color: white;
}

.label-done {
    background-color: #1f883d;
    color: white;
}

/* Child job metadata styling */
.child-job-url {
    font-size: 0.75rem;
    color: var(--text-secondary);
    font-family: 'SF Mono', Monaco, Consolas, monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 400px;
}

/* Hover effects for tree items */
.job-card-parent:hover,
.job-card-child:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

/* Loading indicator for expanding parents */
.loading-children {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    color: var(--text-secondary);
    font-size: 0.875rem;
}

.loading-children i {
    margin-right: 0.5rem;
}

/* Empty state for no children */
.no-children-message {
    margin-left: 2rem;
    padding: 1rem;
    color: var(--text-secondary);
    font-size: 0.875rem;
    font-style: italic;
}

/* Responsive adjustments */
@media (max-width: 768px) {
    .job-card-child {
        margin-left: 1rem;
    }
    
    .child-job-url {
        max-width: 200px;
    }
}
```

**Update existing card styles** (around line 120-146):

Add transition for smooth expand/collapse:
```css
.card {
    border-radius: var(--border-radius);
    margin-bottom: 1.5rem;
    transition: box-shadow 0.2s ease; /* Add smooth transition */
    /* ... existing styles ... */
}
```

**Add animation for expanding children** (after tree view styles):

```css
/* Slide-in animation for children */
@keyframes slideIn {
    from {
        opacity: 0;
        transform: translateY(-10px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}

.job-card-child {
    animation: slideIn 0.2s ease-out;
}
```

### pages\static\common.js(MODIFY)

**No changes required** to `common.js`.

The Alpine.js components (`queueStats`, `serviceLogs`, etc.) are independent and don't need modification for the tree view implementation. The tree view will be implemented using plain JavaScript in `queue.html` to maintain consistency with the existing job rendering approach.

However, if you want to **optionally** add a helper function for formatting job hierarchy information, you could add this after line 983:

```javascript
// Helper function to format parent job progress
window.formatParentProgress = function(completedChildren, totalChildren) {
    if (!totalChildren || totalChildren === 0) {
        return 'No child jobs';
    }
    
    const percentage = Math.round((completedChildren / totalChildren) * 100);
    return `${completedChildren}/${totalChildren} URLs completed (${percentage}%)`;
};

// Helper function to determine if a job is a parent
window.isParentJob = function(job) {
    return !job.parent_id || job.parent_id === '';
};

// Helper function to determine if a job is a child
window.isChildJob = function(job) {
    return job.parent_id && job.parent_id !== '';
};
```

These are **optional utility functions** that can be used across multiple pages if needed. The main implementation will be in `queue.html`.