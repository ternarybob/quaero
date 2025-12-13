# Plan: Job Queue UI Optimization

Type: feature | Workdir: docs/feature/20251211-codebase-assessment-optimization/

## User Intent (from manifest)

The user wants to fix multiple issues in the Job Queue UI:
1. Steps Not Showing - Only 1 step shows when there should be 3
2. Running Icon Bug - Spinner shown for completed jobs
3. Clean Architecture - Services emit standard logs with key/value context
4. Live Tree Expansion - Tree expands as events arrive, 100-item limit
5. Light Theme - Black text on light gray background
6. Div vs Scrollable - Use divs instead of scrollable text boxes

## Active Skills

go, frontend

## Root Cause Analysis

### Issue 1: Steps Not Showing
The `GetJobTreeHandler` incorrectly groups all child jobs by `step_name` metadata. The actual model is:
- Manager Job → Step Jobs (1 per step definition) → Work Items (grandchildren)
- The handler should use `step_definitions` from metadata or map step jobs directly

### Issue 2: Running Icon Bug
The handler sets `step.Status = string(child.Status)` from the first matching child, then only overrides for "failed"/"running". This logic is broken - it should use the step job's own status, not aggregate from grandchildren.

### Issue 3-6: UI Improvements
Frontend changes needed for theming, live expansion, log limits.

## Tasks

| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Refactor GetJobTreeHandler to use step_definitions and proper job hierarchy | - | no | sonnet | go |
| 2 | Update tree view to light theme (black text, light gray background) | - | no | sonnet | frontend |
| 3 | Add live tree expansion with WebSocket event integration | 1 | no | sonnet | frontend |
| 4 | Implement 100-item log limit with "..." indicator for earlier logs | 1 | no | sonnet | frontend |
| 5 | Convert scrollable text boxes to divs where possible | 2 | no | sonnet | frontend |
| 6 | Build and verify all changes work together | 1,2,3,4,5 | no | sonnet | go |

## Order

[1,2] → [3,4,5] → [6]

## Implementation Details

### Task 1: Refactor GetJobTreeHandler

The current handler incorrectly builds steps. It should:

1. Get `step_definitions` from parent job metadata (already set by orchestrator)
2. For each step definition, find the matching step job by `step_name` in metadata
3. Use the step job's own status for the step status
4. Get grandchildren of each step job for ChildSummary counts
5. Fetch logs for each step job separately

**New logic:**
```go
// Get step_definitions from parent metadata
stepDefs := parentJob.Metadata["step_definitions"].([]map[string]interface{})

// For each step definition, find matching step job
for _, stepDef := range stepDefs {
    stepName := stepDef["name"].(string)
    // Find step job where metadata.step_name == stepName
    // Use step job's status directly
    // Get grandchildren of step job for counts
}
```

### Task 2: Light Theme

Update these hardcoded colors in queue.html tree view:
- `background-color: #1e1e1e;` → `background-color: #f5f5f5;`
- `background-color: #252526;` → `background-color: #e8e8e8;`
- `background-color: #1a1a1a;` → `background-color: #fafafa;`
- `color: #d4d4d4;` → `color: #333333;`
- `border-bottom: 1px solid #333;` → `border-bottom: 1px solid #ddd;`
- `border-left: 2px solid #333;` → `border-left: 2px solid #ccc;`
- Input: `background: #3c3c3c; border: 1px solid #555; color: #d4d4d4;` → light version

### Task 3: Live Tree Expansion

- Subscribe to WebSocket events for step status updates
- Auto-expand tree view for running jobs
- Update step status/logs in real-time without full refresh

### Task 4: 100-Item Log Limit

- Add `maxLogsPerStep: 100` constant
- Show logs in earliest-to-latest order
- Display "..." indicator at top when logs exceed limit
- Trim older logs when limit exceeded

### Task 5: Div Instead of Scrollable

- Remove `overflow-y: auto` from log containers
- Use div with natural height expansion
- Keep max-height constraint on outer container only
