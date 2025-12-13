# Feature: Step Progress and Document Counts

## Overview
Add document counts and progress information to each step row in the Queue UI.
Currently this info is shown at the parent job level; it should be incorporated into individual step rows.

## Current State
- Parent jobs show overall progress (pending, running, completed, failed counts)
- Parent jobs show total document count
- Step rows only show: step number, name, type badge, status badge, and description

## Desired State
- Each step row displays:
  - Document count: unique count of documents created/updated/deleted in that step
  - Progress: child job status counts for that specific step

## Technical Analysis

### Data Flow
1. Backend tracks step metadata in `internal/queue/manager.go`:
   - `current_step`, `current_step_name`, `current_step_type`, `current_step_status`
   - `completed_steps`, `total_steps`
2. WebSocket events broadcast `job_step_progress` with step info
3. Frontend renders step rows in `renderJobs()` method

### Key Files
- `internal/queue/manager.go` - Step execution and metadata tracking
- `internal/handlers/websocket.go` - Step progress WebSocket events
- `pages/queue.html` - Step row UI rendering

## Implementation Plan

### Phase 1: Backend - Track Step-Level Statistics
1. Add step-level tracking fields to job metadata:
   - `step_document_counts`: map of step index -> document count
   - `step_child_counts`: map of step index -> child job statistics (pending/running/completed/failed)
2. Update `ExecuteJobDefinition` to track which step created which child jobs
3. On step completion, capture document count from step's child jobs
4. Include step stats in WebSocket events

### Phase 2: Frontend - Display Step Stats in UI
1. Update `renderJobs()` to pass step statistics to step objects
2. Update step row template to display:
   - Document count badge/indicator
   - Progress indicator (child job status counts)
3. Handle real-time updates via WebSocket `job_step_progress` events

## Files to Modify
1. `internal/queue/manager.go` - Add step-level stat tracking
2. `internal/handlers/websocket.go` - Include step stats in events
3. `pages/queue.html` - Update step row UI

## Testing
- Run multi-step job definition
- Verify step rows show document counts
- Verify progress updates in real-time
