# Plan: WebSocket Log Debounce and Job Status UI
Type: fix | Workdir: ./docs/fix/20251212-websocket-log-debounce/ | Date: 2025-12-12

## Context
Project: Quaero
Related files:
- `pages/queue.html` - main UI file with Alpine.js job list component

## User Intent (from manifest)
1. **Stop excessive log API calls**: The UI is calling the log API way too often (flooding). The trigger should be via WebSocket, buffered to:
   - 1 second interval, OR
   - 500 log entries threshold, OR
   - Job/step status change

2. **Fix job status display**: The running job UI shows incorrect status:
   - Job shows "Completed" status badge but steps show "running" spinner icons
   - Steps are not auto-expanding when logs become available (via WebSocket notification)
   - Status icons don't match actual step status

3. **Pass the test**: Run `test\ui\job_definition_codebase_classify_test.go` and refactor until it passes

## Success Criteria (from manifest)
- [ ] Log API calls are debounced (triggered by WebSocket only, max 1 call per second unless status changes)
- [ ] No API flooding visible in network panel
- [ ] Job status badge reflects actual job status correctly
- [ ] Step status icons match step status (no "running" spinner on completed/pending steps)
- [ ] Steps auto-expand when logs become available via WebSocket
- [ ] Test `test\ui\job_definition_codebase_classify_test.go` passes

## Active Skills
| Skill | Key Patterns to Apply |
|-------|----------------------|
| go | Service/handler patterns if backend changes needed |
| frontend | Alpine.js reactive data, event handling |

## Technical Approach
1. Add debouncing to `fetchStepLogs` function - prevent multiple concurrent calls for same step
2. Add debouncing to `handleRefreshStepEvents` - aggregate step_ids before triggering fetches
3. Update step status in `jobTreeData` when `handleJobUpdate` receives step status changes
4. Ensure `handleJobUpdate` with context="job_step" updates the correct step's status in tree data

## Files to Change
| File | Action | Purpose |
|------|--------|---------|
| pages/queue.html | modify | Add debouncing to fetchStepLogs, fix step status sync in handleJobUpdate |

## Tasks
| # | Desc | Depends | Critical | Model | Skill | Est. Files |
|---|------|---------|----------|-------|-------|------------|
| 1 | Add debouncing to fetchStepLogs with per-step tracking | - | no | opus | frontend | 1 |
| 2 | Fix step status sync in handleJobUpdate for job_step context | 1 | no | opus | frontend | 1 |
| 3 | Run test and verify both fixes | 2 | no | opus | - | 0 |

## Execution Order
[1] → [2] → [3]

## Risks/Decisions
- Need to ensure debouncing doesn't delay critical status updates (use immediate fetch for status changes)
- Step status updates must propagate to jobTreeData to fix icon display issue
