# Task 4: Update renderJobs to skip collapsed steps

Depends: 2 | Critical: no | Model: sonnet | Skill: none

## Addresses User Intent

This task makes collapsed jobs actually hide their steps by skipping them during render. When a job is collapsed, its step rows won't be added to itemsToRender.

## Skill Patterns to Apply

N/A - no skill for this task

## Do

- In `renderJobs()`, check `isJobStepsCollapsed(parentJob.id)` before adding step items
- If collapsed, skip the entire stepDefs.forEach block
- Steps and children should not appear in itemsToRender for collapsed jobs

## Accept

- [ ] Steps are hidden when job is collapsed
- [ ] Steps appear when job is expanded
- [ ] renderJobs correctly checks collapsed state
