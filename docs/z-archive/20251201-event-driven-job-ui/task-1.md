# Task 1: Add job event logging to all workers
Depends: - | Critical: no | Model: sonnet

## Context
Workers currently have inconsistent event publishing. Need unified approach where all workers:
1. Log events to both their own job AND the parent job
2. Use consistent event format for UI display
3. Include metadata for aggregation (step_name, event_type)

## Do
- Review existing worker implementations in `internal/queue/workers/`
- Add helper function for unified event logging to manager
- Update AgentWorker to log events to parent
- Update PlacesWorker to log events to parent
- Update WebSearchWorker to log events to parent
- Update CrawlerWorker to log events to parent
- Ensure all status changes log to parent

## Event Types to Log
- `job_started`: When child job starts
- `job_completed`: When child job completes
- `job_failed`: When child job fails
- `document_created`: When documents are created
- `progress`: Periodic progress updates

## Accept
- [ ] All workers log events with consistent format
- [ ] Parent job receives logs from all child jobs
- [ ] Events include step_name metadata
- [ ] Build compiles without errors
