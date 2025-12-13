# Step 2: Update renderJobs() to Include Children

- Task: task-2.md | Group: 2 | Model: sonnet

## Actions
Combined with Step 1 - renderJobs() update was included in the same edit:
1. Added child job filtering: `this.allJobs.filter(job => job.parent_id === parentJob.id)`
2. Sorted children by created_at (oldest first)
3. Added child items with type='child' and metadata (childIndex, totalChildren, parentId)

## Files
- `pages/queue.html` - Updated renderJobs() function at lines 2086-2103

## Decisions
- Children are rendered from allJobs (which includes all jobs fetched)
- Children appear after any step rows for the parent
- Used consistent item format for Alpine.js template rendering

## Verify
Compile: PASS | Tests: PASS

## Status: COMPLETE
