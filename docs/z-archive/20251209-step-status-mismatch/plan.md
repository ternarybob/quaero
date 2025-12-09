# Plan: Step Status Mismatch Fix
Type: fix | Workdir: ./docs/fix/20251209-step-status-mismatch/

## User Intent (from manifest)
When a step fails (e.g., "worker init failed: no documents found matching tags"), the UI should show that step as **Failed** (red), not **Completed** (green). Currently steps that fail are incorrectly showing "Completed" status.

## Root Cause Analysis

### Problem Flow
1. When a step fails in orchestrator, `UpdateJobStatus(ctx, stepID, "failed")` is called (line 262)
2. The step job's status is correctly set to "failed" in the database
3. BUT the UI in `queue.html` renders steps based on `step_definitions` from parent metadata, NOT from step job records
4. The UI calculates status using logic at lines 2493-2535 that **overrides** failed status:
   ```javascript
   // Line 2533-2535: BUG - overrides failed steps to "completed"
   if (!hasActiveChildren && parentStatus === 'completed' && stepNum <= stepDefs.length) {
       status = 'completed';
   }
   ```
5. When `error_tolerance` allows job to continue after failures, parent ends as "completed"
6. This override marks ALL steps as "completed" regardless of actual step status

### Missing Data
- `step_stats` in metadata contains: `step_index`, `step_id`, `step_name`, `step_type`, `child_count`, `document_count`
- **Missing**: `status` field for each step
- The `step_job_ids` array exists but isn't used to fetch actual step job statuses

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Add `status` field to step_stats in orchestrator.go | - | no | sonnet |
| 2 | Update UI to use step_stats.status for step display | 1 | no | sonnet |
| 3 | Build and verify fix | 2 | no | sonnet |

## Order
[1] → [2] → [3]

## Implementation Details

### Task 1: Add status field to step_stats
Location: `internal/queue/orchestrator.go`

When a step completes (successfully or with failure), store the status in step_stats:
```go
stepStats[i] = map[string]interface{}{
    "step_index":     i,
    "step_id":        stepID,
    "step_name":      step.Name,
    "step_type":      step.Type.String(),
    "child_count":    stepChildCount,
    "document_count": stepDocCount,
    "status":         stepStatus,  // ADD THIS
}
```

Also need to handle the failure cases (lines 258-268 and 279-306) to update step_stats with "failed" status before continuing.

### Task 2: Update UI to use step_stats.status
Location: `pages/queue.html` around lines 2492-2536

Update the status determination logic:
```javascript
stepDefs.forEach((stepDef, index) => {
    let status = 'pending';
    const stepNum = index + 1;

    // First, check for persisted status from step_stats
    const stepStat = stepStats[index] || {};
    if (stepStat.status) {
        status = stepStat.status;  // Use persisted status from backend
    }

    // Then check for real-time status from _stepProgress (WebSocket updates)
    if (parentJob._stepProgress && parentJob._stepProgress[stepDef.name]) {
        const rtProgress = parentJob._stepProgress[stepDef.name];
        if (rtProgress.status) {
            status = rtProgress.status;  // Real-time overrides persisted
        }
    }

    // Only calculate status if not already set from backend sources
    if (status === 'pending') {
        // ... existing calculation logic for running steps ...
    }

    // REMOVE the override that marks all steps as completed
});
```

### Task 3: Build and verify
- Run `go build` to ensure no compilation errors
- Manually verify the fix by restarting service and running codebase_assess job
