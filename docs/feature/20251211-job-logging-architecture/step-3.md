# Step 3 & 4: Simplify Frontend to Use Backend Expansion State
Model: sonnet | Skill: frontend | Status: ✅

## Done
- Simplified `loadJobTreeData()` to use backend's `step.expanded` field
- Removed redundant client-side `shouldExpand` computation
- Removed redundant `hasLogs` variable computation

## Changes Made

### pages/queue.html - loadJobTreeData() (lines 2425-2436)

Before (computing expansion client-side):
```javascript
// Auto-expand steps that are failed, running, or have logs
// Force expand ALL steps with logs (not just those not already expanded)
// This ensures steps auto-expand when new logs arrive via refresh
if (treeData.steps) {
    treeData.steps.forEach((step, idx) => {
        const key = `${jobId}:${idx}`;
        const hasLogs = step.logs && step.logs.length > 0;
        const shouldExpand = step.status === 'failed' || step.status === 'running' || hasLogs;
        // Auto-expand steps with logs or running/failed status
        // Only skip if user explicitly collapsed (value is false, not undefined)
        if (shouldExpand && this.jobTreeExpandedSteps[key] !== false) {
            this.jobTreeExpandedSteps = { ...this.jobTreeExpandedSteps, [key]: true };
        }
    });
}
```

After (using backend expansion state):
```javascript
// Use backend-driven expansion state (step.expanded)
// Only apply if user hasn't explicitly collapsed the step
if (treeData.steps) {
    treeData.steps.forEach((step, idx) => {
        const key = `${jobId}:${idx}`;
        // Backend provides step.expanded based on: failed, running, hasLogs, isCurrentStep
        // Only skip if user explicitly collapsed (value is false, not undefined)
        if (step.expanded && this.jobTreeExpandedSteps[key] !== false) {
            this.jobTreeExpandedSteps = { ...this.jobTreeExpandedSteps, [key]: true };
        }
    });
}
```

## Removed Redundant Logic (Task 4)

| Removed | Reason |
|---------|--------|
| `const hasLogs = step.logs && step.logs.length > 0` | Backend now computes this |
| `const shouldExpand = step.status === 'failed' \|\| step.status === 'running' \|\| hasLogs` | Backend now provides `step.expanded` |

## User Override Behavior Preserved

The user can still manually collapse steps:
- `this.jobTreeExpandedSteps[key] !== false` check preserved
- When user clicks to collapse, value is set to `false`
- Backend's `expanded=true` won't override user's explicit collapse

## Files Changed
- `pages/queue.html`

## Skill Compliance
Following frontend/SKILL.md patterns:
- Alpine.js reactive data patterns preserved
- Immutable state updates with spread operator
- Clear comments explaining logic

## Build Check
Build: ✅ PASS | Tests: Pending (Task 5)
