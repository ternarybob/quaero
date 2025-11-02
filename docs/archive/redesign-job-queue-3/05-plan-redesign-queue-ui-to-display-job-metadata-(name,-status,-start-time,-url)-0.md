I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Existing Job Card Structure (lines 143-361 in queue.html):**
- **Card Title (lines 162-180)**: Shows "Job ID: [8-char truncated ID]" with expand/collapse button for parents, folder/file icon, and "PARENT" badge
- **Card Subtitle (lines 181-183)**: Shows source type (Jira/Confluence/GitHub)
- **Metadata Section (lines 186-220)**: Status badge, job type badge, documents count, created date, configuration toggle
- **Failure Reason (lines 223-228)**: Error message display for failed jobs
- **Parent Progress (lines 231-329)**: Progress text, errors, warnings, and child jobs tree

**Missing Elements:**
1. **Job Name**: Available in `item.job.name` but not displayed (only Job ID shown)
2. **Start Time**: Available in `item.job.started_at` but not displayed (only created_at shown)
3. **URL for Crawler Jobs**: Available in `item.job.config.seed_urls[0]` or `item.job.progress.current_url` but not displayed in parent cards
4. **Status Icons**: Exist in child tree (line 294-299) but not in main job cards
5. **Prominent Visual Hierarchy**: Parent jobs need more prominent styling to stand out from children

**Available Data in Job Object:**
- `item.job.name` - User-friendly job name
- `item.job.started_at` - Job start timestamp (ISO 8601 format)
- `item.job.created_at` - Job creation timestamp
- `item.job.status` - Job status (pending, running, completed, failed, cancelled)
- `item.job.job_type` - Job type (parent, crawler_url, pre_validation, post_summary)
- `item.job.config.seed_urls` - Array of seed URLs for crawler jobs
- `item.job.progress.current_url` - Currently processing URL
- `item.job.source_type` - Source type (jira, confluence, github)

**Available Helper Functions:**
- `getStatusIcon(status)` - Returns Font Awesome icon class (lines 1679-1688)
- `getStatusDisplayText(status)` - Returns human-readable status text (lines 1690-1699)
- `window.getJobTypeIcon(jobType)` - Returns Font Awesome icon class (common.js line 251-259)
- `window.getJobTypeBadgeClass(jobType)` - Returns Bulma label class (common.js line 241-249)
- `getStatusBadgeClass(type, status, job)` - Returns status badge class (lines 1901-1916)

## Design Decisions

**1. Job Name Display Strategy**
- **Decision**: Display job name prominently in card title, with Job ID as secondary information
- **Rationale**: Job names are user-friendly and descriptive; Job IDs are technical identifiers
- **Implementation**: Change card title from "Job ID: [id]" to "[name]" with Job ID shown in smaller text or tooltip
- **Fallback**: If `item.job.name` is empty, fall back to "Job [truncated-id]" (current behavior)

