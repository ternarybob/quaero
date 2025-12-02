# Task 4: Fix job_monitor event publishing

Depends: 3 | Critical: no | Model: sonnet

## Addresses User Intent

Job Manager layer must properly publish events for the entire job, maintaining proper hierarchy.

## Do

1. Review monitor.go `publishParentJobProgress()` and related methods
2. Ensure job-level events don't contain step-specific data that could confuse UI
3. Verify job_log events from monitor use proper unified logging
4. Check that parent job progress is clearly distinguished from step progress

## Accept

- [ ] Job-level events are clearly separate from step events
- [ ] Parent progress events use proper Job Manager methods
- [ ] Code compiles without errors
