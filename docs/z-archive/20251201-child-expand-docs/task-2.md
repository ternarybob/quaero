# Task 2: Debug and Fix Child Expand Functionality
Depends: - | Critical: no | Model: sonnet

## Problem
The "20 children" button is visible with chevron icon, but clicking it doesn't expand to show child job rows. The screenshot shows the button exists but children aren't being rendered.

## Analysis
From code review:
1. `toggleParentExpand(parentId)` at line 1920 toggles `this.expandedParents[parentId]` and calls `renderJobs()`
2. `isParentExpanded(parentId)` at line 1927 returns the expanded state
3. `renderJobs()` at line 2248 checks expansion state and renders children
4. Children are fetched in parallel at lines 1993-2014

Potential issues to investigate:
1. `allJobs` may not contain the fetched children
2. The child filtering at line 2247 may not find children
3. Alpine.js reactivity may not be triggering properly

## Do
1. Check if children are being stored in `allJobs` after fetch (line 2023-2031)
2. Verify the child filtering logic `this.allJobs.filter(job => job.parent_id === parentJob.id)`
3. Add console.log debugging if needed
4. Fix any issues found with child storage or rendering

## Accept
- [ ] Clicking "N children" button expands to show child job rows
- [ ] Chevron icon changes from right to down when expanded
- [ ] Child rows display with correct status, name, and document count
- [ ] Clicking again collapses the children
- [ ] Build compiles successfully
