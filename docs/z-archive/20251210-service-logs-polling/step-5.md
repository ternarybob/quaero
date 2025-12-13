# Step 5: Skip child refresh for completed jobs
Model: opus | Skill: frontend | Status: Done

## Issue
The child refresh interval was polling `/api/jobs?parent_id=...` every 2 seconds even for completed parent jobs.

## Done
- Added status filter to child refresh interval
- Only fetches children for parents with status `pending` or `running`
- Completed, failed, and cancelled jobs no longer trigger child fetches

## Files Changed
- `pages/queue.html` - Lines 1880-1886: Added activeStatuses filter

## Build Check
Build: N/A (JS only) | Tests: Skipped
