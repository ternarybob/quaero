# Task 2: Add toggle method and helper

Depends: 1 | Critical: no | Model: sonnet | Skill: none

## Addresses User Intent

This task adds the method to toggle collapse state and a helper to check if a job's steps are collapsed. These are needed to respond to user clicks and control step visibility.

## Skill Patterns to Apply

N/A - no skill for this task

## Do

- Add `toggleJobStepsCollapse(jobId)` method that toggles the collapsed state for a parent job
- Add `isJobStepsCollapsed(jobId)` helper that returns true if steps are collapsed
- Follow existing patterns from `toggleParentExpand` / `isParentExpanded`

## Accept

- [ ] `toggleJobStepsCollapse(jobId)` method exists and toggles state
- [ ] `isJobStepsCollapsed(jobId)` helper returns correct boolean
- [ ] No JavaScript errors
