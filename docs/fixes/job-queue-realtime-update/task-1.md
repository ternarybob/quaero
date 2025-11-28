# Task 1: Fix monitor.go completion event to include document_count

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: ./docs/fixes/job-queue-realtime-update/

## Files
- `internal/queue/state/monitor.go` - modify completion handling

## Requirements

In `monitorChildJobs()` at line ~185-207, when the parent job completes:

1. Get fresh child stats using `GetJobChildStats()`
2. Format progress text using `formatProgressText()`
3. Replace `publishParentJobProgress()` call with `publishParentJobProgressUpdate()` which includes document_count

### Current Code (lines 185-207):
```go
if completed {
    // All child jobs are complete
    jobLogger.Debug().Msg("All child jobs completed, finishing parent job")

    // Update job status to completed
    if err := m.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
        jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
        return fmt.Errorf("failed to update job status: %w", err)
    }

    // Set finished_at timestamp for completed parent jobs
    if err := m.jobMgr.SetJobFinished(ctx, job.ID); err != nil {
        jobLogger.Warn().Err(err).Msg("Failed to set finished_at timestamp")
    }

    // Add final job log
    m.jobMgr.AddJobLog(ctx, job.ID, "info", "Parent job completed successfully")

    // Publish completion event
    m.publishParentJobProgress(ctx, job, "completed", "All child jobs completed")

    jobLogger.Debug().Str("job_id", job.ID).Msg("Parent job execution completed successfully")
    return nil
}
```

### Fix:
Replace the `publishParentJobProgress()` call with code that:
1. Gets fresh child stats
2. Calls `publishParentJobProgressUpdate()` with stats (which includes document_count)

## Acceptance
- [ ] Completion event includes document_count
- [ ] Compiles
- [ ] Tests pass
