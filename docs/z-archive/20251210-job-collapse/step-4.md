# Step 4: Update renderJobs to skip collapsed steps

Model: sonnet | Skill: none | Status: ✅

## Done

- Added early return in renderJobs when `isJobStepsCollapsed(parentJob.id)` is true
- Steps and children are not added to itemsToRender for collapsed jobs

## Files Changed

- `pages/queue.html` - Added collapsed check at lines 2607-2610

## Skill Compliance (if skill used)

No skill applied

## Build Check

Build: ⏭️ | Tests: ⏭️ (frontend-only change, will verify at end)
