# Plan: WebSocket UI Sync for Running Jobs
Type: fix | Workdir: ./docs/fix/20251211-websocket-ui-sync/

## User Intent (from manifest)
Fix the disconnect between backend state and frontend UI during job execution. The UI shows incorrect step status (pending when should be completed) and doesn't update in real-time. Simplify the WebSocket protocol with clear context and reduce API endpoint complexity.

## Active Skills
- go (backend changes)

## Problem Analysis

### Current Architecture Issues
1. **Multiple overlapping WebSocket message types**: `job_status_change`, `job_step_progress`, `step_progress`, `refresh_logs` - confusing which does what
2. **Indirect step status updates**: UI relies on inferring step status from child job counts rather than explicit step status
3. **Log aggregator complicates real-time updates**: The unified log aggregator batches events, causing delays
4. **Step status not included in step_progress**: The `step_progress` event only includes child counts, not the step's own status

### Root Cause
Looking at the screenshot:
- Parent job shows "Completed"
- Steps `import_files`, `rule_classify_files` show yellow (pending) status
- The step_progress events update child counts but the step status itself isn't being propagated correctly to the UI

The issue is the `step_progress` event from `StepMonitor.publishStepProgress()` goes through the unified log aggregator which only triggers a `refresh_logs` message with step_ids. This doesn't actually update the step status in the UI.

### Solution
1. **Add new `/api/jobs/{id}/structure` endpoint**: Lightweight endpoint returning just job status and step statuses (no logs)
2. **Simplify WebSocket protocol**: Single `job_update` message type with context field
3. **Direct step status broadcast**: Bypass aggregator for step status changes
4. **UI fetches structure on job_update**: Single fetch updates all statuses

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Create /api/jobs/{id}/structure endpoint | - | no | sonnet | go |
| 2 | Add job_update WebSocket message for status changes | 1 | no | sonnet | go |
| 3 | Update StepMonitor to broadcast step status directly | 2 | no | sonnet | go |
| 4 | Update UI to use new endpoint and message format | 3 | no | sonnet | - |
| 5 | Test end-to-end with running job | 4 | no | sonnet | - |

## Order
[1] → [2] → [3] → [4] → [5]

## Detailed Design

### Task 1: /api/jobs/{id}/structure endpoint
```go
// GET /api/jobs/{id}/structure
// Returns lightweight job structure for UI status updates
type JobStructureResponse struct {
    JobID     string       `json:"job_id"`
    Status    string       `json:"status"`
    Steps     []StepStatus `json:"steps"`
    UpdatedAt time.Time    `json:"updated_at"`
}

type StepStatus struct {
    Name       string `json:"name"`
    Status     string `json:"status"`
    LogCount   int    `json:"log_count"`
    ChildCount int    `json:"child_count,omitempty"`
}
```

### Task 2: job_update WebSocket message
```json
{
    "type": "job_update",
    "payload": {
        "context": "job_step",  // or "job"
        "job_id": "abc123",
        "step_name": "import_files",  // only if context=job_step
        "status": "running",
        "refresh_logs": true  // trigger log fetch for expanded steps
    }
}
```

### Task 3: Direct step status broadcast
Modify `StepMonitor.publishStepProgress()` to also send a `job_update` message directly (not through aggregator) when step status changes.

### Task 4: UI changes
- Listen for `job_update` messages
- On `job_update` with `refresh_logs: true`, fetch `/api/jobs/{id}/structure`
- Update step statuses from response
- Only fetch logs for expanded steps
