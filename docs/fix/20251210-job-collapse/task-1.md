# Task 1: Add collapsedJobs state

Depends: - | Critical: no | Model: sonnet | Skill: none

## Addresses User Intent

This task adds the state object needed to track which parent jobs have their steps collapsed. Without this state, we can't know which jobs to show/hide steps for.

## Skill Patterns to Apply

N/A - no skill for this task

## Do

- Add `collapsedJobs: {}` to the Alpine.js data object in queue.html
- Place near existing `expandedParents` and `expandedSteps` state objects

## Accept

- [ ] `collapsedJobs` property exists in Alpine.js data object
- [ ] No JavaScript errors
