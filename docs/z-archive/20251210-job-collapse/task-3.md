# Task 3: Add click handler to job card metadata area

Depends: 2 | Critical: no | Model: sonnet | Skill: none

## Addresses User Intent

This task makes the job header/metadata area (timestamps section) clickable to toggle step visibility. Uses `@click.stop` to prevent navigation to job details.

## Skill Patterns to Apply

N/A - no skill for this task

## Do

- Add click handler to metadata area div in job card template
- Use `@click.stop="toggleJobStepsCollapse(item.job.id)"` to prevent event propagation
- Only apply to parent jobs that have step_definitions (multi-step jobs)

## Accept

- [ ] Clicking metadata area toggles collapse state
- [ ] Clicking does NOT navigate to job details page
- [ ] Works only for parent jobs with steps
