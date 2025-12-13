# Task 2: Update UI to use unified /api/logs endpoint
Depends: 1 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
Replace `/api/jobs/{stepJobId}/logs` calls in UI with `/api/logs?scope=job&job_id={stepJobId}&include_children=false`.

## Skill Patterns to Apply
- Alpine.js reactive state
- Async fetch patterns
- Error handling

## Do
1. Update `fetchStepEvents()` to use `/api/logs?scope=job&job_id=${stepJobId}&include_children=false&limit=100&order=asc&level=info`
2. Update `refreshStepEvents()` to use the same endpoint

## Accept
- [ ] Step events load via /api/logs endpoint
- [ ] No more /api/jobs/{id}/logs calls for step logs
- [ ] UI displays step events correctly
