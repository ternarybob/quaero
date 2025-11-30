# Step 1: Add Child Job Rows to UI Template

- Task: task-1.md | Group: 1 | Model: sonnet

## Actions
1. Modified `renderJobs()` function (lines 2086-2103) to add child jobs as separate rows
2. Added new child row template (lines 209-276) with compact display similar to step rows
3. Updated job card template condition to exclude 'child' type (line 279)

## Files
- `pages/queue.html` - Added child job rendering logic and template

## Decisions
- Used compact row layout (similar to step rows) instead of full job cards for children
- Child rows have cyan border (#17a2b8) to distinguish from steps (gray #6c757d)
- Sorted children by created_at for consistent display order
- Show child index (1/N, 2/N) for progress indication

## Verify
Compile: PASS | Tests: PASS

## Status: COMPLETE
