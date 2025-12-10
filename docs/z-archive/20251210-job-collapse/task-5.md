# Task 5: Add visual expand/collapse indicator

Depends: 3 | Critical: no | Model: sonnet | Skill: none

## Addresses User Intent

This task adds a visual indicator showing whether steps are expanded or collapsed, so users know the current state and that clicking will toggle it.

## Skill Patterns to Apply

N/A - no skill for this task

## Do

- Add chevron icon (fa-chevron-down/fa-chevron-right) near the metadata area
- Icon shows down when expanded, right when collapsed
- Add cursor:pointer style to indicate clickable area
- Only show for parent jobs with step_definitions

## Accept

- [ ] Chevron icon visible for multi-step jobs
- [ ] Chevron direction changes based on collapse state
- [ ] Cursor indicates clickable area
