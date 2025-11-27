# Task 1: Add Info Logs to job_processor.go

## Metadata
- **ID:** 1
- **Group:** 1
- **Mode:** concurrent
- **Skill:** @go-coder
- **Critical:** no
- **Depends:** none
- **Blocks:** 4

## Paths
```yaml
sandbox: /tmp/3agents/task-1/
source: C:/development/quaero/
output: docs/features/20251127-queue-job-logging/
```

## Files to Modify
- `internal/queue/workers/job_processor.go` - Add Info-level logs for job start/end

## Requirements
Add Info-level logging at the job processing level:

1. **Job Start** (line ~142): Change from Trace to Info with summary format:
   ```go
   jp.logger.Info().
       Str("job_id", msg.JobID).
       Str("job_type", msg.Type).
       Msg("Job started")
   ```

2. **Job Success** (line ~235): Change from Trace to Info with duration:
   ```go
   jp.logger.Info().
       Str("job_id", msg.JobID).
       Str("job_type", msg.Type).
       Dur("duration", duration).
       Msg("Job completed")
   ```
   Note: Need to track start time and calculate duration.

## Acceptance Criteria
- [ ] Job start is logged at Info level with job_id and job_type
- [ ] Job completion is logged at Info level with job_id, job_type, and duration
- [ ] Compiles successfully

## Context
The job_processor.go is the central dispatcher that routes jobs to workers. It should provide the canonical Info-level start/end logs for all job types, rather than each worker duplicating this logic.

## Dependencies Input
N/A

## Output for Dependents
Job-level Info logging is centralized in the processor.
