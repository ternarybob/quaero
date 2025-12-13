# Plan: Dual Steps UI

## Overview
Update the UI to display step progress for multi-step jobs and create a comprehensive test for the `nearby-restaurants-keywords` job definition.

## Problem Statement
1. The current UI shows "No progress data" for multi-step jobs (see screenshot showing job stuck in "Running")
2. Multi-step jobs (like `nearby-resturants-keywords.toml`) don't show individual step progress
3. No test exists to verify the `nearby-restaurants-keywords` job completes successfully

## Architecture Analysis

### Job Definition Structure
The `nearby-resturants-keywords.toml` has 2 steps:
```toml
[step.search_nearby_restaurants]
type = "places_search"

[step.extract_keywords]
type = "agent"
depends = "search_nearby_restaurants"
```

### Current Progress Tracking
- Parent job stores `ProgressCurrent` and `ProgressTotal` (step count) in `internal/queue/manager.go:828`
- Steps are executed sequentially via `ExecuteJobDefinition()` (line 882-947)
- Progress is updated after each step: `UpdateJobProgress(ctx, parentJobID, i+1, len(jobDef.Steps))`
- WebSocket events broadcast `job_status_change` but NOT individual step progress

### Key Files
- `pages/queue.html` - Queue UI (Alpine.js)
- `pages/static/websocket-manager.js` - WebSocket client
- `internal/queue/manager.go` - Job execution orchestration
- `internal/handlers/websocket_events.go` - WebSocket event handlers
- `test/ui/queue_test.go` - UI test templates

## Tasks

### Task 1: Investigate Job Completion Issue
**Priority: Critical**

The screenshot shows the job stuck in "Running" with "No progress data". Need to investigate:
1. Check if `nearby-resturants-keywords` job completes properly
2. Identify why progress data is not showing
3. Check WebSocket events for step progress

Files to review:
- `internal/queue/workers/places_worker.go` - Places search worker
- `internal/queue/workers/agent_worker.go` - Agent worker (keyword extraction)
- `internal/queue/state/monitor.go` - Job monitoring

### Task 2: Add Step Progress to Job API Response
**Files to modify:**
- `internal/handlers/job_handler.go` - Add steps info to job response
- `internal/models/job_model.go` - Add step progress fields

Add to job response:
```json
{
  "steps": [
    {"name": "search_nearby_restaurants", "type": "places_search", "status": "completed"},
    {"name": "extract_keywords", "type": "agent", "status": "running"}
  ],
  "current_step": 2,
  "total_steps": 2
}
```

### Task 3: Update Queue UI to Display Steps
**Files to modify:**
- `pages/queue.html` - Add step progress display

Requirements:
- Show step count (e.g., "Step 2/2")
- Show step names and types
- Show individual step status
- Integrate with existing WebSocket updates

### Task 4: Add WebSocket Events for Step Progress
**Files to modify:**
- `internal/handlers/websocket_events.go` - Add step progress event
- `internal/queue/manager.go` - Emit step progress events

New WebSocket message type:
```json
{
  "type": "job_step_progress",
  "payload": {
    "job_id": "...",
    "current_step": 2,
    "total_steps": 2,
    "step_name": "extract_keywords",
    "step_status": "running"
  }
}
```

### Task 5: Create Test for nearby-restaurants-keywords Job
**File:** `test/ui/nearby_restaurants_keywords_test.go`

Test should verify:
1. Job completes successfully (status = "completed")
2. Job creates documents (document_count > 0)
3. Job extracts keywords from documents
4. Keywords are stored in the database

Follow `TestNewsCrawlerCrash` pattern from `test/ui/queue_test.go`.

## Execution Order

1. **Task 1** - Investigate/fix completion issue (blocking)
2. **Task 5** - Create test (can run in parallel with Task 2-4)
3. **Task 2** - Add step info to API
4. **Task 4** - Add WebSocket events
5. **Task 3** - Update UI

## Success Criteria

1. Multi-step jobs show step progress in UI
2. `nearby-restaurants-keywords` test passes
3. Job completes without hanging
4. Build passes with no regressions