**2. Start Time Display Strategy**
- **Decision**: Add start time to metadata section alongside created date
- **Rationale**: Users need to know when a job actually started execution, not just when it was created
- **Implementation**: Add new metadata item with clock icon showing "Started: [formatted-time]"
- **Handling**: Only show if `item.job.started_at` is not empty (pending jobs won't have start time)

**3. URL Display for Crawler Jobs**
- **Decision**: Display URL prominently below job name for crawler jobs
- **Rationale**: The URL being crawled is the most important context for crawler jobs
- **Implementation**: Add URL display in card subtitle area (below job name, above metadata)
- **Source Priority**: Use `item.job.config.seed_urls[0]` (primary seed URL) or fall back to `item.job.progress.current_url` (currently processing URL)
- **Truncation**: Truncate long URLs with ellipsis and show full URL in title attribute (tooltip)

**4. Status Icon Integration**
- **Decision**: Add status icon next to status badge in main job cards (matching child tree pattern)
- **Rationale**: Icons provide quick visual recognition of job status without reading text
- **Implementation**: Use existing `getStatusIcon(status)` function to add icon before status badge text
- **Color Coding**: Apply status-specific colors to icons (pending=yellow, running=blue, completed=green, failed=red, cancelled=gray)

**5. Visual Hierarchy Enhancement**
- **Decision**: Make parent jobs visually prominent with larger cards, bolder text, and distinct background
- **Rationale**: Parent jobs are orchestrators and should stand out from child jobs in the tree
- **Implementation**: 
  - Add CSS class `.job-card-parent` with enhanced styling (already exists, needs enhancement)
  - Increase font size for parent job names
  - Add subtle background color or border to distinguish parents
  - Ensure child jobs in tree remain visually subordinate (already indented)

**6. Job Type Badge Prominence**
- **Decision**: Keep existing job type badge but make it more prominent with icons
- **Rationale**: Job type is important context but shouldn't overwhelm other information
- **Implementation**: Badge already includes icon (line 199), ensure it's visible and well-styled
- **No Changes Needed**: Current implementation is adequate

**7. Color Coding Enhancement**
- **Decision**: Apply consistent color coding across status badges, icons, and card borders
- **Rationale**: Color provides instant visual feedback on job status
- **Color Scheme**:
  - Pending: Yellow/Orange (`#f59e0b` or Bulma `label-warning`)
  - Running: Blue (`#3b82f6` or Bulma `label-primary`)
  - Completed: Green (`#10b981` or Bulma `label-success`)
  - Failed: Red (`#ef4444` or Bulma `label-error`)
  - Cancelled: Gray (`#6b7280` or Bulma `label`)
- **Implementation**: Enhance existing `getStatusBadgeClass()` function usage and add CSS for status-specific styling

## Implementation Strategy

**Phase 1: Enhance Card Title with Job Name**
- Modify card title section (lines 162-180)
- Display `item.job.name` as primary text (larger, bold)
- Show Job ID as secondary text (smaller, gray) or in tooltip
- Maintain expand/collapse button and icon positioning
- Add fallback logic for empty job names

**Phase 2: Add URL Display for Crawler Jobs**
- Add new section below card title (after line 183)
- Conditionally display URL for crawler jobs (check `item.job.job_type === 'crawler_url'` or `item.job.config.seed_urls`)
- Extract URL from `item.job.config.seed_urls[0]` or `item.job.progress.current_url`
- Truncate long URLs with CSS `text-overflow: ellipsis`
- Add link icon and make URL clickable (open in new tab)

**Phase 3: Add Start Time to Metadata Section**
- Add new metadata item in metadata section (after line 213)
- Conditionally display if `item.job.started_at` is not empty
- Format timestamp using JavaScript `Date` object: `new Date(item.job.started_at).toLocaleString()`
- Add clock icon (`fa-clock`) for visual consistency
- Label as "Started:" to distinguish from "Created:"

**Phase 4: Integrate Status Icons**
- Modify status badge section (lines 188-192)
- Add status icon before status badge text using `getStatusIcon(item.job.status)`
- Apply status-specific color classes to icon
- Ensure icon and text are vertically aligned

**Phase 5: Enhance Visual Hierarchy with CSS**
- Add CSS rules for `.job-card-parent` class (in `<style>` section at top of file)
- Increase font size for parent job names (1.2rem vs 1rem for children)
- Add subtle background color or left border to parent cards
- Ensure adequate spacing between parent and child cards
- Maintain existing indentation for child jobs in tree

**Phase 6: Improve Color Coding**
- Enhance status badge classes with more prominent colors
- Add CSS for status-specific icon colors (`.status-pending`, `.status-running`, etc.)
- Ensure color contrast meets accessibility standards (WCAG AA)
- Test color visibility in both light and dark themes (if applicable)

## Edge Cases and Error Handling

**Empty Job Name:**
- Fallback to "Job [truncated-id]" (current behavior)
- Ensure Job ID is always visible as fallback

**Missing Start Time:**
- Only display start time metadata if `item.job.started_at` is not empty
- Pending jobs won't have start time - this is expected behavior

**Missing URL for Crawler Jobs:**
- Check both `item.job.config.seed_urls[0]` and `item.job.progress.current_url`
- If both are empty, don't display URL section
- This can happen for parent jobs that haven't spawned children yet

**Long URLs:**
- Truncate with CSS `text-overflow: ellipsis` and `overflow: hidden`
- Set `max-width` to prevent card expansion
- Show full URL in `title` attribute (tooltip on hover)

**Status Derivation for Parent Jobs:**
- Existing `deriveParentStatus()` function (lines 1872-1899) handles parent status calculation
- Ensure status icon reflects derived status, not raw status
- This is already handled by `getStatusBadgeClass()` function

**Job Type Variations:**
- Not all jobs have `job_type` field (legacy jobs)
- Existing template already handles this with `x-if` conditional (line 194)
- No changes needed for job type handling

## Testing Considerations

**Visual Regression Testing:**
- Verify parent job cards are visually distinct from child jobs
- Ensure job names are readable and not truncated unnecessarily
- Check URL display for various URL lengths (short, medium, long)
- Verify status icons are properly aligned with status badges

**Data Availability Testing:**
- Test with jobs that have empty `name` field (fallback to Job ID)
- Test with jobs that have no `started_at` (pending jobs)
- Test with crawler jobs that have no URL yet (parent jobs)
- Test with non-crawler jobs (no URL display)

**Responsive Design Testing:**
- Verify layout works on mobile devices (cards stack properly)
- Ensure long job names wrap or truncate appropriately
- Check that metadata items wrap to new lines on narrow screens

**Accessibility Testing:**
- Verify status icons have proper `title` attributes (tooltips)
- Ensure color coding is supplemented with text/icons (not color-only)
- Check keyboard navigation still works with enhanced cards
- Verify screen reader announcements include job name and status

## No Backend Changes Required

All necessary data is already available in the job object returned by the API:
- `ListJobsHandler` (job_handler.go lines 57-292) returns complete job objects with all fields
- `CrawlJob` model (crawler_job.go lines 44-82) includes all needed fields
- No new API endpoints or database queries needed
- No changes to job processing logic required

This is purely a frontend enhancement task focused on improving the UI presentation of existing data.

### Approach

Enhance the Queue UI job cards to display comprehensive job metadata (name, status, start time, URL) with improved visual hierarchy through status icons, color coding, and prominent parent job styling. All changes are confined to the `queue.html` template and CSS - no backend modifications required since all data is already available in the job object.

### Reasoning

I explored the codebase structure, read the queue.html template (1916 lines with Alpine.js components), examined the CrawlJob model structure (lines 44-82 in crawler_job.go showing all available fields), reviewed the job_handler.go to understand the API response format, analyzed existing helper functions in common.js (getJobTypeIcon, getJobTypeBadgeClass, getStatusIcon), and studied the current job card rendering logic (lines 143-361 in queue.html) to understand the template structure and available data.

## Proposed File Changes

### pages\queue.html(MODIFY)

References: 

- internal\models\crawler_job.go

**Enhance Card Title to Display Job Name (lines 162-180):**

**Current Structure:**
- Card title shows "Job ID: [8-char truncated ID]"
- Expand/collapse button for parent jobs
- Folder/file icon based on job type
- "PARENT" badge for parent jobs

**Required Changes:**

1. **Replace Job ID with Job Name as Primary Text:**
   - Change `<span x-text="item.job.id ? item.job.id.substring(0, 8) : 'N/A'"></span>` to display job name
   - Use `x-text="item.job.name || ('Job ' + (item.job.id ? item.job.id.substring(0, 8) : 'N/A'))"`
   - This shows job name if available, otherwise falls back to "Job [id]"
   - Remove "Job ID:" label prefix

2. **Add Job ID as Secondary Information:**
   - Add Job ID in smaller, gray text after job name
   - Use `<span class="text-gray" style="font-size: 0.8rem; font-weight: normal;" x-text="'(' + (item.job.id ? item.job.id.substring(0, 8) : 'N/A') + ')'"></span>`
   - Place this immediately after the job name span

3. **Increase Font Size for Parent Jobs:**
   - Add conditional styling: `:style="item.type === 'parent' ? 'font-size: 1.2rem; font-weight: 600;' : ''"`
   - This makes parent job names more prominent

4. **Maintain Existing Elements:**
   - Keep expand/collapse button (lines 164-167)
   - Keep folder/file icon (lines 169-172)
   - Keep "PARENT" badge (lines 174-176)
   - Keep "JOB DEFINITION" badge (lines 177-179)

**Example Structure:**
```
[Expand Button] [Icon] Crawl Jira Issues (abc12345) [PARENT Badge]
```

**Accessibility:**
- Add `title` attribute to job name showing full Job ID: `:title="'Job ID: ' + item.job.id"`
- This provides full ID on hover without cluttering the UI
**Add URL Display for Crawler Jobs (after line 183):**

**Insert New Section Between Card Subtitle and Metadata:**
- Add after the source type display (line 183)
- Add before the metadata section (line 186)

**Implementation:**

1. **Add Conditional URL Display:**
   - Use `<template x-if="item.job.config && item.job.config.seed_urls && item.job.config.seed_urls.length > 0">` to check if URL exists
   - Alternative check: `<template x-if="item.job.progress && item.job.progress.current_url">` for running jobs
   - Combine both checks with OR logic

2. **Extract URL with Priority:**
   - Primary source: `item.job.config.seed_urls[0]` (seed URL from configuration)
   - Fallback: `item.job.progress.current_url` (currently processing URL)
   - Use Alpine.js expression: `x-text="(item.job.config?.seed_urls?.[0] || item.job.progress?.current_url || '')"`

3. **Format URL Display:**
   - Add container div with styling: `<div style="margin-top: 0.5rem; display: flex; align-items: center; gap: 0.5rem; font-size: 0.9rem; color: #555;">`
   - Add link icon: `<i class="fas fa-link" style="color: #3b82f6;"></i>`
   - Add URL text with truncation: `<span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 600px;" :title="(item.job.config?.seed_urls?.[0] || item.job.progress?.current_url || '')" x-text="(item.job.config?.seed_urls?.[0] || item.job.progress?.current_url || '')"></span>`

4. **Make URL Clickable (Optional):**
   - Wrap URL in anchor tag: `<a :href="(item.job.config?.seed_urls?.[0] || item.job.progress?.current_url || '')" target="_blank" rel="noopener noreferrer" class="text-primary" style="text-decoration: none;">`
   - Add external link icon: `<i class="fas fa-external-link-alt" style="font-size: 0.7rem; margin-left: 0.25rem;"></i>`
   - This allows users to open the URL in a new tab

5. **Handle Multiple Seed URLs:**
   - If `item.job.config.seed_urls.length > 1`, show count: `<span class="text-gray" style="font-size: 0.8rem; margin-left: 0.5rem;" x-text="'+' + (item.job.config.seed_urls.length - 1) + ' more'"></span>`
   - This indicates there are additional URLs without cluttering the display

**Example Display:**
```
🔗 https://example.atlassian.net/browse/PROJ-123 +5 more
```

**Edge Cases:**
- If no URL available, the entire section is hidden (handled by `x-if` conditional)
- If URL is very long, it truncates with ellipsis and shows full URL on hover
- If job is not a crawler job, this section won't display (no seed_urls in config)
**Add Start Time to Metadata Section (after line 213):**

**Insert New Metadata Item After Created Date:**
- Add after the "Created Date" metadata item (lines 210-213)
- Add before the "Toggle JSON" link (lines 215-219)

**Implementation:**

1. **Add Conditional Start Time Display:**
   - Use `<template x-if="item.job.started_at">` to only show if start time exists
   - Pending jobs won't have `started_at`, so this section will be hidden

2. **Create Metadata Item Structure:**
   - Follow existing pattern from lines 210-213
   - Add container div: `<div>`
   - Add clock icon: `<i class="fas fa-clock"></i>`
   - Add formatted timestamp

3. **Format Start Time:**
   - Use JavaScript Date formatting: `x-text="'Started: ' + (item.job.started_at ? new Date(item.job.started_at).toLocaleString() : 'N/A')"`
   - This formats the ISO 8601 timestamp to local date/time format
   - Example output: "Started: 1/15/2024, 2:30:45 PM"

4. **Add Helper Function for Time Formatting (Optional):**
   - Add to Alpine.js component (around line 1868): `getStartedDate(job) { return job.started_at ? new Date(job.started_at).toLocaleString() : null; }`
   - Use in template: `x-text="'Started: ' + getStartedDate(item.job)"`
   - This centralizes date formatting logic

5. **Distinguish from Created Date:**
   - Created date shows when job was queued
   - Started date shows when job began execution
   - Both are useful for understanding job lifecycle
   - Keep both metadata items visible

**Example Metadata Section:**
```
[Status Badge] [Job Type Badge] [📄 5 Documents] [📅 Created: 1/15/2024, 2:25:00 PM] [🕐 Started: 1/15/2024, 2:30:45 PM] [</> Show Configuration]
```

**Edge Cases:**
- If `started_at` is empty (pending jobs), the entire metadata item is hidden
- If `started_at` is invalid date string, show "N/A" as fallback
- Running, completed, failed, and cancelled jobs should all have `started_at` populated
**Integrate Status Icons in Main Job Cards (lines 188-192):**

**Current Structure:**
- Status badge shows text only: `<span class="label" :class="getStatusBadgeClass(...)" x-text="getStatusBadgeText(...)"></span>`
- No icon displayed in main job cards (icons only in child tree)

**Required Changes:**

1. **Add Status Icon Before Badge Text:**
   - Insert icon element inside the status badge span
   - Use existing `getStatusIcon(status)` function (lines 1679-1688)
   - Structure: `<span class="label" :class="getStatusBadgeClass(...)"><i class="fas" :class="getStatusIcon(item.job.status)"></i> <span x-text="getStatusBadgeText(...)"></span></span>`

2. **Apply Status-Specific Colors to Icons:**
   - Add CSS classes for status-specific icon colors
   - Use `:class` binding to apply color based on status
   - Example: `:class="['fas', getStatusIcon(item.job.status), 'status-icon-' + item.job.status]"`

3. **Handle Derived Status for Parent Jobs:**
   - Parent jobs use derived status from `deriveParentStatus()` function (lines 1872-1899)
   - Ensure icon reflects derived status, not raw status
   - The `getStatusBadgeClass()` function already handles this (lines 1901-1916)
   - Use same status value for icon: `getStatusIcon(item.type === 'parent' && item.job.child_count > 0 ? deriveParentStatus(item.job).status : item.job.status)`

4. **Add Icon Styling:**
   - Add margin-right to icon for spacing: `style="margin-right: 0.25rem;"`
   - Ensure icon and text are vertically aligned
   - Icon should inherit color from badge class (Bulma label classes)

5. **Maintain Existing Badge Behavior:**
   - Keep existing `getStatusBadgeClass()` function call for badge styling
   - Keep existing `getStatusBadgeText()` function call for badge text
   - Only add icon as visual enhancement

**Example Status Badge with Icon:**
```
[⏳ Pending] [🔄 Running] [✅ Completed] [❌ Failed] [🚫 Cancelled]
```

**Icon Mapping (from getStatusIcon function):**
- Pending: `fa-circle` (static circle)
- Running: `fa-spinner fa-pulse` (animated spinner)
- Completed: `fa-check-circle` (check mark)
- Failed: `fa-times-circle` (X mark)
- Cancelled: `fa-ban` (ban/stop icon)

**Accessibility:**
- Icons are decorative (text is still present)
- No need for `aria-label` on icons
- Screen readers will read the badge text
**Enhance Visual Hierarchy with CSS (in <style> section, lines 21-99):**

**Current CSS:**
- Basic styling for job cards exists
- `.job-card-parent`, `.job-card-child`, `.job-card-flat` classes defined but minimal styling

**Required CSS Enhancements:**

1. **Enhance Parent Job Card Styling:**
   - Add to `.job-card-parent` class:
     - `border-left: 4px solid #3b82f6;` (blue left border for prominence)
     - `background-color: #f8fafc;` (subtle background color)
     - `box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);` (subtle shadow for depth)
     - `margin-bottom: 1.2rem;` (increased spacing between parent cards)
   - This makes parent jobs visually distinct from child jobs

2. **Add Status-Specific Icon Colors:**
   - Add new CSS classes for status icon colors:
     - `.status-icon-pending { color: #f59e0b; }` (yellow/orange)
     - `.status-icon-running { color: #3b82f6; }` (blue)
     - `.status-icon-completed { color: #10b981; }` (green)
     - `.status-icon-failed { color: #ef4444; }` (red)
     - `.status-icon-cancelled { color: #6b7280; }` (gray)
   - These colors match Bulma label classes but provide more prominent icon coloring

3. **Enhance Job Name Typography:**
   - Add CSS for parent job names:
     - `.job-card-parent .card-title { font-size: 1.2rem; font-weight: 600; }` (larger, bolder)
     - `.job-card-parent .card-title { color: #1e293b; }` (darker text for better contrast)
   - This makes parent job names stand out

4. **Improve Status Badge Visibility:**
   - Enhance existing Bulma label classes with more prominent colors:
     - `.label-warning { background-color: #fef3c7; color: #92400e; border: 1px solid #fbbf24; }` (pending)
     - `.label-primary { background-color: #dbeafe; color: #1e40af; border: 1px solid #3b82f6; }` (running)
     - `.label-success { background-color: #d1fae5; color: #065f46; border: 1px solid #10b981; }` (completed)
     - `.label-error { background-color: #fee2e2; color: #991b1b; border: 1px solid #ef4444; }` (failed)
   - These provide better color contrast and visibility

5. **Add Hover Effects for Job Cards:**
   - Add hover state for better interactivity:
     - `.job-card-parent:hover { box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15); transform: translateY(-2px); transition: all 0.2s ease; }` (lift effect on hover)
   - This provides visual feedback for interactive elements

6. **Ensure Child Job Indentation:**
   - Existing child tree styling (lines 268-316) already handles indentation
   - No changes needed for child job tree
   - Child jobs use `--depth` CSS variable for indentation (line 271)

7. **Add Responsive Design Considerations:**
   - Add media query for mobile devices:
     - `@media (max-width: 768px) { .job-card-parent { border-left-width: 3px; } .job-card-parent .card-title { font-size: 1.1rem; } }`
   - This ensures cards remain readable on smaller screens

**Color Accessibility:**
- All color combinations meet WCAG AA contrast standards (4.5:1 for normal text)
- Status is conveyed through both color AND icons (not color-only)
- Text labels supplement visual indicators

**Example CSS Block to Add:**
```css
/* Enhanced Parent Job Card Styling */
.job-card-parent {
    border-left: 4px solid #3b82f6;
    background-color: #f8fafc;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
    margin-bottom: 1.2rem;
}

.job-card-parent:hover {
    box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15);
    transform: translateY(-2px);
    transition: all 0.2s ease;
}

.job-card-parent .card-title {
    font-size: 1.2rem;
    font-weight: 600;
    color: #1e293b;
}

/* Status Icon Colors */
.status-icon-pending { color: #f59e0b; }
.status-icon-running { color: #3b82f6; }
.status-icon-completed { color: #10b981; }
.status-icon-failed { color: #ef4444; }
.status-icon-cancelled { color: #6b7280; }

/* Enhanced Status Badge Colors */
.label-warning {
    background-color: #fef3c7;
    color: #92400e;
    border: 1px solid #fbbf24;
}

.label-primary {
    background-color: #dbeafe;
    color: #1e40af;
    border: 1px solid #3b82f6;
}

.label-success {
    background-color: #d1fae5;
    color: #065f46;
    border: 1px solid #10b981;
}

.label-error {
    background-color: #fee2e2;
    color: #991b1b;
    border: 1px solid #ef4444;
}

/* Responsive Design */
@media (max-width: 768px) {
    .job-card-parent {
        border-left-width: 3px;
    }
    .job-card-parent .card-title {
        font-size: 1.1rem;
    }
}
```
**Add Helper Function for Start Time Formatting (in Alpine.js component, after line 1869):**

**Purpose:**
- Centralize date formatting logic for consistency
- Provide reusable function for displaying start time
- Handle edge cases (null, invalid dates)

**Implementation:**

1. **Add getStartedDate Function:**
   - Add after `getCreatedDate(job)` function (line 1868-1870)
   - Signature: `getStartedDate(job)`
   - Logic:
     - Check if `job.started_at` exists and is not empty
     - Parse ISO 8601 timestamp to JavaScript Date object
     - Format using `toLocaleString()` for local date/time format
     - Return formatted string or null if not available

2. **Function Implementation:**
```javascript
getStartedDate(job) {
    if (!job.started_at) return null;
    try {
        return new Date(job.started_at).toLocaleString();
    } catch (error) {
        console.warn('[Queue] Failed to parse started_at:', error);
        return 'Invalid Date';
    }
}
```

3. **Usage in Template:**
   - Replace inline date formatting in metadata section
   - Use: `x-text="'Started: ' + getStartedDate(item.job)"`
   - Conditional display: `<template x-if="getStartedDate(item.job)">`

4. **Error Handling:**
   - Try-catch block handles invalid date strings
   - Returns null if `started_at` is empty (pending jobs)
   - Logs warning to console for debugging

**Alternative: Add getJobURL Helper Function:**

1. **Add getJobURL Function:**
   - Add after `getStartedDate(job)` function
   - Signature: `getJobURL(job)`
   - Logic:
     - Check `job.config.seed_urls[0]` first (primary seed URL)
     - Fall back to `job.progress.current_url` (currently processing URL)
     - Return URL string or null if not available

2. **Function Implementation:**
```javascript
getJobURL(job) {
    // Priority: seed_urls > current_url
    if (job.config?.seed_urls && job.config.seed_urls.length > 0) {
        return job.config.seed_urls[0];
    }
    if (job.progress?.current_url) {
        return job.progress.current_url;
    }
    return null;
}
```

3. **Usage in Template:**
   - Replace inline URL extraction logic
   - Use: `x-text="getJobURL(item.job)"`
   - Conditional display: `<template x-if="getJobURL(item.job)">`

4. **Benefits:**
   - Cleaner template code
   - Centralized URL extraction logic
   - Easier to modify URL priority in future

**Note:**
- These helper functions are optional but recommended for code maintainability
- They follow the existing pattern of helper functions in the Alpine.js component
- They reduce template complexity and improve readability