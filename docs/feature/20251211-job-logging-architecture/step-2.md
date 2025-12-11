# Step 2: Enhance Tree API Backend-Driven Expansion State
Model: sonnet | Skill: go | Status: ✅

## Done
- Updated `GetJobTreeHandler` main path expansion logic
- Updated `buildStepsFromStepJobs` fallback function expansion logic
- Added `currentStepName` extraction from parent metadata

## Changes Made

### 1. Main Path (GetJobTreeHandler) - Lines 1461-1467, 1497, 1552-1558

Added extraction of `currentStepName` from parent job metadata:
```go
// Get current_step_name from parent metadata for expansion logic
var currentStepName string
if parentJob.Metadata != nil {
    if csn, ok := parentJob.Metadata["current_step_name"].(string); ok {
        currentStepName = csn
    }
}
```

Changed line 1497 from immediate expansion setting to deferred:
```go
// Before:
Expanded: stepJob.Status == models.JobStatusFailed,

// After:
// Expanded is set AFTER logs are fetched (see below)
```

Added comprehensive backend-driven expansion after logs are fetched (lines 1552-1558):
```go
// Backend-driven expansion: expand if failed, running, has logs, or is current step
// This moves expansion logic from frontend to backend for simpler UI
hasLogs := len(step.Logs) > 0
isRunning := stepJob.Status == models.JobStatusRunning
isFailed := stepJob.Status == models.JobStatusFailed
isCurrentStep := stepName == currentStepName
step.Expanded = isFailed || isRunning || hasLogs || isCurrentStep
```

### 2. Fallback Path (buildStepsFromStepJobs) - Lines 1774, 1830-1835

Changed line 1774 from immediate expansion setting to deferred:
```go
// Before:
Expanded: stepJob.Status == models.JobStatusFailed,

// After:
// Expanded is set AFTER logs are fetched (see below)
```

Added expansion logic after logs are fetched (lines 1830-1835):
```go
// Backend-driven expansion: expand if failed, running, or has logs
// This moves expansion logic from frontend to backend for simpler UI
hasLogs := len(step.Logs) > 0
isRunning := stepJob.Status == models.JobStatusRunning
isFailed := stepJob.Status == models.JobStatusFailed
step.Expanded = isFailed || isRunning || hasLogs
```

Note: The fallback path doesn't have access to `currentStepName` since it's only used when `step_definitions` is empty (no metadata available).

## Expansion Logic Summary

| Condition | Main Path | Fallback Path |
|-----------|-----------|---------------|
| Failed status | ✅ | ✅ |
| Running status | ✅ | ✅ |
| Has logs | ✅ | ✅ |
| Is current_step | ✅ | ❌ (no metadata) |

## Files Changed
- `internal/handlers/job_handler.go`

## Skill Compliance
Following go/SKILL.md patterns:
- Proper error handling preserved
- Consistent code style
- Clear comments explaining logic

## Build Check
Build: ✅ PASS | Tests: Pending (Task 5)
