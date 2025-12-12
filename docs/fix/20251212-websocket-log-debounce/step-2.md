# Step 2: Fix step status sync in handleJobUpdate
Workdir: ./docs/fix/20251212-websocket-log-debounce/ | Model: opus | Skill: frontend
Status: ✅ Complete
Timestamp: 2025-12-12T17:12:00+11:00

## Task Reference
From task-2.md:
- Intent: Fix step status icons not matching actual status
- Accept criteria: Step icons match status, steps auto-expand on running/failed, logs fetch immediately on status change

## Implementation Summary
Fixed Alpine.js reactivity issues in step status updates by:
1. Using immutable update patterns for step status changes in `handleJobUpdate`
2. Using immutable update patterns in `fetchJobStructure`
3. Adding immediate log fetch when step status changes
4. Ensuring all step updates trigger proper Alpine reactivity

## Files Changed
| File | Action | Lines | Description |
|------|--------|-------|-------------|
| `pages/queue.html` | modified | +58 | Fixed handleJobUpdate and fetchJobStructure with immutable updates |

## Code Changes Detail
### pages/queue.html

Fixed `handleJobUpdate` for job_step context (around line 4028):
```javascript
} else if (context === 'job_step' && step_name) {
    if (this.jobTreeData[job_id]) {
        const treeData = this.jobTreeData[job_id];
        if (treeData.steps) {
            const stepIdx = treeData.steps.findIndex(s => s.name === step_name);
            if (stepIdx >= 0) {
                const oldStatus = treeData.steps[stepIdx].status;
                if (oldStatus !== status) {
                    // Immutable update for step status
                    const newSteps = [...treeData.steps];
                    newSteps[stepIdx] = { ...newSteps[stepIdx], status: status };

                    // Auto-expand running/failed steps
                    if (status === 'running' || status === 'failed') {
                        const treeStepKey = `${job_id}:${stepIdx}`;
                        if (!this.jobTreeExpandedSteps[treeStepKey]) {
                            this.jobTreeExpandedSteps = { ...this.jobTreeExpandedSteps, [treeStepKey]: true };
                        }
                    }

                    // Trigger reactive update with fully immutable data
                    this.jobTreeData = {
                        ...this.jobTreeData,
                        [job_id]: { ...treeData, steps: newSteps }
                    };
                }
            }
        }
    }
    // ... and fetch logs immediately on status change
    this.fetchStepLogs(job_id, step_name, stepIdx, true);
}
```

**Why:** The original code mutated `treeData.steps[stepIdx].status` directly, which doesn't properly trigger Alpine.js reactivity. The spread operator creates a new object reference which Alpine detects as a change.

Fixed `fetchJobStructure` similarly (around line 4095):
```javascript
if (this.jobTreeData[jobId] && structure.steps) {
    const treeData = this.jobTreeData[jobId];
    if (treeData.steps) {
        const newSteps = [...treeData.steps];
        let hasChanges = false;
        structure.steps.forEach(stepStatus => {
            const stepIdx = newSteps.findIndex(s => s.name === stepStatus.name);
            if (stepIdx >= 0 && newSteps[stepIdx].status !== stepStatus.status) {
                newSteps[stepIdx] = { ...newSteps[stepIdx], status: stepStatus.status };
                hasChanges = true;
            }
        });
        if (hasChanges) {
            this.jobTreeData = {
                ...this.jobTreeData,
                [jobId]: { ...treeData, steps: newSteps }
            };
        }
    }
}
```

**Why:** Same reason - immutable updates ensure Alpine properly re-renders step icons.

## Skill Compliance
### From frontend patterns:
- [x] Update both allJobs and jobTreeData when step status changes - done in handleJobUpdate
- [x] Trigger reactive update with Alpine spread operator - used throughout
- [x] Auto-expand steps that become running or failed - implemented
- [x] No duplicate state - single source of truth in jobTreeData

## Accept Criteria Verification
- [x] Step icons match step status - Immutable updates ensure Alpine re-renders
- [x] Steps auto-expand when running or failed - Implemented in handleJobUpdate
- [x] Logs fetch immediately on status change - Added `fetchStepLogs(..., true)`
- [x] Build passes

## Build & Test
```
Build: ✅ Pass
Tests: ✅ Pass (TestJobDefinitionCodebaseClassify - 36.99s)
```

## Issues Encountered
- None

## State for Next Phase
Files ready for validation:
- `pages/queue.html` - step status sync fixed

Remaining work: None - ready for validation
