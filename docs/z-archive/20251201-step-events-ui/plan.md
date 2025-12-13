# Plan: Step Events UI Fix

Type: fix | Workdir: docs/fix/20251201-step-events-ui/

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Move events panel from parent to step rows | - | no | sonnet |
| 2 | Auto-expand events for running/failed steps | 1 | no | sonnet |

## Order
[1] → [2]

## Analysis

From the screenshot:
- Current: "Events (90)" button is at the parent job card level
- Required: Events should be per-step, shown under each step row
- Required: Events should be expanded by default (especially for running jobs)

The logs already include `step_name` in the payload, so we can filter logs by step.
