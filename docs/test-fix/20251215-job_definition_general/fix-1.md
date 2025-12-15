# Fix 1
Iteration: 1

## Failures Addressed

| Test | Root Cause | Fix |
|------|------------|-----|
| TestJobDefinitionLogInitialCount | `toggleTreeStep()` doesn't fetch logs when expanding | Added fetchStepLogs() call when step is expanded |

## Architecture Compliance

| Doc | Requirement | How Fix Complies |
|-----|-------------|------------------|
| QUEUE_UI.md | "Manual Toggle should call fetchStepLogs when expanding" | Now calls fetchStepLogs(jobId, step.name, stepIndex, true) when !wasExpanded |
| QUEUE_UI.md | "Log lines should display when step is expanded" | Logs are now fetched via API when step is manually expanded |

## Changes Made

| File | Change |
|------|--------|
| `pages/queue.html` | Modified toggleTreeStep() to fetch logs when expanding (lines 4783-4801) |

## Code Change
```javascript
// BEFORE
toggleTreeStep(jobId, stepIndex) {
    const key = `${jobId}:${stepIndex}`;
    this.jobTreeExpandedSteps = { ...this.jobTreeExpandedSteps, [key]: !this.jobTreeExpandedSteps[key] };
},

// AFTER
toggleTreeStep(jobId, stepIndex) {
    const key = `${jobId}:${stepIndex}`;
    const wasExpanded = this.jobTreeExpandedSteps[key];
    this.jobTreeExpandedSteps = { ...this.jobTreeExpandedSteps, [key]: !wasExpanded };

    // Per QUEUE_UI.md: fetch logs when expanding a step
    if (!wasExpanded) {
        const treeData = this.jobTreeData[jobId];
        if (treeData?.steps?.[stepIndex]) {
            const step = treeData.steps[stepIndex];
            // Set initial log limit if not already set
            const limitKey = `${jobId}:${step.name}`;
            if (!this.stepLogLimits[limitKey]) {
                this.stepLogLimits = { ...this.stepLogLimits, [limitKey]: 100 };
            }
            this.fetchStepLogs(jobId, step.name, stepIndex, true);
        }
    }
},
```

## NOT Changed (tests are spec)
- test/ui/job_definition_general_test.go - Tests define requirements, not modified
