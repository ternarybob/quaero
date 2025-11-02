I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The bug occurs in the `updateJobInList()` function within the Alpine.js `jobList` component in `pages/queue.html`. When a job's status changes via WebSocket (e.g., from "pending" to "running"), the function checks if the job still matches the active filters using `window.matchesActiveFilters(job)`. If the job no longer matches (e.g., user has filtered to show only "pending" jobs), it gets removed from the `filteredJobs` array, causing it to disappear from the UI.

The current behavior is technically correct from a filtering perspective, but creates a poor user experience where jobs vanish unexpectedly during execution. Users expect jobs to remain visible once they appear in the list, with status changes reflected in-place, unless they explicitly change filters or delete the job.

The default filter configuration includes all statuses (`['pending', 'running', 'completed', 'failed', 'cancelled']`), so this bug primarily affects users who have customized their filters via localStorage to exclude certain statuses.

### Approach

Modify the `updateJobInList()` function to distinguish between two scenarios:
1. **Job already in filteredJobs**: Keep it visible and update in-place, regardless of filter changes
2. **New job arriving**: Apply filters to decide whether to add it

This "sticky visibility" approach ensures jobs remain visible once they appear in the list, while still respecting filters for newly arriving jobs. Jobs will only disappear when explicitly deleted by the user or when the user changes filter settings and triggers a full reload via `loadJobs()`.

### Reasoning

I explored the repository structure, read the `pages/queue.html` file to understand the Alpine.js component architecture, and identified the `updateJobInList()` function (lines 1776-1912) as the source of the bug. I examined the `matchesActiveFilters()` function (lines 640-657) to understand the filter logic, and reviewed the default filter configuration (lines 509-520) to understand when the bug manifests.

## Proposed File Changes

### pages\queue.html(MODIFY)

**Modify the `updateJobInList()` function in the Alpine.js `jobList` component (around lines 1776-1912)**:

1. **Change the filter application logic** (lines 1872-1898):
   - Before checking `matchesActiveFilters()`, first check if the job already exists in `filteredJobs` array
   - If the job is already in `filteredJobs`, update it in-place WITHOUT checking filters
   - Only apply filter checks for NEW jobs that are not yet in `filteredJobs`
   - This ensures jobs remain visible once they appear, even when their status changes

2. **Preserve the existing behavior for new jobs**:
   - For jobs not found in `filteredJobs`, continue to apply `matchesActiveFilters()` check
   - Only add new jobs to `filteredJobs` if they match the active filters
   - This maintains the expected filtering behavior for newly arriving jobs

3. **Keep the deletion behavior unchanged**:
   - Jobs should still be removed from `filteredJobs` when explicitly deleted (handled by `handleDeleteCleanup()`)
   - Jobs will be removed when user changes filters and triggers `loadJobs()`, which fetches fresh data from the server

**Implementation approach**:
- Move the `filteredIndex` lookup (currently line 1876) to occur BEFORE the `matchesActiveFilters()` check
- Use the `filteredIndex` result to determine whether to apply filters:
  - If `filteredIndex >= 0`: Job already visible, update in-place (skip filter check)
  - If `filteredIndex < 0`: New job, apply `matchesActiveFilters()` before adding

**Expected behavior after fix**:
- Jobs that are visible when they start executing will remain visible as their status changes
- New jobs arriving via WebSocket will still be filtered according to active filters
- Jobs will only disappear when: (a) explicitly deleted, (b) user changes filters and reloads, or (c) user navigates to a different page
- The fix maintains backward compatibility with all existing filter functionality