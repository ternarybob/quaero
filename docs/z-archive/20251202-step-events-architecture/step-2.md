# Step 2: Fix Other Workers' Direct Event Publishing

## Status: COMPLETE

## Changes Made

### 1. github_log_worker.go
- **Lines 185, 268**: Replaced direct `eventService.Publish` calls with `logDocumentSaved()` helper
- Added `buildJobLogOptions()` helper to extract step context from job metadata
- Added `logDocumentSaved()` helper that routes through Job Manager

### 2. github_repo_worker.go
- **Line 171**: Replaced direct `eventService.Publish` call with `logDocumentSaved()` helper
- Added `buildJobLogOptions()` and `logDocumentSaved()` helpers (same pattern)

### 3. agent_worker.go
- **Line 708**: Fixed `publishJobError()` to use Job Manager's `AddJobLogWithEvent()`
- Added `buildJobLogOptions()` helper for step context extraction
- Removed direct `PublishSync` for DocumentUpdated events

### 4. places_worker.go
- **Line 277**: Replaced direct `PublishSync` call with Job Manager logging
- Uses `AddJobLogWithEvent()` with proper `JobLogOptions` containing step context

### 5. web_search_worker.go
- **Line 225**: Replaced direct `PublishSync` call with Job Manager logging
- Uses `AddJobLogWithEvent()` with proper `JobLogOptions` containing step context

## Pattern Used

All workers now follow this pattern:
```go
logOpts := &queue.JobLogOptions{
    SourceType:  "<worker_type>",
    StepName:    stepName,
    ParentJobID: parentJobID,
    ManagerID:   managerID,
}
w.jobMgr.AddJobLogWithEvent(ctx, jobID, level, message, logOpts)
```

## Build Verification
- `go build ./internal/queue/workers/...` - SUCCESS (no errors)
