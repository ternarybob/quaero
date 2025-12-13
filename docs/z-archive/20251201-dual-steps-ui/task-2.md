# Task 2: Update renderJobs() to Include Children

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @frontend-developer | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: ./ | Output: ./docs/fix/20251201-dual-steps-ui/

## Files
- `pages/queue.html` - Update renderJobs() function to populate child data

## Requirements

The `renderJobs()` function (lines 2010-2086) currently:
1. Iterates through filteredJobs
2. Adds parent job rows
3. Adds step rows for multi-step job definitions

Update to also:
1. For each parent job, check if it has child jobs (via child_count field or API)
2. Load child jobs from the allJobs array (children have parent_id matching parent's id)
3. Add child job items to itemsToRender with type: 'child'
4. Children should be added after steps (or directly after parent if no steps)
5. Child data should include: job object, parent reference

Child job format for itemsToRender:
```javascript
{
    type: 'child',
    job: childJobObject,
    parentId: parentJob.id
}
```

The child template (from task-1) will use this data to render each child row.

## Acceptance
- [ ] renderJobs() adds child job items to itemsToRender
- [ ] Children are filtered to only show those belonging to current parent
- [ ] Children are sorted by created_at (oldest first) or execution order
- [ ] Children appear after steps in the render order
- [ ] Compiles
- [ ] Tests pass
