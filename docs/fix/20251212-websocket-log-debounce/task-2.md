# Task 2: Fix step status sync in handleJobUpdate for job_step context
Workdir: ./docs/fix/20251212-websocket-log-debounce/ | Depends: 1 | Critical: no
Model: opus | Skill: frontend

## Context
This task is part of: Fixing job status display where step icons don't match actual status

## User Intent Addressed
Fix job status display - The running job UI shows incorrect status:
- Steps showing "running" spinner icons when job is "Completed"
- Status icons don't match actual step status

## Input State
Files that exist before this task:
- `pages/queue.html` - Current `handleJobUpdate` only updates `allJobs` status, not `jobTreeData.steps` status

## Output State
Files after this task completes:
- `pages/queue.html` - Updated `handleJobUpdate` that syncs step status to `jobTreeData`

## Skill Patterns to Apply
### From frontend patterns:
- **DO:** Update both allJobs and jobTreeData when step status changes
- **DO:** Trigger reactive update with Alpine spread operator
- **DO:** Auto-expand steps that become "running" or "failed"
- **DON'T:** Create duplicate state that can get out of sync
- **DON'T:** Forget to trigger Alpine reactivity after state change

## Implementation Steps
1. In `handleJobUpdate` for context="job_step":
   - Find the step in `jobTreeData[job_id].steps` by step_name
   - Update the step's status
   - Trigger Alpine reactivity with spread operator pattern
2. Auto-expand step when it transitions to "running" or "failed"
3. When status changes to terminal (completed/failed), fetch logs immediately

## Code Specifications
```javascript
// In handleJobUpdate, when context === 'job_step':
if (this.jobTreeData[job_id]) {
    const treeData = this.jobTreeData[job_id];
    if (treeData.steps) {
        const stepIdx = treeData.steps.findIndex(s => s.name === step_name);
        if (stepIdx >= 0) {
            // Update step status
            const newSteps = [...treeData.steps];
            newSteps[stepIdx] = { ...newSteps[stepIdx], status: status };

            // Trigger Alpine reactivity
            this.jobTreeData = {
                ...this.jobTreeData,
                [job_id]: { ...treeData, steps: newSteps }
            };

            // Auto-expand on running/failed
            if (status === 'running' || status === 'failed') {
                const key = `${job_id}:${stepIdx}`;
                this.jobTreeExpandedSteps = { ...this.jobTreeExpandedSteps, [key]: true };
            }

            // Fetch logs immediately on status change
            this.fetchStepLogs(job_id, step_name, stepIdx, true);
        }
    }
}
```

## Accept Criteria
- [ ] Step icons match step status (no running spinner on completed steps)
- [ ] Steps auto-expand when they start running or fail
- [ ] Logs fetch immediately on status change (bypass debounce)
- [ ] Build passes

## Handoff
After completion, next task(s): task-3 (run test and verify)
