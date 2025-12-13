# Task 2: Fix RerunJob to Enqueue Job for Execution

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-2/ | Source: C:\development\quaero | Output: C:\development\quaero\docs\plans

## Files
- `internal/services/crawler/service.go` - Add queue enqueue call to RerunJob function

## Requirements

The `RerunJob` function in `internal/services/crawler/service.go` saves the job to storage but does not enqueue it for processing. The job stays in "pending" status forever.

### Current Flow (Broken)
1. Get original job from database
2. Create new QueueJobState
3. Save to storage
4. Return new job ID (job never runs!)

### Expected Flow (Fixed)
1. Get original job from database
2. Create new QueueJobState with proper job type
3. Save to storage
4. **Enqueue job to queue for processing** (MISSING!)
5. Return new job ID

### Implementation

After saving the job (line ~1052), add the enqueue logic:

```go
// Enqueue the job for processing
if s.queueManager != nil {
    // Serialize the job state to JSON for queue payload
    payloadBytes, err := json.Marshal(newJobState)
    if err != nil {
        s.logger.Warn().Err(err).Str("job_id", newJobID).Msg("Failed to serialize job for queue, job saved but not enqueued")
        return newJobID, nil // Job is saved, return success but it won't auto-run
    }

    msg := models.QueueMessage{
        JobID:   newJobID,
        Type:    newJobState.Type,
        Payload: payloadBytes,
    }

    if err := s.queueManager.Enqueue(s.ctx, msg); err != nil {
        s.logger.Warn().Err(err).Str("job_id", newJobID).Msg("Failed to enqueue job, job saved but not enqueued")
        return newJobID, nil // Job is saved, return success but it won't auto-run
    }

    s.logger.Info().
        Str("original_job_id", jobID).
        Str("new_job_id", newJobID).
        Str("job_type", newJobState.Type).
        Msg("Job rerun created and enqueued for processing")
} else {
    s.logger.Warn().
        Str("job_id", newJobID).
        Msg("Queue manager not available, job saved but not enqueued")
}
```

Note: Need to import "encoding/json" if not already imported.

## Acceptance
- [ ] RerunJob function enqueues job to queue after saving
- [ ] Job starts running after being copied (moves from pending to running)
- [ ] Error handling is graceful (job still saved even if enqueue fails)
- [ ] Compiles
- [ ] Tests pass
