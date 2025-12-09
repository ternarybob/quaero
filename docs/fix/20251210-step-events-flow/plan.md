# Plan: Step Events Flow + Timestamp Ordering
Type: fix | Workdir: ./docs/fix/20251210-step-events-flow/

## User Intent (from manifest)
1. Step events should load from API when triggered by WebSocket (running steps with >1sec duration)
2. Step events should load last 100 from API when step completes/fails/cancels
3. Event timestamps need millisecond precision for proper ordering
4. On page load with completed job, all steps should show their events

## Active Skills
go, frontend

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Update timestamp format to include milliseconds | - | no | sonnet | go |
| 2 | Load events for completed steps on page load | 1 | no | sonnet | frontend |
| 3 | Build verification | 2 | no | sonnet | go |

## Order
[1] → [2] → [3]

## Analysis

### Root Cause 1: Timestamp Precision
- `consumer.go:250` uses `time.RFC3339` which only has second precision
- Fast jobs generate multiple events per second
- Events with same timestamp can't be ordered correctly

**Fix**: Change to `RFC3339Nano` or custom format with milliseconds

### Root Cause 2: Events Not Loading on Page Load
- `refreshStepEvents()` only triggers on WebSocket `refresh_step_events` message
- For already-completed jobs on page load, no WebSocket trigger fires
- `fetchStepEvents()` only runs when user clicks to expand
- No automatic loading for completed steps

**Fix**: On initial render, if step is completed/failed/cancelled, fetch events automatically
