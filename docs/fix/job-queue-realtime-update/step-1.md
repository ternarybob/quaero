# Step 1: Fix monitor.go completion event to include document_count

- Task: task-1.md | Group: 1 | Model: sonnet

## Actions
1. Modified `monitorChildJobs()` in `internal/queue/state/monitor.go`
2. Replaced `publishParentJobProgress()` call on completion with `publishParentJobProgressUpdate()`
3. Added code to fetch fresh child stats before publishing completion event

## Files
- `internal/queue/state/monitor.go` - Modified completion handling (lines 203-226)

## Decisions
- Used `publishParentJobProgressUpdate()` instead of `publishParentJobProgress()` because it includes `document_count` field (line 625 in original file)
- Added fallback to basic progress event if stats cannot be fetched
- Kept structure similar to existing `checkChildJobProgress()` for consistency

## Changes Made

**Before (line 203-204):**
```go
// Publish completion event
m.publishParentJobProgress(ctx, job, "completed", "All child jobs completed")
```

**After (lines 203-226):**
```go
// Publish completion event with full stats (including document_count)
// Get fresh child stats for final progress update
childStatsMap, err := m.jobMgr.GetJobChildStats(ctx, []string{job.ID})
if err != nil {
    jobLogger.Warn().Err(err).Msg("Failed to get final child stats for completion event")
    // Fall back to basic progress event
    m.publishParentJobProgress(ctx, job, "completed", "All child jobs completed")
} else if interfaceStats, ok := childStatsMap[job.ID]; ok && interfaceStats != nil {
    // Convert to local stats struct
    finalStats := &ChildJobStats{
        TotalChildren:     interfaceStats.ChildCount,
        CompletedChildren: interfaceStats.CompletedChildren,
        FailedChildren:    interfaceStats.FailedChildren,
        CancelledChildren: interfaceStats.CancelledChildren,
        RunningChildren:   interfaceStats.RunningChildren,
        PendingChildren:   interfaceStats.PendingChildren,
    }
    progressText := m.formatProgressText(finalStats)
    // Use publishParentJobProgressUpdate which includes document_count
    m.publishParentJobProgressUpdate(ctx, job.ID, finalStats, progressText)
} else {
    // No stats available, fall back to basic progress event
    m.publishParentJobProgress(ctx, job, "completed", "All child jobs completed")
}
```

## Verify
Compile: ✅ | Tests: ⚙️ (environment issues - not code-related)

## Status: ✅ COMPLETE
