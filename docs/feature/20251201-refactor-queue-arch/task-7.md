# Task 7: Update UI queue.html

Depends: 5,6 | Critical: no | Model: sonnet

## Do

1. Update WebSocket event handling:
   - Handle `step_progress` events → update job panel
   - Handle `manager_progress` events → update top panel

2. Update job list display:
   - Manager shows: step progress (X/Y steps complete)
   - Step expands to show: job progress (X/Y jobs complete)
   - Jobs show: individual job status

3. Update step row rendering:
   - Step becomes an expandable row
   - Shows step name, type, status
   - Shows job count and progress
   - Expands to show child jobs

4. Update statistics panel:
   - Show manager-level stats (steps)
   - Show step-level stats when expanded (jobs)

5. Update event log panel:
   - Filter by step when viewing step details
   - Show all logs when viewing manager

## Accept

- [ ] UI handles step_progress events
- [ ] UI handles manager_progress events
- [ ] Step rows are expandable
- [ ] Job panel updates correctly for step progress
- [ ] Top panel updates correctly for manager progress
