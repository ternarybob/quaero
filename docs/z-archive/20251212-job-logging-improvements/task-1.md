# Task 1: Fix "Show earlier logs" button to load 200 logs
Workdir: ./docs/feature/20251212-job-logging-improvements/ | Depends: none | Critical: no
Model: opus | Skill: frontend

## Context
This task is part of: Job Logging Improvements - making the tree view step logs more functional
Prior tasks completed: none - this is first

## User Intent Addressed
"The 'Show xxx earlier logs' button does not actually do anything. Implement a load mechanism, to show 200 of the latest logs."

## Input State
Files that exist before this task:
- `pages/queue.html` - Contains tree view with step logs, "Show earlier logs" button, and loadMoreStepLogs function

Current behavior:
- Default step log limit is 50 (line ~4209: `return this.stepLogLimits[key] || 50;`)
- Button increments by 100 (line ~4285: `const newLimit = currentLimit + 100;`)
- API is called but logs may not update correctly in UI

## Output State
Files after this task completes:
- `pages/queue.html` - Default limit changed to 200, increment changed to 200, button works correctly

## Skill Patterns to Apply
### From frontend patterns:
- **DO:** Use Alpine.js reactive state patterns (spread operator for object updates)
- **DO:** Follow existing code patterns for consistency
- **DON'T:** Add unnecessary complexity
- **DON'T:** Change API call structure unless required

## Implementation Steps
1. Find and update `getStepLogLimit` function to default to 200 instead of 50
2. Find and update `loadMoreStepLogs` function to increment by 200 instead of 100
3. Verify the function properly updates state and triggers re-render

## Code Specifications
Functions to modify:
- `getStepLogLimit(jobId, stepName)` - change default from 50 to 200
- `loadMoreStepLogs(jobId, stepName, stepIndex)` - change increment from 100 to 200

## Accept Criteria
- [ ] Default step log limit is 200
- [ ] "Show earlier logs" button increments by 200 logs
- [ ] Button triggers API call and updates UI
- [ ] Build passes

## Handoff
After completion, next task(s): task-2 (add level filter dropdown)
