# Task 1: Remove step progress stats from queue.html

Depends: - | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
Removes the inaccurate "X pending, Y running, Z completed, W failed" stats from step cards that show global queue counts instead of step-specific counts.

## Skill Patterns to Apply
- Alpine.js template modification
- Clean removal of template block

## Do
- Remove lines 191-198 in queue.html (the template block showing step.progress stats)
- Keep the surrounding structure intact

## Accept
- [ ] Step cards no longer display progress stats text
- [ ] No JavaScript errors in console
- [ ] Step cards still show: status badge, docs count, events panel
