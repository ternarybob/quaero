# Plan: Fix Job Statistics Panel Not Aligned with Job Step Status
Type: fix | Workdir: docs/fix/20251202-job-stats-realtime

## User Intent (from manifest)
Fix the real-time synchronization between:
1. **Job Statistics panel** (top of page) - shows aggregate counts across ALL jobs (parent + children)
2. **Job Progress bar** (on each job row) - shows progress of child jobs within a manager job
3. **Step progress** (in step rows) - shows progress of child jobs within each step

The screenshot shows mismatch:
- Job Statistics: 4 pending, 10 running, 2 completed, 7 failed (counts ALL jobs including parent manager and steps)
- Job Progress/Step: 7 pending, 10 running, 2 completed, 2 failed (counts only child agent jobs)

This is actually expected behavior - Job Statistics counts DIFFERENT things than Step Progress!

## Root Cause Analysis

After code analysis, the issue is that **the UI is NOT receiving real-time updates** for Job Statistics:

1. `job_status_change` events trigger `recalculateStats()` which fetches from `/api/jobs/stats`
2. But the step progress and job progress are being updated via `step_progress` and `manager_progress` WebSocket events
3. The issue is that `job_status_change` events may not be firing for child jobs, OR the UI is not receiving them

Looking at the screenshot annotations:
- The user circled "4" pending in Job Statistics and "7" pending in Progress
- These are DIFFERENT counts (Job Statistics = ALL jobs, Progress = child jobs only)
- But the real issue is the UI is not updating at all during execution

Key findings:
1. `updateStepProgress()` does update the step object with progress data (lines 3595-3626)
2. `renderJobs()` at lines 2635-2642 reads step progress from `parentJob.pending_children` etc.
3. The step progress in `updateStepProgress()` updates `step.pending_children` etc., but `renderJobs()` reads from `parentJob.pending_children`
4. **This is the bug**: `updateStepProgress()` updates the step job, but `renderJobs()` reads from the parent job

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Fix step progress rendering to use step-specific progress data | - | no | sonnet |
| 2 | Ensure updateStepProgress updates are reflected in renderJobs | 1 | no | sonnet |
| 3 | Add real-time step progress sync between backend step_progress events and UI | 2 | no | sonnet |
| 4 | Add test verification to TestNearbyRestaurantsKeywordsMultiStep | 3 | no | sonnet |
| 5 | Build and test fix | 4 | no | sonnet |

## Order
[1] → [2] → [3] → [4] → [5]
