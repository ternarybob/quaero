# Plan: Job Queue Real-time Update Fix

## Classification
- Type: fix
- Workdir: ./docs/fixes/job-queue-realtime-update/

## Analysis

### Root Cause
When a parent job completes in `monitor.go:204`, `publishParentJobProgress()` is called. However, this method (lines 275-306) does NOT include `document_count` in its payload.

The correct method that includes `document_count` is `publishParentJobProgressUpdate()` (lines 588-644, specifically line 625).

### Issue Flow
1. All child jobs complete
2. `monitorChildJobs()` detects completion at line 185
3. Job status updated to "completed" at line 190
4. `publishParentJobProgress()` called at line 204 - **MISSING document_count**
5. Frontend receives event with status="completed" but no document_count
6. UI shows stale document count until page refresh

### Solution
Modify `monitorChildJobs()` to call `publishParentJobProgressUpdate()` on completion instead of `publishParentJobProgress()`. This requires:
1. Getting fresh child stats before publishing final event
2. Publishing with the stats-inclusive method that includes document_count

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Fix monitor.go completion event | none | no | low | sonnet |

## Order
Sequential: [1] → Validate

## Files Changed
- `internal/queue/state/monitor.go` - Fix completion event to include document_count
