# Plan: Fix Step Logs Inconsistent
Type: fix | Workdir: ./docs/fix/20251210-step-logs-inconsistent

## User Intent (from manifest)
Fix the inconsistency where job step logs appear in the console (terminal) but show "Events (0)" and "No events yet for this step" in the UI. The logs are being generated but not propagated to the step events display in the queue page.

## Active Skills
- go (backend event flow)

## Root Cause Analysis

### Problem 1: Wrong storage location
`StepMonitor.publishStepLog()` stores logs under **manager job ID** but UI fetches from **step job ID**.

### Problem 2: Missing step_progress events
Orchestrator does NOT publish `step_progress` events for synchronously-completed steps. Only `StepMonitor` publishes these events, but `StepMonitor` is only used for steps with async child jobs ("spawned" status).

The UI only fetches step events when it receives a `refresh_logs` trigger with `finished=true`. This trigger is sent by `TriggerStepImmediately()` which is called when a `step_progress` event has status completed/failed/cancelled.

Without `step_progress` events, the UI never knows to fetch step logs.

## Fix Strategy
1. Change `publishStepLog` to store logs under the **step job ID**
2. Add `step_progress` event publishing to orchestrator for completed/failed steps

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Update StepMonitor.publishStepLog to store logs under stepID | - | no | sonnet | go |
| 2 | Verify step_progress events include correct step_id | 1 | no | sonnet | go |
| 3 | Build and test fix manually | 2 | no | sonnet | go |
| 4 | Add step_progress events to orchestrator | 3 | no | sonnet | go |

## Order
[1] → [2] → [3] → [4]
