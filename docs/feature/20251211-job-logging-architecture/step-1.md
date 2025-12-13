# Step 1: Audit and Document Current Expansion Logic Issues
Model: sonnet | Skill: none | Status: ✅

## Done
- Reviewed loadJobTreeData() in queue.html (lines 2417-2445)
- Reviewed GetJobTreeHandler in job_handler.go (lines 1345-1759)
- Documented current expansion logic and identified gaps

## Current State Analysis

### Backend (job_handler.go)
The backend already has an `Expanded` field in JobTreeStep (line 1347), but it's set minimally:

```go
// Line 1489 - Only expands failed steps
step.Expanded = stepJob.Status == models.JobStatusFailed

// Line 1580 - Parent job fallback always expanded
Expanded: true

// Line 1758 - Alternative path also only failed
Expanded: stepJob.Status == models.JobStatusFailed
```

**Backend Gap**: Does NOT consider:
- Step has logs (hasLogs)
- Step is currently running
- Step is the current_step from metadata

### Frontend (queue.html)
The frontend duplicates and extends the expansion logic (lines 2425-2438):

```javascript
// Lines 2431-2432 - Client computes shouldExpand
const hasLogs = step.logs && step.logs.length > 0;
const shouldExpand = step.status === 'failed' || step.status === 'running' || hasLogs;

// Lines 2435-2436 - Client overrides backend's Expanded
if (shouldExpand && this.jobTreeExpandedSteps[key] !== false) {
    this.jobTreeExpandedSteps[key] = true;
}
```

**Frontend Problem**:
1. Ignores backend `step.expanded` field entirely
2. Computes shouldExpand client-side based on status/hasLogs
3. Only checks `this.jobTreeExpandedSteps[key] !== false` for user override

## Gap Summary

| Expansion Condition | Backend | Frontend |
|---------------------|---------|----------|
| Failed status | ✅ Sets Expanded=true | ✅ Checks status |
| Running status | ❌ Not considered | ✅ Checks status |
| Has logs | ❌ Not considered | ✅ Checks logs.length |
| Is current_step | ❌ Not considered | ❌ Not considered |
| Uses backend Expanded | N/A | ❌ Ignores it |

## Required Changes

### Task 2 (Backend):
Enhance backend to set `Expanded` based on:
1. `status == "failed"` → Expanded = true (already done)
2. `status == "running"` → Expanded = true (ADD)
3. `len(logs) > 0` → Expanded = true (ADD)
4. Step is `current_step` from metadata → Expanded = true (ADD)

### Task 3-4 (Frontend):
Simplify frontend to:
1. Use `step.expanded` from backend as initial value
2. Track user overrides separately
3. Remove redundant shouldExpand computation

## Files Changed
- None (audit only)

## Skill Compliance
No skill applied - research task

## Build Check
Build: N/A | Tests: N/A
